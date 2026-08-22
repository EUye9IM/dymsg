package dymsg

import "testing"

// 未知字段的 fixed32/fixed64 跳过
func TestUnknownFieldSkipFixed(t *testing.T) {
	nameField := func() []byte {
		var b []byte
		b = appendVarint(b, 1<<3|2)
		b = appendVarint(b, 5)
		b = append(b, "alice"...)
		return b
	}
	// 未知字段 wt=1(fixed64)
	blob := appendVarint(nil, 200<<3|1)
	blob = append(blob, make([]byte, 8)...)
	blob = append(blob, nameField()...)
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto fixed64 unknown: %v", err)
	}
	g, _ := m.Get("name")
	eq(t, g.Value(), "alice")

	// 未知字段 wt=5(fixed32)
	blob = appendVarint(nil, 201<<3|5)
	blob = append(blob, make([]byte, 4)...)
	blob = append(blob, nameField()...)
	m = newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto fixed32 unknown: %v", err)
	}
	g, _ = m.Get("name")
	eq(t, g.Value(), "alice")
}

// repeated message 元素整体赋值 / nil 元素
func TestSetRepeatedMessageElement(t *testing.T) {
	m := newMsg(t)
	if err := m.Set("contacts", make([]*Message, 2)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m, "contacts[0].phone", "111")
	mustSet(t, m, "contacts[1].phone", "222")
	g, _ := m.Get("contacts[0].phone")
	eq(t, g.Value(), "111")
}

// repeated message 单元素整体赋值(经另一消息的相同子 schema)
func TestSetRepeatedMessageElemFromOther(t *testing.T) {
	// 源:在另一消息中构造 contacts[0]
	m2 := newMsg(t)
	if err := m2.Set("contacts", make([]*Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m2, "contacts[0].phone", "999")
	src, _ := m2.Get("contacts[0]")

	dst := newMsg(t)
	if err := dst.Set("contacts", make([]*Message, 1)); err != nil {
		t.Fatal(err)
	}
	if err := dst.Set("contacts[0]", src); err != nil {
		t.Fatalf("Set elem: %v", err)
	}
	g, _ := dst.Get("contacts[0].phone")
	eq(t, g.Value(), "999")

	// nil 元素
	if err := dst.Set("contacts[0]", nil); err != nil {
		t.Fatalf("Set elem nil: %v", err)
	}
	if g, _ := dst.Get("contacts[0]"); g != nil {
		t.Fatalf("contacts[0] should be nil after Set nil, got %#v", g)
	}
}

// JSON 解码 repeated message
func TestDecodeJSONRepeatedMessage(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON([]byte(`{"contacts":[{"phone":"111"},{"phone":"222"}]}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	g, _ := m.Get("contacts[0].phone")
	eq(t, g.Value(), "111")
	g, _ = m.Get("contacts[1].phone")
	eq(t, g.Value(), "222")
	// 混合 null 元素
	if err := m.DecodeJSON([]byte(`{"contacts":[null,{"phone":"3"}]}`)); err != nil {
		t.Fatalf("DecodeJSON null elem: %v", err)
	}
	g, _ = m.Get("contacts[1].phone")
	eq(t, g.Value(), "3")
}

// 复合消息 Value() 返回自身
func TestValueOnStructuredMessage(t *testing.T) {
	m := newMsg(t)
	if m.Value() != m {
		t.Fatalf("structured Value() should return self")
	}
	// repeated message 元素
	if err := m.Set("contacts", make([]*Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m, "contacts[0].phone", "x")
	g, _ := m.Get("contacts[0]")
	if g.Value() != g {
		t.Fatalf("message element Value() should return self")
	}
}

// repeated bytes 深拷贝
func TestDeepCopyRepeatedBytes(t *testing.T) {
	m := newAllTypes(t)
	src := [][]byte{{1, 2}, {3}}
	mustSet(t, m, "byr", src)
	src[0][0] = 99
	g, _ := m.Get("byr")
	eq(t, g.Value(), [][]byte{{1, 2}, {3}})
}

// repeated bool
func TestRepeatedBool(t *testing.T) {
	m := newAllTypes(t)
	mustSet(t, m, "br", []bool{true, false, true})
	g, _ := m.Get("br")
	eq(t, g.Value(), []bool{true, false, true})
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	m2 := newAllTypes(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	g, _ = m2.Get("br")
	eq(t, g.Value(), []bool{true, false, true})
}

// toString 各数值类型
func TestToStringNumericVariants(t *testing.T) {
	m := newAllTypes(t)
	for _, tc := range []struct {
		in   any
		want string
	}{
		{uint32(7), "7"},
		{uint64(8), "8"},
		{float32(2.5), "2.5"},
		{int8(-3), "-3"},
		{uint16(65535), "65535"},
	} {
		mustSet(t, m, "s", tc.in)
		g, _ := m.Get("s")
		eq(t, g.Value(), tc.want)
	}
}

// uint64 JSON 解码分支(parseJSONUint)
func TestParseJSONUintEdges(t *testing.T) {
	m := newAllTypes(t)
	if err := m.DecodeJSON([]byte(`{"u64":-1}`)); err != ErrMalformedData {
		t.Fatalf("negative u64 err = %v, want ErrMalformedData", err)
	}
	if err := m.DecodeJSON([]byte(`{"u64":1.5}`)); err != ErrMalformedData {
		t.Fatalf("float u64 err = %v, want ErrMalformedData", err)
	}
}

// 防御:单值 message 字段异常态(present=true, value=nil)不 panic
func TestWrapFieldNilMessageNoPanic(t *testing.T) {
	m := newMsg(t)
	// addr 位于字段索引 5(name,age,active,score,data,addr)
	m.fields[5].present = true
	m.fields[5].value = nil
	g, err := m.Get("addr")
	if err != nil {
		t.Fatalf("Get addr: %v", err)
	}
	if g != nil {
		t.Fatalf("expected nil, got %#v", g)
	}
	// Encode 也应安全
	if _, err := m.EncodeProto(); err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	if _, err := m.EncodeJSON(); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
}
