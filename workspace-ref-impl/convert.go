package dymsg

import (
	"math"
	"reflect"
	"strconv"
)

func convertScalar(ft FieldType, v any) (any, error) {
	switch ft {
	case FieldInt32:
		n, err := convertInt(v, 32)
		if err != nil {
			return nil, err
		}
		return int32(n), nil
	case FieldInt64:
		n, err := convertInt(v, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldUint32:
		n, err := convertUint(v, 32)
		if err != nil {
			return nil, err
		}
		return uint32(n), nil
	case FieldUint64:
		n, err := convertUint(v, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldFloat:
		f, err := convertFloat(v)
		if err != nil {
			return nil, err
		}
		return toFloat32(f)
	case FieldDouble:
		f, err := convertFloat(v)
		if err != nil {
			return nil, err
		}
		return f, nil
	case FieldBool:
		return convertBool(v)
	case FieldString:
		return convertString(v)
	case FieldBytes:
		return convertBytes(v)
	}
	return nil, ErrTypeMismatch
}

func convertInt(v any, bits int) (int64, error) {
	lo, hi := intRange(bits)
	switch x := v.(type) {
	case int:
		return checkInt(int64(x), lo, hi)
	case int8:
		return checkInt(int64(x), lo, hi)
	case int16:
		return checkInt(int64(x), lo, hi)
	case int32:
		return checkInt(int64(x), lo, hi)
	case int64:
		return checkInt(x, lo, hi)
	case uint:
		return checkUintToInt(uint64(x), hi)
	case uint8:
		return checkUintToInt(uint64(x), hi)
	case uint16:
		return checkUintToInt(uint64(x), hi)
	case uint32:
		return checkUintToInt(uint64(x), hi)
	case uint64:
		return checkUintToInt(x, hi)
	case float32:
		return checkFloatToInt(float64(x), bits)
	case float64:
		return checkFloatToInt(x, bits)
	case string:
		n, err := strconv.ParseInt(x, 10, bits)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	default:
		return convertIntReflect(v, lo, hi, bits)
	}
}

func convertIntReflect(v any, lo, hi int64, bits int) (int64, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return 0, ErrTypeMismatch
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := rv.Int()
		if n < lo || n > hi {
			return 0, ErrTypeMismatch
		}
		return n, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return checkUintToInt(rv.Uint(), hi)
	case reflect.Float32, reflect.Float64:
		return checkFloatToInt(rv.Float(), bits)
	case reflect.String:
		n, err := strconv.ParseInt(rv.String(), 10, bits)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	}
	return 0, ErrTypeMismatch
}

func convertUint(v any, bits int) (uint64, error) {
	hi := uint64(math.MaxUint32)
	if bits == 64 {
		hi = math.MaxUint64
	}
	switch x := v.(type) {
	case int:
		return checkIntToUint(int64(x), hi)
	case int8:
		return checkIntToUint(int64(x), hi)
	case int16:
		return checkIntToUint(int64(x), hi)
	case int32:
		return checkIntToUint(int64(x), hi)
	case int64:
		return checkIntToUint(x, hi)
	case uint:
		return checkUintRange(uint64(x), hi)
	case uint8:
		return checkUintRange(uint64(x), hi)
	case uint16:
		return checkUintRange(uint64(x), hi)
	case uint32:
		return checkUintRange(uint64(x), hi)
	case uint64:
		return checkUintRange(x, hi)
	case float32:
		return checkFloatToUint(float64(x), bits)
	case float64:
		return checkFloatToUint(x, bits)
	case string:
		n, err := strconv.ParseUint(x, 10, bits)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	default:
		return convertUintReflect(v, hi, bits)
	}
}

func convertUintReflect(v any, hi uint64, bits int) (uint64, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return 0, ErrTypeMismatch
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return checkIntToUint(rv.Int(), hi)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return checkUintRange(rv.Uint(), hi)
	case reflect.Float32, reflect.Float64:
		return checkFloatToUint(rv.Float(), bits)
	case reflect.String:
		n, err := strconv.ParseUint(rv.String(), 10, bits)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	}
	return 0, ErrTypeMismatch
}

func convertFloat(v any) (float64, error) {
	switch x := v.(type) {
	case int:
		return float64(x), nil
	case int8:
		return float64(x), nil
	case int16:
		return float64(x), nil
	case int32:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint:
		return float64(x), nil
	case uint8:
		return float64(x), nil
	case uint16:
		return float64(x), nil
	case uint32:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return f, nil
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return 0, ErrTypeMismatch
		}
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return rv.Float(), nil
		case reflect.String:
			f, err := strconv.ParseFloat(rv.String(), 64)
			if err != nil {
				return 0, ErrTypeMismatch
			}
			return f, nil
		}
		return 0, ErrTypeMismatch
	}
}

func convertBool(v any) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	rv := reflect.ValueOf(v)
	if rv.IsValid() && rv.Kind() == reflect.Bool {
		return rv.Bool(), nil
	}
	return false, ErrTypeMismatch
}

