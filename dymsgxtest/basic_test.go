package dymsgxtest

import (
	"strings"
	"sync"
	"testing"

	dymsg "dymsg"
)

// ---------- 基础 ----------

func TestBasicSetGet(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	mustSet(t, m, "age", int32(30))
	mustSet(t, m, "active", true)
	mustSet(t, m, "score", 1.5)
	mustSet(t, m, "data", []byte{1, 2, 3})
	eq(t, getValue(t, m, "name"), "alice")
	eq(t, getValue(t, m, "age"), int32(30))
	eq(t, getValue(t, m, "active"), true)
	eq(t, getValue(t, m, "score"), 1.5)
	eq(t, getValue(t, m, "data"), []byte{1, 2, 3})
}

func TestUnsetIsNil(t *testing.T) {
	m := mustNew(t)
	g, err := m.Get("name")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if g != nil {
		t.Fatalf("unset field should be nil, got %#v", g)
	}
}

func TestFieldNotFound(t *testing.T) {
	m := mustNew(t)
	if _, err := m.Get("nope"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("err = %v, want ErrFieldNotFound", err)
	}
}

func TestSetNilClears(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	if err := m.Set("name", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be unset, got %#v", g)
	}
}

func TestGetSelf(t *testing.T) {
	m := mustNew(t)
	g, err := m.Get("")
	if err != nil {
		t.Fatal(err)
	}
	if g != m {
		t.Fatalf("Get(\"\") should return self")
	}
}

func TestClearAll(t *testing.T) {
	m := mustNew(t)
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

// ---------- 嵌套 ----------

func TestNestedPath(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "addr.city", "beijing")
	mustSet(t, m, "addr.zip", "100000")
	eq(t, getValue(t, m, "addr.city"), "beijing")
	eq(t, getValue(t, m, "addr.zip"), "100000")

	m2 := mustNew(t)
	if g, _ := m2.Get("addr.city"); g != nil {
		t.Fatalf("expected nil, got %#v", g)
	}
	mustSet(t, m2, "addr.city", "sh")
	eq(t, getValue(t, m2, "addr.city"), "sh")
}

func TestNestedWholeMessage(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "addr.city", "gz")
	src, _ := m.Get("addr")
	m2 := mustNew(t)
	if err := m2.Set("addr", src); err != nil {
		t.Fatalf("Set addr: %v", err)
	}
	eq(t, getValue(t, m2, "addr.city"), "gz")
}

func TestSetNestedWrongSchema(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("addr", m); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Set addr with top-level msg err = %v, want ErrTypeMismatch", err)
	}
}

// ---------- repeated ----------

func TestRepeatedScalar(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "tags", []any{"a", "b", "c"})
	eq(t, getValue(t, m, "tags[1]"), "b")
	eq(t, getValue(t, m, "tags"), []string{"a", "b", "c"})
	if _, err := m.Get("tags[9]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestRepeatedTypedSlice(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "scores", []int32{1, 2, 3})
	eq(t, getValue(t, m, "scores[2]"), int32(3))
	eq(t, getValue(t, m, "scores"), []int32{1, 2, 3})
}

func TestRepeatedMessageMake(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("contacts", make([]*dymsg.Message, 2)); err != nil {
		t.Fatalf("make: %v", err)
	}
	msgs := getValue(t, m, "contacts").([]*dymsg.Message)
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	mustSet(t, m, "contacts[1].phone", "123")
	eq(t, getValue(t, m, "contacts[1].phone"), "123")
}

func TestRepeatedBytes(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "byr", [][]byte{{1}, {2, 3}})
	eq(t, getValue(t, m, "byr"), [][]byte{{1}, {2, 3}})
}

func TestRepeatedBool(t *testing.T) {
	m := mustNewAllTypes(t)
	mustSet(t, m, "br", []bool{true, false, true})
	eq(t, getValue(t, m, "br"), []bool{true, false, true})
}

func TestEmptyRepeated(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "tags", []any{})
	eq(t, getValue(t, m, "tags"), []string{})
}

func TestSetIndexWhenUnset(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("tags[0]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Set tags[0] on unset err = %v, want ErrIndexOutOfRange", err)
	}
}

// ---------- presence ----------

func TestPresenceZeroValueEncoded(t *testing.T) {
	unset := mustNew(t)
	setZero := mustNew(t)
	mustSet(t, setZero, "age", int32(0))

	bu, _ := unset.EncodeProto()
	bs, _ := setZero.EncodeProto()
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
	m1 := mustNew(t)
	mustSet(t, m1, "name", "alice")
	mustSet(t, m1, "addr.city", "bj")
	mustSet(t, m1, "tags", []any{"x"})

	m2 := mustNew(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatalf("copy: %v", err)
	}
	mustSet(t, m2, "name", "bob")
	mustSet(t, m2, "addr.city", "sh")
	mustSet(t, m2, "tags[0]", "y")

	eq(t, getValue(t, m1, "name"), "alice")
	eq(t, getValue(t, m1, "addr.city"), "bj")
	eq(t, getValue(t, m1, "tags[0]"), "x")
}

