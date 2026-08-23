package dymsgxtest

import (
	"math"
	"testing"

	dymsg "dymsg"
)

// ---------- wire 层异常 ----------

func TestUnknownFieldSkipped(t *testing.T) {
	var blob []byte
	blob = appendVarint(blob, 99<<3|0)
	blob = appendVarint(blob, 12345)
	blob = appendTag(blob, 1, 2)
	blob = flen(blob, []byte("alice"))
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m, "name"), "alice")
}

func TestUnknownFieldSkipFixed(t *testing.T) {
	nameField := func() []byte {
		var b []byte
		b = appendTag(b, 1, 2)
		b = flen(b, []byte("alice"))
		return b
	}
	blob := appendVarint(nil, 200<<3|1) // fixed64 未知
	blob = append(blob, make([]byte, 8)...)
	blob = append(blob, nameField()...)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("fixed64 unknown: %v", err)
	}
	eq(t, getValue(t, m, "name"), "alice")

	blob = appendVarint(nil, 201<<3|5) // fixed32 未知
	blob = append(blob, make([]byte, 4)...)
	blob = append(blob, nameField()...)
	m = mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("fixed32 unknown: %v", err)
	}
	eq(t, getValue(t, m, "name"), "alice")
}

func TestUnknownFieldGroupWireType(t *testing.T) {
	for _, wt := range []int{3, 4} {
		blob := appendVarint(nil, 200<<3|uint64(wt))
		blob = append(blob, 1)
		m := mustNew(t)
		if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
			t.Fatalf("unknown wt=%d err = %v, want ErrMalformedData", wt, err)
		}
	}
}

func TestKnownFieldGroupWireType(t *testing.T) {
	for _, wt := range []int{3, 4} {
		blob := appendVarint(nil, 1<<3|uint64(wt))
		blob = append(blob, 1)
		m := mustNew(t)
		if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
			t.Fatalf("known wt=%d err = %v, want ErrMalformedData", wt, err)
		}
	}
}

func TestWireTypeMismatch(t *testing.T) {
	blob := appendTag(nil, 2, 2) // age(int32) 以 length-delimited
	blob = appendVarint(blob, 3)
	blob = append(blob, 0x01, 0x02, 0x03)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestIllegalWireType(t *testing.T) {
	blob := appendVarint(nil, 2<<3|6)
	blob = append(blob, 0)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestFieldNumZeroKey(t *testing.T) {
	blob := appendVarint(nil, 0<<3|0)
	blob = appendVarint(blob, 1)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestTruncatedProto(t *testing.T) {
	blob := appendTag(nil, 1, 2)
	blob = appendVarint(blob, 100)
	blob = append(blob, "hi"...)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestVarintOverflow(t *testing.T) {
	blob := appendVarint(nil, 1<<3|0)
	for i := 0; i < 10; i++ {
		blob = append(blob, 0xFF)
	}
	blob = append(blob, 0x02)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestUnpackedAccepted(t *testing.T) {
	var blob []byte
	for _, v := range []int64{5, 6, 7} {
		blob = appendTag(blob, 8, 0)
		blob = appendVarint(blob, uint64(v))
	}
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto unpacked: %v", err)
	}
	eq(t, getValue(t, m, "scores"), []int32{5, 6, 7})
}

func TestMixedPackedUnpacked(t *testing.T) {
	var blob []byte
	packed := appendVarint(nil, 1)
	packed = appendVarint(packed, 2)
	packed = appendVarint(packed, 3)
	blob = appendTag(blob, 8, 2)
	blob = flen(blob, packed)
	for _, v := range []int64{4, 5} {
		blob = appendTag(blob, 8, 0)
		blob = appendVarint(blob, uint64(v))
	}
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m, "scores"), []int32{1, 2, 3, 4, 5})
}

func TestProtoFieldTwice(t *testing.T) {
	var blob []byte
	blob = appendTag(blob, 1, 2)
	blob = flen(blob, []byte("a"))
	blob = appendTag(blob, 1, 2)
	blob = flen(blob, []byte("bob"))
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatal(err)
	}
	eq(t, getValue(t, m, "name"), "bob")
}

func TestEmptyNestedMessage(t *testing.T) {
	blob := appendTag(nil, 6, 2)
	blob = appendVarint(blob, 0)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto empty nested: %v", err)
	}
	if g, _ := m.Get("addr"); g == nil {
		t.Fatal("addr should be present (empty message)")
	}
}

