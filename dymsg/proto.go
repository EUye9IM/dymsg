package dymsg

import (
	"encoding/binary"
	"math"
	"reflect"
)

const (
	wireVarint          = 0
	wireFixed64         = 1
	wireLengthDelimited = 2
	wireFixed32         = 5
)

// EncodeProto encodes the message using the Protobuf wire format.
func (m *Message) EncodeProto() ([]byte, error) {
	if m.schema == nil {
		return nil, ErrMalformedData
	}
	var b []byte
	for i, fv := range m.fields {
		if !fv.present {
			continue
		}
		fs := &m.schema.fields[i]
		if fs.Type == FieldMessage && !fs.Repeated && fv.value == nil {
			continue
		}
		b = appendField(b, fs, fv.value)
	}
	return b, nil
}

// DecodeProto decodes Protobuf wire-format data into the message.
func (m *Message) DecodeProto(data []byte) error {
	if m.schema == nil {
		return ErrMalformedData
	}
	if len(data) == 0 {
		return ErrTruncated
	}
	m.clear()

	b := data
	for len(b) > 0 {
		key, n, err := consumeVarint(b)
		if err != nil {
			return err
		}
		b = b[n:]

		num := int(key >> 3)
		wt := int(key & 7)
		if num == 0 {
			return ErrMalformedData
		}
		if wt == 6 || wt == 7 {
			return ErrMalformedData
		}

		idx := m.schema.fieldIndexByNum(num)
		if idx < 0 {
			b, err = skipField(b, wt)
			if err != nil {
				return err
			}
			continue
		}
		fs := &m.schema.fields[idx]
		fv := &m.fields[idx]
		b, err = m.decodeKnownField(b, fs, fv, wt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) decodeKnownField(b []byte, fs *FieldSchema, fv *fieldValue, wt int) ([]byte, error) {
	if fs.Repeated {
		if isPackedScalar(fs.Type) {
			if wt == wireLengthDelimited {
				return m.decodePackedScalar(b, fs, fv)
			}
			if wt == wireTypeFor(fs.Type) {
				return m.decodeUnpackedScalar(b, fs, fv)
			}
			return nil, ErrMalformedData
		}
		if wt != wireLengthDelimited {
			return nil, ErrMalformedData
		}
		return m.decodeRepeatedLengthDelimited(b, fs, fv)
	}

	if wt != wireTypeFor(fs.Type) {
		return nil, ErrMalformedData
	}
	return m.decodeSingleField(b, fs, fv)
}

func (m *Message) decodeSingleField(b []byte, fs *FieldSchema, fv *fieldValue) ([]byte, error) {
	switch fs.Type {
	case FieldString:
		content, rest, err := consumeBytes(b)
		if err != nil {
			return nil, err
		}
		fv.present = true
		fv.value = string(content)
		return rest, nil
	case FieldBytes:
		content, rest, err := consumeBytes(b)
		if err != nil {
			return nil, err
		}
		fv.present = true
		fv.value = content
		return rest, nil
	case FieldMessage:
		content, rest, err := consumeBytes(b)
		if err != nil {
			return nil, err
		}
		nm := newMessage(&fs.Schema)
		if err := nm.DecodeProto(content); err != nil {
			return nil, err
		}
		fv.present = true
		fv.value = nm
		return rest, nil
	default:
		elem, rest, err := decodeScalarValue(b, fs.Type)
		if err != nil {
			return nil, err
		}
		fv.present = true
		fv.value = elem
		return rest, nil
	}
}

func (m *Message) decodePackedScalar(b []byte, fs *FieldSchema, fv *fieldValue) ([]byte, error) {
	length, n, err := consumeVarint(b)
	if err != nil {
		return nil, err
	}
	b = b[n:]
	if uint64(len(b)) < length {
		return nil, ErrTruncated
	}
	packed := b[:length]
	rest := b[length:]

	if !fv.present {
		fv.value = newScalarSlice(fs.Type)
		fv.present = true
	}
	for len(packed) > 0 {
		elem, r, err := decodeScalarValue(packed, fs.Type)
		if err != nil {
			return nil, err
		}
		packed = r
		fv.value = appendSliceElem(fv.value, elem)
	}
	return rest, nil
}

func (m *Message) decodeUnpackedScalar(b []byte, fs *FieldSchema, fv *fieldValue) ([]byte, error) {
	elem, rest, err := decodeScalarValue(b, fs.Type)
	if err != nil {
		return nil, err
	}
	if !fv.present {
		fv.value = newScalarSlice(fs.Type)
		fv.present = true
	}
	fv.value = appendSliceElem(fv.value, elem)
	return rest, nil
}

func (m *Message) decodeRepeatedLengthDelimited(b []byte, fs *FieldSchema, fv *fieldValue) ([]byte, error) {
	content, rest, err := consumeBytes(b)
	if err != nil {
		return nil, err
	}
	if !fv.present {
		if fs.Type == FieldMessage {
			fv.value = []*Message{}
		} else {
			fv.value = newScalarSlice(fs.Type)
		}
		fv.present = true
	}
	switch fs.Type {
	case FieldString:
		fv.value = appendSliceElem(fv.value, string(content))
	case FieldBytes:
		fv.value = appendSliceElem(fv.value, content)
	case FieldMessage:
		nm := newMessage(&fs.Schema)
		if err := nm.DecodeProto(content); err != nil {
			return nil, err
		}
		fv.value = appendSliceElem(fv.value, nm)
	}
	return rest, nil
}

func decodeScalarValue(b []byte, ft FieldType) (any, []byte, error) {
	switch ft {
	case FieldInt32:
		v, n, err := consumeVarint(b)
		if err != nil {
			return nil, nil, err
		}
		return int32(v), b[n:], nil
	case FieldInt64:
		v, n, err := consumeVarint(b)
		if err != nil {
			return nil, nil, err
		}
		return int64(v), b[n:], nil
	case FieldUint32:
		v, n, err := consumeVarint(b)
		if err != nil {
			return nil, nil, err
		}
		return uint32(v), b[n:], nil
	case FieldUint64:
		v, n, err := consumeVarint(b)
		if err != nil {
			return nil, nil, err
		}
		return v, b[n:], nil
	case FieldBool:
		v, n, err := consumeVarint(b)
		if err != nil {
			return nil, nil, err
		}
		return v != 0, b[n:], nil
	case FieldFloat:
		if len(b) < 4 {
			return nil, nil, ErrTruncated
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(b[:4])), b[4:], nil
	case FieldDouble:
		if len(b) < 8 {
			return nil, nil, ErrTruncated
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b[:8])), b[8:], nil
	}
	return nil, nil, ErrMalformedData
}

func appendField(b []byte, fs *FieldSchema, val any) []byte {
	if fs.Repeated {
		return appendRepeated(b, fs, val)
	}
	return appendSingle(b, fs, val)
}

func appendSingle(b []byte, fs *FieldSchema, val any) []byte {
	switch fs.Type {
	case FieldInt32:
		b = appendTag(b, fs.Num, wireVarint)
		return appendVarint(b, uint64(int64(val.(int32))))
	case FieldInt64:
		b = appendTag(b, fs.Num, wireVarint)
		return appendVarint(b, uint64(val.(int64)))
	case FieldUint32:
		b = appendTag(b, fs.Num, wireVarint)
		return appendVarint(b, uint64(val.(uint32)))
	case FieldUint64:
		b = appendTag(b, fs.Num, wireVarint)
		return appendVarint(b, val.(uint64))
	case FieldBool:
		b = appendTag(b, fs.Num, wireVarint)
		if val.(bool) {
			return appendVarint(b, 1)
		}
		return appendVarint(b, 0)
	case FieldFloat:
		b = appendTag(b, fs.Num, wireFixed32)
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(val.(float32)))
		return append(b, tmp[:]...)
	case FieldDouble:
		b = appendTag(b, fs.Num, wireFixed64)
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(val.(float64)))
		return append(b, tmp[:]...)
	case FieldString:
		b = appendTag(b, fs.Num, wireLengthDelimited)
		s := val.(string)
		b = appendVarint(b, uint64(len(s)))
		return append(b, s...)
	case FieldBytes:
		b = appendTag(b, fs.Num, wireLengthDelimited)
		s := val.([]byte)
		b = appendVarint(b, uint64(len(s)))
		return append(b, s...)
	case FieldMessage:
		inner, _ := val.(*Message).EncodeProto()
		b = appendTag(b, fs.Num, wireLengthDelimited)
		b = appendVarint(b, uint64(len(inner)))
		return append(b, inner...)
	}
	return b
}

