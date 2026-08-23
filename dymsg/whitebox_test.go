package dymsg

// 白盒测试:依赖未导出符号,必须留在包内。
// 其余公开 API 测试已迁移至同级独立 module dymsgxtest。

import (
	"sync"
	"testing"
)

var whiteboxOnce sync.Once

const whiteboxConfig = `{
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

func newMsg(t *testing.T) *Message {
	t.Helper()
	whiteboxOnce.Do(func() {
		schemas, err := ParseSchema([]byte(whiteboxConfig))
		if err != nil {
			t.Fatalf("ParseSchema: %v", err)
		}
		for _, s := range schemas {
			if err := Register(s); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}
	})
	m, err := New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
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
	if _, err := m.EncodeProto(); err != nil {
		t.Fatalf("EncodeProto: %v", err)
	}
	if _, err := m.EncodeJSON(); err != nil {
		t.Fatalf("EncodeJSON: %v", err)
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
