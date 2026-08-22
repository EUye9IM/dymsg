package dymsg

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// EncodeJSON encodes the message as a JSON object (or a scalar/array when the
// message is a value wrapper). Fields are emitted in schema declaration order.
func (m *Message) EncodeJSON() ([]byte, error) {
	if m.valueNode {
		return encodeJSONValue(m.value)
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for i, fv := range m.fields {
		if !fv.present {
			continue
		}
		fs := &m.schema.fields[i]
		if fs.Type == FieldMessage && !fs.Repeated && fv.value == nil {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false

		key, _ := json.Marshal(fs.Name)
		buf.Write(key)
		buf.WriteByte(':')
		val, err := encodeJSONValue(fv.value)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func encodeJSONValue(v any) ([]byte, error) {
	switch x := v.(type) {
	case *Message:
		if x == nil {
			return []byte("null"), nil
		}
		return x.EncodeJSON()
	case []*Message:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, m := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if m == nil {
				buf.WriteString("null")
				continue
			}
			b, err := m.EncodeJSON()
			if err != nil {
				return nil, err
			}
			buf.Write(b)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(v)
	}
}

// DecodeJSON decodes a JSON object into the message. Unknown keys are ignored,
// null field values mark the field unset, and type mismatches yield
// ErrMalformedData.
func (m *Message) DecodeJSON(data []byte) error {
	if m.schema == nil {
		return ErrMalformedData
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return ErrTruncated
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return ErrMalformedData
	}
	m.clear()
	if obj == nil {
		return nil
	}
	return m.decodeJSONObject(obj)
}

func (m *Message) decodeJSONObject(obj map[string]json.RawMessage) error {
	for key, raw := range obj {
		idx := m.schema.fieldIndex(key)
		if idx < 0 {
			continue
		}
		fs := &m.schema.fields[idx]
		fv := &m.fields[idx]
		if isJSONNull(raw) {
			fv.present = false
			fv.value = nil
			continue
		}
		val, err := decodeJSONField(fs, raw)
		if err != nil {
			return err
		}
		fv.present = true
		fv.value = val
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func decodeJSONField(fs *FieldSchema, raw json.RawMessage) (any, error) {
	if fs.Repeated {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, ErrMalformedData
		}
		return decodeJSONRepeated(fs, arr)
	}
	if fs.Type == FieldMessage {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, ErrMalformedData
		}
		nm := newMessage(&fs.Schema)
		if err := nm.decodeJSONObject(obj); err != nil {
			return nil, err
		}
		return nm, nil
	}
	return decodeJSONScalar(fs.Type, raw)
}

func decodeJSONRepeated(fs *FieldSchema, arr []json.RawMessage) (any, error) {
	if fs.Type == FieldMessage {
		dst := make([]*Message, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				dst[i] = nil
				continue
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(raw, &obj); err != nil {
				return nil, ErrMalformedData
			}
			nm := newMessage(&fs.Schema)
			if err := nm.decodeJSONObject(obj); err != nil {
				return nil, err
			}
			dst[i] = nm
		}
		return dst, nil
	}

	switch fs.Type {
	case FieldInt32:
		dst := make([]int32, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			n, err := parseJSONInt(raw)
			if err != nil {
				return nil, err
			}
			if n < minInt32 || n > maxInt32 {
				return nil, ErrMalformedData
			}
			dst[i] = int32(n)
		}
		return dst, nil
	case FieldInt64:
		dst := make([]int64, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			n, err := parseJSONInt(raw)
			if err != nil {
				return nil, err
			}
			dst[i] = n
		}
		return dst, nil
	case FieldUint32:
		dst := make([]uint32, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			n, err := parseJSONUint(raw)
			if err != nil {
				return nil, err
			}
			if n > maxUint32 {
				return nil, ErrMalformedData
			}
			dst[i] = uint32(n)
		}
		return dst, nil
	case FieldUint64:
		dst := make([]uint64, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			n, err := parseJSONUint(raw)
			if err != nil {
				return nil, err
			}
			dst[i] = n
		}
		return dst, nil
	case FieldFloat:
		dst := make([]float32, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			f, err := parseJSONFloat(raw)
			if err != nil {
				return nil, err
			}
			if !math.IsNaN(f) && !math.IsInf(f, 0) && (f > math.MaxFloat32 || f < -math.MaxFloat32) {
				return nil, ErrMalformedData
			}
			dst[i] = float32(f)
		}
		return dst, nil
	case FieldDouble:
		dst := make([]float64, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			f, err := parseJSONFloat(raw)
			if err != nil {
				return nil, err
			}
			dst[i] = f
		}
		return dst, nil
	case FieldBool:
		dst := make([]bool, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			var b bool
			if err := json.Unmarshal(raw, &b); err != nil {
				return nil, ErrMalformedData
			}
			dst[i] = b
		}
		return dst, nil
	case FieldString:
		dst := make([]string, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, ErrMalformedData
			}
			dst[i] = s
		}
		return dst, nil
	case FieldBytes:
		dst := make([][]byte, len(arr))
		for i, raw := range arr {
			if isJSONNull(raw) {
				return nil, ErrMalformedData
			}
			var b []byte
			if err := json.Unmarshal(raw, &b); err != nil {
				return nil, ErrMalformedData
			}
			dst[i] = b
		}
		return dst, nil
	}
	return nil, ErrMalformedData
}

func decodeJSONScalar(ft FieldType, raw json.RawMessage) (any, error) {
	switch ft {
	case FieldString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, ErrMalformedData
		}
		return s, nil
	case FieldBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, ErrMalformedData
		}
		return b, nil
	case FieldBytes:
		var b []byte
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, ErrMalformedData
		}
		return b, nil
	case FieldInt32:
		n, err := parseJSONInt(raw)
		if err != nil {
			return nil, err
		}
		if n < minInt32 || n > maxInt32 {
			return nil, ErrMalformedData
		}
		return int32(n), nil
	case FieldInt64:
		n, err := parseJSONInt(raw)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldUint32:
		n, err := parseJSONUint(raw)
		if err != nil {
			return nil, err
		}
		if n > maxUint32 {
			return nil, ErrMalformedData
		}
		return uint32(n), nil
	case FieldUint64:
		n, err := parseJSONUint(raw)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldFloat:
		f, err := parseJSONFloat(raw)
		if err != nil {
			return nil, err
		}
		if !math.IsNaN(f) && !math.IsInf(f, 0) && (f > math.MaxFloat32 || f < -math.MaxFloat32) {
			return nil, ErrMalformedData
		}
		return float32(f), nil
	case FieldDouble:
		f, err := parseJSONFloat(raw)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return nil, ErrMalformedData
}

func parseJSONInt(raw json.RawMessage) (int64, error) {
	s := strings.TrimSpace(string(raw))
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, ErrMalformedData
	}
	return n, nil
}

func parseJSONUint(raw json.RawMessage) (uint64, error) {
	s := strings.TrimSpace(string(raw))
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, ErrMalformedData
	}
	return n, nil
}

func parseJSONFloat(raw json.RawMessage) (float64, error) {
	s := strings.TrimSpace(string(raw))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, ErrMalformedData
	}
	return f, nil
}
