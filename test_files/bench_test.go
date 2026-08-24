package eval

import (
	"fmt"
	"testing"

	"dymsg"
)

// Benchmarks exercise the public dymsg API so the performance dimension of the
// evaluation script has data to score. They reuse the richSchema defined in
// dymsg_test.go (same package) and register it exactly once via init.

var benchSchema dymsg.MessageSchema

var (
	benchSinkInt   int
	benchSinkInt32 int32
	benchSinkStr   string
	benchSinkBytes []byte
	benchSinkMsg   *dymsg.Message
)

func init() {
	schemas, err := dymsg.ParseSchema([]byte(richSchema))
	if err != nil {
		panic(err)
	}
	benchSchema = schemas[0]
	if err := dymsg.Register(benchSchema); err != nil {
		panic(err)
	}
}

func benchNewMessage() *dymsg.Message {
	m, _ := dymsg.New(1001)
	return m
}

func benchRMsg(city string, zip int32) *dymsg.Message {
	parent := benchNewMessage()
	_ = parent.DecodeJSON([]byte(fmt.Sprintf(`{"rmsg":[{"city":%q,"zip":%d}]}`, city, zip)))
	return parent.Get("rmsg").Index(0).Message()
}

func benchPopulate(m *dymsg.Message) {
	_ = m.Set("i32", int32(-1))
	_ = m.Set("i64", int64(-2))
	_ = m.Set("u32", uint32(3))
	_ = m.Set("u64", uint64(4))
	_ = m.Set("f", float32(1.5))
	_ = m.Set("d", float64(-2.25))
	_ = m.Set("b", true)
	_ = m.Set("s", "hello")
	_ = m.Set("by", []byte{0, 1, 2, 255})
	_ = m.Set("msg.city", "beijing")
	_ = m.Set("msg.zip", int32(100))
	_ = m.Set("msg.tags", []string{"a", "b"})
	_ = m.Set("msg.sub.x", int64(99))
	_ = m.Set("ri32", []int32{1, -2, 3})
	_ = m.Set("ri64", []int64{4, 5})
	_ = m.Set("ru32", []uint32{6, 7})
	_ = m.Set("ru64", []uint64{8, 9})
	_ = m.Set("rf", []float32{1.5, 2.5})
	_ = m.Set("rd", []float64{3.5, 4.5})
	_ = m.Set("rb", []bool{true, false, true})
	_ = m.Set("rs", []string{"x", "y", "z"})
	_ = m.Set("rby", [][]byte{{1, 2}, {3, 4}})
	_ = m.Set("rmsg", []*dymsg.Message{benchRMsg("c1", 1), benchRMsg("c2", 2)})
}

// BenchmarkReference is a trivial loop used only to normalize CPU time across
// machines; the relative-time thresholds in the evaluation script are expressed
// as multiples of this baseline.
func BenchmarkReference(b *testing.B) {
	b.ReportAllocs()
	x := 0
	for i := 0; i < b.N; i++ {
		x += i
	}
	benchSinkInt = x
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchSinkMsg = benchNewMessage()
	}
}

func BenchmarkSetScalar(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	for i := 0; i < b.N; i++ {
		_ = m.Set("i32", int32(i))
	}
	benchSinkMsg = m
}

func BenchmarkGetScalar(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	_ = m.Set("i32", int32(42))
	for i := 0; i < b.N; i++ {
		benchSinkInt32 = m.Get("i32").Int32()
	}
	benchSinkMsg = m
}

func BenchmarkSetNested(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	for i := 0; i < b.N; i++ {
		_ = m.Set("msg.sub.x", int64(i))
	}
	benchSinkMsg = m
}

func BenchmarkGetNested(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	_ = m.Set("msg.sub.x", int64(99))
	for i := 0; i < b.N; i++ {
		benchSinkInt = int(m.Get("msg.sub.x").Int64())
	}
	benchSinkMsg = m
}

func BenchmarkEncodeProto(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	benchPopulate(m)
	var out []byte
	for i := 0; i < b.N; i++ {
		var err error
		out, err = m.EncodeProto()
		if err != nil {
			b.Fatal(err)
		}
	}
	benchSinkBytes = out
}

func BenchmarkDecodeProto(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	benchPopulate(m)
	data, err := m.EncodeProto()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		nm := benchNewMessage()
		if err := nm.DecodeProto(data); err != nil {
			b.Fatal(err)
		}
		benchSinkMsg = nm
	}
}

func BenchmarkEncodeJSON(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	benchPopulate(m)
	var out []byte
	for i := 0; i < b.N; i++ {
		var err error
		out, err = m.EncodeJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
	benchSinkBytes = out
}

func BenchmarkDecodeJSON(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	benchPopulate(m)
	data, err := m.EncodeJSON()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		nm := benchNewMessage()
		if err := nm.DecodeJSON(data); err != nil {
			b.Fatal(err)
		}
		benchSinkMsg = nm
	}
}

func BenchmarkRoundTripProto(b *testing.B) {
	b.ReportAllocs()
	m := benchNewMessage()
	benchPopulate(m)
	for i := 0; i < b.N; i++ {
		data, err := m.EncodeProto()
		if err != nil {
			b.Fatal(err)
		}
		nm := benchNewMessage()
		if err := nm.DecodeProto(data); err != nil {
			b.Fatal(err)
		}
		benchSinkMsg = nm
	}
}

func BenchmarkCopyMessage(b *testing.B) {
	b.ReportAllocs()
	src := benchNewMessage()
	benchPopulate(src)
	for i := 0; i < b.N; i++ {
		dst := benchNewMessage()
		if err := dst.Set("", src); err != nil {
			b.Fatal(err)
		}
		benchSinkMsg = dst
	}
}