func TestDeepCopyRepeatedMessage(t *testing.T) {
	m1 := mustNew(t)
	if err := m1.Set("contacts", make([]*dymsg.Message, 1)); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m1, "contacts[0].phone", "111")
	m2 := mustNew(t)
	if err := m2.Set("", m1); err != nil {
		t.Fatal(err)
	}
	mustSet(t, m2, "contacts[0].phone", "222")
	eq(t, getValue(t, m1, "contacts[0].phone"), "111")
}

func TestBytesDeepCopy(t *testing.T) {
	m := mustNew(t)
	b := []byte{1, 2, 3}
	mustSet(t, m, "data", b)
	b[0] = 99
	eq(t, getValue(t, m, "data"), []byte{1, 2, 3})
}

// ---------- 往返 ----------

func TestRoundTripJSON(t *testing.T) {
	m := mustNew(t)
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
	m2 := mustNew(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	eq(t, getValue(t, m2, "name"), "alice")
	eq(t, getValue(t, m2, "age"), int32(30))
	eq(t, getValue(t, m2, "score"), 1.5)
	eq(t, getValue(t, m2, "data"), []byte{0xde, 0xad})
	eq(t, getValue(t, m2, "addr.city"), "bj")
	eq(t, getValue(t, m2, "tags[1]"), "y")
	eq(t, getValue(t, m2, "scores[1]"), int32(-2))
}

func TestRoundTripProto(t *testing.T) {
	m := mustNew(t)
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
	m2 := mustNew(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m2, "name"), "alice")
	eq(t, getValue(t, m2, "age"), int32(-7))
	eq(t, getValue(t, m2, "score"), -2.5)
	eq(t, getValue(t, m2, "addr.city"), "bj")
	eq(t, getValue(t, m2, "scores[1]"), int32(-2))
	eq(t, getValue(t, m2, "tags[0]"), "x")
}

func TestRepeatedMessageProtoRoundTrip(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "contacts", make([]*dymsg.Message, 2))
	mustSet(t, m, "contacts[0].phone", "111")
	mustSet(t, m, "contacts[1].phone", "222")
	data, _ := m.EncodeProto()
	m2 := mustNew(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	eq(t, getValue(t, m2, "contacts[0].phone"), "111")
	eq(t, getValue(t, m2, "contacts[1].phone"), "222")
}

func TestInt32Extremes(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "age", int32(-2147483648))
	data, _ := m.EncodeProto()
	m2 := mustNew(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	eq(t, getValue(t, m2, "age"), int32(-2147483648))
}

func TestEmptyMessageRoundTripProto(t *testing.T) {
	m := mustNew(t)
	data, _ := m.EncodeProto()
	m2 := mustNew(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto(0 bytes) should succeed: %v", err)
	}
	if g, _ := m2.Get("name"); g != nil {
		t.Fatalf("name should be unset")
	}
}

func TestDecodeProtoEmptyBytes(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeProto(nil); err != nil {
		t.Fatalf("DecodeProto(nil): %v", err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be cleared")
	}
}

func TestDecodeJSONEmptyInput(t *testing.T) {
	m := mustNew(t)
	if err := m.DecodeJSON(nil); err != dymsg.ErrTruncated {
		t.Fatalf("DecodeJSON(nil) err = %v, want ErrTruncated", err)
	}
	if err := m.DecodeJSON([]byte("  ")); err != dymsg.ErrTruncated {
		t.Fatalf("DecodeJSON(blank) err = %v, want ErrTruncated", err)
	}
}

func TestDecodeJSONEmptyObject(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "name", "alice")
	if err := m.DecodeJSON([]byte(`{}`)); err != nil {
		t.Fatalf("DecodeJSON({}): %v", err)
	}
	if g, _ := m.Get("name"); g != nil {
		t.Fatalf("name should be unset")
	}
}

// ---------- 转换 ----------

