package dymsg

import "testing"

const basicSchema = `{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "age", "type": "int32", "num": 2},
        {"name": "addr", "type": "message", "num": 3, "schema": {"fields": [
          {"name": "city", "type": "string", "num": 1}
        ]}},
        {"name": "tags", "type": "string", "num": 4, "repeated": true}
      ]
    }
  ]
}`

func TestBasicScenario(t *testing.T) {
	schemas, err := ParseSchema([]byte(basicSchema))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("want 1 schema, got %d", len(schemas))
	}
	if err := Register(schemas[0]); err != nil {
		t.Fatalf("Register: %v", err)
	}

	m, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := m.Set("name", "alice"); err != nil {
		t.Fatalf("Set name: %v", err)
	}
	if err := m.Set("age", int32(30)); err != nil {
		t.Fatalf("Set age: %v", err)
	}
	if err := m.Set("addr.city", "beijing"); err != nil {
		t.Fatalf("Set addr.city: %v", err)
	}
	if err := m.Append("tags", "a"); err != nil {
		t.Fatalf("Append tags: %v", err)
	}
	if err := m.Append("tags", "b"); err != nil {
		t.Fatalf("Append tags: %v", err)
	}

	if got := m.Get("name").String(); got != "alice" {
		t.Fatalf("name = %q, want alice", got)
	}
	if got := m.Get("age").Int32(); got != 30 {
		t.Fatalf("age = %d, want 30", got)
	}
	if got := m.Get("addr.city").String(); got != "beijing" {
		t.Fatalf("city = %q, want beijing", got)
	}
	if got := m.Get("tags").Len(); got != 2 {
		t.Fatalf("tags len = %d, want 2", got)
	}
	if !m.Has("age") {
		t.Fatal("Has(age) = false, want true")
	}

	jsonData, err := m.EncodeJSON()
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	m2, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m2.DecodeJSON(jsonData); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got := m2.Get("name").String(); got != "alice" {
		t.Fatalf("roundtrip name = %q, want alice", got)
	}

	protoData, err := m.EncodeProto()
	if err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	m3, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m3.DecodeProto(protoData); err != nil {
		t.Fatalf("DecodeProto: %v", err)
	}
	if got := m3.Get("name").String(); got != "alice" {
		t.Fatalf("proto roundtrip name = %q, want alice", got)
	}
}
