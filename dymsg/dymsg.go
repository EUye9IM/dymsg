// Package dymsg implements a dynamic message codec library whose message
// structure is defined by configuration at runtime rather than by compiled Go
// structs. Messages can be registered by type ID and support field access plus
// JSON and Protobuf wire-format encoding/decoding.
package dymsg

import (
	"errors"
	"sync"
)

// FieldType identifies the native type of a message field.
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

// FieldSchema describes a single field. Name is also used as the JSON key,
// Num is the protobuf field number, and Schema is the inline nested schema for
// message-typed fields.
type FieldSchema struct {
	Name     string
	Num      int
	Type     FieldType
	Repeated bool
	Schema   MessageSchema
}

// MessageSchema represents one message type. Its internals are private but are
// sufficient to drive field access and both codecs.
type MessageSchema struct {
	typeID uint16
	fields []FieldSchema
	byName map[string]int
	byNum  map[int]int
}

type fieldValue struct {
	present bool
	value   any
}

// Message is a dynamic message instance. A Message either represents a
// structured message (schema != nil) or, when returned from Get, a scalar or
// repeated value wrapper.
type Message struct {
	schema    *MessageSchema
	fields    []fieldValue
	valueNode bool
	value     any
}

// Sentinel errors returned by the package.
var (
	ErrDuplicateID     = errors.New("dymsg: duplicate type id")
	ErrUnknownTypeID   = errors.New("dymsg: unknown type id")
	ErrFieldNotFound   = errors.New("dymsg: field not found")
	ErrIndexOutOfRange = errors.New("dymsg: index out of range")
	ErrTypeMismatch    = errors.New("dymsg: type mismatch")
	ErrMalformedData   = errors.New("dymsg: malformed data")
	ErrTruncated       = errors.New("dymsg: truncated data")
)

var (
	registryMu sync.RWMutex
	registry   = map[uint16]*MessageSchema{}
)

// Register registers a top-level message schema under its type ID. It returns
// ErrDuplicateID if the type ID is already registered.
func Register(s MessageSchema) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[s.typeID]; ok {
		return ErrDuplicateID
	}
	sc := s
	registry[s.typeID] = &sc
	return nil
}

// New creates an empty message for a previously registered type ID.
func New(typeID uint16) (*Message, error) {
	registryMu.RLock()
	sc := registry[typeID]
	registryMu.RUnlock()
	if sc == nil {
		return nil, ErrUnknownTypeID
	}
	return newMessage(sc), nil
}

func newMessage(sc *MessageSchema) *Message {
	return &Message{schema: sc, fields: make([]fieldValue, len(sc.fields))}
}

func (m *Message) clear() {
	for i := range m.fields {
		m.fields[i].present = false
		m.fields[i].value = nil
	}
}