func TestConversion(t *testing.T) {
	m := mustNew(t)
	mustSet(t, m, "age", "18")
	eq(t, getValue(t, m, "age"), int32(18))
	if err := m.Set("age", int64(1<<40)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("overflow err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("age", "abc"); err != dymsg.ErrTypeMismatch {
		t.Fatalf("bad string err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("active", 1); err != dymsg.ErrTypeMismatch {
		t.Fatalf("bool mismatch err = %v, want ErrTypeMismatch", err)
	}
}

// ---------- 契约 ----------

func TestRegisterIdempotent(t *testing.T) {
	cfg := `{"types":[{"typeId":2001,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	sa, _ := dymsg.ParseSchema([]byte(cfg))
	sb, _ := dymsg.ParseSchema([]byte(cfg))
	if err := dymsg.Register(sa[0]); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := dymsg.Register(sb[0]); err != nil {
		t.Fatalf("re-register same content should be idempotent, got %v", err)
	}
}

func TestRegisterDuplicateID(t *testing.T) {
	a := `{"types":[{"typeId":2002,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	b := `{"types":[{"typeId":2002,"fields":[{"name":"b","type":"string","num":1}]}]}`
	sa, _ := dymsg.ParseSchema([]byte(a))
	sb, _ := dymsg.ParseSchema([]byte(b))
	if err := dymsg.Register(sa[0]); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := dymsg.Register(sb[0]); err != dymsg.ErrDuplicateID {
		t.Fatalf("different schema same id err = %v, want ErrDuplicateID", err)
	}
}

func TestNewUnknownTypeID(t *testing.T) {
	if _, err := dymsg.New(59999); err != dymsg.ErrUnknownTypeID {
		t.Fatalf("New unknown err = %v, want ErrUnknownTypeID", err)
	}
}

func TestRegisterConcurrentIdempotent(t *testing.T) {
	cfg := `{"types":[{"typeId":2100,"fields":[{"name":"a","type":"int32","num":1}]}]}`
	var schemas []dymsg.MessageSchema
	for i := 0; i < 8; i++ {
		sc, _ := dymsg.ParseSchema([]byte(cfg))
		schemas = append(schemas, sc[0])
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(schemas)*20)
	for _, s := range schemas {
		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func(s dymsg.MessageSchema) {
				defer wg.Done()
				if err := dymsg.Register(s); err != nil {
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

func TestConcurrentNew(t *testing.T) {
	mustNew(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if _, err := dymsg.New(1001); err != nil {
					t.Errorf("New: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// ---------- 公开 API 全路径不 panic ----------

func TestExternalPathsNoPanic(t *testing.T) {
	m := mustNew(t)
	if err := m.Set("addr", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.Get("addr"); g != nil {
		t.Fatalf("addr should be nil, got %#v", g)
	}

	m.Set("addr.city", "beijing")
	if g, _ := m.Get("addr"); g == nil {
		t.Fatal("addr should be present")
	}

	m2 := mustNew(t)
	if err := m2.Set("", m); err != nil {
		t.Fatal(err)
	}
	if g, _ := m2.Get("addr"); g == nil {
		t.Fatal("copied addr should be present")
	}

	m2.Set("addr", nil)
	m3 := mustNew(t)
	if err := m3.Set("", m2); err != nil {
		t.Fatal(err)
	}
	if g, _ := m3.Get("addr"); g != nil {
		t.Fatalf("addr should be nil after copy of cleared msg, got %#v", g)
	}

	m4 := mustNew(t)
	if err := m4.DecodeJSON([]byte(`{"addr":null}`)); err != nil {
		t.Fatal(err)
	}
	if g, _ := m4.Get("addr"); g != nil {
		t.Fatalf("addr should be nil after null, got %#v", g)
	}
	if err := m4.DecodeJSON([]byte(`{"addr":{"city":"gz"}}`)); err != nil {
		t.Fatal(err)
	}
	if g, _ := m4.Get("addr"); g == nil {
		t.Fatal("addr should be present after object decode")
	}

	m5 := mustNew(t)
	blob := appendTag(nil, 6, 2) // addr 字段(num 6),空子消息
	blob = appendVarint(blob, 0)
	if err := m5.DecodeProto(blob); err != nil {
		t.Fatal(err)
	}
	if g, _ := m5.Get("addr"); g == nil {
		t.Fatal("addr should be present (empty nested message)")
	}

	data, err := m.EncodeProto()
	if err != nil {
		t.Fatal(err)
	}
	m6 := mustNew(t)
	if err := m6.DecodeProto(data); err != nil {
		t.Fatal(err)
	}
	if g, _ := m6.Get("addr"); g == nil {
		t.Fatal("addr should survive round trip")
	}

	m7 := mustNew(t)
	m7.Set("addr.city", "x")
	if err := m7.Set("", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m7.Get("addr"); g != nil {
		t.Fatalf("addr should be nil after full clear, got %#v", g)
	}
}

// ---------- helper ----------

func getValue(t *testing.T, m *dymsg.Message, path string) any {
	t.Helper()
	g, err := m.Get(path)
	if err != nil {
		t.Fatalf("Get(%q): %v", path, err)
	}
	if g == nil {
		t.Fatalf("Get(%q) = nil", path)
	}
	return g.Value()
}
