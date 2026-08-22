package dymsg

import (
	"encoding/json"
	"strings"
)

type configFile struct {
	Types []*configType `json:"types"`
}

type configType struct {
	TypeID *int           `json:"typeId"`
	Fields []*configField `json:"fields"`
}

type configField struct {
	Name     *string       `json:"name"`
	Type     *string       `json:"type"`
	Num      *int          `json:"num"`
	Repeated bool          `json:"repeated"`
	Schema   *configSchema `json:"schema"`
}

type configSchema struct {
	Fields []*configField `json:"fields"`
}

// ParseSchema parses a JSON configuration document and returns one schema per
// top-level message type. Any invalid configuration yields ErrMalformedData.
func ParseSchema(data []byte) ([]MessageSchema, error) {
	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, ErrMalformedData
	}
	if cf.Types == nil {
		return nil, nil
	}

	seen := make(map[int]bool, len(cf.Types))
	out := make([]MessageSchema, 0, len(cf.Types))
	for _, ct := range cf.Types {
		if ct == nil || ct.TypeID == nil || *ct.TypeID < 1 || *ct.TypeID > 65535 {
			return nil, ErrMalformedData
		}
		if seen[*ct.TypeID] {
			return nil, ErrMalformedData
		}
		seen[*ct.TypeID] = true

		ms, err := buildSchema(ct.Fields)
		if err != nil {
			return nil, err
		}
		ms.typeID = uint16(*ct.TypeID)
		out = append(out, ms)
	}
	return out, nil
}

func buildSchema(cfs []*configField) (MessageSchema, error) {
	ms := MessageSchema{
		byName: make(map[string]int),
		byNum:  make(map[int]int),
	}
	ms.fields = make([]FieldSchema, 0, len(cfs))

	for _, cf := range cfs {
		if cf == nil || cf.Name == nil || *cf.Name == "" {
			return MessageSchema{}, ErrMalformedData
		}
		name := *cf.Name
		if strings.ContainsAny(name, ".[]") {
			return MessageSchema{}, ErrMalformedData
		}
		if cf.Type == nil || !validFieldType(FieldType(*cf.Type)) {
			return MessageSchema{}, ErrMalformedData
		}
		if cf.Num == nil || *cf.Num < 1 || *cf.Num > 65535 {
			return MessageSchema{}, ErrMalformedData
		}
		if _, dup := ms.byName[name]; dup {
			return MessageSchema{}, ErrMalformedData
		}
		if _, dup := ms.byNum[*cf.Num]; dup {
			return MessageSchema{}, ErrMalformedData
		}

		ft := FieldType(*cf.Type)
		fs := FieldSchema{
			Name:     name,
			Num:      *cf.Num,
			Type:     ft,
			Repeated: cf.Repeated,
		}
		if ft == FieldMessage {
			if cf.Schema == nil {
				return MessageSchema{}, ErrMalformedData
			}
			nested, err := buildSchema(cf.Schema.Fields)
			if err != nil {
				return MessageSchema{}, err
			}
			fs.Schema = nested
		} else if cf.Schema != nil {
			return MessageSchema{}, ErrMalformedData
		}

		ms.byName[name] = len(ms.fields)
		ms.byNum[*cf.Num] = len(ms.fields)
		ms.fields = append(ms.fields, fs)
	}
	return ms, nil
}

func validFieldType(ft FieldType) bool {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64,
		FieldFloat, FieldDouble, FieldBool, FieldString,
		FieldBytes, FieldMessage:
		return true
	}
	return false
}

func (sc *MessageSchema) fieldIndex(name string) int {
	if sc == nil {
		return -1
	}
	if i, ok := sc.byName[name]; ok {
		return i
	}
	return -1
}

func (sc *MessageSchema) fieldIndexByNum(num int) int {
	if sc == nil {
		return -1
	}
	if i, ok := sc.byNum[num]; ok {
		return i
	}
	return -1
}

// schemasEqual reports whether two schemas are structurally identical.
func schemasEqual(a, b *MessageSchema) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.fields) != len(b.fields) {
		return false
	}
	for i := range a.fields {
		af := &a.fields[i]
		bf := &b.fields[i]
		if af.Name != bf.Name || af.Num != bf.Num || af.Type != bf.Type || af.Repeated != bf.Repeated {
			return false
		}
		if af.Type == FieldMessage {
			if !schemasEqual(&af.Schema, &bf.Schema) {
				return false
			}
		}
	}
	return true
}
