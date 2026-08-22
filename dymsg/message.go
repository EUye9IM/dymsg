package dymsg

import (
	"reflect"
	"strconv"
	"strings"
)

// Value returns the native Go value represented by the message. For a
// structured message it returns the message itself; for a scalar/repeated
// value wrapper it returns the wrapped value.
func (m *Message) Value() any {
	if m.valueNode {
		return m.value
	}
	return m
}

type pathSeg struct {
	name  string
	index int // -1 when no subscript is present
}

func parsePath(path string) ([]pathSeg, error) {
	if path == "" {
		return nil, nil
	}
	parts := strings.Split(path, ".")
	segs := make([]pathSeg, 0, len(parts))
	for _, part := range parts {
		name := part
		idx := -1
		if lb := strings.IndexByte(part, '['); lb >= 0 {
			if !strings.HasSuffix(part, "]") {
				return nil, ErrFieldNotFound
			}
			name = part[:lb]
			numStr := part[lb+1 : len(part)-1]
			if numStr == "" {
				return nil, ErrIndexOutOfRange
			}
			n, err := strconv.Atoi(numStr)
			if err != nil || n < 0 {
				return nil, ErrIndexOutOfRange
			}
			idx = n
		}
		if name == "" || strings.ContainsAny(name, "[]") {
			return nil, ErrFieldNotFound
		}
		segs = append(segs, pathSeg{name: name, index: idx})
	}
	return segs, nil
}

// Get returns the message corresponding to the field path. It returns nil
// (without error) when the addressed field is not set.
func (m *Message) Get(field string) (*Message, error) {
	if field == "" {
		return m, nil
	}
	if m.schema == nil {
		return nil, ErrFieldNotFound
	}
	segs, err := parsePath(field)
	if err != nil {
		return nil, err
	}
	return m.resolve(segs)
}

func (m *Message) resolve(segs []pathSeg) (*Message, error) {
	schema := m.schema
	var msg *Message = m

	for i, seg := range segs {
		idx := schema.fieldIndex(seg.name)
		if idx < 0 {
			return nil, ErrFieldNotFound
		}
		fs := &schema.fields[idx]
		isLast := i == len(segs)-1

		var fv *fieldValue
		if msg != nil {
			fv = &msg.fields[idx]
		}

		if seg.index >= 0 {
			if fs.Repeated {
				if fv == nil || !fv.present {
					return nil, ErrIndexOutOfRange
				}
				elem, err := sliceIndex(fv.value, seg.index)
				if err != nil {
					return nil, err
				}
				if isLast {
					return wrapElement(fs, elem), nil
				}
				if fs.Type != FieldMessage {
					return nil, ErrFieldNotFound
				}
				schema = &fs.Schema
				msg, _ = elem.(*Message)
				continue
			}
			// 非 repeated 字段使用下标是非法路径。
			return nil, ErrFieldNotFound
		}

		if isLast {
			return wrapField(fs, fv), nil
		}
		if fs.Type != FieldMessage {
			return nil, ErrFieldNotFound
		}
		schema = &fs.Schema
		if fv != nil && fv.present {
			msg = fv.value.(*Message)
		} else {
			msg = nil
		}
	}
	return nil, ErrFieldNotFound
}

// Set assigns value to the field path. nil clears the addressed field.
func (m *Message) Set(field string, value any) error {
	if field == "" {
		return m.setSelf(value)
	}
	if m.schema == nil {
		return ErrFieldNotFound
	}
	segs, err := parsePath(field)
	if err != nil {
		return err
	}
	return m.setPath(segs, value)
}

func (m *Message) setSelf(value any) error {
	if m.schema == nil {
		return ErrFieldNotFound
	}
	if value == nil {
		m.clear()
		return nil
	}
	src, ok := value.(*Message)
	if !ok {
		return ErrTypeMismatch
	}
	if m.schema == nil || !schemasEqual(m.schema, src.schema) {
		return ErrTypeMismatch
	}
	return copyMessage(m, src)
}

func (m *Message) setPath(segs []pathSeg, value any) error {
	cur := m
	for i := 0; i < len(segs)-1; i++ {
		next, err := cur.descendForSet(segs[i])
		if err != nil {
			return err
		}
		if next == nil {
			return ErrFieldNotFound
		}
		cur = next
	}
	return cur.setLeaf(segs[len(segs)-1], value)
}

