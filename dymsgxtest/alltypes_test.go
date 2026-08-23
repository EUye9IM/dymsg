package dymsgxtest

import (
	"math"
	"testing"

	dymsg "dymsg"
)

func TestAllTypesSetGet(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "i64", int64(-9_000_000_000_000))
	mustSet(t, m, "u32", uint32(4000000000))
	mustSet(t, m, "u64", uint64(1<<63+7))
	mustSet(t, m, "f32", float32(1.5))
	mustSet(t, m, "f64", 2.25)
	mustSet(t, m, "by", []byte{9, 8})

	eq(t, getValue(t, m, "i64"), int64(-9_000_000_000_000))
	eq(t, getValue(t, m, "u32"), uint32(4000000000))
	eq(t, getValue(t, m, "u64"), uint64(1<<63+7))
	eq(t, getValue(t, m, "f32"), float32(1.5))
	eq(t, getValue(t, m, "f64"), 2.25)
}

func TestUintConversion(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "u64", 42)
	eq(t, getValue(t, m, "u64"), uint64(42))
	mustSet(t, m, "u64", "18446744073709551615")
	eq(t, getValue(t, m, "u64"), uint64(math.MaxUint64))
	if err := m.Set("u64", -1); err != dymsg.ErrTypeMismatch {
		t.Fatalf("negative->uint err = %v, want ErrTypeMismatch", err)
	}
	mustSet(t, m, "u64", 7.0)
	eq(t, getValue(t, m, "u64"), uint64(7))
	if err := m.Set("u64", 1.9e19); err != dymsg.ErrTypeMismatch {
		t.Fatalf("float overflow->uint err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i64", uint64(1<<63)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("u64 overflow->int64 err = %v, want ErrTypeMismatch", err)
	}
}

func TestFloatConversion(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "f64", 3)
	eq(t, getValue(t, m, "f64"), 3.0)
	mustSet(t, m, "f64", "2.5")
	eq(t, getValue(t, m, "f64"), 2.5)
	mustSet(t, m, "i64", 3.9)
	eq(t, getValue(t, m, "i64"), int64(3))
	if err := m.Set("f32", 3.5e38); err != dymsg.ErrTypeMismatch {
		t.Fatalf("f32 overflow err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i64", math.NaN()); err != dymsg.ErrTypeMismatch {
		t.Fatalf("NaN->int err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i64", math.Inf(1)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Inf->int err = %v, want ErrTypeMismatch", err)
	}
}

func TestToStringConversion(t *testing.T) {
	m := mustNewAllTypes(t)
	for _, tc := range []struct {
		in   any
		want string
	}{
		{42, "42"},
		{3.5, "3.5"},
		{[]byte("raw"), "raw"},
		{uint32(7), "7"},
		{uint64(8), "8"},
		{float32(2.5), "2.5"},
		{int8(-3), "-3"},
		{uint16(65535), "65535"},
	} {
		mustSet(t, m, "s", tc.in)
		eq(t, getValue(t, m, "s"), tc.want)
	}
	if err := m.Set("s", true); err != dymsg.ErrTypeMismatch {
		t.Fatalf("bool->string err = %v, want ErrTypeMismatch", err)
	}
}

func TestToBytesConversion(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "by", "str")
	eq(t, getValue(t, m, "by"), []byte("str"))
	mustSet(t, m, "by", []byte{1, 2})
	eq(t, getValue(t, m, "by"), []byte{1, 2})
	type myBytes []byte
	mustSet(t, m, "by", myBytes{1, 2, 3})
	eq(t, getValue(t, m, "by"), []byte{1, 2, 3})
}

func TestInt64ViaBytesString(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "i64", []byte("12345"))
	eq(t, getValue(t, m, "i64"), int64(12345))
}

func TestAllTypesJSONRoundTrip(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "i64", int64(-9223372036854775808))
	mustSet(t, m, "u64", uint64(18446744073709551615))
	mustSet(t, m, "u32", uint32(4000000000))
	mustSet(t, m, "f32", float32(1.25))
	mustSet(t, m, "f64", -3.75)
	mustSet(t, m, "by", []byte{0x01, 0xff})
	mustSet(t, m, "i64r", []int64{1, -2})
	mustSet(t, m, "u64r", []uint64{9, 1 << 40})
	mustSet(t, m, "f32r", []float32{1.5, -2.5})
	mustSet(t, m, "f64r", []float64{3.0, 4.5})
	mustSet(t, m, "byr", [][]byte{{1}, {2, 3}})
	mustSet(t, m, "br", []bool{true, false})

	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := mustNewAllTypes(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	eq(t, getValue(t, m2, "i64"), int64(-9223372036854775808))
	eq(t, getValue(t, m2, "u64"), uint64(18446744073709551615))
	eq(t, getValue(t, m2, "u32"), uint32(4000000000))
	eq(t, getValue(t, m2, "f32"), float32(1.25))
	eq(t, getValue(t, m2, "f64"), -3.75)
	eq(t, getValue(t, m2, "by"), []byte{0x01, 0xff})
	eq(t, getValue(t, m2, "i64r"), []int64{1, -2})
	eq(t, getValue(t, m2, "u64r"), []uint64{9, 1 << 40})
	eq(t, getValue(t, m2, "f32r"), []float32{1.5, -2.5})
	eq(t, getValue(t, m2, "f64r"), []float64{3.0, 4.5})
	eq(t, getValue(t, m2, "byr"), [][]byte{{1}, {2, 3}})
	eq(t, getValue(t, m2, "br"), []bool{true, false})
}

func TestAllTypesProtoRoundTrip(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "i32", int32(-2147483648))
	mustSet(t, m, "i64", int64(-9000000000000))
	mustSet(t, m, "u32", uint32(4000000000))
	mustSet(t, m, "u64", uint64(1<<63+7))
	mustSet(t, m, "f32", float32(1.25))
	mustSet(t, m, "f64", -3.75)
	mustSet(t, m, "by", []byte{0x01, 0xff})
	mustSet(t, m, "i64r", []int64{1, -2})
	mustSet(t, m, "u64r", []uint64{9, 1 << 40})
	mustSet(t, m, "f32r", []float32{1.5, -2.5})
	mustSet(t, m, "f64r", []float64{3.0, 4.5})
	mustSet(t, m, "byr", [][]byte{{1}, {2, 3}})
	mustSet(t, m, "br", []bool{true, false, true})

	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := mustNewAllTypes(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m2, "i32"), int32(-2147483648))
	eq(t, getValue(t, m2, "i64"), int64(-9000000000000))
	eq(t, getValue(t, m2, "u32"), uint32(4000000000))
	eq(t, getValue(t, m2, "u64"), uint64(1<<63+7))
	eq(t, getValue(t, m2, "f32"), float32(1.25))
	eq(t, getValue(t, m2, "f64"), -3.75)
	eq(t, getValue(t, m2, "by"), []byte{0x01, 0xff})
	eq(t, getValue(t, m2, "i64r"), []int64{1, -2})
	eq(t, getValue(t, m2, "u64r"), []uint64{9, 1 << 40})
	eq(t, getValue(t, m2, "f32r"), []float32{1.5, -2.5})
	eq(t, getValue(t, m2, "f64r"), []float64{3.0, 4.5})
	eq(t, getValue(t, m2, "byr"), [][]byte{{1}, {2, 3}})
	eq(t, getValue(t, m2, "br"), []bool{true, false, true})
}
