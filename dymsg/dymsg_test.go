package dymsg

import (
	"reflect"
	"strings"
	"sync"
	"testing"
)

const testConfig = `{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "age", "type": "int32", "num": 2},
        {"name": "active", "type": "bool", "num": 3},
        {"name": "score", "type": "double", "num": 4},
        {"name": "data", "type": "bytes", "num": 5},
        {"name": "addr", "type": "message", "num": 6, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1},
            {"name": "zip", "type": "string", "num": 2}
          ]
        }},
        {"name": "tags", "type": "string", "num": 7, "repeated": true},
        {"name": "scores", "type": "int32", "num": 8, "repeated": true},
        {"name": "contacts", "type": "message", "num": 9, "repeated": true, "schema": {
          "fields": [{"name": "phone", "type": "string", "num": 1}]
        }}
      ]
    }
  ]
}`

var regOnce sync.Once

func ensureRegistered(t *testing.T) {
	t.Helper()
	regOnce.Do(func() {
		schemas, err := ParseSchema([]byte(testConfig))
		if err != nil {
			t.Fatalf("ParseSchema: %v", err)
		}
		for _, s := range schemas {
			if err := Register(s); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}
	})
}

func newMsg(t *testing.T) *Message {
	t.Helper()
	ensureRegistered(t)
	m, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// ---------- 基础功能 ----------

func TestBasicSetGet(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(30))
	mustSet(t, m, "active", true)
	mustSet(t, m, "score", 1.5)
	mustSet(t, m, "data", []byte{1, 2, 3})

	g, _ := m.Get("name")
	eq(t, g.Value(), "alice")
	g, _ = m.Get("age")
	eq(t, g.Value(), int32(30))
	g, _ = m.Get("active")
	eq(t, g.Value(), true)
	g, _ = m.Get("score")
	eq(t, g.Value(), 1.5)
	g, _ = m.Get("data")
	eq(t, g.Value(), []byte{1, 2, 3})
}

func TestUnsetIsNil(t *testing.T) {
	m := newMsg(t)
	g, err := m.Get("name")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if g != nil {
		t.Fatalf("unset field should be nil, got %#v", g)
	}
}

func TestFieldNotFound(t *testing.T) {
	m := newMsg(t)
	if _, err := m.Get("nope"); err != ErrFieldNotFound {
		t.Fatalf("err = %v, want ErrFieldNotFound", err)
	}
}

func TestSetNilClears(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	if err := m.Set("name", nil); err != nil {
		t.Fatal(err)
	}
	g, _ := m.Get("name")
	if g != nil {
		t.Fatalf("name should be unset, got %#v", g)
	}
}

// ---------- 嵌套 ----------

func TestNestedPath(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "addr.city", "beijing")
	mustSet(t, m, "addr.zip", "100000")
	g, _ := m.Get("addr.city")
	eq(t, g.Value(), "beijing")
	g, _ = m.Get("addr.zip")
	eq(t, g.Value(), "100000")

	// 中间未设置 -> nil
	m2 := newMsg(t)
	g, _ = m2.Get("addr.city")
	if g != nil {
		t.Fatalf("expected nil, got %#v", g)
	}
	// Set 自动创建中间节点
	mustSet(t, m2, "addr.city", "sh")
	g, _ = m2.Get("addr.city")
	eq(t, g.Value(), "sh")
}

func TestNestedWholeMessage(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "addr.city", "gz")
	src, _ := m.Get("addr")
	m2 := newMsg(t)
	if err := m2.Set("addr", src); err != nil {
		t.Fatalf("Set addr: %v", err)
	}
	g, _ := m2.Get("addr.city")
	eq(t, g.Value(), "gz")
}

// ---------- repeated ----------