func (m *Message) descendForSet(seg pathSeg) (*Message, error) {
	if m.schema == nil {
		return nil, ErrFieldNotFound
	}
	idx := m.schema.fieldIndex(seg.name)
	if idx < 0 {
		return nil, ErrFieldNotFound
	}
	fs := &m.schema.fields[idx]
	fv := &m.fields[idx]

	if fs.Repeated {
		if seg.index < 0 {
			return nil, ErrFieldNotFound
		}
		if fs.Type != FieldMessage {
			return nil, ErrFieldNotFound
		}
		if !fv.present {
			return nil, ErrIndexOutOfRange
		}
		elem, err := sliceIndex(fv.value, seg.index)
		if err != nil {
			return nil, err
		}
		nm, ok := elem.(*Message)
		if !ok {
			return nil, ErrFieldNotFound
		}
		if nm == nil {
			nm = newMessage(&fs.Schema)
			if err := setSliceIndex(fv.value, seg.index, nm); err != nil {
				return nil, err
			}
		}
		return nm, nil
	}

	// 非 repeated 字段使用下标是非法路径。
	if seg.index >= 0 {
		return nil, ErrFieldNotFound
	}

	if fs.Type != FieldMessage {
		return nil, ErrFieldNotFound
	}
	if !fv.present || fv.value == nil {
		nm := newMessage(&fs.Schema)
		fv.present = true
		fv.value = nm
		return nm, nil
	}
	return fv.value.(*Message), nil
}

func (m *Message) setLeaf(seg pathSeg, value any) error {
	if m.schema == nil {
		return ErrFieldNotFound
	}
	idx := m.schema.fieldIndex(seg.name)
	if idx < 0 {
		return ErrFieldNotFound
	}
	fs := &m.schema.fields[idx]
	fv := &m.fields[idx]

	if seg.index >= 0 {
		if !fs.Repeated {
			return ErrFieldNotFound
		}
		return m.setLeafIndex(fs, fv, seg.index, value)
	}
	return m.setFieldWhole(fs, fv, value)
}

func (m *Message) setFieldWhole(fs *FieldSchema, fv *fieldValue, value any) error {
	if value == nil {
		fv.present = false
		fv.value = nil
		return nil
	}
	if fs.Repeated {
		return setRepeated(fs, fv, value)
	}
	if fs.Type == FieldMessage {
		src, ok := value.(*Message)
		if !ok {
			return ErrTypeMismatch
		}
		if src.schema == nil || !schemasEqual(&fs.Schema, src.schema) {
			return ErrTypeMismatch
		}
		nm := newMessage(&fs.Schema)
		if err := copyMessage(nm, src); err != nil {
			return err
		}
		fv.present = true
		fv.value = nm
		return nil
	}
	cv, err := convertScalar(fs.Type, value)
	if err != nil {
		return err
	}
	fv.present = true
	fv.value = cv
	return nil
}

func setRepeated(fs *FieldSchema, fv *fieldValue, value any) error {
	if fs.Type == FieldMessage {
		src, ok := value.([]*Message)
		if !ok {
			return ErrTypeMismatch
		}
		dst := make([]*Message, len(src))
		for i, sm := range src {
			if sm == nil {
				continue
			}
			if sm.schema == nil || !schemasEqual(&fs.Schema, sm.schema) {
				return ErrTypeMismatch
			}
			nm := newMessage(&fs.Schema)
			if err := copyMessage(nm, sm); err != nil {
				return err
			}
			dst[i] = nm
		}
		fv.present = true
		fv.value = dst
		return nil
	}

	dst, err := convertRepeatedScalar(fs.Type, value)
	if err != nil {
		return err
	}
	fv.present = true
	fv.value = dst
	return nil
}

