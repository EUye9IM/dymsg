package msgcodec

import (
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
        {"name": "addr", "type": "message", "num": 4, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1},
            {"name": "zip", "type": "string", "num": 2}
          ]
        }},
        {"name": "tags", "type": "string", "num": 5, "repeated": true},
        {"name": "scores", "type": "int32", "num": 6, "repeated": true},
        {"name": "contacts", "type": "message", "num": 7, "repeated": true, "schema": {
          "fields": [
            {"name": "phone", "type": "string", "num": 1}
          ]
        }}
      ]
    }
  ]
}`

func resetRegistry() {
	regMu.Lock()
	defer regMu.Unlock()
	clear(reg)
	clear(regObj)
}

var regState struct {
	mu     sync.Mutex
	inited bool
}

func ensureRegistered(t *testing.T) {
	t.Helper()
	regState.mu.Lock()
	defer regState.mu.Unlock()
	if regState.inited {
		return
	}
	resetRegistry()
	schemas, err := ParseSchema([]byte(testConfig))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	for _, s := range schemas {
		if err := Register(s); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	regState.inited = true
}

func invalidateRegState() {
	regState.mu.Lock()
	regState.inited = false
	regState.mu.Unlock()
}

func newTestMsg(t *testing.T) Message {
	t.Helper()
	ensureRegistered(t)
	m, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func get(t *testing.T, m Message, path string) (Message, error) {
	t.Helper()
	return m.Get(path)
}

func TestBasics(t *testing.T) {
	m := newTestMsg(t)
	if err := m.Set("name", "alice"); err != nil {
		t.Fatalf("Set name: %v", err)
	}
	v, _ := m.Get("name")
	if v == nil || v.Value() != "alice" {
		t.Fatalf("Get name = %#v, want alice", v)
	}
	if err := m.Set("age", int32(30)); err != nil {
		t.Fatalf("Set age: %v", err)
	}
	v, _ = m.Get("age")
	if v == nil || v.Value() != int32(30) {
		t.Fatalf("Get age = %#v, want int32(30)", v)
	}
	// presence: 未设置字段 -> nil
	v, _ = m.Get("active")
	if v != nil {
		t.Fatalf("Get active should be nil (unset), got %#v", v)
	}
	// 字段不存在
	if _, err := m.Get("nope"); err != ErrFieldNotFound {
		t.Fatalf("Get nope err = %v, want ErrFieldNotFound", err)
	}
}

func TestNestedAndPath(t *testing.T) {
	m := newTestMsg(t)
	if err := m.Set("addr.city", "beijing"); err != nil {
		t.Fatalf("Set addr.city: %v", err)
	}
	if err := m.Set("addr.zip", "100000"); err != nil {
		t.Fatalf("Set addr.zip: %v", err)
	}
	// 中间未设置 -> nil
	m2 := newTestMsg(t)
	v, _ := m2.Get("addr.city")
	if v != nil {
		t.Fatalf("addr.city should be nil when addr unset")
	}
	// 中间节点自动创建
	if err := m2.Set("addr.city", "sh"); err != nil {
		t.Fatalf("Set addr.city on fresh: %v", err)
	}
	v, _ = m2.Get("addr.city")
	if v == nil || v.Value() != "sh" {
		t.Fatalf("addr.city = %#v", v)
	}
}

func TestRepeated(t *testing.T) {
	m := newTestMsg(t)
	if err := m.Set("tags", []any{"a", "b", "c"}); err != nil {
		t.Fatalf("Set tags: %v", err)
	}
	v, _ := m.Get("tags[1]")
	if v == nil || v.Value() != "b" {
		t.Fatalf("tags[1] = %#v", v)
	}
	if _, err := m.Get("tags[9]"); err != ErrIndexOutOfRange {
		t.Fatalf("tags[9] err = %v, want ErrIndexOutOfRange", err)
	}
	lst, _ := m.Get("tags")
	if lst == nil {
		t.Fatal("Get tags should not be nil")
	}
	got := lst.Value().([]any)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("tags Value = %#v", got)
	}
	// scores 用 []int32
	if err := m.Set("scores", []int32{1, 2, 3}); err != nil {
		t.Fatalf("Set scores: %v", err)
	}
	v, _ = m.Get("scores[2]")
	if v == nil || v.Value() != int32(3) {
		t.Fatalf("scores[2] = %#v", v)
	}
}

func TestRepeatedMessageAndMake(t *testing.T) {
	// make([]Message, 2)
	m := newTestMsg(t)
	if err := m.Set("contacts", make([]Message, 2)); err != nil {
		t.Fatalf("Set contacts via make: %v", err)
	}
	lst, _ := m.Get("contacts")
	msgs := lst.Value().([]Message)
	if len(msgs) != 2 {
		t.Fatalf("contacts len = %d, want 2", len(msgs))
	}
	c2 := newTestMsg(t)
	if err := c2.Set("contacts", make([]Message, 2)); err != nil {
		t.Fatalf("Set contacts via make: %v", err)
	}
	if err := c2.Set("contacts[1].phone", "123"); err != nil {
		t.Fatalf("Set contacts[1].phone: %v", err)
	}
	v, _ := c2.Get("contacts[1].phone")
	if v == nil || v.Value() != "123" {
		t.Fatalf("contacts[1].phone = %#v", v)
	}
}

func TestPresenceEncoding(t *testing.T) {
	// 未设置 age: proto 不含 age;显式设 0:包含
	a := newTestMsg(t)
	pa, err := a.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	if len(pa) != 0 {
		t.Fatalf("empty msg proto len = %d, want 0", len(pa))
	}
	b := newTestMsg(t)
	if err := b.Set("age", int32(0)); err != nil {
		t.Fatalf("Set age: %v", err)
	}
	pb, err := b.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	if len(pb) == 0 {
		t.Fatal("presence with zero value should encode")
	}
	// JSON 同样区分
	ja, _ := a.EncodeJSON()
	jb, _ := b.EncodeJSON()
	if strings.Contains(string(ja), "age") {
		t.Fatalf("json(a) should not contain age: %s", ja)
	}
	if !strings.Contains(string(jb), "age") {
		t.Fatalf("json(b) should contain age: %s", jb)
	}
	// Set nil 清除
	if err := b.Set("age", nil); err != nil {
		t.Fatalf("Set age nil: %v", err)
	}
	v, _ := b.Get("age")
	if v != nil {
		t.Fatalf("age should be unset after Set nil")
	}
}

func TestDeepCopy(t *testing.T) {
	m1 := newTestMsg(t)
	m1.Set("name", "alice")
	m1.Set("addr.city", "bj")
	m2 := newTestMsg(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatalf("copy: %v", err)
	}
	m2.Set("name", "bob")
	m2.Set("addr.city", "sh")
	v, _ := m1.Get("name")
	if v.Value() != "alice" {
		t.Fatalf("m1.name mutated: %#v", v.Value())
	}
	v, _ = m1.Get("addr.city")
	if v.Value() != "bj" {
		t.Fatalf("m1.addr.city mutated: %#v", v.Value())
	}
}

func TestRoundTripJSON(t *testing.T) {
	m := newTestMsg(t)
	m.Set("name", "alice")
	m.Set("age", int32(30))
	m.Set("active", true)
	m.Set("addr.city", "bj")
	m.Set("tags", []any{"x", "y"})
	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := newTestMsg(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	assertField(t, m2, "name", "alice")
	assertField(t, m2, "age", int32(30))
	assertField(t, m2, "active", true)
	assertField(t, m2, "addr.city", "bj")
	v, _ := m2.Get("tags[0]")
	if v == nil || v.Value() != "x" {
		t.Fatalf("tags[0] = %#v", v)
	}
}

func TestRoundTripProto(t *testing.T) {
	m := newTestMsg(t)
	m.Set("name", "alice")
	m.Set("age", int32(-7))
	m.Set("addr.city", "bj")
	m.Set("scores", []int32{1, -2, 3})
	m.Set("tags", []any{"x", "y"})
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := newTestMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	assertField(t, m2, "name", "alice")
	assertField(t, m2, "age", int32(-7))
	assertField(t, m2, "addr.city", "bj")
	v, _ := m2.Get("scores[1]")
	if v == nil || v.Value() != int32(-2) {
		t.Fatalf("scores[1] = %#v", v)
	}
	v, _ = m2.Get("tags[1]")
	if v == nil || v.Value() != "y" {
		t.Fatalf("tags[1] = %#v", v)
	}
}

func TestUnknownFieldSkipped(t *testing.T) {
	b := newTestMsg(t)
	b.Set("name", "alice")
	bb, _ := b.EncodeProto()
	// 手工构造:前插一个未知字段 num=99(wire type 0,varint),再拼接 name 字段
	unknown := appendTag(nil, 99, wireVarint)
	unknown = appendVarint(unknown, 12345)
	blob := append(unknown, bb...)
	m := newTestMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto with unknown field: %v", err)
	}
	v, _ := m.Get("name")
	if v == nil || v.Value() != "alice" {
		t.Fatalf("name = %#v", v)
	}
}

func TestWireTypeMismatch(t *testing.T) {
	// age 是 int32(varint),用 wire type 2(length-delimited)编码 -> 报错
	blob := appendTag(nil, 2, wireBytes)
	blob = appendVarint(blob, 5)
	blob = append(blob, []byte("12345")...)
	m := newTestMsg(t)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("DecodeProto err = %v, want ErrMalformedData", err)
	}
}

func TestIllegalWireType(t *testing.T) {
	blob := appendTag(nil, 2, 6)
	blob = appendVarint(blob, 0)
	m := newTestMsg(t)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("illegal wire type err = %v", err)
	}
}

func TestUnpackedAccepted(t *testing.T) {
	// 构造 unpacked scores(逐元素 varint)
	var blob []byte
	for _, v := range []int32{5, 6, 7} {
		blob = appendTag(blob, 6, wireVarint)
		blob = appendVarint(blob, uint64(int64(v)))
	}
	m := newTestMsg(t)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto unpacked: %v", err)
	}
	for i, want := range []int32{5, 6, 7} {
		v, _ := m.Get("scores[" + itoa(i) + "]")
		if v == nil || v.Value() != want {
			t.Fatalf("scores[%d] = %#v, want %v", i, v, want)
		}
	}
}

func TestParseSchemaErrors(t *testing.T) {
	if _, err := ParseSchema([]byte(`{invalid`)); err != ErrMalformedData {
		t.Fatalf("bad json err = %v", err)
	}
	dup := `{"types":[{"typeId":1,"fields":[]},{"typeId":1,"fields":[]}]}`
	if _, err := ParseSchema([]byte(dup)); err != ErrMalformedData {
		t.Fatalf("dup typeId err = %v", err)
	}
	fdup := `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"b","type":"int32","num":1}]}]}`
	if _, err := ParseSchema([]byte(fdup)); err != ErrMalformedData {
		t.Fatalf("dup num err = %v", err)
	}
	badtype := `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int99","num":1}]}]}`
	if _, err := ParseSchema([]byte(badtype)); err != ErrMalformedData {
		t.Fatalf("bad type err = %v", err)
	}
}