func convertString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []byte:
		return string(x), nil
	case int:
		return strconv.FormatInt(int64(x), 10), nil
	case int8:
		return strconv.FormatInt(int64(x), 10), nil
	case int16:
		return strconv.FormatInt(int64(x), 10), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return "", ErrTypeMismatch
		}
		switch rv.Kind() {
		case reflect.String:
			return rv.String(), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(rv.Int(), 10), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return strconv.FormatUint(rv.Uint(), 10), nil
		case reflect.Float32, reflect.Float64:
			return strconv.FormatFloat(rv.Float(), 'g', -1, 64), nil
		case reflect.Slice:
			if rv.Type().Elem().Kind() == reflect.Uint8 {
				return string(rv.Bytes()), nil
			}
		}
		return "", ErrTypeMismatch
	}
}

func convertBytes(v any) ([]byte, error) {
	switch x := v.(type) {
	case []byte:
		b := make([]byte, len(x))
		copy(b, x)
		return b, nil
	case string:
		return []byte(x), nil
	default:
		rv := reflect.ValueOf(v)
		if rv.IsValid() && rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(b), rv)
			return b, nil
		}
		return nil, ErrTypeMismatch
	}
}

func convertMessage(v any, schema MessageSchema) (*Message, error) {
	mm, ok := v.(*Message)
	if !ok {
		return nil, ErrTypeMismatch
	}
	if mm == nil {
		return nil, ErrTypeMismatch
	}
	if !schemasEqual(schema, mm.schema) {
		return nil, ErrTypeMismatch
	}
	return mm.deepCopy(), nil
}

func convertToSlice(fs FieldSchema, v any) (any, error) {
	if fs.Type == FieldMessage {
		return convertMessageSlice(v, fs.Schema)
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil, ErrTypeMismatch
	}
	n := rv.Len()
	out := makeSlice(fs.Type, n)
	for i := 0; i < n; i++ {
		elem, err := convertScalar(fs.Type, rv.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		setSliceElem(out, i, elem)
	}
	return out, nil
}

func convertMessageSlice(v any, schema MessageSchema) ([]*Message, error) {
	if msgs, ok := v.([]*Message); ok {
		out := make([]*Message, len(msgs))
		for i, mm := range msgs {
			if mm == nil {
				out[i] = nil
				continue
			}
			if !schemasEqual(schema, mm.schema) {
				return nil, ErrTypeMismatch
			}
			out[i] = mm.deepCopy()
		}
		return out, nil
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil, ErrTypeMismatch
	}
	n := rv.Len()
	out := make([]*Message, n)
	for i := 0; i < n; i++ {
		ev := rv.Index(i)
		if ev.Kind() == reflect.Interface {
			if ev.IsNil() {
				out[i] = nil
				continue
			}
			mm, ok := ev.Interface().(*Message)
			if !ok {
				return nil, ErrTypeMismatch
			}
			if !schemasEqual(schema, mm.schema) {
				return nil, ErrTypeMismatch
			}
			out[i] = mm.deepCopy()
			continue
		}
		if ev.Kind() != reflect.Ptr {
			return nil, ErrTypeMismatch
		}
		if ev.IsNil() {
			out[i] = nil
			continue
		}
		mm, ok := ev.Interface().(*Message)
		if !ok {
			return nil, ErrTypeMismatch
		}
		if !schemasEqual(schema, mm.schema) {
			return nil, ErrTypeMismatch
		}
		out[i] = mm.deepCopy()
	}
	return out, nil
}

func intRange(bits int) (int64, int64) {
	if bits == 32 {
		return math.MinInt32, math.MaxInt32
	}
	return math.MinInt64, math.MaxInt64
}

func checkInt(n, lo, hi int64) (int64, error) {
	if n < lo || n > hi {
		return 0, ErrTypeMismatch
	}
	return n, nil
}

func checkUintToInt(u uint64, hi int64) (int64, error) {
	if u > uint64(hi) {
		return 0, ErrTypeMismatch
	}
	return int64(u), nil
}

func checkIntToUint(n int64, hi uint64) (uint64, error) {
	if n < 0 {
		return 0, ErrTypeMismatch
	}
	u := uint64(n)
	if u > hi {
		return 0, ErrTypeMismatch
	}
	return u, nil
}

func checkUintRange(u, hi uint64) (uint64, error) {
	if u > hi {
		return 0, ErrTypeMismatch
	}
	return u, nil
}

func checkFloatToInt(f float64, bits int) (int64, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrTypeMismatch
	}
	t := math.Trunc(f)
	if bits == 32 {
		if t < -2147483648.0 || t > 2147483647.0 {
			return 0, ErrTypeMismatch
		}
		return int64(t), nil
	}
	if t < -9223372036854775808.0 || t >= 9223372036854775808.0 {
		return 0, ErrTypeMismatch
	}
	return int64(t), nil
}

func checkFloatToUint(f float64, bits int) (uint64, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrTypeMismatch
	}
	t := math.Trunc(f)
	if t < 0 {
		return 0, ErrTypeMismatch
	}
	if bits == 32 {
		if t > 4294967295.0 {
			return 0, ErrTypeMismatch
		}
		return uint64(t), nil
	}
	if t >= 18446744073709551616.0 {
		return 0, ErrTypeMismatch
	}
	return uint64(t), nil
}

func toFloat32(f float64) (float32, error) {
	f32 := float32(f)
	if math.IsInf(float64(f32), 0) && !math.IsInf(f, 0) {
		return 0, ErrTypeMismatch
	}
	return f32, nil
}
