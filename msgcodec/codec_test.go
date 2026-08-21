package msgcodec

// codec_test.go —— 公开测试(任务的一部分,agent 可见)。
// 覆盖单线程基本功能:加载/注册/New、Get/Set、JSON 与 Proto 往返。

import (
	"testing"
)

const publicConfig = `{
  "types": [
    {
      "typeId": 5000,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "age", "type": "int32", "num": 2},
        {"name": "addr", "type": "message", "num": 3, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1}
          ]
        }},
        {"name": "tags", "type": "string", "num": 4, "repeated": true}
      ]
    }
  ]
}`

func publicSchema(t *testing.T) {
	t.Helper()
	schemas, err := ParseSchema([]byte(publicConfig))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	for _, s := range schemas {
		if err := Register(s); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
}

func publicMsg(t *testing.T) Message {
	t.Helper()
	publicSchema(t)
	m, err := New(5000)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestPublicBasics(t *testing.T) {
	m := publicMsg(t)
	if err := m.Set("name", "alice"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := m.Set("age", int32(18)); err != nil {
		t.Fatalf("Set age: %v", err)
	}
	v, _ := m.Get("name")
	if v == nil || v.Value() != "alice" {
		t.Fatalf("name = %#v", v)
	}
	v, _ = m.Get("age")
	if v == nil || v.Value() != int32(18) {
		t.Fatalf("age = %#v", v)
	}
}

func TestPublicNested(t *testing.T) {
	m := publicMsg(t)
	if err := m.Set("addr.city", "beijing"); err != nil {
		t.Fatalf("Set addr.city: %v", err)
	}
	v, _ := m.Get("addr.city")
	if v == nil || v.Value() != "beijing" {
		t.Fatalf("addr.city = %#v", v)
	}
}

func TestPublicRepeated(t *testing.T) {
	m := publicMsg(t)
	if err := m.Set("tags", []any{"a", "b"}); err != nil {
		t.Fatalf("Set tags: %v", err)
	}
	v, _ := m.Get("tags[0]")
	if v == nil || v.Value() != "a" {
		t.Fatalf("tags[0] = %#v", v)
	}
	v, _ = m.Get("tags[1]")
	if v == nil || v.Value() != "b" {
		t.Fatalf("tags[1] = %#v", v)
	}
}

func TestPublicJSONRoundTrip(t *testing.T) {
	m := publicMsg(t)
	m.Set("name", "alice")
	m.Set("addr.city", "bj")
	m.Set("tags", []any{"x", "y"})
	data, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2 := publicMsg(t)
	if err := m2.DecodeJSON(data); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	v, _ := m2.Get("name")
	if v == nil || v.Value() != "alice" {
		t.Fatalf("name = %#v", v)
	}
	v, _ = m2.Get("addr.city")
	if v == nil || v.Value() != "bj" {
		t.Fatalf("addr.city = %#v", v)
	}
	v, _ = m2.Get("tags[0]")
	if v == nil || v.Value() != "x" {
		t.Fatalf("tags[0] = %#v", v)
	}
}

func TestPublicProtoRoundTrip(t *testing.T) {
	m := publicMsg(t)
	m.Set("name", "alice")
	m.Set("addr.city", "bj")
	data, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m2 := publicMsg(t)
	if err := m2.DecodeProto(data); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	v, _ := m2.Get("name")
	if v == nil || v.Value() != "alice" {
		t.Fatalf("name = %#v", v)
	}
}
