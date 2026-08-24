package eval

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"

	"dymsg"
)

const richSchema = `{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        {"name":"i32","type":"int32","num":1},
        {"name":"i64","type":"int64","num":2},
        {"name":"u32","type":"uint32","num":3},
        {"name":"u64","type":"uint64","num":4},
        {"name":"f","type":"float","num":5},
        {"name":"d","type":"double","num":6},
        {"name":"b","type":"bool","num":7},
        {"name":"s","type":"string","num":8},
        {"name":"by","type":"bytes","num":9},
        {"name":"msg","type":"message","num":10,"schema":{"fields":[
          {"name":"city","type":"string","num":1},
          {"name":"zip","type":"int32","num":2},
          {"name":"tags","type":"string","num":3,"repeated":true},
          {"name":"sub","type":"message","num":4,"schema":{"fields":[
            {"name":"x","type":"int64","num":1}
          ]}}
        ]}},
        {"name":"ri32","type":"int32","num":11,"repeated":true},
        {"name":"ri64","type":"int64","num":12,"repeated":true},
        {"name":"ru32","type":"uint32","num":13,"repeated":true},
        {"name":"ru64","type":"uint64","num":14,"repeated":true},
        {"name":"rf","type":"float","num":15,"repeated":true},
        {"name":"rd","type":"double","num":16,"repeated":true},
        {"name":"rb","type":"bool","num":17,"repeated":true},
        {"name":"rs","type":"string","num":18,"repeated":true},
        {"name":"rby","type":"bytes","num":19,"repeated":true},
        {"name":"rmsg","type":"message","num":20,"repeated":true,"schema":{"fields":[
          {"name":"city","type":"string","num":1},
          {"name":"zip","type":"int32","num":2}
        ]}}
      ]
    }
  ]
}`

func parseOne(t *testing.T, cfg string) dymsg.MessageSchema {
	t.Helper()
	schemas, err := dymsg.ParseSchema([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("ParseSchema: want 1 schema, got %d", len(schemas))
	}
	return schemas[0]
}

func registerOne(t *testing.T, cfg string) dymsg.MessageSchema {
	t.Helper()
	s := parseOne(t, cfg)
	if err := dymsg.Register(s); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return s
}

func newRich(t *testing.T) *dymsg.Message {
	t.Helper()
	registerOne(t, richSchema)
	m, err := dymsg.New(1001)
	if err != nil {
		t.Fatalf("New(1001): %v", err)
	}
	return m
}

func rmsgElement(t *testing.T, city string, zip int32) *dymsg.Message {
	t.Helper()
	tmp := newRich(t)
	if err := tmp.DecodeJSON([]byte(fmt.Sprintf(`{"rmsg":[{"city":%q,"zip":%d}]}`, city, zip))); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	return tmp.Get("rmsg").Index(0).Message()
}

func populate(t *testing.T, m *dymsg.Message) {
	t.Helper()
	type set struct {
		path  string
		value any
	}
	sets := []set{
		{"i32", int32(-1)},
		{"i64", int64(-2)},
		{"u32", uint32(3)},
		{"u64", uint64(4)},
		{"f", float32(1.5)},
		{"d", float64(-2.25)},
		{"b", true},
		{"s", "hello"},
		{"by", []byte{0, 1, 2, 255}},
		{"msg.city", "beijing"},
		{"msg.zip", int32(100)},
		{"msg.tags", []string{"a", "b"}},
		{"msg.sub.x", int64(99)},
		{"ri32", []int32{1, -2, 3}},
		{"ri64", []int64{4, 5}},
		{"ru32", []uint32{6, 7}},
		{"ru64", []uint64{8, 9}},
		{"rf", []float32{1.5, 2.5}},
		{"rd", []float64{3.5, 4.5}},
		{"rb", []bool{true, false, true}},
		{"rs", []string{"x", "y", "z"}},
		{"rby", [][]byte{{1, 2}, {3, 4}}},
	}
	for _, s := range sets {
		if err := m.Set(s.path, s.value); err != nil {
			t.Fatalf("Set(%q): %v", s.path, err)
		}
	}
	m.Set("rmsg", []*dymsg.Message{rmsgElement(t, "c1", 1), rmsgElement(t, "c2", 2)})
}

func floatEq(a, b float64) bool {
	const eps = 1e-6
	return math.Abs(a-b) < eps
}

func verifyPopulated(t *testing.T, m *dymsg.Message) {
	t.Helper()
	if got := m.Get("i32").Int32(); got != -1 {
		t.Errorf("i32 = %d, want -1", got)
	}
	if got := m.Get("i64").Int64(); got != -2 {
		t.Errorf("i64 = %d, want -2", got)
	}
	if got := m.Get("u32").Uint32(); got != 3 {
		t.Errorf("u32 = %d, want 3", got)
	}
	if got := m.Get("u64").Uint64(); got != 4 {
		t.Errorf("u64 = %d, want 4", got)
	}
	if got := m.Get("f").Float32(); !floatEq(float64(got), 1.5) {
		t.Errorf("f = %v, want 1.5", got)
	}
	if got := m.Get("d").Float64(); !floatEq(got, -2.25) {
		t.Errorf("d = %v, want -2.25", got)
	}
	if got := m.Get("b").Bool(); got != true {
		t.Errorf("b = %v, want true", got)
	}
	if got := m.Get("s").String(); got != "hello" {
		t.Errorf("s = %q, want hello", got)
	}
	if got := m.Get("by").Bytes(); !bytes.Equal(got, []byte{0, 1, 2, 255}) {
		t.Errorf("by = %v", got)
	}
	if got := m.Get("msg.city").String(); got != "beijing" {
		t.Errorf("msg.city = %q", got)
	}
	if got := m.Get("msg.zip").Int32(); got != 100 {
		t.Errorf("msg.zip = %d, want 100", got)
	}
	if got := m.Get("msg.tags").Len(); got != 2 {
		t.Errorf("msg.tags len = %d, want 2", got)
	}
	if got := m.Get("msg.tags").Index(0).String(); got != "a" {
		t.Errorf("msg.tags[0] = %q", got)
	}
	if got := m.Get("msg.sub.x").Int64(); got != 99 {
		t.Errorf("msg.sub.x = %d, want 99", got)
	}
	if got := m.Get("ri32").Int32s(); !reflect.DeepEqual(got, []int32{1, -2, 3}) {
		t.Errorf("ri32 = %v", got)
	}
	if got := m.Get("ri64").Int64s(); !reflect.DeepEqual(got, []int64{4, 5}) {
		t.Errorf("ri64 = %v", got)
	}
	if got := m.Get("ru32").Uint32s(); !reflect.DeepEqual(got, []uint32{6, 7}) {
		t.Errorf("ru32 = %v", got)
	}
	if got := m.Get("ru64").Uint64s(); !reflect.DeepEqual(got, []uint64{8, 9}) {
		t.Errorf("ru64 = %v", got)
	}
	if got := m.Get("rf").Float32s(); len(got) != 2 || !floatEq(float64(got[0]), 1.5) || !floatEq(float64(got[1]), 2.5) {
		t.Errorf("rf = %v", got)
	}
	if got := m.Get("rd").Float64s(); !reflect.DeepEqual(got, []float64{3.5, 4.5}) {
		t.Errorf("rd = %v", got)
	}
	if got := m.Get("rb").Bools(); !reflect.DeepEqual(got, []bool{true, false, true}) {
		t.Errorf("rb = %v", got)
	}
	if got := m.Get("rs").Strings(); !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Errorf("rs = %v", got)
	}
	if got := m.Get("rby").BytesSlice(); len(got) != 2 || !bytes.Equal(got[0], []byte{1, 2}) || !bytes.Equal(got[1], []byte{3, 4}) {
		t.Errorf("rby = %v", got)
	}
	// Len/Index across every repeated slice type (exercises sliceLen/sliceIndex).
	for _, p := range []string{"ri32", "ri64", "ru32", "ru64", "rf", "rd", "rb", "rs", "rby", "rmsg"} {
		if got := m.Get(p).Len(); got == 0 {
			t.Errorf("%s Len = 0", p)
		}
		if got := m.Get(p).Index(0).Any(); got == nil {
			t.Errorf("%s Index(0).Any() = nil", p)
		}
	}
	if got := m.Get("rmsg").Messages(); len(got) != 2 {
		t.Errorf("rmsg.Messages() len = %d, want 2", len(got))
	}
	if got := m.Get("rmsg").Len(); got != 2 {
		t.Fatalf("rmsg len = %d, want 2", got)
	}
	if got := m.Get("rmsg").Index(0).Message().Get("city").String(); got != "c1" {
		t.Errorf("rmsg[0].city = %q", got)
	}
	if got := m.Get("rmsg").Index(1).Message().Get("zip").Int32(); got != 2 {
		t.Errorf("rmsg[1].zip = %d", got)
	}
}

