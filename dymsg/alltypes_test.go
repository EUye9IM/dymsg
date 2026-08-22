package dymsg

import (
	"math"
	"sync"
	"testing"
)

const allTypesConfig = `{
  "types": [{
    "typeId": 1002,
    "fields": [
      {"name": "i32", "type": "int32", "num": 1},
      {"name": "i64", "type": "int64", "num": 2},
      {"name": "u32", "type": "uint32", "num": 3},
      {"name": "u64", "type": "uint64", "num": 4},
      {"name": "f32", "type": "float", "num": 5},
      {"name": "f64", "type": "double", "num": 6},
      {"name": "b", "type": "bool", "num": 7},
      {"name": "s", "type": "string", "num": 8},
      {"name": "by", "type": "bytes", "num": 9},
      {"name": "i64r", "type": "int64", "num": 10, "repeated": true},
      {"name": "u64r", "type": "uint64", "num": 11, "repeated": true},
      {"name": "f32r", "type": "float", "num": 12, "repeated": true},
      {"name": "f64r", "type": "double", "num": 13, "repeated": true},
      {"name": "byr", "type": "bytes", "num": 14, "repeated": true},
      {"name": "br", "type": "bool", "num": 15, "repeated": true}
    ]
  }]
}`

var allTypesMu sync.Mutex
var allTypesRegistered bool

func newAllTypes(t *testing.T) *Message {
	t.Helper()
	allTypesMu.Lock()
	defer allTypesMu.Unlock()
	if !allTypesRegistered {
		schemas, err := ParseSchema([]byte(allTypesConfig))
		if err != nil {
			t.Fatalf("ParseSchema: %v", err)
		}
		for _, s := range schemas {
			if err := Register(s); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}
		allTypesRegistered = true
	}
	m, err := New(1002)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestAllTypesSetGet(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "i64", int64(-9_000_000_000_000))
	mustSet(t, m, "u32", uint32(4000000000))
	mustSet(t, m, "u64", uint64(1<<63+7))
	mustSet(t, m, "f32", float32(1.5))
	mustSet(t, m, "f64", 2.25)
	mustSet(t, m, "by", []byte{9, 8})

	g, _ := m.Get("i64")
	eq(t, g.Value(), int64(-9_000_000_000_000))
	g, _ = m.Get("u32")
	eq(t, g.Value(), uint32(4000000000))
	g, _ = m.Get("u64")
	eq(t, g.Value(), uint64(1<<63+7))
	g, _ = m.Get("f32")
	eq(t, g.Value(), float32(1.5))
	g, _ = m.Get("f64")
	eq(t, g.Value(), 2.25)
}

func TestUintConversion(t *testing.T) {
	m := newAllTypes(t)
	// int -> uint
	mustSet(t, m, "u64", 42)
	g, _ := m.Get("u64")
	eq(t, g.Value(), uint64(42))
	// string -> uint
	mustSet(t, m, "u64", "18446744073709551615")
	g, _ = m.Get("u64")
	eq(t, g.Value(), uint64(math.MaxUint64))
	// 负数 -> uint 失败
	if err := m.Set("u64", -1); err != ErrTypeMismatch {
		t.Fatalf("negative->uint err = %v, want ErrTypeMismatch", err)
	}
	// float -> uint
	mustSet(t, m, "u64", 7.0)
	g, _ = m.Get("u64")
	eq(t, g.Value(), uint64(7))
	// float 越界 -> uint 失败
	if err := m.Set("u64", 1.9e19); err != ErrTypeMismatch {
		t.Fatalf("float overflow->uint err = %v, want ErrTypeMismatch", err)
	}
	// uint64 转 int64 溢出
	if err := m.Set("i64", uint64(1<<63)); err != ErrTypeMismatch {
		t.Fatalf("u64 overflow->int64 err = %v, want ErrTypeMismatch", err)
	}
}

