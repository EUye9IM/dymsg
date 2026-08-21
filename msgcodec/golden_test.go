package msgcodec

// golden_test.go —— 隐藏黄金测试(评测用,agent 不可见)。
// 仅依赖公开 API(ParseSchema/Register/New/Message),不依赖任何内部符号。
// 评测方式:用被测实现替换本包源码后,执行:
//
//	go test -race -run Golden ./...
//
// 全部通过视为通过。运行需在 -race 下验证并发安全。

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ---------- 测试内自含 wire 工具(不依赖实现内部) ----------

func vt(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func tag(num, wt int) []byte {
	return vt(nil, uint64(num)<<3|uint64(wt))
}

func flen(b []byte, content []byte) []byte {
	b = vt(b, uint64(len(content)))
	return append(b, content...)
}

// ---------- Schema 与注册 ----------

const goldenConfig = `{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "age", "type": "int32", "num": 2},
        {"name": "active", "type": "bool", "num": 3},
        {"name": "score", "type": "double", "num": 4},
        {"name": "addr", "type": "message", "num": 5, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1},
            {"name": "zip", "type": "string", "num": 2}
          ]
        }},
        {"name": "tags", "type": "string", "num": 6, "repeated": true},
        {"name": "scores", "type": "int32", "num": 7, "repeated": true},
        {"name": "contacts", "type": "message", "num": 8, "repeated": true, "schema": {
          "fields": [{"name": "phone", "type": "string", "num": 1}]
        }}
      ]
    }
  ]
}`

var goldenOnce sync.Once

func goldenSetup(t *testing.T) {
	t.Helper()
	goldenOnce.Do(func() {
		schemas, err := ParseSchema([]byte(goldenConfig))
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

func goldenNew(t *testing.T, id uint16) Message {
	t.Helper()
	goldenSetup(t)
	m, err := New(id)
	if err != nil {
		t.Fatalf("New(%d): %v", id, err)
	}
	return m
}

func mustGet(t *testing.T, m Message, path string, want any) {
	t.Helper()
	v, err := m.Get(path)
	if err != nil {
		t.Fatalf("Get(%q): %v", path, err)
	}
	if v == nil {
		t.Fatalf("Get(%q) = nil, want %#v", path, want)
	}
	if v.Value() != want {
		t.Fatalf("Get(%q) = %#v, want %#v", path, v.Value(), want)
	}
}

func mustSet(t *testing.T, m Message, path string, value any) {
	t.Helper()
	if err := m.Set(path, value); err != nil {
		t.Fatalf("Set(%q, %#v): %v", path, value, err)
	}
}

// ---------- 契约层 ----------

func TestGoldenRegisterDuplicateID(t *testing.T) {
	a := `{"types":[{"typeId":700,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	b := `{"types":[{"typeId":700,"fields":[{"name":"b","type":"string","num":1}]}]}`
	sa, err := ParseSchema([]byte(a))
	if err != nil {
		t.Fatal(err)
	}
	sb, err := ParseSchema([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := Register(sa[0]); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := Register(sb[0]); err != ErrDuplicateID {
		t.Fatalf("second register err = %v, want ErrDuplicateID", err)
	}
}

func TestGoldenRegisterIdempotent(t *testing.T) {
	schemas, err := ParseSchema([]byte(goldenConfig))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range schemas {
		if err := Register(s); err != nil {
			t.Fatalf("idempotent re-register: %v", err)
		}
	}
}

func TestGoldenNewUnknownTypeID(t *testing.T) {
	if _, err := New(59999); err != ErrUnknownTypeID {
		t.Fatalf("New unknown err = %v, want ErrUnknownTypeID", err)
	}
}

func TestGoldenGetFieldNotFound(t *testing.T) {
	m := goldenNew(t, 1001)
	if _, err := m.Get("nope"); err != ErrFieldNotFound {
		t.Fatalf("err = %v, want ErrFieldNotFound", err)
	}
}

func TestGoldenIndexOutOfRange(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "tags", []any{"a"})
	if _, err := m.Get("tags[5]"); err != ErrIndexOutOfRange {
		t.Fatalf("err = %v, want ErrIndexOutOfRange", err)
	}
	if v, _ := m.Get("tags"); v == nil {
		t.Fatal("Get tags should not be nil")
	}
	if v, _ := m.Get("tags[0]"); v == nil || v.Value() != "a" {
		t.Fatalf("tags[0] = %#v", v)
	}
}

func TestGoldenDecodeEmpty(t *testing.T) {
	m := goldenNew(t, 1001)
	if err := m.DecodeJSON(nil); err != ErrTruncated {
		t.Fatalf("DecodeJSON empty err = %v, want ErrTruncated", err)
	}
	m2 := goldenNew(t, 1001)
	if err := m2.DecodeProto(nil); err != ErrTruncated {
		t.Fatalf("DecodeProto empty err = %v, want ErrTruncated", err)
	}
}

// ---------- 边界/安全层 ----------

func TestGoldenSchemaValidation(t *testing.T) {
	cases := []string{
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":0}]}]}`,                                     // num 越界(0)
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":70000}]}]}`,                                 // num 越界
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"b","type":"int32","num":1}]}]}`, // num 重复
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"b","type":"int32","num":2}]}]}`,
		`{"types":[{"typeId":1,"fields":[{"name":"a.b","type":"int32","num":1}]}]}`, // 字段名含点
		`{"types":[{"typeId":1,"fields":[{"name":"a","type":"int64","num":1}]}]}`,   // 合法
		`{"types":[{"typeId":0,"fields":[]}]}`,                                      // typeId=0
		`{"types":[{"typeId":1,"fields":[]},{"typeId":1,"fields":[]}]}`,             // typeId 重复
	}
	wantErr := []bool{true, true, true, false, true, false, true, true}
	for i, c := range cases {
		_, err := ParseSchema([]byte(c))
		if (err != nil) != wantErr[i] {
			t.Fatalf("case %d: err = %v, wantErr = %v", i, err, wantErr[i])
		}
	}
}