func TestParseSchemaValid(t *testing.T) {
	cfg := `{"types":[
		{"typeId":3001,"fields":[{"name":"a","type":"int32","num":1}]},
		{"typeId":3002,"fields":[{"name":"b","type":"string","num":1}]}
	]}`
	schemas, err := dymsg.ParseSchema([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(schemas) != 2 {
		t.Fatalf("len = %d, want 2", len(schemas))
	}
	for _, s := range schemas {
		if err := dymsg.Register(s); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	if _, err := dymsg.New(3001); err != nil {
		t.Fatalf("New(3001): %v", err)
	}
	if _, err := dymsg.New(3002); err != nil {
		t.Fatalf("New(3002): %v", err)
	}
}

func TestParseSchemaEmpty(t *testing.T) {
	for _, cfg := range []string{`{}`, `{"types":[]}`, `{"types":null}`} {
		schemas, err := dymsg.ParseSchema([]byte(cfg))
		if err != nil {
			t.Fatalf("ParseSchema(%s): %v", cfg, err)
		}
		if len(schemas) != 0 {
			t.Fatalf("ParseSchema(%s): len = %d, want 0", cfg, len(schemas))
		}
	}
}

func TestParseSchemaMalformed(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"invalid json", `{`},
		{"typeId zero", `{"types":[{"typeId":0,"fields":[{"name":"a","type":"int32","num":1}]}]}`},
		{"typeId too large", `{"types":[{"typeId":65536,"fields":[{"name":"a","type":"int32","num":1}]}]}`},
		{"typeId negative", `{"types":[{"typeId":-1,"fields":[{"name":"a","type":"int32","num":1}]}]}`},
		{"duplicate typeId", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1}]},{"typeId":1,"fields":[{"name":"b","type":"int32","num":1}]}]}`},
		{"empty name", `{"types":[{"typeId":1,"fields":[{"name":"","type":"int32","num":1}]}]}`},
		{"name with dot", `{"types":[{"typeId":1,"fields":[{"name":"a.b","type":"int32","num":1}]}]}`},
		{"name with open bracket", `{"types":[{"typeId":1,"fields":[{"name":"a[b","type":"int32","num":1}]}]}`},
		{"name with close bracket", `{"types":[{"typeId":1,"fields":[{"name":"a]b","type":"int32","num":1}]}]}`},
		{"invalid type", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"nope","num":1}]}]}`},
		{"num zero", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":0}]}]}`},
		{"num too large", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":65536}]}]}`},
		{"num negative", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":-1}]}]}`},
		{"duplicate num", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"b","type":"string","num":1}]}]}`},
		{"duplicate name", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1},{"name":"a","type":"string","num":2}]}]}`},
		{"non-message with schema", `{"types":[{"typeId":1,"fields":[{"name":"a","type":"int32","num":1,"schema":{"fields":[]}}]}]}`},
		{"message missing schema", `{"types":[{"typeId":1,"fields":[{"name":"m","type":"message","num":1}]}]}`},
		{"message null schema", `{"types":[{"typeId":1,"fields":[{"name":"m","type":"message","num":1,"schema":null}]}]}`},
		{"message bad schema", `{"types":[{"typeId":1,"fields":[{"name":"m","type":"message","num":1,"schema":[1,2]}]}]}`},
		{"nested duplicate name", `{"types":[{"typeId":1,"fields":[{"name":"m","type":"message","num":1,"schema":{"fields":[{"name":"x","type":"int32","num":1},{"name":"x","type":"string","num":2}]}}]}]}`},
		{"nested empty name", `{"types":[{"typeId":1,"fields":[{"name":"m","type":"message","num":1,"schema":{"fields":[{"name":"","type":"int32","num":1}]}}]}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dymsg.ParseSchema([]byte(tc.cfg))
			if err != dymsg.ErrMalformedData {
				t.Fatalf("err = %v, want ErrMalformedData", err)
			}
		})
	}
}

func TestRegisterDuplicate(t *testing.T) {
	a := parseOne(t, `{"types":[{"typeId":9001,"fields":[{"name":"x","type":"int32","num":1}]}]}`)
	if err := dymsg.Register(a); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := dymsg.Register(a); err != nil {
		t.Fatalf("idempotent Register: %v", err)
	}
	b := parseOne(t, `{"types":[{"typeId":9001,"fields":[{"name":"y","type":"int32","num":1}]}]}`)
	if err := dymsg.Register(b); err != dymsg.ErrDuplicateID {
		t.Fatalf("err = %v, want ErrDuplicateID", err)
	}
}

func TestNewUnknownTypeID(t *testing.T) {
	if _, err := dymsg.New(60000); err != dymsg.ErrUnknownTypeID {
		t.Fatalf("err = %v, want ErrUnknownTypeID", err)
	}
}

func TestTypeIDBoundaries(t *testing.T) {
	cfg := `{"types":[
		{"typeId":1,"fields":[{"name":"a","type":"int32","num":1}]},
		{"typeId":65535,"fields":[{"name":"b","type":"string","num":1}]}
	]}`
	schemas, err := dymsg.ParseSchema([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	for _, s := range schemas {
		if err := dymsg.Register(s); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	if _, err := dymsg.New(1); err != nil {
		t.Fatalf("New(1): %v", err)
	}
	if _, err := dymsg.New(65535); err != nil {
		t.Fatalf("New(65535): %v", err)
	}
}

func TestGetSelf(t *testing.T) {
	m := newRich(t)
	v := m.Get("")
	if !v.Exists() || !v.IsSet() || v.Err() != nil {
		t.Fatalf("Get(\"\") state = %v/%v/%v", v.Exists(), v.IsSet(), v.Err())
	}
	if v.Message() != m {
		t.Fatalf("Get(\"\").Message() != m")
	}
	if v.Any() != m {
		t.Fatalf("Get(\"\").Any() != m")
	}
}

func TestGetUnsetField(t *testing.T) {
	m := newRich(t)
	v := m.Get("s")
	if !v.Exists() {
		t.Fatalf("Exists = false, want true")
	}
	if v.IsSet() {
		t.Fatalf("IsSet = true, want false")
	}
	if v.Err() != nil {
		t.Fatalf("Err = %v, want nil", v.Err())
	}
	if v.Any() != nil || v.String() != "" {
		t.Fatalf("unset getter should be zero/nil")
	}
}

func TestGetUnknownField(t *testing.T) {
	m := newRich(t)
	v := m.Get("unknown")
	if v.Exists() || v.IsSet() {
		t.Fatalf("Exists/IsSet should be false")
	}
	if v.Err() != dymsg.ErrFieldNotFound {
		t.Fatalf("Err = %v, want ErrFieldNotFound", v.Err())
	}
	if v.Any() != nil || v.String() != "" || v.Int32() != 0 {
		t.Fatalf("unknown getter should be zero/nil")
	}
}

func TestGetScalar(t *testing.T) {
	m := newRich(t)
	m.Set("i32", int32(42))
	v := m.Get("i32")
	if !v.Exists() || !v.IsSet() || v.Err() != nil {
		t.Fatalf("state = %v/%v/%v", v.Exists(), v.IsSet(), v.Err())
	}
	if v.Int32() != 42 || v.Int64() != 0 || v.String() != "" {
		t.Fatalf("scalar getters wrong")
	}
	if v.Any() != int32(42) {
		t.Fatalf("Any = %#v, want int32(42)", v.Any())
	}
}

func TestValueTypeMismatchGetters(t *testing.T) {
	m := newRich(t)
	m.Set("msg.city", "bj")
	msgV := m.Get("msg")
	if !msgV.IsSet() || msgV.Message() == nil {
		t.Fatalf("msg not set")
	}
	if msgV.String() != "" || msgV.Int32() != 0 {
		t.Fatalf("scalar getters on message should be zero")
	}
	if msgV.Any() != msgV.Message() {
		t.Fatalf("Any() on message should be the *Message")
	}
}

func TestRepeatedValueAccessors(t *testing.T) {
	m := newRich(t)
	m.Set("rs", []string{"a", "b", "c"})
	v := m.Get("rs")
	if v.Len() != 3 {
		t.Fatalf("Len = %d, want 3", v.Len())
	}
	if got := v.Index(0).String(); got != "a" {
		t.Fatalf("Index(0) = %q", got)
	}
	if got := v.Index(2).String(); got != "c" {
		t.Fatalf("Index(2) = %q", got)
	}
	if v.Index(3).Err() != dymsg.ErrIndexOutOfRange || v.Index(-1).Err() != dymsg.ErrIndexOutOfRange {
		t.Fatalf("out of range should be ErrIndexOutOfRange")
	}
	if got := v.Strings(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Strings() = %v", got)
	}
	// non-repeated Len and Index
	sv := m.Get("s")
	if sv.Len() != 0 {
		t.Fatalf("non-repeated Len = %d, want 0", sv.Len())
	}
	if sv.Index(0).Err() != dymsg.ErrIndexOutOfRange {
		t.Fatalf("non-repeated Index should be ErrIndexOutOfRange")
	}
	// unset repeated
	uv := m.Get("ri32")
	if uv.Len() != 0 || uv.Int32s() != nil {
		t.Fatalf("unset repeated should be zero/nil")
	}
}

func TestPathErrors(t *testing.T) {
	m := newRich(t)
	m.Set("rs", []string{"a"})
	cases := []struct {
		path string
		want error
	}{
		{"unknown", dymsg.ErrFieldNotFound},
		{"s[0]", dymsg.ErrFieldNotFound},
		{"s.x", dymsg.ErrFieldNotFound},
		{"rs[0].x", dymsg.ErrFieldNotFound},
		{"msg.city.x", dymsg.ErrFieldNotFound},
		{"rs[-1]", dymsg.ErrIndexOutOfRange},
		{"rs[abc]", dymsg.ErrIndexOutOfRange},
		{"rs[]", dymsg.ErrIndexOutOfRange},
		{"rs[999999999999999999999999]", dymsg.ErrIndexOutOfRange},
		{"rs[0", dymsg.ErrFieldNotFound},
		{"rs[0]x", dymsg.ErrFieldNotFound},
		{".rs", dymsg.ErrFieldNotFound},
		{"rs.", dymsg.ErrFieldNotFound},
		{"rs..x", dymsg.ErrFieldNotFound},
		{"rs[1]", dymsg.ErrIndexOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			v := m.Get(tc.path)
			if v.Err() != tc.want {
				t.Fatalf("Get(%q).Err() = %v, want %v", tc.path, v.Err(), tc.want)
			}
			if v.Exists() {
				t.Fatalf("Get(%q).Exists() = true, want false", tc.path)
			}
		})
	}
}

func TestSetScalar(t *testing.T) {
	m := newRich(t)
	if err := m.Set("i32", int32(5)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if m.Get("i32").Int32() != 5 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
}

func TestSetConversion(t *testing.T) {
	m := newRich(t)
	type myInt int
	type myString string
	type myBool bool
	type myBytes []byte
	cases := []struct {
		path  string
		value any
		check func()
	}{
		{"i64", int(7), func() {
			if m.Get("i64").Int64() != 7 {
				t.Fatalf("i64 = %d", m.Get("i64").Int64())
			}
		}},
		{"i32", float64(3.9), func() {
			if m.Get("i32").Int32() != 3 {
				t.Fatalf("i32 = %d", m.Get("i32").Int32())
			}
		}},
		{"i32", "-12", func() {
			if m.Get("i32").Int32() != -12 {
				t.Fatalf("i32 = %d", m.Get("i32").Int32())
			}
		}},
		{"s", int32(123), func() {
			if m.Get("s").String() != "123" {
				t.Fatalf("s = %q", m.Get("s").String())
			}
		}},
		{"s", []byte("raw"), func() {
			if m.Get("s").String() != "raw" {
				t.Fatalf("s = %q", m.Get("s").String())
			}
		}},
		{"by", "bytes", func() {
			if !bytes.Equal(m.Get("by").Bytes(), []byte("bytes")) {
				t.Fatalf("by = %v", m.Get("by").Bytes())
			}
		}},
		{"u32", myInt(9), func() {
			if m.Get("u32").Uint32() != 9 {
				t.Fatalf("u32 = %d", m.Get("u32").Uint32())
			}
		}},
		{"s", myString("ms"), func() {
			if m.Get("s").String() != "ms" {
				t.Fatalf("s = %q", m.Get("s").String())
			}
		}},
		{"b", myBool(true), func() {
			if m.Get("b").Bool() != true {
				t.Fatalf("b = %v", m.Get("b").Bool())
			}
		}},
		{"s", myBytes("mb"), func() {
			if m.Get("s").String() != "mb" {
				t.Fatalf("s = %q", m.Get("s").String())
			}
		}},
		{"by", myBytes{1, 2}, func() {
			if !bytes.Equal(m.Get("by").Bytes(), []byte{1, 2}) {
				t.Fatalf("by = %v", m.Get("by").Bytes())
			}
		}},
	}
	for _, tc := range cases {
		if err := m.Set(tc.path, tc.value); err != nil {
			t.Fatalf("Set(%q, %#v): %v", tc.path, tc.value, err)
		}
		tc.check()
	}
}

func TestSetTypeMismatch(t *testing.T) {
	m := newRich(t)
	cases := []struct {
		path  string
		value any
	}{
		{"i32", "not-a-number"},
		{"i32", math.NaN()},
		{"i32", math.Inf(1)},
		{"i32", 2147483648},
		{"i32", -2147483649},
		{"u32", -1},
		{"u32", 4294967296},
		{"f", 3.5e38},
		{"b", "true"},
		{"s", struct{}{}},
		{"by", 123},
		{"msg", "not-a-message"},
		{"ri32", []string{"a"}},
		{"rmsg", []int{1}},
	}
	for _, tc := range cases {
		if err := m.Set(tc.path, tc.value); err != dymsg.ErrTypeMismatch {
			t.Errorf("Set(%q, %#v) err = %v, want ErrTypeMismatch", tc.path, tc.value, err)
		}
	}
}

func TestSetNilClears(t *testing.T) {
	m := newRich(t)
	m.Set("s", "x")
	if err := m.Set("s", nil); err != nil {
		t.Fatalf("Set nil: %v", err)
	}
	if m.Get("s").IsSet() {
		t.Fatalf("s should be unset after Set nil")
	}
	m.Set("msg.city", "bj")
	if err := m.Set("msg", nil); err != nil {
		t.Fatalf("Set msg nil: %v", err)
	}
	if m.Get("msg").IsSet() {
		t.Fatalf("msg should be unset")
	}
}

func TestSetSelf(t *testing.T) {
	m := newRich(t)
	src := newRich(t)
	src.Set("s", "hello")
	if err := m.Set("", src); err != nil {
		t.Fatalf("Set(\"\", src): %v", err)
	}
	if m.Get("s").String() != "hello" {
		t.Fatalf("s = %q", m.Get("s").String())
	}
	// deep copy independence
	src.Set("s", "changed")
	if m.Get("s").String() != "hello" {
		t.Fatalf("deep copy violated")
	}

	if err := m.Set("", nil); err != nil {
		t.Fatalf("Set(\"\", nil): %v", err)
	}
	if len(m.SetFields()) != 0 {
		t.Fatalf("Set(\"\", nil) should clear all")
	}
	if err := m.Set("", 42); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Set(\"\", 42) err = %v, want ErrTypeMismatch", err)
	}
}

func TestSetSelfDifferentSchema(t *testing.T) {
	m := newRich(t)
	other := registerOne(t, `{"types":[{"typeId":7001,"fields":[{"name":"x","type":"int32","num":1}]}]}`)
	om, _ := dymsg.New(7001)
	if err := m.Set("", om); err != dymsg.ErrTypeMismatch {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("msg", om); err != dymsg.ErrTypeMismatch {
		t.Fatalf("msg err = %v, want ErrTypeMismatch", err)
	}
	_ = other
}

func TestSetNestedAutoCreate(t *testing.T) {
	m := newRich(t)
	if err := m.Set("msg.city", "beijing"); err != nil {
		t.Fatalf("Set msg.city: %v", err)
	}
	if m.Get("msg").IsSet() != true {
		t.Fatalf("msg should be auto-created")
	}
	if m.Get("msg.city").String() != "beijing" {
		t.Fatalf("msg.city = %q", m.Get("msg.city").String())
	}
}

func TestSetRepeated(t *testing.T) {
	m := newRich(t)
	if err := m.Set("rs", []string{"a", "b"}); err != nil {
		t.Fatalf("Set rs: %v", err)
	}
	if got := m.Get("rs").Strings(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("rs = %v", got)
	}
	// array input
	if err := m.Set("ri32", [3]int32{1, 2, 3}); err != nil {
		t.Fatalf("Set ri32 array: %v", err)
	}
	if got := m.Get("ri32").Int32s(); !reflect.DeepEqual(got, []int32{1, 2, 3}) {
		t.Fatalf("ri32 = %v", got)
	}
	// element set
	if err := m.Set("rs[0]", "x"); err != nil {
		t.Fatalf("Set rs[0]: %v", err)
	}
	if m.Get("rs").Index(0).String() != "x" {
		t.Fatalf("rs[0] = %q", m.Get("rs").Index(0).String())
	}
	// element set on unset repeated
	unset := newRich(t)
	if err := unset.Set("rs[0]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Set unset rs[0] err = %v, want ErrIndexOutOfRange", err)
	}
	// element out of range
	if err := m.Set("rs[9]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Set rs[9] err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestAppend(t *testing.T) {
	m := newRich(t)
	if err := m.Append("rs", "a"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := m.Append("rs", "b"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := m.Get("rs").Strings(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("rs = %v", got)
	}
	// non-repeated
	if err := m.Append("s", "x"); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Append non-repeated err = %v, want ErrTypeMismatch", err)
	}
	// unknown
	if err := m.Append("unknown", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Append unknown err = %v, want ErrFieldNotFound", err)
	}
	// with index path
	if err := m.Append("rs[0]", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Append with index err = %v, want ErrFieldNotFound", err)
	}
	// empty path
	if err := m.Append("", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Append empty path err = %v, want ErrFieldNotFound", err)
	}
	// conversion failure
	if err := m.Append("ri32", "nope"); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Append conversion err = %v, want ErrTypeMismatch", err)
	}
	// message element append and deep copy
	elem := rmsgElement(t, "city", 7)
	if err := m.Append("rmsg", elem); err != nil {
		t.Fatalf("Append message: %v", err)
	}
	elem.Set("city", "changed")
	if got := m.Get("rmsg").Index(0).Message().Get("city").String(); got != "city" {
		t.Fatalf("rmsg[0].city = %q, want city (deep copy)", got)
	}
	// message schema mismatch
	other, _ := dymsg.New(7001)
	if err := m.Append("rmsg", other); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Append mismatched message err = %v, want ErrTypeMismatch", err)
	}
	// nil message element is not allowed
	if err := m.Append("rmsg", nil); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Append nil message err = %v, want ErrTypeMismatch", err)
	}
}

func TestClear(t *testing.T) {
	m := newRich(t)
	m.Set("s", "x")
	if err := m.Clear("s"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if m.Get("s").IsSet() {
		t.Fatalf("s should be unset")
	}
	// idempotent
	if err := m.Clear("s"); err != nil {
		t.Fatalf("Clear unset: %v", err)
	}
	// unknown
	if err := m.Clear("unknown"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Clear unknown err = %v, want ErrFieldNotFound", err)
	}
	// clear nested
	m.Set("msg.city", "bj")
	if err := m.Clear("msg.city"); err != nil {
		t.Fatalf("Clear msg.city: %v", err)
	}
	if m.Get("msg.city").IsSet() {
		t.Fatalf("msg.city should be unset")
	}
	// clear whole message
	m.Set("s", "x")
	if err := m.Clear(""); err != nil {
		t.Fatalf("Clear \"\": %v", err)
	}
	if len(m.SetFields()) != 0 {
		t.Fatalf("Clear(\"\") should clear all")
	}
}

func TestClearPathIdempotentWhenParentUnset(t *testing.T) {
	m := newRich(t)
	if err := m.Clear("msg.city"); err != nil {
		t.Fatalf("Clear unset nested: %v", err)
	}
	if err := m.Clear("msg.sub.x"); err != nil {
		t.Fatalf("Clear unset deep nested: %v", err)
	}
	if err := m.Clear("msg.unknown"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Clear unset nested unknown err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Clear("msg.tags[0]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Clear unset nested indexed err = %v, want ErrIndexOutOfRange", err)
	}
	if err := m.Clear("rmsg[0]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Clear unset repeated indexed err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestClearRepeatedElement(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{rmsgElement(t, "a", 1), rmsgElement(t, "b", 2)})
	if err := m.Clear("rmsg[0]"); err != nil {
		t.Fatalf("Clear rmsg[0]: %v", err)
	}
	if m.Get("rmsg").Len() != 2 {
		t.Fatalf("len = %d, want 2", m.Get("rmsg").Len())
	}
	if m.Get("rmsg").Index(0).Message() != nil {
		t.Fatalf("rmsg[0] should be nil after clear")
	}
	// scalar repeated element clear is invalid
	m.Set("rs", []string{"a", "b"})
	if err := m.Clear("rs[0]"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Clear scalar repeated element err = %v, want ErrFieldNotFound", err)
	}
}

func TestHasAndSetFields(t *testing.T) {
	m := newRich(t)
	if m.Has("s") {
		t.Fatalf("Has unset = true")
	}
	m.Set("s", "x")
	if !m.Has("s") {
		t.Fatalf("Has set = false")
	}
	if m.Has("unknown") {
		t.Fatalf("Has unknown = true")
	}
	if !m.Has("") {
		t.Fatalf("Has(\"\") = false")
	}
	m.Set("i32", int32(1))
	got := m.SetFields()
	want := []string{"i32", "s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetFields = %v, want %v", got, want)
	}
	m.Clear("")
	if got := m.SetFields(); len(got) != 0 {
		t.Fatalf("SetFields after clear = %v, want empty", got)
	}
}

func TestDeepCopyNested(t *testing.T) {
	src := newRich(t)
	src.Set("msg.city", "beijing")
	dst := newRich(t)
	if err := dst.Set("msg", src.Get("msg").Message()); err != nil {
		t.Fatalf("Set msg: %v", err)
	}
	src.Set("msg.city", "changed")
	if got := dst.Get("msg.city").String(); got != "beijing" {
		t.Fatalf("deep copy violated: %q", got)
	}
}

func TestDeepCopyRepeatedMessage(t *testing.T) {
	e1 := rmsgElement(t, "beijing", 100)
	dst := newRich(t)
	if err := dst.Set("rmsg", []*dymsg.Message{e1, nil}); err != nil {
		t.Fatalf("Set rmsg: %v", err)
	}
	e1.Set("city", "shanghai")
	if got := dst.Get("rmsg").Index(0).Message().Get("city").String(); got != "beijing" {
		t.Fatalf("deep copy violated: %q", got)
	}
	if dst.Get("rmsg").Index(1).Message() != nil {
		t.Fatalf("rmsg[1] should be nil")
	}
	if dst.Get("rmsg").Len() != 2 {
		t.Fatalf("len = %d", dst.Get("rmsg").Len())
	}
}

func TestDeepCopyBytes(t *testing.T) {
	m := newRich(t)
	orig := []byte{1, 2, 3}
	m.Set("by", orig)
	orig[0] = 9
	if !bytes.Equal(m.Get("by").Bytes(), []byte{1, 2, 3}) {
		t.Fatalf("bytes deep copy violated: %v", m.Get("by").Bytes())
	}
}

func TestEncodeJSONOrderingAndOmission(t *testing.T) {
	m := newRich(t)
	m.Set("i32", int32(0))
	m.Set("b", true)
	m.Set("s", "hi")
	got, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	want := `{"i32":0,"b":true,"s":"hi"}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEncodeJSONBytesBase64(t *testing.T) {
	m := newRich(t)
	m.Set("by", []byte{1, 2})
	got, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	want := `{"by":"AQI="}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEncodeJSONRepeatedMessageNil(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{nil, rmsgElement(t, "a", 1)})
	got, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	want := `{"rmsg":[null,{"city":"a","zip":1}]}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEncodeJSONNonFiniteFloatFails(t *testing.T) {
	m := newRich(t)
	m.Set("d", math.NaN())
	if _, err := m.EncodeJSON(); err == nil {
		t.Fatalf("NaN should fail encode")
	}
	m2 := newRich(t)
	m2.Set("f", float32(math.Inf(1)))
	if _, err := m2.EncodeJSON(); err == nil {
		t.Fatalf("Inf should fail encode")
	}
}

func TestDecodeJSONRoundTrip(t *testing.T) {
	m := newRich(t)
	populate(t, m)
	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	dst := newRich(t)
	if err := dst.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	verifyPopulated(t, dst)
}

func TestDecodeJSONEmpty(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeJSON(nil); err != dymsg.ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
	if err := m.DecodeJSON([]byte{}); err != dymsg.ErrTruncated {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestDecodeJSONTopLevelNull(t *testing.T) {
	m := newRich(t)
	m.Set("s", "x")
	if err := m.DecodeJSON([]byte("null")); err != nil {
		t.Fatalf("DecodeJSON null: %v", err)
	}
	if len(m.SetFields()) != 0 {
		t.Fatalf("top-level null should clear all")
	}
}

func TestDecodeJSONTopLevelNonObject(t *testing.T) {
	for _, input := range []string{`[]`, `"x"`, `123`, `true`} {
		m := newRich(t)
		if err := m.DecodeJSON([]byte(input)); err != dymsg.ErrMalformedData {
			t.Errorf("DecodeJSON(%s) err = %v, want ErrMalformedData", input, err)
		}
	}
}

func TestDecodeJSONUnknownKeyIgnored(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeJSON([]byte(`{"unknown":1,"s":"hi"}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if m.Get("s").String() != "hi" {
		t.Fatalf("s = %q", m.Get("s").String())
	}
	if len(m.SetFields()) != 1 {
		t.Fatalf("SetFields = %v", m.SetFields())
	}
}

func TestDecodeJSONFieldNull(t *testing.T) {
	m := newRich(t)
	m.Set("s", "x")
	if err := m.DecodeJSON([]byte(`{"s":null}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if m.Get("s").IsSet() {
		t.Fatalf("s should be unset")
	}
}

func TestDecodeJSONScalarRepeatedNull(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeJSON([]byte(`{"rs":[1,null]}`)); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestDecodeJSONMessageRepeatedNull(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeJSON([]byte(`{"rmsg":[null,{"city":"a","zip":1}]}`)); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if m.Get("rmsg").Len() != 2 {
		t.Fatalf("len = %d", m.Get("rmsg").Len())
	}
	if m.Get("rmsg").Index(0).Message() != nil {
		t.Fatalf("rmsg[0] should be nil")
	}
	if m.Get("rmsg").Index(1).Message().Get("city").String() != "a" {
		t.Fatalf("rmsg[1].city wrong")
	}
}

func TestDecodeJSONTypeMismatch(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"string to number", `{"i32":"18"}`},
		{"number to string", `{"s":18}`},
		{"number to bool", `{"b":1}`},
		{"array to scalar", `{"s":[1]}`},
		{"scalar to repeated", `{"rs":"x"}`},
		{"object to message missing", `{"msg":123}`},
		{"int32 overflow", `{"i32":2147483648}`},
		{"uint32 overflow", `{"u32":4294967296}`},
		{"invalid base64", `{"by":"!!!"}`},
		{"trailing content", `{"s":"a"}{"s":"b"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newRich(t)
			if err := m.DecodeJSON([]byte(tc.input)); err != dymsg.ErrMalformedData {
				t.Fatalf("err = %v, want ErrMalformedData", err)
			}
		})
	}
}

// proto wire helpers (test-local)
func varintBytes(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func wireKey(num, wt int) []byte {
	return varintBytes(uint64(num)<<3 | uint64(wt))
}

func lengthBytes(b []byte) []byte {
	return append(varintBytes(uint64(len(b))), b...)
}

func TestDecodeProtoRoundTrip(t *testing.T) {
	m := newRich(t)
	populate(t, m)
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	dst := newRich(t)
	if err := dst.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	verifyPopulated(t, dst)
}

func TestDecodeProtoEmpty(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeProto(nil); err != nil {
		t.Fatalf("DecodeProto(empty): %v", err)
	}
	if len(m.SetFields()) != 0 {
		t.Fatalf("empty proto should be empty message")
	}
}

func TestDecodeProtoErrors(t *testing.T) {
	cases := []struct {
		name  string
		data  []byte
		check func(*dymsg.Message) // optional, runs only when nil error expected
	}{
		{"field number zero", varintBytes(0), nil},
		{"invalid wire type 6", wireKey(1, 6), nil},
		{"invalid wire type 7", wireKey(1, 7), nil},
		{"wire type mismatch", append(wireKey(1, 2), lengthBytes([]byte{7})...), nil},
		{"truncated key varint", []byte{0x80}, nil},
		{"truncated length", append(wireKey(8, 2), varintBytes(100)...), nil},
		{"unknown field truncated", append(wireKey(99, 2), append(varintBytes(10), 1)...), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newRich(t)
			if err := m.DecodeProto(tc.data); err == nil {
				t.Fatalf("expected error")
			} else if err != dymsg.ErrMalformedData && err != dymsg.ErrTruncated {
				t.Fatalf("err = %v, want ErrMalformedData or ErrTruncated", err)
			}
		})
	}
}

func TestDecodeProtoFieldNumberZeroIsMalformed(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeProto(varintBytes(0)); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestDecodeProtoWireTypeMismatch(t *testing.T) {
	m := newRich(t)
	data := append(wireKey(1, 2), lengthBytes([]byte{7})...)
	if err := m.DecodeProto(data); err != dymsg.ErrMalformedData {
		t.Fatalf("err = %v, want ErrMalformedData", err)
	}
}

func TestDecodeProtoUnknownFieldSkip(t *testing.T) {
	m := newRich(t)
	// unknown varint field 99 = 5, then known i32 field 1 = 7
	data := append(wireKey(99, 0), varintBytes(5)...)
	data = append(data, wireKey(1, 0)...)
	data = append(data, varintBytes(7)...)
	if err := m.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if m.Get("i32").Int32() != 7 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
}

func TestDecodeProtoUnknownLengthDelimitedSkip(t *testing.T) {
	m := newRich(t)
	data := append(wireKey(99, 2), lengthBytes([]byte("ab"))...)
	data = append(data, wireKey(1, 0)...)
	data = append(data, varintBytes(9)...)
	if err := m.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if m.Get("i32").Int32() != 9 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
}

func TestDecodeProtoPackedUnpackedMixed(t *testing.T) {
	m := newRich(t)
	// packed [1,2], then unpacked 3, then packed [4]
	data := append(wireKey(11, 2), varintBytes(2)...)
	data = append(data, varintBytes(1)...)
	data = append(data, varintBytes(2)...)
	data = append(data, wireKey(11, 0)...)
	data = append(data, varintBytes(3)...)
	data = append(data, wireKey(11, 2)...)
	data = append(data, varintBytes(1)...)
	data = append(data, varintBytes(4)...)
	if err := m.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if got := m.Get("ri32").Int32s(); !reflect.DeepEqual(got, []int32{1, 2, 3, 4}) {
		t.Fatalf("ri32 = %v", got)
	}
}

func TestDecodeProtoRepeatedMessageNilSkip(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{nil, rmsgElement(t, "a", 1)})
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	dst := newRich(t)
	if err := dst.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if dst.Get("rmsg").Len() != 1 {
		t.Fatalf("len = %d, want 1 (nil skipped)", dst.Get("rmsg").Len())
	}
	if dst.Get("rmsg").Index(0).Message().Get("city").String() != "a" {
		t.Fatalf("rmsg[0].city wrong")
	}
}

func TestDecodeProtoBoolFalseAndNegativeInt(t *testing.T) {
	m := newRich(t)
	m.Set("b", false)
	m.Set("i32", int32(-1))
	m.Set("i64", int64(-2))
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	dst := newRich(t)
	if err := dst.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if !dst.Get("b").IsSet() || dst.Get("b").Bool() != false {
		t.Fatalf("b = %v (set=%v)", dst.Get("b").Bool(), dst.Get("b").IsSet())
	}
	if dst.Get("i32").Int32() != -1 {
		t.Fatalf("i32 = %d", dst.Get("i32").Int32())
	}
	if dst.Get("i64").Int64() != -2 {
		t.Fatalf("i64 = %d", dst.Get("i64").Int64())
	}
}

func TestConcurrentRegisterNew(t *testing.T) {
	cfg := `{"types":[{"typeId":8001,"fields":[{"name":"x","type":"int32","num":1}]}]}`
	schemas, err := dymsg.ParseSchema([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if err := dymsg.Register(schemas[0]); err != nil {
		t.Fatalf("Register: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = dymsg.Register(schemas[0])
		}()
		go func() {
			defer wg.Done()
			_, _ = dymsg.New(8001)
		}()
	}
	wg.Wait()
	if _, err := dymsg.New(8001); err != nil {
		t.Fatalf("New after concurrency: %v", err)
	}
}

func TestDeepCopyFullMessage(t *testing.T) {
	src := newRich(t)
	populate(t, src)
	dst := newRich(t)
	if err := dst.Set("", src); err != nil {
		t.Fatalf("Set(\"\", src): %v", err)
	}
	verifyPopulated(t, dst)

	// Mutate every category in src; dst must stay independent.
	src.Set("s", "changed")
	src.Set("by", []byte{9})
	src.Set("msg.city", "changed")
	src.Set("ri32", []int32{9})
	src.Get("rmsg").Index(0).Message().Set("city", "changed")

	if got := dst.Get("s").String(); got != "hello" {
		t.Fatalf("s deep copy violated: %q", got)
	}
	if got := dst.Get("by").Bytes(); !bytes.Equal(got, []byte{0, 1, 2, 255}) {
		t.Fatalf("by deep copy violated: %v", got)
	}
	if got := dst.Get("msg.city").String(); got != "beijing" {
		t.Fatalf("msg deep copy violated: %q", got)
	}
	if got := dst.Get("ri32").Int32s(); !reflect.DeepEqual(got, []int32{1, -2, 3}) {
		t.Fatalf("ri32 deep copy violated: %v", got)
	}
	if got := dst.Get("rmsg").Index(0).Message().Get("city").String(); got != "c1" {
		t.Fatalf("rmsg deep copy violated: %q", got)
	}
}

func TestScalarConversionSuccess(t *testing.T) {
	m := newRich(t)
	intVals := []any{int(1), int8(2), int16(3), int32(4), int64(5), uint(6), uint8(7), uint16(8), uint32(9), uint64(10), float32(11.9), float64(12.9), "13"}
	for _, v := range intVals {
		if err := m.Set("i64", v); err != nil {
			t.Fatalf("Set(i64, %#v): %v", v, err)
		}
	}
	uintVals := []any{int(1), int8(2), int16(3), int32(4), int64(5), uint(6), uint8(7), uint16(8), uint32(9), uint64(10), float32(11.9), float64(12.9), "13"}
	for _, v := range uintVals {
		if err := m.Set("u64", v); err != nil {
			t.Fatalf("Set(u64, %#v): %v", v, err)
		}
	}
	floatVals := []any{int(1), int8(2), int16(3), int32(4), int64(5), uint(6), uint8(7), uint16(8), uint32(9), uint64(10), float32(11.5), float64(12.5), "13.5"}
	for _, v := range floatVals {
		if err := m.Set("d", v); err != nil {
			t.Fatalf("Set(d, %#v): %v", v, err)
		}
	}
	strVals := []any{int(1), int8(2), int16(3), int32(4), int64(5), uint(6), uint8(7), uint16(8), uint32(9), uint64(10), float32(11.5), float64(12.5)}
	for _, v := range strVals {
		if err := m.Set("s", v); err != nil {
			t.Fatalf("Set(s, %#v): %v", v, err)
		}
	}
}

func TestReflectConversions(t *testing.T) {
	type myInt int
	type myUint uint
	type myFloat float64
	type myString string

	m := newRich(t)
	if err := m.Set("i32", myInt(5)); err != nil {
		t.Fatalf("myInt to i32: %v", err)
	}
	if m.Get("i32").Int32() != 5 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
	if err := m.Set("i32", myUint(6)); err != nil {
		t.Fatalf("myUint to i32: %v", err)
	}
	if err := m.Set("i32", myFloat(7.9)); err != nil {
		t.Fatalf("myFloat to i32: %v", err)
	}
	if m.Get("i32").Int32() != 7 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
	if err := m.Set("i32", myString("8")); err != nil {
		t.Fatalf("myString to i32: %v", err)
	}
	if m.Get("i32").Int32() != 8 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}
	if err := m.Set("u32", myUint(9)); err != nil {
		t.Fatalf("myUint to u32: %v", err)
	}
	if err := m.Set("u32", myFloat(10.9)); err != nil {
		t.Fatalf("myFloat to u32: %v", err)
	}
	if err := m.Set("u32", myString("11")); err != nil {
		t.Fatalf("myString to u32: %v", err)
	}
	if m.Get("u32").Uint32() != 11 {
		t.Fatalf("u32 = %d", m.Get("u32").Uint32())
	}
	if err := m.Set("d", myInt(12)); err != nil {
		t.Fatalf("myInt to d: %v", err)
	}
	if err := m.Set("d", myUint(13)); err != nil {
		t.Fatalf("myUint to d: %v", err)
	}
	if err := m.Set("d", myFloat(14.5)); err != nil {
		t.Fatalf("myFloat to d: %v", err)
	}
	if err := m.Set("d", myString("15.5")); err != nil {
		t.Fatalf("myString to d: %v", err)
	}
	if err := m.Set("s", myInt(16)); err != nil {
		t.Fatalf("myInt to s: %v", err)
	}
	if err := m.Set("s", myUint(17)); err != nil {
		t.Fatalf("myUint to s: %v", err)
	}
	if err := m.Set("s", myFloat(18.5)); err != nil {
		t.Fatalf("myFloat to s: %v", err)
	}
	if m.Get("s").String() != "18.5" {
		t.Fatalf("s = %q", m.Get("s").String())
	}
}

func TestConvertOverflowEdges(t *testing.T) {
	m := newRich(t)
	if err := m.Set("i32", ^uint64(0)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uint64 max to i32 err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i64", uint64(1<<63)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("uint64 1<<63 to i64 err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("u32", float64(-1)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("negative float to u32 err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("u32", float64(5e9)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("overflow float to u32 err = %v, want ErrTypeMismatch", err)
	}
	if err := m.Set("i32", (*dymsg.Message)(nil)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("typed nil message to i32 err = %v, want ErrTypeMismatch", err)
	}
}

func TestConvertMessageTypedNil(t *testing.T) {
	m := newRich(t)
	if err := m.Set("msg", (*dymsg.Message)(nil)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("typed nil message err = %v, want ErrTypeMismatch", err)
	}
}

func TestSetElementMessage(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{rmsgElement(t, "a", 1), rmsgElement(t, "b", 2)})
	if err := m.Set("rmsg[0]", rmsgElement(t, "c", 3)); err != nil {
		t.Fatalf("Set rmsg[0]: %v", err)
	}
	if got := m.Get("rmsg").Index(0).Message().Get("city").String(); got != "c" {
		t.Fatalf("rmsg[0].city = %q", got)
	}
	if err := m.Set("rmsg[1]", nil); err != nil {
		t.Fatalf("Set rmsg[1] nil: %v", err)
	}
	if m.Get("rmsg").Index(1).Message() != nil {
		t.Fatalf("rmsg[1] should be nil")
	}
	m.Set("rs", []string{"x"})
	if err := m.Set("rs[0]", nil); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Set scalar element nil err = %v, want ErrTypeMismatch", err)
	}
}

func TestAppendAllScalarTypes(t *testing.T) {
	m := newRich(t)
	if err := m.Append("ri32", int32(1)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("ri64", int64(2)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("ru32", uint32(3)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("ru64", uint64(4)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("rf", float32(1.5)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("rd", float64(2.5)); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("rb", true); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("rs", "a"); err != nil {
		t.Fatal(err)
	}
	if err := m.Append("rby", []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"ri32", "ri64", "ru32", "ru64", "rf", "rd", "rb", "rs", "rby"} {
		if m.Get(p).Len() != 1 {
			t.Fatalf("%s len = %d, want 1", p, m.Get(p).Len())
		}
	}
}

func TestSetMessageSliceVariants(t *testing.T) {
	e1 := rmsgElement(t, "a", 1)
	e2 := rmsgElement(t, "b", 2)

	m := newRich(t)
	if err := m.Set("rmsg", []*dymsg.Message{e1, nil, e2}); err != nil {
		t.Fatalf("Set []*Message: %v", err)
	}
	if m.Get("rmsg").Len() != 3 || m.Get("rmsg").Index(1).Message() != nil {
		t.Fatalf("[]*Message variant wrong")
	}

	m2 := newRich(t)
	if err := m2.Set("rmsg", [3]*dymsg.Message{e1, nil, e2}); err != nil {
		t.Fatalf("Set array *Message: %v", err)
	}
	if m2.Get("rmsg").Len() != 3 || m2.Get("rmsg").Index(1).Message() != nil {
		t.Fatalf("array *Message variant wrong")
	}

	m3 := newRich(t)
	if err := m3.Set("rmsg", []any{e1, nil, e2}); err != nil {
		t.Fatalf("Set []any: %v", err)
	}
	if m3.Get("rmsg").Len() != 3 || m3.Get("rmsg").Index(1).Message() != nil {
		t.Fatalf("[]any variant wrong")
	}
}

func TestSetIndexedNested(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{rmsgElement(t, "a", 1)})
	if err := m.Set("rmsg[0].city", "b"); err != nil {
		t.Fatalf("Set rmsg[0].city: %v", err)
	}
	if got := m.Get("rmsg[0].city").String(); got != "b" {
		t.Fatalf("rmsg[0].city = %q", got)
	}
	m.Set("rmsg", []*dymsg.Message{nil})
	if err := m.Set("rmsg[0].city", "b"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Set on nil element err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("rmsg[5].city", "b"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Set out-of-range element err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestClearIndexedNested(t *testing.T) {
	m := newRich(t)
	m.Set("rmsg", []*dymsg.Message{rmsgElement(t, "a", 1)})
	m.Set("rmsg[0].city", "b")
	if err := m.Clear("rmsg[0].city"); err != nil {
		t.Fatalf("Clear rmsg[0].city: %v", err)
	}
	if m.Get("rmsg[0].city").IsSet() {
		t.Fatalf("rmsg[0].city should be unset")
	}
	m.Set("rmsg", []*dymsg.Message{nil})
	if err := m.Clear("rmsg[0].city"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Clear on nil element err = %v, want ErrFieldNotFound", err)
	}
}

func TestJSONStringEscaping(t *testing.T) {
	s := "a\"b\\c\nd\re\tf\x01g"
	m := newRich(t)
	m.Set("s", s)
	got, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	dst := newRich(t)
	if err := dst.DecodeJSON(got); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got := dst.Get("s").String(); got != s {
		t.Fatalf("roundtrip string = %q, want %q", got, s)
	}
}

func TestDecodeProtoSkipFixed(t *testing.T) {
	m := newRich(t)
	data := append(wireKey(99, 1), make([]byte, 8)...)
	data = append(data, wireKey(1, 0)...)
	data = append(data, varintBytes(5)...)
	if err := m.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if m.Get("i32").Int32() != 5 {
		t.Fatalf("i32 = %d", m.Get("i32").Int32())
	}

	m2 := newRich(t)
	data2 := append(wireKey(99, 5), make([]byte, 4)...)
	data2 = append(data2, wireKey(1, 0)...)
	data2 = append(data2, varintBytes(6)...)
	if err := m2.DecodeProto(data2); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if m2.Get("i32").Int32() != 6 {
		t.Fatalf("i32 = %d", m2.Get("i32").Int32())
	}

	m3 := newRich(t)
	if err := m3.DecodeProto(append(wireKey(99, 1), 1, 2, 3)); err != dymsg.ErrTruncated {
		t.Fatalf("truncated fixed64 err = %v, want ErrTruncated", err)
	}
	m4 := newRich(t)
	if err := m4.DecodeProto(append(wireKey(99, 5), 1, 2)); err != dymsg.ErrTruncated {
		t.Fatalf("truncated fixed32 err = %v, want ErrTruncated", err)
	}
	m5 := newRich(t)
	if err := m5.DecodeProto(wireKey(99, 3)); err != dymsg.ErrMalformedData {
		t.Fatalf("unknown group wire type err = %v, want ErrMalformedData", err)
	}
}

func TestDecodeProtoScalarTruncation(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeProto(append(wireKey(8, 2), varintBytes(5)...)); err != dymsg.ErrTruncated {
		t.Fatalf("string truncation err = %v, want ErrTruncated", err)
	}
	m2 := newRich(t)
	if err := m2.DecodeProto(append(wireKey(9, 2), varintBytes(5)...)); err != dymsg.ErrTruncated {
		t.Fatalf("bytes truncation err = %v, want ErrTruncated", err)
	}
	m3 := newRich(t)
	if err := m3.DecodeProto(append(wireKey(5, 5), 1, 2)); err != dymsg.ErrTruncated {
		t.Fatalf("float truncation err = %v, want ErrTruncated", err)
	}
	m4 := newRich(t)
	if err := m4.DecodeProto(append(wireKey(6, 1), 1, 2, 3)); err != dymsg.ErrTruncated {
		t.Fatalf("double truncation err = %v, want ErrTruncated", err)
	}
	m5 := newRich(t)
	if err := m5.DecodeProto(append(wireKey(2, 0), 0x80)); err != dymsg.ErrTruncated {
		t.Fatalf("varint truncation err = %v, want ErrTruncated", err)
	}
}

func TestValueZeroGetters(t *testing.T) {
	m := newRich(t)
	u := m.Get("s")
	if u.String() != "" || u.Int32() != 0 || u.Int64() != 0 || u.Uint32() != 0 ||
		u.Uint64() != 0 || u.Float32() != 0 || u.Float64() != 0 || u.Bool() != false ||
		u.Bytes() != nil || u.Message() != nil {
		t.Fatalf("unset scalar getters should be zero/nil")
	}
	r := m.Get("rs")
	if r.Strings() != nil || r.Int32s() != nil || r.Int64s() != nil || r.Uint32s() != nil ||
		r.Uint64s() != nil || r.Float32s() != nil || r.Float64s() != nil || r.Bools() != nil ||
		r.BytesSlice() != nil || r.Messages() != nil {
		t.Fatalf("unset repeated getters should be nil")
	}
}

func TestWritePathErrors(t *testing.T) {
	m := newRich(t)
	if err := m.Set("rs[-1]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Set rs[-1] err = %v, want ErrIndexOutOfRange", err)
	}
	if err := m.Set("rs[0", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Set rs[0 err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Set("rs[0]x", "x"); err != dymsg.ErrFieldNotFound {
		t.Fatalf("Set rs[0]x err = %v, want ErrFieldNotFound", err)
	}
	if err := m.Append("rs[abc]", "x"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Append rs[abc] err = %v, want ErrIndexOutOfRange", err)
	}
	if err := m.Clear("rs[]"); err != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Clear rs[] err = %v, want ErrIndexOutOfRange", err)
	}
}

func TestGetIndexedNestedStates(t *testing.T) {
	m := newRich(t)
	if v := m.Get("ri32[0]"); v.Err() != dymsg.ErrIndexOutOfRange {
		t.Fatalf("Get unset repeated index err = %v, want ErrIndexOutOfRange", v.Err())
	}
	m.Set("rmsg", []*dymsg.Message{nil})
	v := m.Get("rmsg[0].city")
	if v.Err() != nil || !v.Exists() || v.IsSet() {
		t.Fatalf("Get nil element field = %v/%v/%v, want nil/true/false", v.Err(), v.Exists(), v.IsSet())
	}
}

func TestMoreTypeMismatch(t *testing.T) {
	m := newRich(t)
	cases := []struct {
		path  string
		value any
	}{
		{"i32", true},
		{"u32", true},
		{"d", true},
		{"u32", uint64(4294967296)},
		{"rs", 5},
		{"rmsg", []any{123}},
	}
	for _, tc := range cases {
		if err := m.Set(tc.path, tc.value); err != dymsg.ErrTypeMismatch {
			t.Errorf("Set(%q, %#v) err = %v, want ErrTypeMismatch", tc.path, tc.value, err)
		}
	}
}

func TestDecodeJSONWhitespace(t *testing.T) {
	m := newRich(t)
	if err := m.DecodeJSON([]byte("   ")); err != dymsg.ErrTruncated {
		t.Fatalf("whitespace input err = %v, want ErrTruncated", err)
	}
}

func TestSetSelfTypedNil(t *testing.T) {
	m := newRich(t)
	if err := m.Set("", (*dymsg.Message)(nil)); err != dymsg.ErrTypeMismatch {
		t.Fatalf("Set(\"\", typed nil) err = %v, want ErrTypeMismatch", err)
	}
}
