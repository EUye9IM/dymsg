package dymsgxtest

import (
	"sync"
	"testing"

	dymsg "dymsg"
)

var benchOnce sync.Once

func benchSetup() {
	benchOnce.Do(func() {
		schemas, err := dymsg.ParseSchema([]byte(testConfig))
		if err != nil {
			panic(err)
		}
		for _, s := range schemas {
			if err := dymsg.Register(s); err != nil {
				panic(err)
			}
		}
	})
}

// 构造一个带典型字段的消息(供编解码 benchmark 复用)。
func benchMessage() *dymsg.Message {
	m, err := dymsg.New(1001)
	if err != nil {
		panic(err)
	}
	mustSetIgnore(m, "name", "alice")
	mustSetIgnore(m, "age", int32(30))
	mustSetIgnore(m, "active", true)
	mustSetIgnore(m, "score", 1.5)
	mustSetIgnore(m, "addr.city", "beijing")
	mustSetIgnore(m, "addr.zip", "100000")
	mustSetIgnore(m, "tags", []any{"a", "b", "c"})
	mustSetIgnore(m, "scores", []int32{1, 2, 3})
	return m
}

func mustSetIgnore(m *dymsg.Message, path string, v any) {
	if err := m.Set(path, v); err != nil {
		panic(err)
	}
}

func BenchmarkNew(b *testing.B) {
	benchSetup()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := dymsg.New(1001)
		if err != nil {
			b.Fatal(err)
		}
		_ = m
	}
}

func BenchmarkSetScalar(b *testing.B) {
	benchSetup()
	m, _ := dymsg.New(1001)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.Set("name", "alice"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetScalar(b *testing.B) {
	benchSetup()
	m, _ := dymsg.New(1001)
	mustSetIgnore(m, "name", "alice")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.Get("name"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetNested(b *testing.B) {
	benchSetup()
	m, _ := dymsg.New(1001)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.Set("addr.city", "beijing"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetNested(b *testing.B) {
	benchSetup()
	m, _ := dymsg.New(1001)
	mustSetIgnore(m, "addr.city", "beijing")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.Get("addr.city"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeProto(b *testing.B) {
	benchSetup()
	m := benchMessage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.EncodeProto(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeProto(b *testing.B) {
	benchSetup()
	m := benchMessage()
	blob, err := m.EncodeProto()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm, _ := dymsg.New(1001)
		if err := mm.DecodeProto(blob); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeJSON(b *testing.B) {
	benchSetup()
	m := benchMessage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.EncodeJSON(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeJSON(b *testing.B) {
	benchSetup()
	m := benchMessage()
	blob, err := m.EncodeJSON()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm, _ := dymsg.New(1001)
		if err := mm.DecodeJSON(blob); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTripProto(b *testing.B) {
	benchSetup()
	m := benchMessage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blob, err := m.EncodeProto()
		if err != nil {
			b.Fatal(err)
		}
		mm, _ := dymsg.New(1001)
		if err := mm.DecodeProto(blob); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopyMessage(b *testing.B) {
	benchSetup()
	m := benchMessage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst, _ := dymsg.New(1001)
		if err := dst.Set("", m); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReference 提供同机器的基本运算耗时参照。
// 用于将其他 benchmark 的时间归一化为机器无关的相对度量。
// 工作量取 1000 次整数运算,使参照耗时足够长以稳定测量。
func BenchmarkReference(b *testing.B) {
	x := 0
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			x += j
		}
	}
	_ = x
}