func TestFloatConversion(t *testing.T) {
	m := newAllTypes(t)
	// int -> float
	mustSet(t, m, "f64", 3)
	g, _ := m.Get("f64")
	eq(t, g.Value(), 3.0)
	// string -> float
	mustSet(t, m, "f64", "2.5")
	g, _ = m.Get("f64")
	eq(t, g.Value(), 2.5)
	// float -> int(截断)
	mustSet(t, m, "i64", 3.9)
	g, _ = m.Get("i64")
	eq(t, g.Value(), int64(3))
	// float32 溢出
	if err := m.Set("f32", 3.5e38); err != ErrTypeMismatch {
		t.Fatalf("f32 overflow err = %v, want ErrTypeMismatch", err)
	}
	// float NaN/Inf 转 int 失败
	if err := m.Set("i64", math.NaN()); err != ErrTypeMismatch {
		t.Fatalf("NaN->int err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i64", math.Inf(1)); err != ErrTypeMismatch {
		t.Fatalf("Inf->int err = %v, want ErrTypeMismatch", err)
	}
}

func TestToStringConversion(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "s", 42)
	g, _ := m.Get("s")
	eq(t, g.Value(), "42")
	mustSet(t, m, "s", 3.5)
	g, _ = m.Get("s")
	eq(t, g.Value(), "3.5")
	mustSet(t, m, "s", []byte("raw"))
	g, _ = m.Get("s")
	eq(t, g.Value(), "raw")
	// bool -> string 不支持
	if err := m.Set("s", true); err != ErrTypeMismatch {
		t.Fatalf("bool->string err = %v, want ErrTypeMismatch", err)
	}
}

func TestToBytesConversion(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "by", "str")
	g, _ := m.Get("by")
	eq(t, g.Value(), []byte("str"))
	mustSet(t, m, "by", []byte{1, 2})
	g, _ = m.Get("by")
	eq(t, g.Value(), []byte{1, 2})
}

func TestInt64ViaBytesString(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "i64", []byte("12345"))
	g, _ := m.Get("i64")
	eq(t, g.Value(), int64(12345))
}