func appendRepeated(b []byte, fs *FieldSchema, val any) []byte {
	if isPackedScalar(fs.Type) {
		var tmp []byte
		rv := reflect.ValueOf(val)
		for i := 0; i < rv.Len(); i++ {
			tmp = appendScalarValue(tmp, fs.Type, rv.Index(i))
		}
		b = appendTag(b, fs.Num, wireLengthDelimited)
		b = appendVarint(b, uint64(len(tmp)))
		return append(b, tmp...)
	}

	rv := reflect.ValueOf(val)
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		switch fs.Type {
		case FieldString:
			b = appendTag(b, fs.Num, wireLengthDelimited)
			s := elem.String()
			b = appendVarint(b, uint64(len(s)))
			b = append(b, s...)
		case FieldBytes:
			b = appendTag(b, fs.Num, wireLengthDelimited)
			s := elem.Bytes()
			b = appendVarint(b, uint64(len(s)))
			b = append(b, s...)
		case FieldMessage:
			m := elem.Interface().(*Message)
			if m == nil {
				continue
			}
			inner, _ := m.EncodeProto()
			b = appendTag(b, fs.Num, wireLengthDelimited)
			b = appendVarint(b, uint64(len(inner)))
			b = append(b, inner...)
		}
	}
	return b
}

