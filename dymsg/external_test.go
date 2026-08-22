// Package dymsg_test 是包外(黑盒)测试,仅使用公开 API。
// 目的:验证"单值 message 字段 present=true 但 value=nil"的异常态无法通过
// 公开 API 触发,所有公开调用序列都不会 panic。
package dymsg_test

import (
	"testing"

	dymsg "dymsg"
)

const cfg = `{
  "types": [
    {
      "typeId": 3001,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "addr", "type": "message", "num": 2, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1}
          ]
        }}
      ]
    }
  ]
}`

func mustNew(t *testing.T) *dymsg.Message {
	t.Helper()
	schemas, err := dymsg.ParseSchema([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	for _, s := range schemas {
		if err := dymsg.Register(s); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	m, err := dymsg.New(3001)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

// 遍历公开 API 组合,Get("addr") 不应 panic。
func TestExternalPathsNoPanic(t *testing.T) {
	// 1. Set(nil) 清除
	m := mustNew(t)
	if err := m.Set("addr", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.Get("addr"); g != nil {
		t.Fatalf("addr should be nil, got %#v", g)
	}

	// 2. 设子字段后 Get 整字段
	m.Set("addr.city", "beijing")
	if g, _ := m.Get("addr"); g == nil {
		t.Fatal("addr should be present")
	}

	// 3. 整体复制(源含已设 addr)
	m2 := mustNew(t)
	if err := m2.Set("", m); err != nil {
		t.Fatal(err)
	}
	if g, _ := m2.Get("addr"); g == nil {
		t.Fatal("copied addr should be present")
	}

	// 4. 复制后 Set(nil) 再复制
	m2.Set("addr", nil)
	m3 := mustNew(t)
	if err := m3.Set("", m2); err != nil {
		t.Fatal(err)
	}
	if g, _ := m3.Get("addr"); g != nil {
		t.Fatalf("addr should be nil after copy of cleared msg, got %#v", g)
	}

	// 5. DecodeJSON 置 null / 空对象
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

	// 6. DecodeProto 空子消息(长度 0)
	m5 := mustNew(t)
	blob := appendVarint(nil, 2<<3|2)
	blob = appendVarint(blob, 0)
	if err := m5.DecodeProto(blob); err != nil {
		t.Fatal(err)
	}
	if g, _ := m5.Get("addr"); g == nil {
		t.Fatal("addr should be present (empty nested message)")
	}

	// 7. Encode 后再 Decode 到新消息
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

	// 8. Set("", nil) 清空后 Get
	m7 := mustNew(t)
	m7.Set("addr.city", "x")
	if err := m7.Set("", nil); err != nil {
		t.Fatal(err)
	}
	if g, _ := m7.Get("addr"); g != nil {
		t.Fatalf("addr should be nil after full clear, got %#v", g)
	}
}

// varint 辅助(黑盒测试自含)
func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}