func TestAllTypesJSONRoundTrip(t *testing.T) {
	m := newAllTypes(t)
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

	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := newAllTypes(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	g, _ := m2.Get("i64")
	eq(t, g.Value(), int64(-9223372036854775808))
	g, _ = m2.Get("u64")
	eq(t, g.Value(), uint64(18446744073709551615))
	g, _ = m2.Get("u32")
	eq(t, g.Value(), uint32(4000000000))
	g, _ = m2.Get("f32")
	eq(t, g.Value(), float32(1.25))
	g, _ = m2.Get("f64")
	eq(t, g.Value(), -3.75)
	g, _ = m2.Get("by")
	eq(t, g.Value(), []byte{0x01, 0xff})
	g, _ = m2.Get("i64r")
	eq(t, g.Value(), []int64{1, -2})
	g, _ = m2.Get("u64r")
	eq(t, g.Value(), []uint64{9, 1 << 40})
	g, _ = m2.Get("f32r")
	eq(t, g.Value(), []float32{1.5, -2.5})
	g, _ = m2.Get("f64r")
	eq(t, g.Value(), []float64{3.0, 4.5})
	g, _ = m2.Get("byr")
	eq(t, g.Value(), [][]byte{{1}, {2, 3}})
}

func TestAllTypesProtoRoundTrip(t *testing.T) {
	m := newAllTypes(t)
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

	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := newAllTypes(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m2.Get("i32")
	eq(t, g.Value(), int32(-2147483648))
	g, _ = m2.Get("i64")
	eq(t, g.Value(), int64(-9000000000000))
	g, _ = m2.Get("u32")
	eq(t, g.Value(), uint32(4000000000))
	g, _ = m2.Get("u64")
	eq(t, g.Value(), uint64(1<<63+7))
	g, _ = m2.Get("f32")
	eq(t, g.Value(), float32(1.25))
	g, _ = m2.Get("f64")
	eq(t, g.Value(), -3.75)
	g, _ = m2.Get("by")
	eq(t, g.Value(), []byte{0x01, 0xff})
	g, _ = m2.Get("i64r")
	eq(t, g.Value(), []int64{1, -2})
	g, _ = m2.Get("u64r")
	eq(t, g.Value(), []uint64{9, 1 << 40})
	g, _ = m2.Get("f32r")
	eq(t, g.Value(), []float32{1.5, -2.5})
	g, _ = m2.Get("f64r")
	eq(t, g.Value(), []float64{3.0, 4.5})
	g, _ = m2.Get("byr")
	eq(t, g.Value(), [][]byte{{1}, {2, 3}})
}

// JSON 边界:类型不匹配 / 溢出 / 布尔 / 数值形态
func TestAllTypesJSONDecodeEdges(t *testing.T) {
	m := newAllTypes(t)
	// u32 溢出
	if err := m.DecodeJSON([]byte(`{"u32":4294967296}`)); err != ErrMalformedData {
		t.Fatalf("u32 overflow err = %v, want ErrMalformedData", err)
	}
	// i32 溢出
	if err := m.DecodeJSON([]byte(`{"i32":2147483648}`)); err != ErrMalformedData {
		t.Fatalf("i32 overflow err = %v, want ErrMalformedData", err)
	}
	// f32 溢出
	if err := m.DecodeJSON([]byte(`{"f32":3.5e38}`)); err != ErrMalformedData {
		t.Fatalf("f32 overflow err = %v, want ErrMalformedData", err)
	}
	// 字符串不能给数值
	if err := m.DecodeJSON([]byte(`{"i64":"18"}`)); err != ErrMalformedData {
		t.Fatalf("string->int64 err = %v, want ErrMalformedData", err)
	}
	// 数组不能给单值
	if err := m.DecodeJSON([]byte(`{"i64":[1,2]}`)); err != ErrMalformedData {
		t.Fatalf("array->int64 err = %v, want ErrMalformedData", err)
	}
	// repeated 单值不能给数组字段
	if err := m.DecodeJSON([]byte(`{"i64r":5}`)); err != ErrMalformedData {
		t.Fatalf("scalar->repeated err = %v, want ErrMalformedData", err)
	}
	// 合法 u64 大数
	if err := m.DecodeJSON([]byte(`{"u64":18446744073709551615}`)); err != nil {
		t.Fatalf("u64 max: %v", err)
	}
	g, _ := m.Get("u64")
	eq(t, g.Value(), uint64(math.MaxUint64))
}

// repeated message 含 nil 元素编解码
func TestRepeatedMessageWithNil(t *testing.T) {
	m := newMsg(t)
	if err := m.Set("contacts", []*Message{nil, nil}); err != nil {
		t.Fatal(err)
	}
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("nil elements should not be encoded, len=%d", len(data))
	}
	// JSON: nil -> null
	jb, _ := m.EncodeJSON()
	if string(jb) != `{"contacts":[null,null]}` {
		t.Fatalf("contacts json = %s, want {\"contacts\":[null,null]}", jb)
	}
}

// 空子消息(nil/空 payload)解码
func TestEmptyNestedMessage(t *testing.T) {
	// addr 字段带空 payload(长度 0 的 length-delimited)
	blob := appendVarint(nil, 6<<3|2)
	blob = appendVarint(blob, 0)
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto empty nested: %v", err)
	}
	// addr 存在但空
	g, _ := m.Get("addr")
	if g == nil {
		t.Fatal("addr should be present (empty message)")
	}
}

// valueNode(标量包装)的 EncodeProto 应报错(无字段号)
func TestValueNodeEncodeProto(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "x")
	g, _ := m.Get("name")
	if _, err := g.EncodeProto(); err != ErrMalformedData {
		t.Fatalf("valueNode EncodeProto err = %v, want ErrMalformedData", err)
	}
}
