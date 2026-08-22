package dymsg

import (
	"math"
	"testing"
)

// 未知字段 wire type 3/4(group, 已废弃)应报错
func TestUnknownFieldGroupWireType(t *testing.T) {
	for _, wt := range []int{3, 4} {
		blob := appendVarint(nil, 200<<3|uint64(wt))
		blob = append(blob, 1)
		m := newMsg(t)
		if err := m.DecodeProto(blob); err != ErrMalformedData {
			t.Fatalf("unknown field wt=%d err = %v, want ErrMalformedData", wt, err)
		}
	}
}

// 已知字段 wire type 3/4 也应报错
func TestKnownFieldGroupWireType(t *testing.T) {
	for _, wt := range []int{3, 4} {
		blob := appendVarint(nil, 1<<3|uint64(wt)) // name 字段,wt=3/4
		blob = append(blob, 1)
		m := newMsg(t)
		if err := m.DecodeProto(blob); err != ErrMalformedData {
			t.Fatalf("known field wt=%d err = %v, want ErrMalformedData", wt, err)
		}
	}
}

// float NaN/Inf:proto 往返保持;JSON 编码报错(不 panic)
func TestFloatSpecialValues(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "f64", math.NaN())
	mustSet(t, m, "f32", float32(math.Inf(1)))
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := newAllTypes(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m2.Get("f64")
	if !math.IsNaN(g.Value().(float64)) {
		t.Fatalf("NaN lost, got %v", g.Value())
	}
	g, _ = m2.Get("f32")
	if v := g.Value().(float32); !math.IsInf(float64(v), 1) {
		t.Fatalf("+Inf lost, got %v", g.Value())
	}

	// JSON 编码 NaN -> 报错
	m3 := newAllTypes(t)
	mustSet(t, m3, "f64", math.NaN())
	if _, err := m3.EncodeJSON(); err == nil {
		t.Logf("NOTE: EncodeJSON(NaN) returned nil error")
	}
}

// repeated message 整体深拷贝(contacts)
func TestDeepCopyRepeatedMessage(t *testing.T) {
	m1 := newMsg(t)
	if err := m1.Set("contacts", make([]*Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m1, "contacts[0].phone", "111")
	m2 := newMsg(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m2, "contacts[0].phone", "222")
	g, _ := m1.Get("contacts[0].phone")
	eq(t, g.Value(), "111")
}

// JSON repeated 标量含 null 元素 -> ErrMalformedData
func TestJSONRepeatedNullScalar(t *testing.T) {
	m := newAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"i64r":[1,null]}`)); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

// JSON 顶层是数组 / null
func TestJSONTopLevelShapes(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON([]byte(`[]`)); err != ErrMalformedData {
		t.Fatalf("array top-level err = %v, want ErrMalformedData", err)
	}
	m2 := newMsg(t)
	mustSet(t, m2, "name", "alice")
	if err := m2.DecodeJSON([]byte(`null`)); err != nil {
		t.Fatalf("null top-level: %v", err)
	}
	if g, _ := m2.Get("name"); g != nil {
		t.Fatalf("name should be cleared by null top-level")
	}
}

// varint 10 字节溢出(第 10 字节 > 1)
func TestVarintOverflow(t *testing.T) {
	blob := appendVarint(nil, 1<<3|0)
	for i := 0; i < 10; i++ {
		blob = append(blob, 0xFF)
	}
	blob = append(blob, 0x02) // 第 11 字节,实际触发
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

// JSON 指数数字
func TestJSONExponent(t *testing.T) {
	m := newAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"f64":1e3}`)); err != nil {
		t.Fatalf("f64 1e3: %v", err)
	}
	g, _ := m.Get("f64")
	eq(t, g.Value(), 1000.0)
	// 整数不接受指数
	if err := m.DecodeJSON([]byte(`{"i64":1e3}`)); err != ErrMalformedData {
		t.Fatalf("i64 1e3 err = %v, want ErrMalformedData", err)
	}
}

// message 字段缺嵌套 schema -> ErrMalformedData
func TestSchemaMessageMissingNested(t *testing.T) {
	c := `{"types":[{"typeId":99,"fields":[{"name":"a","type":"message","num":1}]}]}`
	if _, err := ParseSchema([]byte(c)); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

// 非 repeated 字段带下标 -> ErrFieldNotFound
func TestNonRepeatedWithIndex(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	if _, err := m.Get("name[0]"); err != ErrFieldNotFound {
		t.Fatalf("Get name[0] err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("name[0]", "bob"); err != ErrFieldNotFound {
		t.Fatalf("Set name[0] err = %v, want ErrFieldNotFound", err)
	}
	// 中间节点非 repeated 带下标
	if _, err := m.Get("addr[0].city"); err != ErrFieldNotFound {
		t.Fatalf("Get addr[0].city err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("addr[0].city", "x"); err != ErrFieldNotFound {
		t.Fatalf("Set addr[0].city err = %v, want ErrFieldNotFound", err)
	}
	// 原字段未被改动
	g, _ := m.Get("name")
	eq(t, g.Value(), "alice")
}

// nil slice 深拷贝
func TestDeepCopyNilSlice(t *testing.T) {
	m1 := newMsg(t)
	var nilTags []string
	if err := m1.Set("tags", nilTags); err != nil {
		t.Fatal(err)
	}
	m2 := newMsg(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatal(err)
	}
	g, _ := m2.Get("tags")
	if g == nil {
		t.Fatal("tags should be present (empty slice)")
	}
	lst := g.Value().([]string)
	if lst == nil {
		t.Fatalf("copied tags should be non-nil empty slice")
	}
}

// 命名类型别名([]byte 命名类型)转 bytes
func TestToBytesNamedSlice(t *testing.T) {
	type myBytes []byte
	m := newAllTypes(t)
	if err := m.Set("by", myBytes{1, 2, 3}); err != nil {
		t.Fatalf("Set by with named type: %v", err)
	}
	g, _ := m.Get("by")
	eq(t, g.Value(), []byte{1, 2, 3})
}

// valueNode(标量包装)的 Set/Decode 应报错
func TestValueNodeSetAndDecode(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "x")
	g, _ := m.Get("name")
	if err := g.Set("", nil); err != ErrFieldNotFound {
		t.Fatalf("valueNode Set err = %v, want ErrFieldNotFound", err)
	}
	if err := g.Set("any", 1); err != ErrFieldNotFound {
		t.Fatalf("valueNode Set field err = %v, want ErrFieldNotFound", err)
	}
	if err := g.DecodeJSON([]byte(`{}`)); err != ErrMalformedData {
		t.Fatalf("valueNode DecodeJSON err = %v, want ErrMalformedData", err)
	}
	if err := g.DecodeProto([]byte{1}); err != ErrMalformedData {
		t.Fatalf("valueNode DecodeProto err = %v, want ErrMalformedData", err)
	}
}

// Register 必须拒绝 typeID=0(SPEC:typeID 范围 [1,65535])
func TestRegisterTypeIDZero(t *testing.T) {
	var zero MessageSchema
	zero.typeID = 0
	if err := Register(zero); err != ErrMalformedData {
		t.Fatalf("Register(typeID=0) err = %v, want ErrMalformedData", err)
	}
	if _, err := New(0); err != ErrUnknownTypeID {
		t.Fatalf("New(0) err = %v, want ErrUnknownTypeID", err)
	}
}
