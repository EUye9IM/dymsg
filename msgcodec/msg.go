package msgcodec

import (
	"math"
	"reflect"
	"strconv"
	"strings"
)

// msgImpl 是复合消息(有 schema)的实现。
type msgImpl struct {
	schema *runtimeSchema
	values map[string]*fieldValue
}

var _ Message = (*msgImpl)(nil)
var _ Message = (*valueMsg)(nil)

func newMessage(rs *runtimeSchema) *msgImpl {
	return &msgImpl{schema: rs, values: make(map[string]*fieldValue, len(rs.decl))}
}

// fieldValue 存储一个字段的设置状态与值。
// 按字段类型只使用其中一个承载字段:
//
//	标量单值:   scalar
//	标量 repeated:list
//	message 单值: child
//	message repeated: kids
type fieldValue struct {
	present bool
	scalar  any
	list    []any
	child   *msgImpl
	kids    []*msgImpl
}

func (m *msgImpl) ensure(rf *runtimeField) *fieldValue {
	fv := m.values[rf.name]
	if fv == nil {
		fv = &fieldValue{}
		m.values[rf.name] = fv
	}
	return fv
}

// valueMsg 是对标量值/列表的只读包装,由 Get 返回。
type valueMsg struct {
	typ    FieldType
	scalar any
	list   []any
	msgs   []Message
	isList bool
}

func wrapScalar(ft FieldType, v any) Message {
	return &valueMsg{typ: ft, scalar: v}
}

// ---------- 路径解析 ----------

type seg struct {
	name   string
	index  int
	hasIdx bool
}

func parseSeg(s string) (seg, error) {
	lb := strings.IndexByte(s, '[')
	if lb < 0 {
		if !validFieldName(s) {
			return seg{}, ErrFieldNotFound
		}
		return seg{name: s, index: -1}, nil
	}
	name := s[:lb]
	if !validFieldName(name) || len(s) == 0 || s[len(s)-1] != ']' {
		return seg{}, ErrFieldNotFound
	}
	n, err := strconv.ParseUint(s[lb+1:len(s)-1], 10, 32)
	if err != nil {
		return seg{}, ErrIndexOutOfRange
	}
	return seg{name: name, index: int(n), hasIdx: true}, nil
}

func parsePath(path string) ([]seg, error) {
	if path == "" {
		return nil, nil
	}
	parts := strings.Split(path, ".")
	segs := make([]seg, len(parts))
	for i, p := range parts {
		s, err := parseSeg(p)
		if err != nil {
			return nil, err
		}
		segs[i] = s
	}
	return segs, nil
}

// ---------- Get ----------

func (m *msgImpl) Get(field string) (Message, error) {
	segs, err := parsePath(field)
	if err != nil {
		return nil, err
	}
	node := m
	for i, s := range segs {
		rf := node.schema.byName[s.name]
		if rf == nil {
			return nil, ErrFieldNotFound
		}
		fv := node.values[rf.name]
		if fv == nil || !fv.present {
			return nil, nil
		}
		if rf.repeated {
			if !s.hasIdx {
				if rf.typ == FieldMessage {
					msgs := make([]Message, len(fv.kids))
					for j, k := range fv.kids {
						msgs[j] = k
					}
					return &valueMsg{typ: rf.typ, msgs: msgs, isList: true}, nil
				}
				return &valueMsg{typ: rf.typ, list: fv.list, isList: true}, nil
			}
			if rf.typ == FieldMessage {
				if s.index >= len(fv.kids) {
					return nil, ErrIndexOutOfRange
				}
				node = fv.kids[s.index]
			} else {
				if s.index >= len(fv.list) {
					return nil, ErrIndexOutOfRange
				}
				if i == len(segs)-1 {
					return wrapScalar(rf.typ, fv.list[s.index]), nil
				}
				return nil, ErrFieldNotFound
			}
		} else {
			if rf.typ == FieldMessage {
				node = fv.child
			} else {
				if i == len(segs)-1 {
					return wrapScalar(rf.typ, fv.scalar), nil
				}
				return nil, ErrFieldNotFound
			}
		}
	}
	return node, nil
}

// ---------- Set ----------

func (m *msgImpl) Set(field string, value any) error {
	segs, err := parsePath(field)
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return m.setSelf(value)
	}
	node, err := m.descend(segs[:len(segs)-1])
	if err != nil {
		return err
	}
	return node.setField(segs[len(segs)-1], value)
}

func (m *msgImpl) descend(segs []seg) (*msgImpl, error) {
	node := m
	for _, s := range segs {
		rf := node.schema.byName[s.name]
		if rf == nil {
			return nil, ErrFieldNotFound
		}
		if rf.typ != FieldMessage {
			return nil, ErrFieldNotFound
		}
		fv := node.ensure(rf)
		if rf.repeated {
			if !s.hasIdx {
				return nil, ErrFieldNotFound
			}
			if s.index >= len(fv.kids) {
				return nil, ErrIndexOutOfRange
			}
			node = fv.kids[s.index]
		} else {
			if fv.child == nil {
				fv.child = newMessage(rf.child)
				fv.present = true
			}
			node = fv.child
		}
	}
	return node, nil
}