func TestGoldenWireTypeMismatch(t *testing.T) {
	// age(num=2,int32) 以 wire type 2 编码 -> ErrMalformedData
	blob := flen(tag(2, 2), []byte("12345"))
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestGoldenIllegalWireType(t *testing.T) {
	blob := append(tag(2, 6), 0)
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestGoldenFieldNumZeroKey(t *testing.T) {
	blob := append(tag(0, 0), 1)
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestGoldenTruncatedProto(t *testing.T) {
	// name 声明长度 100,但实际不足
	blob := append(tag(1, 2), vt(nil, 100)...)
	blob = append(blob, []byte("hello")...)
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestGoldenUnknownFieldSkipped(t *testing.T) {
	var blob []byte
	blob = append(blob, vt(tag(99, 0), 12345)...) // 未知 varint 字段
	blob = append(blob, tag(1, 2)...)             // name 字段
	blob = flen(blob, []byte("alice"))
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	mustGet(t, m, "name", "alice")
}

func TestGoldenConversionErrors(t *testing.T) {
	m := goldenNew(t, 1001)
	if err := m.Set("age", "notanumber"); err != ErrTypeMismatch {
		t.Fatalf("age<-string err = %v, want ErrTypeMismatch", err)
	}
	// 数值溢出:int32 越界
	if err := m.Set("age", int64(1<<40)); err != ErrTypeMismatch {
		t.Fatalf("overflow err = %v, want ErrTypeMismatch", err)
	}
	mustSet(t, m, "name", int64(42)) // string<-数值 允许
	mustGet(t, m, "name", "42")
}

// ---------- 功能层 ----------

func TestGoldenBasics(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(30))
	mustGet(t, m, "name", "alice")
	mustGet(t, m, "age", int32(30))
	// 未设置 -> nil
	if v, _ := m.Get("active"); v != nil {
		t.Fatalf("active should be nil (unset)")
	}
	// 自身
	if v, _ := m.Get(""); v == nil {
		t.Fatal("Get(\"\") should return self")
	}
}

func TestGoldenNested(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "addr.city", "beijing")
	mustSet(t, m, "addr.zip", "100000")
	mustGet(t, m, "addr.city", "beijing")
	mustGet(t, m, "addr.zip", "100000")
	// 中间未设置 -> nil
	m2 := goldenNew(t, 1001)
	if v, _ := m2.Get("addr.city"); v != nil {
		t.Fatalf("addr.city should be nil when addr unset")
	}
	// 中间节点自动创建
	mustSet(t, m2, "addr.city", "sh")
	mustGet(t, m2, "addr.city", "sh")
}

func TestGoldenPresenceEncoding(t *testing.T) {
	a := goldenNew(t, 1001)
	pa, _ := a.EncodeProto()
	if len(pa) != 0 {
		t.Fatalf("unset-only proto len = %d, want 0", len(pa))
	}
	b := goldenNew(t, 1001)
	mustSet(t, b, "age", int32(0)) // 显式零值
	pb, _ := b.EncodeProto()
	if len(pb) == 0 {
		t.Fatal("explicit zero value should be encoded")
	}
	ja, _ := a.EncodeJSON()
	jb, _ := b.EncodeJSON()
	if strings.Contains(string(ja), "age") {
		t.Fatalf("json(unset) should not contain age: %s", ja)
	}
	if !strings.Contains(string(jb), "age") {
		t.Fatalf("json(set) should contain age: %s", jb)
	}
}

func TestGoldenClearField(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "age", int32(1))
	if err := m.Set("age", nil); err != nil {
		t.Fatalf("Set age nil: %v", err)
	}
	if v, _ := m.Get("age"); v != nil {
		t.Fatalf("age should be unset")
	}
	// Set("", nil) 清空全部
	mustSet(t, m, "name", "x")
	if err := m.Set("", nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Get("name"); v != nil {
		t.Fatalf("name should be unset after full clear")
	}
}

func TestGoldenDeepCopy(t *testing.T) {
	m1 := goldenNew(t, 1001)
	mustSet(t, m1, "name", "alice")
	mustSet(t, m1, "addr.city", "bj")
	m2 := goldenNew(t, 1001)
	if err := m2.Set("", m1); err != nil {
		t.Fatalf("copy: %v", err)
	}
	mustSet(t, m2, "name", "bob")
	mustSet(t, m2, "addr.city", "sh")
	mustGet(t, m1, "name", "alice")
	mustGet(t, m1, "addr.city", "bj")
}

func TestGoldenJSONRoundTrip(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(30))
	mustSet(t, m, "active", true)
	mustSet(t, m, "score", 1.5)
	mustSet(t, m, "addr.city", "bj")
	mustSet(t, m, "tags", []any{"x", "y"})
	mustSet(t, m, "scores", []int32{1, 2, 3})
	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := goldenNew(t, 1001)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	mustGet(t, m2, "name", "alice")
	mustGet(t, m2, "age", int32(30))
	mustGet(t, m2, "active", true)
	mustGet(t, m2, "addr.city", "bj")
	mustGet(t, m2, "tags[1]", "y")
	mustGet(t, m2, "scores[2]", int32(3))
}

func TestGoldenProtoRoundTrip(t *testing.T) {
	m := goldenNew(t, 1001)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(-7)) // 负数补码
	mustSet(t, m, "score", -2.5)
	mustSet(t, m, "addr.city", "bj")
	mustSet(t, m, "scores", []int32{1, -2, 3}) // packed
	mustSet(t, m, "tags", []any{"x", "y"})     // 非 packed
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := goldenNew(t, 1001)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	mustGet(t, m2, "name", "alice")
	mustGet(t, m2, "age", int32(-7))
	mustGet(t, m2, "score", -2.5)
	mustGet(t, m2, "addr.city", "bj")
	mustGet(t, m2, "scores[1]", int32(-2))
	mustGet(t, m2, "tags[0]", "x")
}

