// 包外(黑盒)测试的公共辅助:通过公开 API 使用 dymsg。
package dymsgxtest

import (
	"reflect"
	"sync"
	"testing"

	dymsg "dymsg"
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

const allTypesConfig = `{
  "types": [{
    "typeId": 1002,
    "fields": [
      {"name": "i32", "type": "int32", "num": 1},
      {"name": "i64", "type": "int64", "num": 2},
      {"name": "u32", "type": "uint32", "num": 3},
      {"name": "u64", "type": "uint64", "num": 4},
      {"name": "f32", "type": "float", "num": 5},
      {"name": "f64", "type": "double", "num": 6},
      {"name": "b", "type": "bool", "num": 7},
      {"name": "s", "type": "string", "num": 8},
      {"name": "by", "type": "bytes", "num": 9},
      {"name": "i64r", "type": "int64", "num": 10, "repeated": true},
      {"name": "u64r", "type": "uint64", "num": 11, "repeated": true},
      {"name": "f32r", "type": "float", "num": 12, "repeated": true},
      {"name": "f64r", "type": "double", "num": 13, "repeated": true},
      {"name": "byr", "type": "bytes", "num": 14, "repeated": true},
      {"name": "br", "type": "bool", "num": 15, "repeated": true}
    ]
  }]
}`

var (
	mainOnce     sync.Once
	allTypesOnce sync.Once
)

func registerConfig(t *testing.T, cfg string, once *sync.Once) {
	t.Helper()
	once.Do(func() {
		schemas, err := dymsg.ParseSchema([]byte(cfg))
		if err != nil {
			t.Fatalf("ParseSchema: %v", err)
		}
		for _, s := range schemas {
			if err := dymsg.Register(s); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}
	})
}

func mustNew(t *testing.T) *dymsg.Message {
	t.Helper()
	registerConfig(t, testConfig, &mainOnce)
	m, err := dymsg.New(1001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func mustNewAllTypes(t *testing.T) *dymsg.Message {
	t.Helper()
	registerConfig(t, allTypesConfig, &allTypesOnce)
	m, err := dymsg.New(1002)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func mustSet(t *testing.T, m *dymsg.Message, path string, v any) {
	t.Helper()
	if err := m.Set(path, v); err != nil {
		t.Fatalf("Set(%q, %#v): %v", path, v, err)
	}
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// ---------- 自含 wire 工具(黑盒测试使用) ----------

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func appendTag(b []byte, num, wt int) []byte {
	return appendVarint(b, uint64(num)<<3|uint64(wt))
}

func flen(b []byte, content []byte) []byte {
	b = appendVarint(b, uint64(len(content)))
	return append(b, content...)
}
