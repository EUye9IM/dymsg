package dymsg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
)

// EncodeJSON encodes the message as a JSON object, emitting fields in schema
// declaration order and omitting unset fields.
func (m *Message) EncodeJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := appendJSONMessage(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func appendJSONMessage(buf *bytes.Buffer, m *Message) error {
	buf.WriteByte('{')
	first := true
	for i, fs := range m.schema.fields {
		if !m.set[i] {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.Write(appendJSONString(nil, fs.Name))
		buf.WriteByte(':')
		if err := appendJSONValue(buf, fs, m.vals[i]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func appendJSONValue(buf *bytes.Buffer, fs FieldSchema, val any) error {
	if fs.Repeated {
		switch fs.Type {
		case FieldString:
			buf.WriteByte('[')
			s := val.([]string)
			for i, e := range s {
				if i > 0 {
					buf.WriteByte(',')
				}
				buf.Write(appendJSONString(nil, e))
			}
			buf.WriteByte(']')
		case FieldMessage:
			buf.WriteByte('[')
			s := val.([]*Message)
			for i, e := range s {
				if i > 0 {
					buf.WriteByte(',')
				}
				if e == nil {
					buf.WriteString("null")
					continue
				}
				if err := appendJSONMessage(buf, e); err != nil {
					return err
				}
			}
			buf.WriteByte(']')
		default:
			b, err := json.Marshal(val)
			if err != nil {
				return ErrMalformedData
			}
			buf.Write(b)
		}
		return nil
	}

	if fs.Type == FieldMessage {
		mm, _ := val.(*Message)
		if mm == nil {
			buf.WriteString("null")
			return nil
		}
		return appendJSONMessage(buf, mm)
	}

	switch fs.Type {
	case FieldString:
		buf.Write(appendJSONString(nil, val.(string)))
	case FieldBytes:
		b, _ := json.Marshal(val.([]byte))
		buf.Write(b)
	case FieldBool:
		if val.(bool) {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return ErrMalformedData
		}
		buf.Write(b)
	}
	return nil
}

// DecodeJSON replaces the message contents from a JSON object.
func (m *Message) DecodeJSON(data []byte) error {
	if len(data) == 0 {
		return ErrTruncated
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		if err == io.EOF {
			return ErrTruncated
		}
		return ErrMalformedData
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return ErrMalformedData
	}

	m.clearAll()
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return ErrMalformedData
	}
	return m.decodeJSONObject(obj)
}

func (m *Message) decodeJSONObject(obj map[string]any) error {
	for key, raw := range obj {
		idx, fs, ok := m.schema.lookupName(key)
		if !ok {
			continue
		}
		if err := m.decodeJSONField(idx, fs, raw); err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) decodeJSONField(idx int, fs FieldSchema, raw any) error {
	if raw == nil {
		m.set[idx] = false
		m.vals[idx] = nil
		return nil
	}
	if fs.Repeated {
		arr, ok := raw.([]any)
		if !ok {
			return ErrMalformedData
		}
		return m.decodeJSONRepeated(idx, fs, arr)
	}
	if fs.Type == FieldMessage {
		obj, ok := raw.(map[string]any)
		if !ok {
			return ErrMalformedData
		}
		nm := newMessage(fs.Schema)
		if err := nm.decodeJSONObject(obj); err != nil {
			return err
		}
		m.vals[idx] = nm
		m.set[idx] = true
		return nil
	}
	v, err := decodeJSONScalar(fs.Type, raw)
	if err != nil {
		return err
	}
	m.vals[idx] = v
	m.set[idx] = true
	return nil
}

func (m *Message) decodeJSONRepeated(idx int, fs FieldSchema, arr []any) error {
	if fs.Type == FieldMessage {
		msgs := make([]*Message, len(arr))
		for i, elem := range arr {
			if elem == nil {
				msgs[i] = nil
				continue
			}
			obj, ok := elem.(map[string]any)
			if !ok {
				return ErrMalformedData
			}
			nm := newMessage(fs.Schema)
			if err := nm.decodeJSONObject(obj); err != nil {
				return err
			}
			msgs[i] = nm
		}
		m.vals[idx] = msgs
		m.set[idx] = true
		return nil
	}
	slice := makeSlice(fs.Type, len(arr))
	for i, elem := range arr {
		if elem == nil {
			return ErrMalformedData
		}
		v, err := decodeJSONScalar(fs.Type, elem)
		if err != nil {
			return err
		}
		setSliceElem(slice, i, v)
	}
	m.vals[idx] = slice
	m.set[idx] = true
	return nil
}

func decodeJSONScalar(ft FieldType, raw any) (any, error) {
	switch ft {
	case FieldInt32:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		i, err := strconv.ParseInt(string(n), 10, 32)
		if err != nil {
			return nil, ErrMalformedData
		}
		return int32(i), nil
	case FieldInt64:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		i, err := strconv.ParseInt(string(n), 10, 64)
		if err != nil {
			return nil, ErrMalformedData
		}
		return i, nil
	case FieldUint32:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		u, err := strconv.ParseUint(string(n), 10, 32)
		if err != nil {
			return nil, ErrMalformedData
		}
		return uint32(u), nil
	case FieldUint64:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		u, err := strconv.ParseUint(string(n), 10, 64)
		if err != nil {
			return nil, ErrMalformedData
		}
		return u, nil
	case FieldFloat:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		f, err := strconv.ParseFloat(string(n), 32)
		if err != nil {
			return nil, ErrMalformedData
		}
		return float32(f), nil
	case FieldDouble:
		n, ok := raw.(json.Number)
		if !ok {
			return nil, ErrMalformedData
		}
		f, err := strconv.ParseFloat(string(n), 64)
		if err != nil {
			return nil, ErrMalformedData
		}
		return f, nil
	case FieldBool:
		b, ok := raw.(bool)
		if !ok {
			return nil, ErrMalformedData
		}
		return b, nil
	case FieldString:
		s, ok := raw.(string)
		if !ok {
			return nil, ErrMalformedData
		}
		return s, nil
	case FieldBytes:
		s, ok := raw.(string)
		if !ok {
			return nil, ErrMalformedData
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, ErrMalformedData
		}
		return b, nil
	}
	return nil, ErrMalformedData
}

func appendJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			if c < 0x20 {
				buf = append(buf, '\\', 'u', '0', '0', hexDigit(c>>4), hexDigit(c&0x0F))
			} else {
				buf = append(buf, c)
			}
		}
	}
	return append(buf, '"')
}

func hexDigit(b byte) byte {
	const hex = "0123456789abcdef"
	return hex[b]
}
