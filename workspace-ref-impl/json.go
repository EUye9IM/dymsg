package dymsg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math"
	"strconv"
)

// EncodeJSON encodes the message as a JSON object, emitting fields in schema
// declaration order and omitting unset fields.
func (m *Message) EncodeJSON() ([]byte, error) {
	buf, err := appendJSONMessage(make([]byte, 0, 128), m)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func appendJSONMessage(buf []byte, m *Message) ([]byte, error) {
	buf = append(buf, '{')
	first := true
	for i, fs := range m.schema.fields {
		if !m.set[i] {
			continue
		}
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = appendJSONString(buf, fs.Name)
		buf = append(buf, ':')
		var err error
		if buf, err = appendJSONValue(buf, fs, m.vals[i]); err != nil {
			return nil, err
		}
	}
	return append(buf, '}'), nil
}

func appendJSONValue(buf []byte, fs FieldSchema, val any) ([]byte, error) {
	if fs.Repeated {
		switch fs.Type {
		case FieldString:
			buf = append(buf, '[')
			for i, e := range val.([]string) {
				if i > 0 {
					buf = append(buf, ',')
				}
				buf = appendJSONString(buf, e)
			}
			return append(buf, ']'), nil
		case FieldMessage:
			buf = append(buf, '[')
			for i, e := range val.([]*Message) {
				if i > 0 {
					buf = append(buf, ',')
				}
				if e == nil {
					buf = append(buf, "null"...)
					continue
				}
				var err error
				if buf, err = appendJSONMessage(buf, e); err != nil {
					return nil, err
				}
			}
			return append(buf, ']'), nil
		default:
			return appendJSONScalarSlice(buf, fs.Type, val)
		}
	}

	if fs.Type == FieldMessage {
		mm, _ := val.(*Message)
		if mm == nil {
			return append(buf, "null"...), nil
		}
		return appendJSONMessage(buf, mm)
	}
	return appendJSONScalar(buf, fs.Type, val)
}

func appendJSONScalar(buf []byte, ft FieldType, val any) ([]byte, error) {
	switch ft {
	case FieldString:
		return appendJSONString(buf, val.(string)), nil
	case FieldBytes:
		buf = append(buf, '"')
		buf = base64.StdEncoding.AppendEncode(buf, val.([]byte))
		return append(buf, '"'), nil
	case FieldBool:
		if val.(bool) {
			return append(buf, "true"...), nil
		}
		return append(buf, "false"...), nil
	case FieldInt32:
		return strconv.AppendInt(buf, int64(val.(int32)), 10), nil
	case FieldInt64:
		return strconv.AppendInt(buf, val.(int64), 10), nil
	case FieldUint32:
		return strconv.AppendUint(buf, uint64(val.(uint32)), 10), nil
	case FieldUint64:
		return strconv.AppendUint(buf, val.(uint64), 10), nil
	case FieldFloat:
		return appendJSONFloat(buf, float64(val.(float32)), 32)
	case FieldDouble:
		return appendJSONFloat(buf, val.(float64), 64)
	}
	return buf, ErrMalformedData
}

func appendJSONScalarSlice(buf []byte, ft FieldType, val any) ([]byte, error) {
	buf = append(buf, '[')
	switch x := val.(type) {
	case []int32:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendInt(buf, int64(e), 10)
		}
	case []int64:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendInt(buf, e, 10)
		}
	case []uint32:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendUint(buf, uint64(e), 10)
		}
	case []uint64:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = strconv.AppendUint(buf, e, 10)
		}
	case []float32:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			var err error
			if buf, err = appendJSONFloat(buf, float64(e), 32); err != nil {
				return nil, err
			}
		}
	case []float64:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			var err error
			if buf, err = appendJSONFloat(buf, e, 64); err != nil {
				return nil, err
			}
		}
	case []bool:
		for i, e := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			if e {
				buf = append(buf, "true"...)
			} else {
				buf = append(buf, "false"...)
			}
		}
	case [][]byte:
		for i, b := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = base64.StdEncoding.AppendEncode(buf, b)
			buf = append(buf, '"')
		}
	default:
		return nil, ErrMalformedData
	}
	return append(buf, ']'), nil
}

// appendJSONFloat appends f using the same formatting rules as encoding/json:
// the shortest round-tripping representation, switching to exponent form for
// very small or very large magnitudes. NaN and ±Inf are rejected because JSON
// cannot represent them.
func appendJSONFloat(buf []byte, f float64, bits int) ([]byte, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, ErrMalformedData
	}
	abs := math.Abs(f)
	format := byte('f')
	if abs != 0 {
		if bits == 64 && (abs < 1e-6 || abs >= 1e21) ||
			bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
			format = 'e'
		}
	}
	buf = strconv.AppendFloat(buf, f, format, -1, bits)
	if format == 'e' {
		// Clean up e-09 to e-9, matching encoding/json.
		n := len(buf)
		if n >= 4 && buf[n-4] == 'e' && buf[n-3] == '-' && buf[n-2] == '0' {
			buf[n-2] = buf[n-1]
			buf = buf[:n-1]
		}
	}
	return buf, nil
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
