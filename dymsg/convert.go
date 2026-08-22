package dymsg

import (
	"math"
	"reflect"
	"strconv"
)

const (
	maxInt32  = int64(1<<31 - 1)
	minInt32  = int64(-1 << 31)
	maxUint32 = uint64(1<<32 - 1)
	maxInt64  = int64(1<<63 - 1)
	minInt64  = int64(-1 << 63)
)

var maxUint64 = ^uint64(0)

const (
	maxInt64AsFloat  = 9223372036854775808.0 // 2^63
	maxUint64AsFloat = 18446744073709551616.0
)

func convertScalar(ft FieldType, v any) (any, error) {
	switch ft {
	case FieldInt32:
		return toInt32(v)
	case FieldInt64:
		return toInt64(v)
	case FieldUint32:
		return toUint32(v)
	case FieldUint64:
		return toUint64(v)
	case FieldFloat:
		return toFloat32(v)
	case FieldDouble:
		return toFloat64(v)
	case FieldBool:
		return toBool(v)
	case FieldString:
		return toString(v)
	case FieldBytes:
		return toBytes(v)
	}
	return nil, ErrTypeMismatch
}

func toInt32(v any) (int32, error) {
	if x, ok := v.(int32); ok {
		return x, nil
	}
	i, ok := convertToInt64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	if i < minInt32 || i > maxInt32 {
		return 0, ErrTypeMismatch
	}
	return int32(i), nil
}

func toInt64(v any) (int64, error) {
	if x, ok := v.(int64); ok {
		return x, nil
	}
	i, ok := convertToInt64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	return i, nil
}

func toUint32(v any) (uint32, error) {
	if x, ok := v.(uint32); ok {
		return x, nil
	}
	u, ok := convertToUint64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	if u > maxUint32 {
		return 0, ErrTypeMismatch
	}
	return uint32(u), nil
}

func toUint64(v any) (uint64, error) {
	if x, ok := v.(uint64); ok {
		return x, nil
	}
	u, ok := convertToUint64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	return u, nil
}

func toFloat32(v any) (float32, error) {
	if x, ok := v.(float32); ok {
		return x, nil
	}
	f, ok := convertToFloat64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	if !math.IsNaN(f) && !math.IsInf(f, 0) && (f > math.MaxFloat32 || f < -math.MaxFloat32) {
		return 0, ErrTypeMismatch
	}
	return float32(f), nil
}

func toFloat64(v any) (float64, error) {
	if x, ok := v.(float64); ok {
		return x, nil
	}
	f, ok := convertToFloat64(v)
	if !ok {
		return 0, ErrTypeMismatch
	}
	return f, nil
}

func toBool(v any) (bool, error) {
	if x, ok := v.(bool); ok {
		return x, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Bool {
		return rv.Bool(), nil
	}
	return false, ErrTypeMismatch
}

func toString(v any) (string, error) {
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
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64), nil
	}
	return "", ErrTypeMismatch
}

func toBytes(v any) ([]byte, error) {
	switch x := v.(type) {
	case []byte:
		return append([]byte(nil), x...), nil
	case string:
		return []byte(x), nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		b := make([]byte, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			b[i] = byte(rv.Index(i).Uint())
		}
		return b, nil
	}
	return nil, ErrTypeMismatch
}

func convertToInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		if uint64(x) > uint64(maxInt64) {
			return 0, false
		}
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > uint64(maxInt64) {
			return 0, false
		}
		return int64(x), true
	case float32:
		return floatToInt64(float64(x))
	case float64:
		return floatToInt64(x)
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case []byte:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := rv.Uint()
		if u > uint64(maxInt64) {
			return 0, false
		}
		return int64(u), true
	case reflect.Float32, reflect.Float64:
		return floatToInt64(rv.Float())
	case reflect.String:
		n, err := strconv.ParseInt(rv.String(), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func convertToUint64(v any) (uint64, bool) {
	switch x := v.(type) {
	case int:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int8:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int16:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int32:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case uint:
		return uint64(x), true
	case uint8:
		return uint64(x), true
	case uint16:
		return uint64(x), true
	case uint32:
		return uint64(x), true
	case uint64:
		return x, true
	case float32:
		return floatToUint64(float64(x))
	case float64:
		return floatToUint64(x)
	case string:
		n, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case []byte:
		n, err := strconv.ParseUint(string(x), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rv.Int()
		if i < 0 {
			return 0, false
		}
		return uint64(i), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint(), true
	case reflect.Float32, reflect.Float64:
		return floatToUint64(rv.Float())
	case reflect.String:
		n, err := strconv.ParseUint(rv.String(), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func convertToFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case []byte:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	case reflect.String:
		f, err := strconv.ParseFloat(rv.String(), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func floatToInt64(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f >= maxInt64AsFloat || f < -maxInt64AsFloat {
		return 0, false
	}
	return int64(f), true
}

func floatToUint64(f float64) (uint64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f < 0 || f >= maxUint64AsFloat {
		return 0, false
	}
	return uint64(f), true
}

func convertRepeatedScalar(ft FieldType, value any) (any, error) {
	var elems []any
	if a, ok := value.([]any); ok {
		elems = a
	} else {
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Slice {
			return nil, ErrTypeMismatch
		}
		elems = make([]any, rv.Len())
		for i := range elems {
			elems[i] = rv.Index(i).Interface()
		}
	}

	switch ft {
	case FieldInt32:
		d := make([]int32, len(elems))
		for i, e := range elems {
			c, err := toInt32(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldInt64:
		d := make([]int64, len(elems))
		for i, e := range elems {
			c, err := toInt64(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldUint32:
		d := make([]uint32, len(elems))
		for i, e := range elems {
			c, err := toUint32(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldUint64:
		d := make([]uint64, len(elems))
		for i, e := range elems {
			c, err := toUint64(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldFloat:
		d := make([]float32, len(elems))
		for i, e := range elems {
			c, err := toFloat32(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldDouble:
		d := make([]float64, len(elems))
		for i, e := range elems {
			c, err := toFloat64(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldBool:
		d := make([]bool, len(elems))
		for i, e := range elems {
			c, err := toBool(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldString:
		d := make([]string, len(elems))
		for i, e := range elems {
			c, err := toString(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	case FieldBytes:
		d := make([][]byte, len(elems))
		for i, e := range elems {
			c, err := toBytes(e)
			if err != nil {
				return nil, err
			}
			d[i] = c
		}
		return d, nil
	}
	return nil, ErrTypeMismatch
}