func (m *msgImpl) setField(s seg, value any) error {
	rf := m.schema.byName[s.name]
	if rf == nil {
		return ErrFieldNotFound
	}
	fv := m.ensure(rf)
	if rf.repeated {
		if s.hasIdx {
			return m.setRepeatedElem(fv, rf, s.index, value)
		}
		return m.setRepeated(fv, rf, value)
	}
	if rf.typ == FieldMessage {
		return m.setMessage(fv, rf, value)
	}
	return m.setScalar(fv, rf, value)
}

func (m *msgImpl) setRepeatedElem(fv *fieldValue, rf *runtimeField, idx int, value any) error {
	if idx < 0 {
		return ErrIndexOutOfRange
	}
	if value == nil {
		fv.present = false
		fv.list, fv.kids, fv.scalar, fv.child = nil, nil, nil, nil
		return nil
	}
	if rf.typ == FieldMessage {
		sim, ok := value.(*msgImpl)
		if !ok || sim.schema != rf.child {
			return ErrTypeMismatch
		}
		if idx >= len(fv.kids) {
			return ErrIndexOutOfRange
		}
		nk := newMessage(rf.child)
		nk.copyFrom(sim)
		fv.kids[idx] = nk
		fv.present = true
		return nil
	}
	if idx >= len(fv.list) {
		return ErrIndexOutOfRange
	}
	cv, err := convertTo(value, rf.typ)
	if err != nil {
		return err
	}
	fv.list[idx] = cv
	fv.present = true
	return nil
}

func (m *msgImpl) setRepeated(fv *fieldValue, rf *runtimeField, value any) error {
	if value == nil {
		for k := range m.values {
			if m.values[k] == fv {
				delete(m.values, k)
				break
			}
		}
		return nil
	}
	if rf.typ == FieldMessage {
		ms, ok := value.([]Message)
		if !ok {
			rv := reflect.ValueOf(value)
			if rv.Kind() != reflect.Slice {
				return ErrTypeMismatch
			}
			ms = make([]Message, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				mv, ok := rv.Index(i).Interface().(Message)
				if !ok {
					return ErrTypeMismatch
				}
				ms[i] = mv
			}
		}
		kids := make([]*msgImpl, len(ms))
		for i, mm := range ms {
			if mm == nil {
				kids[i] = newMessage(rf.child)
				continue
			}
			sim, ok := mm.(*msgImpl)
			if !ok || sim.schema != rf.child {
				return ErrTypeMismatch
			}
			nk := newMessage(rf.child)
			nk.copyFrom(sim)
			kids[i] = nk
		}
		fv.kids = kids
		fv.list, fv.scalar, fv.child = nil, nil, nil
		fv.present = true
		return nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return ErrTypeMismatch
	}
	list := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		cv, err := convertTo(rv.Index(i).Interface(), rf.typ)
		if err != nil {
			return err
		}
		list[i] = cv
	}
	fv.list = list
	fv.kids, fv.scalar, fv.child = nil, nil, nil
	fv.present = true
	return nil
}

func (m *msgImpl) setMessage(fv *fieldValue, rf *runtimeField, value any) error {
	if value == nil {
		fv.child = nil
		fv.present = false
		return nil
	}
	sim, ok := value.(*msgImpl)
	if !ok || sim.schema != rf.child {
		return ErrTypeMismatch
	}
	nc := newMessage(rf.child)
	nc.copyFrom(sim)
	fv.child = nc
	fv.scalar, fv.list, fv.kids = nil, nil, nil
	fv.present = true
	return nil
}

func (m *msgImpl) setScalar(fv *fieldValue, rf *runtimeField, value any) error {
	if value == nil {
		fv.scalar = nil
		fv.present = false
		return nil
	}
	cv, err := convertTo(value, rf.typ)
	if err != nil {
		return err
	}
	fv.scalar = cv
	fv.list, fv.kids, fv.child = nil, nil, nil
	fv.present = true
	return nil
}

func (m *msgImpl) setSelf(value any) error {
	if value == nil {
		for k := range m.values {
			delete(m.values, k)
		}
		return nil
	}
	sim, ok := value.(*msgImpl)
	if !ok || sim.schema != m.schema {
		return ErrTypeMismatch
	}
	m.copyFrom(sim)
	return nil
}

// ---------- 深拷贝 ----------

