package msgcodec

import (
	"bytes"
	"encoding/json"
	"strings"
)

// FieldType 是字段类型。
type FieldType string

const (
	FieldInt32   FieldType = "int32"
	FieldInt64   FieldType = "int64"
	FieldUint32  FieldType = "uint32"
	FieldUint64  FieldType = "uint64"
	FieldFloat   FieldType = "float"
	FieldDouble  FieldType = "double"
	FieldBool    FieldType = "bool"
	FieldString  FieldType = "string"
	FieldBytes   FieldType = "bytes"
	FieldMessage FieldType = "message"
)

var typeNames = map[string]FieldType{
	"int32": FieldInt32, "int64": FieldInt64,
	"uint32": FieldUint32, "uint64": FieldUint64,
	"float": FieldFloat, "double": FieldDouble,
	"bool": FieldBool, "string": FieldString,
	"bytes": FieldBytes, "message": FieldMessage,
}

// FieldSchema 描述消息类型的一个字段。
type FieldSchema struct {
	Name     string // 字段名,同时作为 JSON key
	Num      int    // proto 字段号,范围 [1, 65535]
	Type     FieldType
	Repeated bool          // 是否为数组
	Schema   MessageSchema // Type == message 时指向内联嵌套 schema
}

// MessageSchema 抽象消息类型的结构。
type MessageSchema interface {
	TypeID() uint16
	Fields() []*FieldSchema
	Field(name string) (*FieldSchema, bool)
}

type schemaImpl struct {
	typeID uint16
	fields []*FieldSchema
	byName map[string]*FieldSchema
}

func (s *schemaImpl) TypeID() uint16         { return s.typeID }
func (s *schemaImpl) Fields() []*FieldSchema { return s.fields }
func (s *schemaImpl) Field(name string) (*FieldSchema, bool) {
	f, ok := s.byName[name]
	return f, ok
}

func wrapSchema(typeID uint16, fields []*FieldSchema) *schemaImpl {
	byName := make(map[string]*FieldSchema, len(fields))
	for _, f := range fields {
		byName[f.Name] = f
	}
	return &schemaImpl{typeID: typeID, fields: fields, byName: byName}
}

type schemaDoc struct {
	Types []typeDoc `json:"types"`
}

type typeDoc struct {
	TypeID uint16     `json:"typeId"`
	Fields []fieldDoc `json:"fields"`
}

type fieldsDoc struct {
	Fields []fieldDoc `json:"fields"`
}

type fieldDoc struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Num      int        `json:"num"`
	Repeated bool       `json:"repeated"`
	Schema   *fieldsDoc `json:"schema"`
}

func validFieldName(name string) bool {
	return name != "" && !strings.ContainsAny(name, ".[]")
}

func buildFields(docs []fieldDoc) ([]*FieldSchema, error) {
	fields := make([]*FieldSchema, 0, len(docs))
	seenName := make(map[string]bool, len(docs))
	seenNum := make(map[int]bool, len(docs))
	for _, d := range docs {
		if !validFieldName(d.Name) {
			return nil, ErrMalformedData
		}
		if seenName[d.Name] {
			return nil, ErrMalformedData
		}
		seenName[d.Name] = true
		ft, ok := typeNames[d.Type]
		if !ok {
			return nil, ErrMalformedData
		}
		if d.Num < 1 || d.Num > 65535 {
			return nil, ErrMalformedData
		}
		if seenNum[d.Num] {
			return nil, ErrMalformedData
		}
		seenNum[d.Num] = true
		fs := &FieldSchema{Name: d.Name, Num: d.Num, Type: ft, Repeated: d.Repeated}
		if ft == FieldMessage {
			if d.Schema == nil {
				return nil, ErrMalformedData
			}
			sub, err := buildFields(d.Schema.Fields)
			if err != nil {
				return nil, err
			}
			fs.Schema = wrapSchema(0, sub)
		} else if d.Schema != nil {
			return nil, ErrMalformedData
		}
		fields = append(fields, fs)
	}
	return fields, nil
}

// ParseSchema 解析配置文件(JSON),返回顶层消息类型列表。
func ParseSchema(data []byte) ([]MessageSchema, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, ErrMalformedData
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, ErrMalformedData
	}
	seenID := make(map[uint16]bool, len(doc.Types))
	out := make([]MessageSchema, 0, len(doc.Types))
	for i := range doc.Types {
		td := &doc.Types[i]
		if td.TypeID == 0 || seenID[td.TypeID] {
			return nil, ErrMalformedData
		}
		seenID[td.TypeID] = true
		fields, err := buildFields(td.Fields)
		if err != nil {
			return nil, err
		}
		out = append(out, wrapSchema(td.TypeID, fields))
	}
	return out, nil
}