func (m *Message) setLeafIndex(fs *FieldSchema, fv *fieldValue, idx int, value any) error {
	if !fv.present {
		return ErrIndexOutOfRange
	}
	if fs.Type == FieldMessage {
		s, ok := fv.value.([]*Message)
		if !ok {
			return ErrIndexOutOfRange
		}
		if idx < 0 || idx >= len(s) {
			return ErrIndexOutOfRange
		}
		if value == nil {
			s[idx] = nil
			return nil
		}
		src, ok := value.(*Message)
		if !ok {
			return ErrTypeMismatch
		}
		if src.schema == nil || !schemasEqual(&fs.Schema, src.schema) {
			return ErrTypeMismatch
		}
		nm := newMessage(&fs.Schema)
		if err := copyMessage(nm, src); err != nil {
			return err
		}
		s[idx] = nm
		return nil
	}

	rv := reflect.ValueOf(fv.value)
	if idx < 0 || idx >= rv.Len() {
		return ErrIndexOutOfRange
	}
	if value == nil {
		rv.Index(idx).Set(reflect.Zero(rv.Type().Elem()))
		return nil
	}
	cv, err := convertScalar(fs.Type, value)
	if err != nil {
		return err
	}
	rv.Index(idx).Set(reflect.ValueOf(cv))
	return nil
}

func wrapField(fs *FieldSchema, fv *fieldValue) *Message {
	if fv == nil || !fv.present {
		return nil
	}
	if fs.Type == FieldMessage && !fs.Repeated {
		if v, ok := fv.value.(*Message); ok && v != nil {
			return v
		}
		return nil
	}
	return &Message{valueNode: true, value: fv.value}
}

func wrapElement(fs *FieldSchema, elem any) *Message {
	if fs.Type == FieldMessage {
		return elem.(*Message)
	}
	return &Message{valueNode: true, value: elem}
}

func copyMessage(dst, src *Message) error {
	if !schemasEqual(dst.schema, src.schema) {
		return ErrTypeMismatch
	}
	for i := range dst.schema.fields {
		df := &dst.schema.fields[i]
		if src.fields[i].present {
			dst.fields[i].present = true
			dst.fields[i].value = cloneFieldValue(df, src.fields[i].value)
		} else {
			dst.fields[i].present = false
			dst.fields[i].value = nil
		}
	}
	return nil
}

func cloneFieldValue(fs *FieldSchema, val any) any {
	if fs.Type == FieldMessage {
		if fs.Repeated {
			src := val.([]*Message)
			dst := make([]*Message, len(src))
			for i, sm := range src {
				if sm == nil {
					continue
				}
				nm := newMessage(&fs.Schema)
				_ = copyMessage(nm, sm)
				dst[i] = nm
			}
			return dst
		}
		sm := val.(*Message)
		if sm == nil {
			return nil
		}
		nm := newMessage(&fs.Schema)
		_ = copyMessage(nm, sm)
		return nm
	}
	if fs.Repeated {
		return cloneSlice(val)
	}
	if b, ok := val.([]byte); ok {
		return append([]byte(nil), b...)
	}
	return val
}

func cloneSlice(val any) any {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Slice {
		return val
	}
	if rv.IsNil() {
		return reflect.Zero(rv.Type()).Interface()
	}
	nv := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Slice && elem.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, elem.Len())
			reflect.Copy(reflect.ValueOf(b), elem)
			nv.Index(i).Set(reflect.ValueOf(b))
		} else {
			nv.Index(i).Set(elem)
		}
	}
	return nv.Interface()
}

func sliceIndex(val any, i int) (any, error) {
	rv := reflect.ValueOf(val)
	if i < 0 || i >= rv.Len() {
		return nil, ErrIndexOutOfRange
	}
	return rv.Index(i).Interface(), nil
}

func setSliceIndex(val any, i int, elem any) error {
	rv := reflect.ValueOf(val)
	if i < 0 || i >= rv.Len() {
		return ErrIndexOutOfRange
	}
	rv.Index(i).Set(reflect.ValueOf(elem))
	return nil
}

func newScalarSlice(ft FieldType) any {
	switch ft {
	case FieldInt32:
		return []int32{}
	case FieldInt64:
		return []int64{}
	case FieldUint32:
		return []uint32{}
	case FieldUint64:
		return []uint64{}
	case FieldFloat:
		return []float32{}
	case FieldDouble:
		return []float64{}
	case FieldBool:
		return []bool{}
	case FieldString:
		return []string{}
	case FieldBytes:
		return [][]byte{}
	}
	return nil
}

func appendSliceElem(val any, elem any) any {
	rv := reflect.ValueOf(val)
	return reflect.Append(rv, reflect.ValueOf(elem)).Interface()
}
