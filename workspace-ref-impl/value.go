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

func (v Value) Exists() bool { return v.exists }
func (v Value) IsSet() bool  { return v.isSet }
func (v Value) Err() error   { return v.err }

func (v Value) Any() any {
	if !v.isSet {
		return nil
	}
	return v.val
}

func (v Value) String() string {
	if v.typ != FieldString || v.repeated || !v.isSet {
		return ""
	}
	s, _ := v.val.(string)
	return s
}

func (v Value) Int32() int32 {
	if v.typ != FieldInt32 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(int32)
	return n
}

func (v Value) Int64() int64 {
	if v.typ != FieldInt64 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(int64)
	return n
}

func (v Value) Uint32() uint32 {
	if v.typ != FieldUint32 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(uint32)
	return n
}

func (v Value) Uint64() uint64 {
	if v.typ != FieldUint64 || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(uint64)
	return n
}

func (v Value) Float32() float32 {
	if v.typ != FieldFloat || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(float32)
	return n
}

func (v Value) Float64() float64 {
	if v.typ != FieldDouble || v.repeated || !v.isSet {
		return 0
	}
	n, _ := v.val.(float64)
	return n
}

func (v Value) Bool() bool {
	if v.typ != FieldBool || v.repeated || !v.isSet {
		return false
	}
	b, _ := v.val.(bool)
	return b
}

func (v Value) Bytes() []byte {
	if v.typ != FieldBytes || v.repeated || !v.isSet {
		return nil
	}
	b, _ := v.val.([]byte)
	return b
}

func (v Value) Message() *Message {
	if v.typ != FieldMessage || v.repeated || !v.isSet {
		return nil
	}
	m, _ := v.val.(*Message)
	return m
}

func (v Value) Len() int {
	if v.err != nil || !v.isSet || !v.repeated {
		return 0
	}
	return sliceLen(v.val)
}

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

func (v Value) Strings() []string {
	if v.typ != FieldString || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]string)
	return s
}

func (v Value) Int32s() []int32 {
	if v.typ != FieldInt32 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]int32)
	return s
}

func (v Value) Int64s() []int64 {
	if v.typ != FieldInt64 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]int64)
	return s
}

func (v Value) Uint32s() []uint32 {
	if v.typ != FieldUint32 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]uint32)
	return s
}

func (v Value) Uint64s() []uint64 {
	if v.typ != FieldUint64 || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]uint64)
	return s
}

func (v Value) Float32s() []float32 {
	if v.typ != FieldFloat || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]float32)
	return s
}

func (v Value) Float64s() []float64 {
	if v.typ != FieldDouble || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]float64)
	return s
}

func (v Value) Bools() []bool {
	if v.typ != FieldBool || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([]bool)
	return s
}

func (v Value) BytesSlice() [][]byte {
	if v.typ != FieldBytes || !v.repeated || !v.isSet {
		return nil
	}
	s, _ := v.val.([][]byte)
	return s
}

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