// ---------- JSON 异常 ----------

func TestJSONNullIsUnset(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeJSON([]byte(`{"name":null}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be unset after null, got %#v", g)
	}
}

func TestJSONTopLevelShapes(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON([]byte(`[]`)); err != dymsg.ErrMalformedData {
		t.Fatalf("array top-level err = %v, want ErrMalformedData", err)
	}
	m2 := mustNew(t)
	mustSet(t, m2, "name", "alice")
	if err := m2.DecodeJSON([]byte(`null`)); err != nil {
		t.Fatalf("null top-level: %v", err)
	}
	if g, _ := m2.Get("name"); g != nil {
		t.Fatalf("name should be cleared by null top-level")
	}
}

func TestJSONStringForNumber(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON([]byte(`{"age":"18"}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("JSON string for int field err = %v, want ErrMalformedData", err)
	}
}

func TestJSONTrailingGarbage(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON([]byte(`{"name":"a"} xyz`)); err == nil {
		t.Fatalf("trailing garbage should error")
	}
}

func TestJSONRepeatedNullScalar(t *testing.T) {
	m := mustNewAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"i64r":[1,null]}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestJSONExponent(t *testing.T) {
	m := mustNewAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"f64":1e3}`)); err != nil {
		t.Fatalf("f64 1e3: %v", err)
	}
	eq(t, getValue(t, m, "f64"), 1000.0)
	if err := m.DecodeJSON([]byte(`{"i64":1e3}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("i64 1e3 err = %v, want ErrMalformedData", err)
	}
}