func TestGoldenUnpackedAccepted(t *testing.T) {
	var blob []byte
	for _, v := range []int64{5, 6, 7} {
		blob = vt(blob, 7<<3|0) // field 7 wt 0
		blob = vt(blob, uint64(v))
	}
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto unpacked: %v", err)
	}
	mustGet(t, m, "scores[1]", int32(6))
}

func TestGoldenMixedPackedUnpacked(t *testing.T) {
	var blob []byte
	// packed 块:元素 1,2,3
	packed := vt(nil, 1)
	packed = vt(packed, 2)
	packed = vt(packed, 3)
	blob = append(blob, tag(7, 2)...) // field 7 packed(wt 2)
	blob = flen(blob, packed)
	blob = vt(blob, 7<<3|0) // unpacked 元素 4
	blob = vt(blob, 4)
	blob = vt(blob, 7<<3|0) // unpacked 元素 5
	blob = vt(blob, 5)
	m := goldenNew(t, 1001)
	if err := m.DecodeProto(blob); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	lst, _ := m.Get("scores")
	got := lst.Value().([]any)
	if len(got) != 5 {
		t.Fatalf("scores len = %d, want 5", len(got))
	}
	for i, want := range []int32{1, 2, 3, 4, 5} {
		if got[i] != want {
			t.Fatalf("scores[%d] = %v, want %v", i, got[i], want)
		}
	}
}

func TestGoldenMessagesRepeated(t *testing.T) {
	m := goldenNew(t, 1001)
	if err := m.Set("contacts", make([]Message, 2)); err != nil {
		t.Fatalf("make([]Message,2): %v", err)
	}
	lst, _ := m.Get("contacts")
	if msgs := lst.Value().([]Message); len(msgs) != 2 {
		t.Fatalf("contacts len = %d", len(msgs))
	}
	mustSet(t, m, "contacts[0].phone", "111")
	mustSet(t, m, "contacts[1].phone", "222")
	mustGet(t, m, "contacts[0].phone", "111")
	mustGet(t, m, "contacts[1].phone", "222")
	// proto 往返
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	m2 := goldenNew(t, 1001)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	mustGet(t, m2, "contacts[0].phone", "111")
}

// ---------- 并发层(配合 -race) ----------

func concConfig(n int) string {
	var sb strings.Builder
	sb.WriteString(`{"types":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"typeId":%d,"fields":[{"name":"f","type":"int32","num":1}]}`, 8000+i)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func TestGoldenConcurrentNew(t *testing.T) {
	goldenSetup(t)
	const g = 32
	var wg sync.WaitGroup
	errs := make(chan error, g)
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if m, err := New(1001); err != nil {
					errs <- err
					return
				} else {
					_ = m
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent New: %v", err)
	}
}

func TestGoldenConcurrentRegisterAndNew(t *testing.T) {
	goldenSetup(t)
	schemas, err := ParseSchema([]byte(concConfig(24)))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	// 并发注册 24 个不同类型(写注册表)
	for _, s := range schemas {
		wg.Add(1)
		go func(s MessageSchema) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := Register(s); err != nil {
					t.Errorf("Register: %v", err)
					return
				}
			}
		}(s)
	}
	// 并发 New 已注册类型(读注册表)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				if _, err := New(1001); err != nil {
					t.Errorf("New: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
