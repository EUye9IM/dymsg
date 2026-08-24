// Package dymsg provides dynamic message encoding/decoding where message
// schemas are defined at runtime (from configuration) rather than compiled
// into Go structs.
package dymsg

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

// FieldType identifies the scalar/message type of a field.
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

// FieldSchema describes a single field.
type FieldSchema struct {
	Name     string
	Num      int
	Type     FieldType
	Repeated bool
	Schema   MessageSchema
}

// MessageSchema describes a message type. Use ParseSchema to build one.
type MessageSchema struct {
	typeID uint16
	fields []FieldSchema
	byName map[string]int
	byNum  map[int]int
}

// Sentinel errors.
var (
	ErrDuplicateID     = errors.New("dymsg: duplicate type id with different schema")
	ErrUnknownTypeID   = errors.New("dymsg: unknown type id")
	ErrFieldNotFound   = errors.New("dymsg: field not found or invalid path")
	ErrIndexOutOfRange = errors.New("dymsg: index out of range")
	ErrTypeMismatch    = errors.New("dymsg: type mismatch")
	ErrMalformedData   = errors.New("dymsg: malformed data")
	ErrTruncated       = errors.New("dymsg: truncated data")
)

var (
	registryMu sync.RWMutex
	registry   = map[uint16]MessageSchema{}
)

// Register registers a schema by its type ID. Registering the same schema
// again is idempotent; registering a different schema under an existing type
// ID returns ErrDuplicateID.
func Register(s MessageSchema) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	if existing, ok := registry[s.typeID]; ok {
		if schemasEqual(existing, s) {
			return nil
		}
		return ErrDuplicateID
	}
	registry[s.typeID] = s
	return nil
}

// New creates a new empty message of the registered type.
func New(typeID uint16) (*Message, error) {
	registryMu.RLock()
	s, ok := registry[typeID]
	registryMu.RUnlock()
	if !ok {
		return nil, ErrUnknownTypeID
	}
	return newMessage(s), nil
}

// ParseSchema parses a JSON configuration into top-level message schemas.
func ParseSchema(data []byte) ([]MessageSchema, error) {
	var cfg rawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, ErrMalformedData
	}
	if len(cfg.Types) == 0 {
		return []MessageSchema{}, nil
	}
	out := make([]MessageSchema, 0, len(cfg.Types))
	seen := make(map[int]bool, len(cfg.Types))
	for _, rt := range cfg.Types {
		if rt.TypeID < 1 || rt.TypeID > 65535 {
			return nil, ErrMalformedData
		}
		if seen[rt.TypeID] {
			return nil, ErrMalformedData
		}
		seen[rt.TypeID] = true
		s, err := buildMessageSchema(uint16(rt.TypeID), rt.Fields)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

type rawConfig struct {
	Types []rawType `json:"types"`
}

type rawType struct {
	TypeID int        `json:"typeId"`
	Fields []rawField `json:"fields"`
}

type rawField struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Num      int             `json:"num"`
	Repeated bool            `json:"repeated"`
	Schema   json.RawMessage `json:"schema"`
}

type rawNestedSchema struct {
	Fields []rawField `json:"fields"`
}

func buildMessageSchema(typeID uint16, rawFields []rawField) (MessageSchema, error) {
	fields := make([]FieldSchema, 0, len(rawFields))
	byName := make(map[string]int, len(rawFields))
	byNum := make(map[int]int, len(rawFields))
	for i, rf := range rawFields {
		if rf.Name == "" {
			return MessageSchema{}, ErrMalformedData
		}
		if strings.ContainsAny(rf.Name, ".[]") {
			return MessageSchema{}, ErrMalformedData
		}
		ft, ok := parseFieldType(rf.Type)
		if !ok {
			return MessageSchema{}, ErrMalformedData
		}
		if rf.Num < 1 || rf.Num > 65535 {
			return MessageSchema{}, ErrMalformedData
		}
		if _, dup := byName[rf.Name]; dup {
			return MessageSchema{}, ErrMalformedData
		}
		if _, dup := byNum[rf.Num]; dup {
			return MessageSchema{}, ErrMalformedData
		}
		fs := FieldSchema{Name: rf.Name, Num: rf.Num, Type: ft, Repeated: rf.Repeated}
		if ft == FieldMessage {
			s := strings.TrimSpace(string(rf.Schema))
			if s == "" || s == "null" {
				return MessageSchema{}, ErrMalformedData
			}
			var ns rawNestedSchema
			if err := json.Unmarshal(rf.Schema, &ns); err != nil {
				return MessageSchema{}, ErrMalformedData
			}
			nested, err := buildMessageSchema(0, ns.Fields)
			if err != nil {
				return MessageSchema{}, err
			}
			fs.Schema = nested
		} else if len(rf.Schema) > 0 {
			return MessageSchema{}, ErrMalformedData
		}
		fields = append(fields, fs)
		byName[rf.Name] = i
		byNum[rf.Num] = i
	}
	return MessageSchema{typeID: typeID, fields: fields, byName: byName, byNum: byNum}, nil
}

func parseFieldType(s string) (FieldType, bool) {
	switch FieldType(s) {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldFloat, FieldDouble,
		FieldBool, FieldString, FieldBytes, FieldMessage:
		return FieldType(s), true
	}
	return "", false
}

func schemasEqual(a, b MessageSchema) bool {
	if a.typeID != b.typeID {
		return false
	}
	return fieldsEqual(a.fields, b.fields)
}

func fieldsEqual(a, b []FieldSchema) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Num != b[i].Num ||
			a[i].Type != b[i].Type || a[i].Repeated != b[i].Repeated {
			return false
		}
		if a[i].Type == FieldMessage {
			if !fieldsEqual(a[i].Schema.fields, b[i].Schema.fields) {
				return false
			}
		}
	}
	return true
}

func (s MessageSchema) lookupName(name string) (int, FieldSchema, bool) {
	i, ok := s.byName[name]
	if !ok {
		return 0, FieldSchema{}, false
	}
	return i, s.fields[i], true
}

func (s MessageSchema) lookupNum(num int) (int, FieldSchema, bool) {
	i, ok := s.byNum[num]
	if !ok {
		return 0, FieldSchema{}, false
	}
	return i, s.fields[i], true
}