func TestDuplicateID(t *testing.T) {
	ensureRegistered(t)
	// 再次注册相同配置 -> 幂等
	schemas, _ := ParseSchema([]byte(testConfig))
	if err := Register(schemas[0]); err != nil {
		t.Fatalf("idempotent err = %v", err)
	}
	// 不同实现同 ID -> ErrDuplicateID
	other := &schemaImpl{typeID: 1001}
	if err := Register(other); err != ErrDuplicateID {
		t.Fatalf("Register other err = %v, want ErrDuplicateID", err)
	}
	// 清理,让后续测试重新注册
	invalidateRegState()
	resetRegistry()
}

func TestNestedMessageRoundTrip(t *testing.T) {
	// 整体赋值嵌套消息:先创建 addr 实例,再 Set 到一个新消息
	m := newTestMsg(t)
	m.Set("addr.city", "gz")
	src, _ := m.Get("addr")
	m2 := newTestMsg(t)
	if err := m2.Set("addr", src); err != nil {
		t.Fatalf("Set addr src: %v", err)
	}
	v, _ := m2.Get("addr.city")
	if v == nil || v.Value() != "gz" {
		t.Fatalf("addr.city = %#v", v)
	}
}

// 并发:并发首次 New
func TestConcurrentNew(t *testing.T) {
	ensureRegistered(t)
	const n = 64
	done := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				if _, err := New(1001); err != nil {
					t.Errorf("New: %v", err)
					break
				}
			}
			done <- true
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
}

func assertField(t *testing.T, m Message, path string, want any) {
	t.Helper()
	v, err := m.Get(path)
	if err != nil {
		t.Fatalf("Get(%q): %v", path, err)
	}
	if v == nil {
		t.Fatalf("Get(%q) nil", path)
	}
	if v.Value() != want {
		t.Fatalf("Get(%q) = %#v, want %#v", path, v.Value(), want)
	}
}

func itoa(n int) string {
	return strings.TrimSpace(itoaBytes(n))
}

func itoaBytes(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
