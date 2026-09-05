package dymsg

// Value is the result of a field read. It carries the value together with
// presence and error information so callers can distinguish "not found",
// "not set" and "set (including zero value)".
type Value struct {
	typ      FieldType
	repeated bool
	exists   bool
	isSet    bool
	err      error
	val      any
}

// Exists reports whether the path resolved to an existing field or element.
func (v Value) Exists() bool { return v.exists }

// IsSet reports whether the field has been set, including with a zero value.
func (v Value) IsSet() bool { return v.isSet }

// Err returns the path resolution or lookup error; nil on success.
func (v Value) Err() error { return v.err }

// Any returns the native Go value, or nil when the field is unset or absent.
func (v Value) Any() any {
	if !v.isSet {
		return nil
	}
	return v.val
}

// String returns the string value, or "" when the field is not a set string.
func (v Value) String() string {
	if v.typ != FieldString || v.repeated || !v.isSet {
		return ""
	}
	s, _ := v.val.(string)
	return s
}

// Int32 returns the int32 value, or 0 when the field is not a set int32.
func (v Value) Int32() int32 {
	if v.typ != FieldInt32 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(int32)
	return n
}

// Int64 returns the int64 value, or 0 when the field is not a set int64.
func (v Value) Int64() int64 {
	if v.typ != FieldInt64 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(int64)
	return n
}

// Uint32 returns the uint32 value, or 0 when the field is not a set uint32.
func (v Value) Uint32() uint32 {
	if v.typ != FieldUint32 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(uint32)
	return n
}

// Uint64 returns the uint64 value, or 0 when the field is not a set uint64.
func (v Value) Uint64() uint64 {
	if v.typ != FieldUint64 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(uint64)
	return n
}

// Float32 returns the float value, or 0 when the field is not a set float.
func (v Value) Float32() float32 {
	if v.typ != FieldFloat || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(float32)
	return n
}

// Float64 returns the double value, or 0 when the field is not a set double.
func (v Value) Float64() float64 {
	if v.typ != FieldDouble || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(float64)
	return n
}

// Bool returns the bool value, or false when the field is not a set bool.
func (v Value) Bool() bool {
	if v.typ != FieldBool || v.repeated || !v.isSet {
		return false
	}
	b, _ := v.val.(bool)
	return b
}

// Bytes returns the bytes value, or nil when the field is not a set bytes.
func (v Value) Bytes() []byte {
	if v.typ != FieldBytes || v.repeated || !v.isSet {
		return nil
	}
	b, _ := v.val.([]byte)
	return b
}

// Message returns the nested message pointer, or nil unless the field is a set
// message.
func (v Value) Message() *Message {
	if v.typ != FieldMessage || v.repeated || !v.isSet {
		return nil
	}
	m, _ := v.val.(*Message)
	return m
}

// Len returns the length of a set repeated field; any other state yields 0.
func (v Value) Len() int {
	if v.err != nil || !v.isSet || !v.repeated {
		return 0
	}
	return sliceLen(v.val)
}

// Index returns the i-th element of a repeated field as a Value; an
// out-of-range or non-repeated receiver yields ErrIndexOutOfRange.
func (v Value) Index(i int) Value {
	if v.err != nil || !v.isSet || !v.repeated {
		return Value{err: ErrIndexOutOfRange}
	}
	if i < 0 || i >= sliceLen(v.val) {
		return Value{err: ErrIndexOutOfRange}
	}
	elem, _ := sliceIndex(v.val, i)
	return elementValue(v.typ, elem)
}

// Strings returns the whole repeated string array, or nil when unset or absent.
func (v Value) Strings() []string {
	if v.typ != FieldString || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]string)
	return s
}

// Int32s returns the whole repeated int32 array, or nil when unset or absent.
func (v Value) Int32s() []int32 {
	if v.typ != FieldInt32 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]int32)
	return s
}

// Int64s returns the whole repeated int64 array, or nil when unset or absent.
func (v Value) Int64s() []int64 {
	if v.typ != FieldInt64 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]int64)
	return s
}

// Uint32s returns the whole repeated uint32 array, or nil when unset or absent.
func (v Value) Uint32s() []uint32 {
	if v.typ != FieldUint32 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]uint32)
	return s
}

// Uint64s returns the whole repeated uint64 array, or nil when unset or absent.
func (v Value) Uint64s() []uint64 {
	if v.typ != FieldUint64 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]uint64)
	return s
}

// Float32s returns the whole repeated float array, or nil when unset or absent.
func (v Value) Float32s() []float32 {
	if v.typ != FieldFloat || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]float32)
	return s
}

// Float64s returns the whole repeated double array, or nil when unset or
// absent.
func (v Value) Float64s() []float64 {
	if v.typ != FieldDouble || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]float64)
	return s
}

// Bools returns the whole repeated bool array, or nil when unset or absent.
func (v Value) Bools() []bool {
	if v.typ != FieldBool || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]bool)
	return s
}

// BytesSlice returns the whole repeated bytes array, or nil when unset or
// absent.
func (v Value) BytesSlice() [][]byte {
	if v.typ != FieldBytes || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([][]byte)
	return s
}

// Messages returns the whole repeated message array, or nil when unset or
// absent.
func (v Value) Messages() []*Message {
	if v.typ != FieldMessage || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]*Message)
	return s
}

func elementValue(ft FieldType, elem any) Value {
	if ft == FieldMessage {
		if elem == nil {
			return Value{typ: FieldMessage, exists: true, isSet: true}
		}
		return Value{typ: FieldMessage, exists: true, isSet: true, val: elem}
	}
	return Value{typ: ft, exists: true, isSet: true, val: elem}
}
