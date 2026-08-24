package dymsg

func sliceLen(v any) int {
	switch s := v.(type) {
	case []int32:
		return len(s)
	case []int64:
		return len(s)
	case []uint32:
		return len(s)
	case []uint64:
		return len(s)
	case []float32:
		return len(s)
	case []float64:
		return len(s)
	case []bool:
		return len(s)
	case []string:
		return len(s)
	case [][]byte:
		return len(s)
	case []*Message:
		return len(s)
	}
	return 0
}

func sliceIndex(v any, i int) (any, bool) {
	switch s := v.(type) {
	case []int32:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []int64:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []uint32:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []uint64:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []float32:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []float64:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []bool:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []string:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case [][]byte:
		if i >= 0 && i < len(s) {
			return s[i], true
		}
	case []*Message:
		if i >= 0 && i < len(s) {
			if s[i] == nil {
				return nil, true
			}
			return s[i], true
		}
	}
	return nil, false
}

func makeSlice(ft FieldType, n int) any {
	switch ft {
	case FieldInt32:
		return make([]int32, n)
	case FieldInt64:
		return make([]int64, n)
	case FieldUint32:
		return make([]uint32, n)
	case FieldUint64:
		return make([]uint64, n)
	case FieldFloat:
		return make([]float32, n)
	case FieldDouble:
		return make([]float64, n)
	case FieldBool:
		return make([]bool, n)
	case FieldString:
		return make([]string, n)
	case FieldBytes:
		return make([][]byte, n)
	case FieldMessage:
		return make([]*Message, n)
	}
	return nil
}

func setSliceElem(slice any, i int, elem any) {
	switch s := slice.(type) {
	case []int32:
		s[i] = elem.(int32)
	case []int64:
		s[i] = elem.(int64)
	case []uint32:
		s[i] = elem.(uint32)
	case []uint64:
		s[i] = elem.(uint64)
	case []float32:
		s[i] = elem.(float32)
	case []float64:
		s[i] = elem.(float64)
	case []bool:
		s[i] = elem.(bool)
	case []string:
		s[i] = elem.(string)
	case [][]byte:
		s[i] = elem.([]byte)
	}
}

func appendToSlice(ft FieldType, slice any, elem any) any {
	switch s := slice.(type) {
	case []int32:
		return append(s, elem.(int32))
	case []int64:
		return append(s, elem.(int64))
	case []uint32:
		return append(s, elem.(uint32))
	case []uint64:
		return append(s, elem.(uint64))
	case []float32:
		return append(s, elem.(float32))
	case []float64:
		return append(s, elem.(float64))
	case []bool:
		return append(s, elem.(bool))
	case []string:
		return append(s, elem.(string))
	case [][]byte:
		return append(s, elem.([]byte))
	case []*Message:
		return append(s, elem.(*Message))
	}
	return slice
}
