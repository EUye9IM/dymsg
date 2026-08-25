package dymsg

import "math"

// EncodeProto encodes the message in protobuf wire format.
func (m *Message) EncodeProto() ([]byte, error) {
	return m.encodeProtoBytes(), nil
}

func (m *Message) encodeProtoBytes() []byte {
	var buf []byte
	for i, fs := range m.schema.fields {
		if !m.set[i] {
			continue
		}
		buf = appendProtoField(buf, fs, m.vals[i])
	}
	return buf
}

func appendProtoField(buf []byte, fs FieldSchema, val any) []byte {
	if fs.Repeated {
		return appendProtoRepeated(buf, fs, val)
	}
	return appendProtoSingle(buf, fs, val)
}

func appendProtoSingle(buf []byte, fs FieldSchema, val any) []byte {
	switch fs.Type {
	case FieldInt32:
		buf = appendVarint(buf, uint64(fs.Num)<<3)
		buf = appendVarint(buf, uint64(int64(val.(int32))))
	case FieldInt64:
		buf = appendVarint(buf, uint64(fs.Num)<<3)
		buf = appendVarint(buf, uint64(val.(int64)))
	case FieldUint32:
		buf = appendVarint(buf, uint64(fs.Num)<<3)
		buf = appendVarint(buf, uint64(val.(uint32)))
	case FieldUint64:
		buf = appendVarint(buf, uint64(fs.Num)<<3)
		buf = appendVarint(buf, val.(uint64))
	case FieldBool:
		buf = appendVarint(buf, uint64(fs.Num)<<3)
		if val.(bool) {
			buf = appendVarint(buf, 1)
		} else {
			buf = appendVarint(buf, 0)
		}
	case FieldFloat:
		buf = appendVarint(buf, uint64(fs.Num)<<3|5)
		buf = appendFixed32(buf, math.Float32bits(val.(float32)))
	case FieldDouble:
		buf = appendVarint(buf, uint64(fs.Num)<<3|1)
		buf = appendFixed64(buf, math.Float64bits(val.(float64)))
	case FieldString:
		buf = appendVarint(buf, uint64(fs.Num)<<3|2)
		buf = appendBytes(buf, []byte(val.(string)))
	case FieldBytes:
		buf = appendVarint(buf, uint64(fs.Num)<<3|2)
		buf = appendBytes(buf, val.([]byte))
	case FieldMessage:
		mm, _ := val.(*Message)
		if mm == nil {
			return buf
		}
		inner := mm.encodeProtoBytes()
		buf = appendVarint(buf, uint64(fs.Num)<<3|2)
		buf = appendBytes(buf, inner)
	}
	return buf
}

func appendProtoRepeated(buf []byte, fs FieldSchema, val any) []byte {
	switch fs.Type {
	case FieldMessage:
		msgs := val.([]*Message)
		for _, mm := range msgs {
			if mm == nil {
				continue
			}
			inner := mm.encodeProtoBytes()
			buf = appendVarint(buf, uint64(fs.Num)<<3|2)
			buf = appendBytes(buf, inner)
		}
	case FieldString:
		for _, s := range val.([]string) {
			buf = appendVarint(buf, uint64(fs.Num)<<3|2)
			buf = appendBytes(buf, []byte(s))
		}
	case FieldBytes:
		for _, b := range val.([][]byte) {
			buf = appendVarint(buf, uint64(fs.Num)<<3|2)
			buf = appendBytes(buf, b)
		}
	default:
		var body []byte
		switch fs.Type {
		case FieldInt32:
			for _, e := range val.([]int32) {
				body = appendVarint(body, uint64(int64(e)))
			}
		case FieldInt64:
			for _, e := range val.([]int64) {
				body = appendVarint(body, uint64(e))
			}
		case FieldUint32:
			for _, e := range val.([]uint32) {
				body = appendVarint(body, uint64(e))
			}
		case FieldUint64:
			for _, e := range val.([]uint64) {
				body = appendVarint(body, e)
			}
		case FieldBool:
			for _, e := range val.([]bool) {
				if e {
					body = appendVarint(body, 1)
				} else {
					body = appendVarint(body, 0)
				}
			}
		case FieldFloat:
			for _, e := range val.([]float32) {
				body = appendFixed32(body, math.Float32bits(e))
			}
		case FieldDouble:
			for _, e := range val.([]float64) {
				body = appendFixed64(body, math.Float64bits(e))
			}
		}
		if len(body) == 0 {
			return buf
		}
		buf = appendVarint(buf, uint64(fs.Num)<<3|2)
		buf = appendVarint(buf, uint64(len(body)))
		buf = append(buf, body...)
	}
	return buf
}