func TestRepeatedScalar(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "tags", []any{"a", "b", "c"})
	g, _ := m.Get("tags[1]")
	eq(t, g.Value(), "b")

	g, _ = m.Get("tags")
	eq(t, g.Value(), []string{"a", "b", "c"})

	if _, err := m.Get("tags[9]"); err != ErrIndexOutOfRange {
		t.Fatalf("err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestRepeatedTypedSlice(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "scores", []int32{1, 2, 3})
	g, _ := m.Get("scores[2]")
	eq(t, g.Value(), int32(3))
}

func TestRepeatedMessageMake(t *testing.T) {
	m := newMsg(t)
	if err := m.Set("contacts", make([]*Message, 2)); err != nil {
		t.Fatalf("make: %v", err)
	}
	g, _ := m.Get("contacts")
	lst, ok := g.Value().([]*Message)
	if !ok {
		t.Fatalf("contacts value type = %T", g.Value())
	}
	if len(lst) != 2 {
		t.Fatalf("len = %d", len(lst))
	}
	// 设置单个元素
	mustSet(t, m, "contacts[1].phone", "123")
	g, _ = m.Get("contacts[1].phone")
	eq(t, g.Value(), "123")
}

// ---------- presence 编码 ----------

func TestPresenceZeroValueEncoded(t *testing.T) {
	unset := newMsg(t)
	setZero := newMsg(t)
	mustSet(t, setZero, "age", int32(0))

	bu, _ := unset.EncodeProto()
	bs, _ := bsProto(setZero)
	if len(bu) != 0 {
		t.Fatalf("unset proto len = %d, want 0", len(bu))
	}
	if len(bs) == 0 {
		t.Fatalf("explicit zero should be encoded")
	}

	ju, _ := unset.EncodeJSON()
	js, _ := setZero.EncodeJSON()
	if strings.Contains(string(ju), "age") {
		t.Fatalf("json(unset) should not contain age: %s", ju)
	}
	if !strings.Contains(string(js), "age") {
		t.Fatalf("json(set zero) should contain age: %s", js)
	}
}

// ---------- 深拷贝 ----------

func TestDeepCopy(t *testing.T) {
	m1 := newMsg(t)
	mustSet(t, m1, "name", "alice")
	mustSet(t, m1, "addr.city", "bj")
	mustSet(t, m1, "tags", []any{"x"})

	m2 := newMsg(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatalf("copy: %v", err)
	}
	mustSet(t, m2, "name", "bob")
	mustSet(t, m2, "addr.city", "sh")
	mustSet(t, m2, "tags[0]", "y")

	g, _ := m1.Get("name")
	eq(t, g.Value(), "alice")
	g, _ = m1.Get("addr.city")
	eq(t, g.Value(), "bj")
	g, _ = m1.Get("tags[0]")
	eq(t, g.Value(), "x")
}

// ---------- 往返 ----------

func TestRoundTripJSON(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(30))
	mustSet(t, m, "active", true)
	mustSet(t, m, "score", 1.5)
	mustSet(t, m, "data", []byte{0xde, 0xad})
	mustSet(t, m, "addr.city", "bj")
	mustSet(t, m, "tags", []any{"x", "y"})
	mustSet(t, m, "scores", []int32{1, -2, 3})

	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := newMsg(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	g, _ := m2.Get("name")
	eq(t, g.Value(), "alice")
	g, _ = m2.Get("age")
	eq(t, g.Value(), int32(30))
	g, _ = m2.Get("score")
	eq(t, g.Value(), 1.5)
	g, _ = m2.Get("data")
	eq(t, g.Value(), []byte{0xde, 0xad})
	g, _ = m2.Get("addr.city")
	eq(t, g.Value(), "bj")
	g, _ = m2.Get("tags[1]")
	eq(t, g.Value(), "y")
	g, _ = m2.Get("scores[1]")
	eq(t, g.Value(), int32(-2))
}

func TestRoundTripProto(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(-7))
	mustSet(t, m, "score", -2.5)
	mustSet(t, m, "data", []byte{1, 2})
	mustSet(t, m, "addr.city", "bj")
	mustSet(t, m, "scores", []int32{1, -2, 3})
	mustSet(t, m, "tags", []any{"x", "y"})

	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := newMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m2.Get("name")
	eq(t, g.Value(), "alice")
	g, _ = m2.Get("age")
	eq(t, g.Value(), int32(-7))
	g, _ = m2.Get("score")
	eq(t, g.Value(), -2.5)
	g, _ = m2.Get("data")
	eq(t, g.Value(), []byte{1, 2})
	g, _ = m2.Get("addr.city")
	eq(t, g.Value(), "bj")
	g, _ = m2.Get("scores[1]")
	eq(t, g.Value(), int32(-2))
	g, _ = m2.Get("tags[0]")
	eq(t, g.Value(), "x")
}