func appendScalarValue(b []byte, ft FieldType, rv reflect.Value) []byte {
	switch ft {
	case FieldInt32:
		return appendVarint(b, uint64(int64(int32(rv.Int()))))
	case FieldInt64:
		return appendVarint(b, uint64(rv.Int()))
	case FieldUint32:
		return appendVarint(b, uint64(uint32(rv.Uint())))
	case FieldUint64:
		return appendVarint(b, rv.Uint())
	case FieldBool:
		if rv.Bool() {
			return appendVarint(b, 1)
		}
		return appendVarint(b, 0)
	case FieldFloat:
		var tmp [4]byte
		binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(float32(rv.Float())))
		return append(b, tmp[:]...)
	case FieldDouble:
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(rv.Float()))
		return append(b, tmp[:]...)
	}
	return b
}

func appendTag(b []byte, num int, wt int) []byte {
	return appendVarint(b, uint64(num)<<3|uint64(wt))
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func consumeVarint(b []byte) (uint64, int, error) {
	var v uint64
	for i := 0; i < len(b); i++ {
		if i >= 10 {
			return 0, 0, ErrMalformedData
		}
		c := b[i]
		if i == 9 && c > 1 {
			return 0, 0, ErrMalformedData
		}
		v |= uint64(c&0x7f) << (7 * uint(i))
		if c < 0x80 {
			return v, i + 1, nil
		}
	}
	return 0, 0, ErrTruncated
}

func consumeBytes(b []byte) ([]byte, []byte, error) {
	length, n, err := consumeVarint(b)
	if err != nil {
		return nil, nil, err
	}
	b = b[n:]
	if uint64(len(b)) < length {
		return nil, nil, ErrTruncated
	}
	return b[:length], b[length:], nil
}

func skipField(b []byte, wt int) ([]byte, error) {
	switch wt {
	case wireVarint:
		_, n, err := consumeVarint(b)
		if err != nil {
			return nil, err
		}
		return b[n:], nil
	case wireFixed64:
		if len(b) < 8 {
			return nil, ErrTruncated
		}
		return b[8:], nil
	case wireLengthDelimited:
		length, n, err := consumeVarint(b)
		if err != nil {
			return nil, err
		}
		b = b[n:]
		if uint64(len(b)) < length {
			return nil, ErrTruncated
		}
		return b[length:], nil
	case wireFixed32:
		if len(b) < 4 {
			return nil, ErrTruncated
		}
		return b[4:], nil
	}
	return nil, ErrMalformedData
}

func isPackedScalar(ft FieldType) bool {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldBool, FieldFloat, FieldDouble:
		return true
	}
	return false
}

func wireTypeFor(ft FieldType) int {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldBool:
		return wireVarint
	case FieldFloat:
		return wireFixed32
	case FieldDouble:
		return wireFixed64
	case FieldString, FieldBytes, FieldMessage:
		return wireLengthDelimited
	}
	return -1
}