// DecodeProto replaces the message contents from protobuf wire format bytes.
func (m *Message) DecodeProto(data []byte) error {
	m.clearAll()
	i := 0
	for i < len(data) {
		key, n := consumeVarint(data[i:])
		if n < 0 {
			return ErrTruncated
		}
		i += n
		fieldNum := int(key >> 3)
		wt := int(key & 7)
		if fieldNum == 0 {
			return ErrMalformedData
		}
		if wt == 6 || wt == 7 {
			return ErrMalformedData
		}
		idx, fs, ok := m.schema.lookupNum(fieldNum)
		if !ok {
			ni, err := skipField(data, i, wt)
			if err != nil {
				return err
			}
			i = ni
			continue
		}
		ni, err := m.decodeProtoField(data, i, idx, fs, wt)
		if err != nil {
			return err
		}
		i = ni
	}
	return nil
}

func (m *Message) decodeProtoField(data []byte, i, idx int, fs FieldSchema, wt int) (int, error) {
	if fs.Repeated {
		return m.decodeProtoRepeated(data, i, idx, fs, wt)
	}
	if wt != wireTypeFor(fs.Type) {
		return 0, ErrMalformedData
	}
	if fs.Type == FieldMessage {
		ln, n := consumeVarint(data[i:])
		if n < 0 {
			return 0, ErrTruncated
		}
		i += n
		if uint64(len(data)-i) < ln {
			return 0, ErrTruncated
		}
		nm := newMessage(fs.Schema)
		if err := nm.DecodeProto(data[i : i+int(ln)]); err != nil {
			return 0, err
		}
		m.vals[idx] = nm
		m.set[idx] = true
		return i + int(ln), nil
	}
	v, ni, err := decodeProtoScalar(data, i, fs.Type)
	if err != nil {
		return 0, err
	}
	m.vals[idx] = v
	m.set[idx] = true
	return ni, nil
}

func (m *Message) decodeProtoRepeated(data []byte, i, idx int, fs FieldSchema, wt int) (int, error) {
	switch fs.Type {
	case FieldMessage, FieldString, FieldBytes:
		if wt != 2 {
			return 0, ErrMalformedData
		}
		ln, n := consumeVarint(data[i:])
		if n < 0 {
			return 0, ErrTruncated
		}
		i += n
		if uint64(len(data)-i) < ln {
			return 0, ErrTruncated
		}
		elem := data[i : i+int(ln)]
		i += int(ln)
		if !m.set[idx] {
			m.vals[idx] = makeSlice(fs.Type, 0)
			m.set[idx] = true
		}
		switch fs.Type {
		case FieldMessage:
			nm := newMessage(fs.Schema)
			if err := nm.DecodeProto(elem); err != nil {
				return 0, err
			}
			m.vals[idx] = append(m.vals[idx].([]*Message), nm)
		case FieldString:
			m.vals[idx] = append(m.vals[idx].([]string), string(elem))
		case FieldBytes:
			b := make([]byte, len(elem))
			copy(b, elem)
			m.vals[idx] = append(m.vals[idx].([][]byte), b)
		}
		return i, nil
	default:
		if wt == 2 {
			ln, n := consumeVarint(data[i:])
			if n < 0 {
				return 0, ErrTruncated
			}
			i += n
			if uint64(len(data)-i) < ln {
				return 0, ErrTruncated
			}
			end := i + int(ln)
			if !m.set[idx] {
				m.vals[idx] = makeSlice(fs.Type, 0)
				m.set[idx] = true
			}
			for i < end {
				v, ni, err := decodeProtoScalar(data, i, fs.Type)
				if err != nil {
					return 0, err
				}
				m.vals[idx] = appendToSlice(fs.Type, m.vals[idx], v)
				i = ni
			}
			return end, nil
		}
		if wt != wireTypeFor(fs.Type) {
			return 0, ErrMalformedData
		}
		v, ni, err := decodeProtoScalar(data, i, fs.Type)
		if err != nil {
			return 0, err
		}
		if !m.set[idx] {
			m.vals[idx] = makeSlice(fs.Type, 0)
			m.set[idx] = true
		}
		m.vals[idx] = appendToSlice(fs.Type, m.vals[idx], v)
		return ni, nil
	}
}