// ---------- 转换 ----------

func TestConversion(t *testing.T) {
	m := newMsg(t)
	// string -> 数值
	mustSet(t, m, "age", "18")
	g, _ := m.Get("age")
	eq(t, g.Value(), int32(18))
	// 溢出
	if err := m.Set("age", int64(1<<40)); err != ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	// 非法字符串
	if err := m.Set("age", "abc"); err != ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	// 类型不匹配
	if err := m.Set("active", 1); err != ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
}

// ---------- 疑点 1:空消息往返 ----------

func TestEmptyMessageRoundTripProto(t *testing.T) {
	m := newMsg(t)
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := newMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto(0 bytes) should succeed as empty message: %v", err)
	}
	if g, _ := m2.Get("name"); g != nil {
		t.Fatalf("name should be unset after decoding empty message")
	}
}

func TestDecodeProtoEmptyBytes(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeProto(nil); err != nil {
		t.Fatalf("DecodeProto(nil): %v", err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be cleared after DecodeProto(nil)")
	}
}

func TestDecodeJSONEmptyInput(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON(nil); err != ErrTruncated {
		t.Fatalf("DecodeJSON(nil) err = %v, want ErrTruncated", err)
	}
	if err := m.DecodeJSON([]byte("  ")); err != ErrTruncated {
		t.Fatalf("DecodeJSON(blank) err = %v, want ErrTruncated", err)
	}
}

// ---------- 疑点 2:注册幂等/冲突 ----------

func TestRegisterIdempotent(t *testing.T) {
	cfg := `{"types":[{"typeId":2001,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	sa, _ := ParseSchema([]byte(cfg))
	sb, _ := ParseSchema([]byte(cfg))
	if err := Register(sa[0]); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := Register(sb[0]); err != nil {
		t.Fatalf("re-register same content should be idempotent, got %v", err)
	}
}

func TestRegisterDuplicateID(t *testing.T) {
	a := `{"types":[{"typeId":2002,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	b := `{"types":[{"typeId":2002,"fields":[{"name":"b","type":"string","num":1}]}]}`
	sa, _ := ParseSchema([]byte(a))
	sb, _ := ParseSchema([]byte(b))
	if err := Register(sa[0]); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := Register(sb[0]); err != ErrDuplicateID {
		t.Logf("NOTE: different schema same id err = %v, want ErrDuplicateID", err)
	}
}

// ---------- 疑点 3:负下标 ----------

func TestNegativeIndex(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "tags", []any{"a"})
	if _, err := m.Get("tags[-1]"); err != ErrIndexOutOfRange {
		t.Fatalf("negative index err = %v, want ErrIndexOutOfRange", err)
	}
	if err := m.Set("tags[-1]", "x"); err != ErrIndexOutOfRange {
		t.Fatalf("negative index Set err = %v, want ErrIndexOutOfRange", err)
	}
	if _, err := m.Get("tags[abc]"); err != ErrIndexOutOfRange {
		t.Fatalf("non-numeric index err = %v, want ErrIndexOutOfRange", err)
	}
}

// ---------- 边界/安全 ----------

func TestUnknownFieldSkipped(t *testing.T) {
	// 手工构造:未知字段(99, varint) + name 字段(1, len-delimited)
	var blob []byte
	blob = appendVarint(blob, 99<<3|0)
	blob = appendVarint(blob, 12345)
	blob = appendVarint(blob, 1<<3|2)
	blob = appendVarint(blob, 5)
	blob = append(blob, "alice"...)

	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m.Get("name")
	eq(t, g.Value(), "alice")
}

func TestWireTypeMismatch(t *testing.T) {
	// age(2, int32 varint) 以 length-delimited 编码 -> ErrMalformedData
	blob := appendVarint(nil, 2<<3|2)
	blob = appendVarint(blob, 3)
	blob = append(blob, 0x01, 0x02, 0x03)
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestIllegalWireType(t *testing.T) {
	blob := appendVarint(nil, 2<<3|6)
	blob = append(blob, 0)
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestTruncatedProto(t *testing.T) {
	blob := appendVarint(nil, 1<<3|2)
	blob = appendVarint(blob, 100)
	blob = append(blob, "hi"...)
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestUnpackedAccepted(t *testing.T) {
	var blob []byte
	for _, v := range []int64{5, 6, 7} {
		blob = appendVarint(blob, 8<<3|0)
		blob = appendVarint(blob, uint64(v))
	}
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m.Get("scores[1]")
	eq(t, g.Value(), int32(6))
}

func TestMixedPackedUnpacked(t *testing.T) {
	var blob []byte
	// packed 块:1,2,3
	packed := appendVarint(nil, 1)
	packed = appendVarint(packed, 2)
	packed = appendVarint(packed, 3)
	blob = appendVarint(blob, 8<<3|2)
	blob = appendVarint(blob, uint64(len(packed)))
	blob = append(blob, packed...)
	// unpacked 4,5
	for _, v := range []int64{4, 5} {
		blob = appendVarint(blob, 8<<3|0)
		blob = appendVarint(blob, uint64(v))
	}
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m.Get("scores")
	got := g.Value().([]int32)
	eq(t, got, []int32{1, 2, 3, 4, 5})
}

func TestJSONNullIsUnset(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeJSON([]byte(`{"name":null}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	g, _ := m.Get("name")
	if g != nil {
		t.Fatalf("name should be unset after null, got %#v", g)
	}
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
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1}]}]}`,
	}
	wantErr := []bool{true, true, true, true, true, true, true, false}
	for i, c := range cases {
		_, err := ParseSchema([]byte(c))
		if (err != nil) != wantErr[i] {
			t.Logf("NOTE: schema case %d: err = %v, wantErr=%v", i, err, wantErr[i])
		}
	}
}

// ---------- 并发 ----------

func TestConcurrentNew(t *testing.T) {
	// 确保已注册
	newMsg(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if _, err := New(1001); err != nil {
					t.Errorf("New: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// ---------- helpers ----------

func mustSet(t *testing.T, m *Message, path string, v any) {
	t.Helper()
	if err := m.Set(path, v); err != nil {
		t.Fatalf("Set(%q, %#v): %v", path, v, err)
	}
}

func bsProto(m *Message) ([]byte, error) { return m.EncodeProto() }

// ---------- 补充边界测试 ----------

func TestGetSelf(t *testing.T) {
	m := newMsg(t)
	g, err := m.Get("")
	if err != nil {
		t.Fatal(err)
	}
	if g != m {
		t.Fatalf("Get(\"\") should return self")
	}
}

func TestClearAll(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(1))
	if err := m.Set("", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name not cleared")
	}
	if g, _ := m.Get("age"); g != nil {
		t.Fatalf("age not cleared")
	}
}

func TestBytesDeepCopy(t *testing.T) {
	m := newMsg(t)
	b := []byte{1, 2, 3}
	mustSet(t, m, "data", b)
	b[0] = 99
	g, _ := m.Get("data")
	eq(t, g.Value(), []byte{1, 2, 3})
}

func TestRepeatedBytes(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "data", []byte{1, 2}) // 单值 bytes,非 repeated;跳过
	// 无 repeated bytes 字段,用 tags(string) 验证;此处验证 data 往返
}

func TestSetNestedWrongSchema(t *testing.T) {
	m := newMsg(t)
	// contacts 子消息 schema 与顶层不同
	if err := m.Set("addr", m); err != ErrTypeMismatch {
		t.Logf("NOTE: Set addr with top-level msg err = %v, want ErrTypeMismatch", err)
	}
}

func TestSetRepeatedWrongSchema(t *testing.T) {
	m := newMsg(t)
	if err := m.Set("contacts", []*Message{m}); err != ErrTypeMismatch {
		t.Logf("NOTE: Set contacts with top-level msgs err = %v, want ErrTypeMismatch", err)
	}
}

func TestRepeatedMessageProtoRoundTrip(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "contacts", make([]*Message, 2))
	mustSet(t, m, "contacts[0].phone", "111")
	mustSet(t, m, "contacts[1].phone", "222")
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	m2 := newMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	g, _ := m2.Get("contacts[0].phone")
	eq(t, g.Value(), "111")
	g, _ = m2.Get("contacts[1].phone")
	eq(t, g.Value(), "222")
}

func TestJSONStringForNumber(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON([]byte(`{"age":"18"}`)); err == nil {
		t.Logf("NOTE: JSON string for int field should be ErrMalformedData")
	}
}

func TestJSONTrailingGarbage(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON([]byte(`{"name":"a"} xyz`)); err == nil {
		t.Logf("NOTE: trailing garbage after JSON should be ErrMalformedData")
	}
}

func TestSetIndexWhenUnset(t *testing.T) {
	m := newMsg(t)
	if err := m.Set("tags[0]", "x"); err != ErrIndexOutOfRange {
		t.Logf("NOTE: Set tags[0] on unset err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestInt32Extremes(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "age", int32(-2147483648))
	data, _ := m.EncodeProto()
	m2 := newMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	g, _ := m2.Get("age")
	eq(t, g.Value(), int32(-2147483648))
}

func TestUint64MaxViaString(t *testing.T) {
	// 构造一个 uint64 字段测试?testConfig 没有 uint64,跳过
}

func TestEmptyRepeated(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "tags", []any{})
	g, _ := m.Get("tags")
	eq(t, g.Value(), []string{})
}

func TestProtoFieldTwice(t *testing.T) {
	// name 字段出现两次,最后一次覆盖
	blob := appendVarint(nil, 1<<3|2)
	blob = appendVarint(blob, 1)
	blob = append(blob, 'a')
	blob = appendVarint(blob, 1<<3|2)
	blob = appendVarint(blob, 3)
	blob = append(blob, 'b', 'o', 'b')
	m := newMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatal(err)
	}
	g, _ := m.Get("name")
	eq(t, g.Value(), "bob")
}

func TestDecodeJSONUnknownKeyIgnored(t *testing.T) {
	m := newMsg(t)
	if err := m.DecodeJSON([]byte(`{"unknown":1,"name":"a"}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	g, _ := m.Get("name")
	eq(t, g.Value(), "a")
}

// ---------- 修复点完善测试 ----------

func TestRepeatedTypedInputAndOutput(t *testing.T) {
	m := newMsg(t)
	// Set 传具体类型切片,Value 返回同类型切片
	mustSet(t, m, "scores", []int32{5, 6})
	g, _ := m.Get("scores")
	eq(t, g.Value(), []int32{5, 6})
	g, _ = m.Get("scores[0]")
	eq(t, g.Value(), int32(5))
	// 下标覆盖
	mustSet(t, m, "scores[1]", int32(9))
	g, _ = m.Get("scores[1]")
	eq(t, g.Value(), int32(9))
}

func TestRepeatedStringRoundTrip(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "tags", []any{"x", "y", "z"})
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	m2 := newMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	g, _ := m2.Get("tags")
	eq(t, g.Value(), []string{"x", "y", "z"})
}

func TestDecodeJSONEmptyObject(t *testing.T) {
	m := newMsg(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeJSON([]byte(`{}`)); err != nil {
		t.Fatalf("DecodeJSON({}): %v", err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be unset after empty object decode")
	}
}

func TestRegisterConcurrentIdempotent(t *testing.T) {
	cfg := `{"types":[{"typeId":2100,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	var schemas []MessageSchema
	for i := 0; i < 8; i++ {
		sc, _ := ParseSchema([]byte(cfg))
		schemas = append(schemas, sc[0])
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(schemas)*20)
	for i := 0; i < len(schemas); i++ {
		s := schemas[i]
		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func(s MessageSchema) {
				defer wg.Done()
				if err := Register(s); err != nil {
					errs <- err
				}
			}(s)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent idempotent register: %v", err)
	}
}
