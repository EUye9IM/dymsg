package msgcodec

import (
	"encoding/binary"
	"math"
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

// fieldWire 返回字段期望的 wire type。
func fieldWire(ft FieldType) (int, bool) {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldBool:
		return wireVarint, true
	case FieldFloat:
		return wireFixed32, true
	case FieldDouble:
		return wireFixed64, true
	case FieldString, FieldBytes, FieldMessage:
		return wireBytes, true
	}
	return 0, false
}

// isPacked 判断标量 repeated 是否使用 packed 编码。
func isPacked(ft FieldType) bool {
	switch ft {
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldBool, FieldFloat, FieldDouble:
		return true
	}
	return false
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func appendTag(b []byte, num int, wt int) []byte {
	return appendVarint(b, uint64(num)<<3|uint64(wt))
}

func readVarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	for i := 0; i < len(b); i++ {
		bb := b[i]
		if bb < 0x80 {
			if i == 10 {
				return 0, 0, ErrMalformedData
			}
			if s >= 64 {
				return 0, 0, ErrMalformedData
			}
			x |= uint64(bb) << s
			return x, i + 1, nil
		}
		if s >= 64 {
			return 0, 0, ErrMalformedData
		}
		x |= uint64(bb&0x7f) << s
		s += 7
	}
	return 0, 0, ErrTruncated
}

// appendScalarVal 追加一个不带 key 的标量值。
func appendScalarVal(b []byte, ft FieldType, v any) ([]byte, error) {
	switch ft {
	case FieldInt32:
		return appendVarint(b, uint64(int64(v.(int32)))), nil
	case FieldInt64:
		return appendVarint(b, uint64(v.(int64))), nil
	case FieldUint32:
		return appendVarint(b, uint64(v.(uint32))), nil
	case FieldUint64:
		return appendVarint(b, v.(uint64)), nil
	case FieldBool:
		if v.(bool) {
			return append(b, 1), nil
		}
		return append(b, 0), nil
	case FieldFloat:
		return binary.LittleEndian.AppendUint32(b, math.Float32bits(v.(float32))), nil
	case FieldDouble:
		return binary.LittleEndian.AppendUint64(b, math.Float64bits(v.(float64))), nil
	case FieldString:
		s := v.(string)
		b = appendVarint(b, uint64(len(s)))
		return append(b, s...), nil
	case FieldBytes:
		d := v.([]byte)
		b = appendVarint(b, uint64(len(d)))
		return append(b, d...), nil
	}
	return b, nil
}

// decodeScalarVal 从 b 解码一个不带 key 的标量值,返回值与消费字节数。
func decodeScalarVal(ft FieldType, b []byte) (any, int, error) {
	switch ft {
	case FieldInt32:
		v, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		return int32(v), n, nil
	case FieldInt64:
		v, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		return int64(v), n, nil
	case FieldUint32:
		v, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		return uint32(v), n, nil
	case FieldUint64:
		v, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		return v, n, nil
	case FieldBool:
		v, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		return v != 0, n, nil
	case FieldFloat:
		if len(b) < 4 {
			return nil, 0, ErrTruncated
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(b)), 4, nil
	case FieldDouble:
		if len(b) < 8 {
			return nil, 0, ErrTruncated
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b)), 8, nil
	case FieldString:
		ln, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		if int(ln) > len(b)-n {
			return nil, 0, ErrTruncated
		}
		return string(b[n : n+int(ln)]), n + int(ln), nil
	case FieldBytes:
		ln, n, err := readVarint(b)
		if err != nil {
			return nil, 0, err
		}
		if int(ln) > len(b)-n {
			return nil, 0, ErrTruncated
		}
		d := append([]byte(nil), b[n:n+int(ln)]...)
		return d, n + int(ln), nil
	}
	return nil, 0, ErrMalformedData
}

// skipField 按 wire type 跳过该字段的值部分,返回消费字节数。
func skipField(b []byte, wt int) (int, error) {
	switch wt {
	case wireVarint:
		_, n, err := readVarint(b)
		return n, err
	case wireFixed64:
		if len(b) < 8 {
			return 0, ErrTruncated
		}
		return 8, nil
	case wireBytes:
		ln, n, err := readVarint(b)
		if err != nil {
			return 0, err
		}
		if int(ln) > len(b)-n {
			return 0, ErrTruncated
		}
		return n + int(ln), nil
	case wireFixed32:
		if len(b) < 4 {
			return 0, ErrTruncated
		}
		return 4, nil
	}
	return 0, ErrMalformedData
}