func decodeProtoScalar(data []byte, i int, ft FieldType) (any, int, error) {
	switch ft {
	case FieldInt32:
		v, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		return int32(v), i + n, nil
	case FieldInt64:
		v, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		return int64(v), i + n, nil
	case FieldUint32:
		v, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		return uint32(v), i + n, nil
	case FieldUint64:
		v, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		return v, i + n, nil
	case FieldBool:
		v, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		return v != 0, i + n, nil
	case FieldFloat:
		if i+4 > len(data) {
			return nil, 0, ErrTruncated
		}
		bits := uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
		return math.Float32frombits(bits), i + 4, nil
	case FieldDouble:
		if i+8 > len(data) {
			return nil, 0, ErrTruncated
		}
		bits := uint64(data[i]) | uint64(data[i+1])<<8 | uint64(data[i+2])<<16 | uint64(data[i+3])<<24 |
			uint64(data[i+4])<<32 | uint64(data[i+5])<<40 | uint64(data[i+6])<<48 | uint64(data[i+7])<<56
		return math.Float64frombits(bits), i + 8, nil
	case FieldString:
		ln, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		i += n
		if uint64(len(data)-i) < ln {
			return nil, 0, ErrTruncated
		}
		return string(data[i : i+int(ln)]), i + int(ln), nil
	case FieldBytes:
		ln, n := consumeVarint(data[i:])
		if n < 0 {
			return nil, 0, ErrTruncated
		}
		i += n
		if uint64(len(data)-i) < ln {
			return nil, 0, ErrTruncated
		}
		b := make([]byte, int(ln))
		copy(b, data[i:i+int(ln)])
		return b, i + int(ln), nil
	}
	return nil, 0, ErrMalformedData
}

func wireTypeFor(ft FieldType) int {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldBool:
		return 0
	case FieldDouble:
		return 1
	case FieldString, FieldBytes, FieldMessage:
		return 2
	case FieldFloat:
		return 5
	}
	return -1
}

func skipField(data []byte, i, wt int) (int, error) {
	switch wt {
	case 0:
		_, n := consumeVarint(data[i:])
		if n < 0 {
			return 0, ErrTruncated
		}
		return i + n, nil
	case 1:
		if i+8 > len(data) {
			return 0, ErrTruncated
		}
		return i + 8, nil
	case 2:
		ln, n := consumeVarint(data[i:])
		if n < 0 {
			return 0, ErrTruncated
		}
		i += n
		if uint64(len(data)-i) < ln {
			return 0, ErrTruncated
		}
		return i + int(ln), nil
	case 5:
		if i+4 > len(data) {
			return 0, ErrTruncated
		}
		return i + 4, nil
	}
	return 0, ErrMalformedData
}

func consumeVarint(b []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(b); i++ {
		c := b[i]
		if i == 9 {
			v |= uint64(c&0x01) << 63
			return v, 10
		}
		v |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			return v, i + 1
		}
	}
	return 0, -1
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func appendBytes(buf []byte, b []byte) []byte {
	buf = appendVarint(buf, uint64(len(b)))
	return append(buf, b...)
}

func appendFixed32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendFixed64(buf []byte, v uint64) []byte {
	return append(buf,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}
