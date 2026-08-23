package dymsgxtest

import (
	"testing"

	dymsg "dymsg"
)

type namedInt int
type namedUint uint

// 转换函数各类型变体覆盖
func TestConversionMatrix(t *testing.T) {
	m := mustNewAllTypes(t)

	// toUint64 / convertToUint64 分支
	for _, v := range []any{
		uint8(1), uint16(2), uint32(3),
		int8(5), int16(6), int32(7), float32(8), []byte("9"),
		namedInt(10), namedUint(11),
	} {
		mustSet(t, m, "u64", v)
	}
	// uintptr 不作为业务数值支持
	if err := m.Set("u64", uintptr(4)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uintptr->u64 err = %v, want ErrTypeMismatch", err)
	}

	// toInt64 / convertToInt64 分支
	for _, v := range []any{
		uint8(1), uint16(2), uint32(3),
		int8(-1), int16(-2), float32(8), []byte("12"),
	} {
		mustSet(t, m, "i64", v)
	}
	if err := m.Set("i64", uintptr(4)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uintptr->i64 err = %v, want ErrTypeMismatch", err)
	}

	// toFloat64 / convertToFloat64 分支
	for _, v := range []any{
		uint8(1), uint16(2), uint32(3), uint64(4),
		int8(6), int16(7), float32(8), []byte("9.5"),
	} {
		mustSet(t, m, "f64", v)
	}
	if err := m.Set("f64", uintptr(4)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uintptr->f64 err = %v, want ErrTypeMismatch", err)
	}

	// toString 分支
	for _, v := range []any{
		uint8(1), uint16(2), int8(-3), int16(-4), namedInt(6),
	} {
		mustSet(t, m, "s", v)
	}
	if err := m.Set("s", uintptr(4)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uintptr->string err = %v, want ErrTypeMismatch", err)
	}

	// 校验最终值正确(末尾值)
	eq(t, getValue(t, m, "u64"), uint64(11))
	eq(t, getValue(t, m, "i64"), int64(12))
	eq(t, getValue(t, m, "f64"), 9.5)
	eq(t, getValue(t, m, "s"), "6")
}

// 未知字段 length-delimited 跳过
func TestUnknownFieldLengthDelimited(t *testing.T) {
	var blob []byte
	blob = appendVarint(blob, 200<<3|2)
	blob = appendVarint(blob, 5)
	blob = append(blob, "hello"...)
	blob = appendTag(blob, 1, 2)
	blob = flen(blob, []byte("alice"))
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m, "name"), "alice")
}

// varint 第 10 字节 > 1 溢出
func TestVarint10thByteOverflow(t *testing.T) {
	var blob []byte
	blob = appendTag(blob, 2, 0) // age(int32)
	for i := 0; i < 9; i++ {
		blob = append(blob, 0xFF)
	}
	blob = append(blob, 0x02) // 第 10 字节 = 2 > 1
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

// Set 标量 repeated 元素为 nil -> 置零值
func TestSetScalarElemNil(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "scores", []int32{1, 2, 3})
	mustSet(t, m, "scores[1]", nil)
	eq(t, getValue(t, m, "scores"), []int32{1, 0, 3})
}

// Set repeated message 元素为不同 schema -> ErrTypeMismatch
func TestSetRepeatedMessageElemWrongSchema(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("contacts", make([]*dymsg.Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m, "addr.city", "x")
	addr, _ := m.Get("addr")
	if err := m.Set("contacts[0]", addr); err != dymsg.ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
}

// Set repeated 整体为不同 schema -> ErrTypeMismatch
func TestSetRepeatedWrongSchema(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("contacts", []*dymsg.Message{m}); err != dymsg.ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
}

// float/double 定长字段截断 -> ErrTruncated
func TestDecodeScalarTruncatedFixed(t *testing.T) {
	// score(num 4, double, fixed64) 长度不足
	blob := appendTag(nil, 4, 1)
	blob = append(blob, 1, 2, 3)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrTruncated {
		t.Fatalf("double truncated err = %v, want ErrTruncated", err)
	}
	// f32 无 fixed32 字段于 testConfig;用 alltypes 的 f32(num 5)
	ma := mustNewAllTypes(t)
	blob = appendTag(nil, 5, 5)
	blob = append(blob, 1, 2)
	if err := ma.DecodeProto(blob); err != dymsg.ErrTruncated {
		t.Fatalf("float truncated err = %v, want ErrTruncated", err)
	}
}

// toUint32 各类型 + 溢出
func TestToUint32Edges(t *testing.T) {
	m := mustNewAllTypes(t)
	for _, v := range []any{int32(1), uint64(2), float32(3), "4"} {
		mustSet(t, m, "u32", v)
	}
	eq(t, getValue(t, m, "u32"), uint32(4))
	if err := m.Set("u32", uint64(1<<32)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("u32 overflow err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("u32", -1); err != dymsg.ErrTypeMismatch {
		t.Fatalf("u32 negative err = %v, want ErrTypeMismatch", err)
	}
}

// float32 溢出(Set 路径)
func TestToFloat32Overflow(t *testing.T) {
	m := mustNewAllTypes(t)
	if err := m.Set("f32", 3.5e38); err != dymsg.ErrTypeMismatch {
		t.Fatalf("f32 overflow err = %v, want ErrTypeMismatch", err)
	}
	// 有限但接近上限的 float32 应接受
	if err := m.Set("f32", 1.5e38); err != nil {
		t.Fatalf("finite float32 should be accepted: %v", err)
	}
}