func TestDecodeJSONRepeatedMessage(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON([]byte(`{"contacts":[{"phone":"111"},{"phone":"222"}]}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	eq(t, getValue(t, m, "contacts[0].phone"), "111")
	eq(t, getValue(t, m, "contacts[1].phone"), "222")
	if err := m.DecodeJSON([]byte(`{"contacts":[null,{"phone":"3"}]}`)); err != nil {
		t.Fatalf("DecodeJSON null elem: %v", err)
	}
	eq(t, getValue(t, m, "contacts[1].phone"), "3")
}

func TestDecodeJSONUnknownKeyIgnored(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON([]byte(`{"unknown":1,"name":"a"}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	eq(t, getValue(t, m, "name"), "a")
}

func TestRepeatedMessageWithNil(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("contacts", []*dymsg.Message{nil, nil}); err != nil {
		t.Fatal(err)
	}
	data, _ := m.EncodeProto()
	if len(data) != 0 {
		t.Fatalf("nil elements should not be encoded, len=%d", len(data))
	}
	jb, _ := m.EncodeJSON()
	if string(jb) != `{"contacts":[null,null]}` {
		t.Fatalf("contacts json = %s", jb)
	}
}

// ---------- 数值边界 ----------

func TestAllTypesJSONDecodeEdges(t *testing.T) {
	m := mustNewAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"u32":4294967296}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("u32 overflow err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"i32":2147483648}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("i32 overflow err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"f32":3.5e38}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("f32 overflow err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"i64":"18"}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("string->int64 err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"i64":[1,2]}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("array->int64 err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"i64r":5}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("scalar->repeated err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"u64":18446744073709551615}`)); err != nil {
		t.Fatalf("u64 max: %v", err)
	}
	eq(t, getValue(t, m, "u64"), uint64(math.MaxUint64))
	if err := m.DecodeJSON([]byte(`{"u64":-1}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("negative u64 err = %v", err)
	}
	if err := m.DecodeJSON([]byte(`{"u64":1.5}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("float u64 err = %v", err)
	}
}

func TestFloatSpecialValues(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "f64", math.NaN())
	mustSet(t, m, "f32", float32(math.Inf(1)))
	data, _ := m.EncodeProto()
	m2 := mustNewAllTypes(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	if v := getValue(t, m2, "f64").(float64); !math.IsNaN(v) {
		t.Fatalf("NaN lost, got %v", v)
	}
	if v := getValue(t, m2, "f32").(float32); !math.IsInf(float64(v), 1) {
		t.Fatalf("+Inf lost, got %v", v)
	}
	m3 := mustNewAllTypes(t)
	mustSet(t, m3, "f64", math.NaN())
	if _, err := m3.EncodeJSON(); err == nil {
		t.Fatalf("EncodeJSON(NaN) should error")
	}
}

// ---------- 路径语义 ----------

func TestNegativeIndex(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "tags", []any{"a"})
	if _, err := m.Get("tags[-1]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("negative index err = %v, want ErrIndexOutOfRange", err)
	}
	if err := m.Set("tags[-1]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("negative Set err = %v, want ErrIndexOutOfRange", err)
	}
	if _, err := m.Get("tags[abc]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("non-numeric index err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestNonRepeatedWithIndex(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	if _, err := m.Get("name[0]"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Get name[0] err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("name[0]", "bob"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Set name[0] err = %v, want ErrFieldNotFound", err)
	}
	if _, err := m.Get("addr[0].city"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Get addr[0].city err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("addr[0].city", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Set addr[0].city err = %v, want ErrFieldNotFound", err)
	}
	eq(t, getValue(t, m, "name"), "alice")
}

func TestSchemaValidation(t *testing.T) {
	cases := []string{
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":0}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":70000}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"b","type":"int32","num":1}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a.b","type":"int32","num":1}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"nope","num":1}]}]}`,
		`{"types":[{"typeId":0,"fields":[]}]}`,
		`{"types":[{"typeId":1,"fields":[]},{"typeId":1,"fields":[]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"message","num":1}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1}]}]}`,
	}
	wantErr := []bool{true, true, true, true, true, true, true, true, false}
	for i, c := range cases {
		_, err := dymsg.ParseSchema([]byte(c))
		if (err != nil) != wantErr[i] {
			t.Fatalf("case %d: err = %v, wantErr = %v", i, err, wantErr[i])
		}
	}
}

// valueNode(标量包装)应拒绝 Set/Decode
func TestValueNodeRejections(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "x")
	g, _ := m.Get("name")
	if err := g.Set("", nil); err != dymsg.ErrFieldNotFound {
		t.Fatalf("valueNode Set err = %v, want ErrFieldNotFound", err)
	}
	if err := g.Set("any", 1); err != dymsg.ErrFieldNotFound {
		t.Fatalf("valueNode Set field err = %v, want ErrFieldNotFound", err)
	}
	if err := g.DecodeJSON([]byte(`{}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("valueNode DecodeJSON err = %v, want ErrMalformedData", err)
	}
	if err := g.DecodeProto([]byte{1}); err != dymsg.ErrMalformedData {
		t.Fatalf("valueNode DecodeProto err = %v, want ErrMalformedData", err)
	}
	if _, err := g.EncodeProto(); err != dymsg.ErrMalformedData {
		t.Fatalf("valueNode EncodeProto err = %v, want ErrMalformedData", err)
	}
}

func TestStructuredValueIsSelf(t *testing.T) {
	m := mustNew(t)
	if m.Value() != m {
		t.Fatalf("structured Value() should return self")
	}
	if err := m.Set("contacts", make([]*dymsg.Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m, "contacts[0].phone", "x")
	g, _ := m.Get("contacts[0]")
	if g.Value() != g {
		t.Fatalf("message element Value() should return self")
	}
}

func TestDeepCopyNilSlice(t *testing.T) {
	m1 := mustNew(t)
	var nilTags []string
	if err := m1.Set("tags", nilTags); err != nil {
		t.Fatal(err)
	}
	m2 := mustNew(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatal(err)
	}
	g, _ := m2.Get("tags")
	if g == nil {
		t.Fatal("tags should be present (empty slice)")
	}
	if lst := g.Value().([]string); lst == nil {
		t.Fatalf("copied tags should be non-nil empty slice")
	}
}

// repeated length-delimited 字段截断 -> ErrTruncated
func TestRepeatedLengthDelimitedTruncated(t *testing.T) {
	blob := appendTag(nil, 7, 2) // tags(num 7, repeated string)
	blob = appendVarint(blob, 100)
	blob = append(blob, "hi"...)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

// repeated message 子消息内容非法 -> ErrMalformedData
func TestRepeatedMessageBadNested(t *testing.T) {
	var inner []byte
	inner = appendVarint(inner, 1<<3|6) // phone(num 1) 非法 wire type 6
	inner = append(inner, 0)
	blob := appendTag(nil, 9, 2) // contacts(num 9, repeated message)
	blob = flen(blob, inner)
	m := mustNew(t)
	if err := m.DecodeProto(blob); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}