func (m *msgImpl) copyFrom(src *msgImpl) {
	for k := range m.values {
		delete(m.values, k)
	}
	for name, fv := range src.values {
		if !fv.present {
			continue
		}
		nf := &fieldValue{present: true}
		nf.scalar = copyScalar(fv.scalar)
		nf.list = make([]any, len(fv.list))
		for i, v := range fv.list {
			nf.list[i] = copyScalar(v)
		}
		if fv.child != nil {
			nc := newMessage(fv.child.schema)
			nc.copyFrom(fv.child)
			nf.child = nc
		}
		if fv.kids != nil {
			nf.kids = make([]*msgImpl, len(fv.kids))
			for i, k := range fv.kids {
				if k != nil {
					nk := newMessage(k.schema)
					nk.copyFrom(k)
					nf.kids[i] = nk
				}
			}
		}
		m.values[name] = nf
	}
}

func copyScalar(v any) any {
	if b, ok := v.([]byte); ok {
		return append([]byte(nil), b...)
	}
	return v
}

// ---------- Value ----------

func (m *msgImpl) Value() any { return m }

func (v *valueMsg) Value() any {
	if !v.isList {
		return v.scalar
	}
	if v.typ == FieldMessage {
		return v.msgs
	}
	return v.list
}

// ---------- valueMsg 接口兜底 ----------

func (v *valueMsg) Get(field string) (Message, error) {
	if field == "" {
		return v, nil
	}
	return nil, ErrFieldNotFound
}

func (v *valueMsg) Set(field string, value any) error {
	if field == "" {
		return ErrTypeMismatch
	}
	return ErrFieldNotFound
}

func (v *valueMsg) DecodeJSON(data []byte) error  { return ErrTypeMismatch }
func (v *valueMsg) DecodeProto(data []byte) error { return ErrTypeMismatch }

// ---------- 折中类型转换 ----------

type convertFunc func(reflect.Value) (any, error)

func convertTo(v any, target FieldType) (any, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, ErrTypeMismatch
	}
	switch target {
	case FieldInt32:
		n, err := reflectToInt64(rv)
		if err != nil || n < math.MinInt32 || n > math.MaxInt32 {
			return nil, ErrTypeMismatch
		}
		return int32(n), nil
	case FieldInt64:
		n, err := reflectToInt64(rv)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldUint32:
		n, err := reflectToUint64(rv)
		if err != nil || n > math.MaxUint32 {
			return nil, ErrTypeMismatch
		}
		return uint32(n), nil
	case FieldUint64:
		n, err := reflectToUint64(rv)
		if err != nil {
			return nil, err
		}
		return n, nil
	case FieldFloat:
		f, err := reflectToFloat64(rv)
		if err != nil || f > math.MaxFloat32 || f < -math.MaxFloat32 {
			return nil, ErrTypeMismatch
		}
		return float32(f), nil
	case FieldDouble:
		f, err := reflectToFloat64(rv)
		if err != nil {
			return nil, err
		}
		return f, nil
	case FieldBool:
		if rv.Kind() == reflect.Bool {
			return rv.Bool(), nil
		}
		return nil, ErrTypeMismatch
	case FieldString:
		return reflectToString(rv)
	case FieldBytes:
		return reflectToBytes(rv)
	}
	return nil, ErrTypeMismatch
}

func reflectToInt64(rv reflect.Value) (int64, error) {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := rv.Uint()
		if u > math.MaxInt64 {
			return 0, ErrTypeMismatch
		}
		return int64(u), nil
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if f < -9.2233720368547758e18 || f > 9.2233720368547758e18 {
			return 0, ErrTypeMismatch
		}
		return int64(f), nil
	case reflect.String:
		n, err := strconv.ParseInt(strings.TrimSpace(rv.String()), 10, 64)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	}
	return 0, ErrTypeMismatch
}

func reflectToUint64(rv reflect.Value) (uint64, error) {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rv.Int()
		if i < 0 {
			return 0, ErrTypeMismatch
		}
		return uint64(i), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint(), nil
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if f < 0 || f > 1.8446744073709552e19 {
			return 0, ErrTypeMismatch
		}
		return uint64(f), nil
	case reflect.String:
		n, err := strconv.ParseUint(strings.TrimSpace(rv.String()), 10, 64)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return n, nil
	}
	return 0, ErrTypeMismatch
}

func reflectToFloat64(rv reflect.Value) (float64, error) {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return rv.Float(), nil
	case reflect.String:
		f, err := strconv.ParseFloat(strings.TrimSpace(rv.String()), 64)
		if err != nil {
			return 0, ErrTypeMismatch
		}
		return f, nil
	}
	return 0, ErrTypeMismatch
}

func reflectToString(rv reflect.Value) (any, error) {
	switch rv.Kind() {
	case reflect.String:
		return rv.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(rv.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64), nil
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return string(rv.Bytes()), nil
		}
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				b[i] = byte(rv.Index(i).Uint())
			}
			return string(b), nil
		}
	}
	return nil, ErrTypeMismatch
}

func reflectToBytes(rv reflect.Value) (any, error) {
	switch rv.Kind() {
	case reflect.String:
		return []byte(rv.String()), nil
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return append([]byte(nil), rv.Bytes()...), nil
		}
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				b[i] = byte(rv.Index(i).Uint())
			}
			return b, nil
		}
	}
	return nil, ErrTypeMismatch
}
