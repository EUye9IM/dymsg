package dymsg

// Message is a structured message instance holding a schema and field values.
type Message struct {
	schema MessageSchema
	vals   []any
	set    []bool
}

func newMessage(s MessageSchema) *Message {
	return &Message{
		schema: s,
		vals:   make([]any, len(s.fields)),
		set:    make([]bool, len(s.fields)),
	}
}

// Get returns the value at path.
func (m *Message) Get(path string) Value {
	var pbuf [8]pathSeg
	segs, err := parsePathBuf(path, pbuf[:0])
	if err != nil {
		return Value{err: err}
	}
	if len(segs) == 0 {
		return Value{typ: FieldMessage, exists: true, isSet: true, val: m}
	}
	return m.getValue(segs)
}

func (m *Message) getValue(segs []pathSeg) Value {
	schema := m.schema
	msg := m
	for i, seg := range segs {
		last := i == len(segs)-1
		idx, fs, ok := schema.lookupName(seg.field)
		if !ok {
			return Value{err: ErrFieldNotFound}
		}
		if seg.hasIdx {
			if !fs.Repeated {
				return Value{err: ErrFieldNotFound}
			}
			if msg == nil || !msg.set[idx] {
				return Value{err: ErrIndexOutOfRange}
			}
			if seg.idx < 0 || seg.idx >= sliceLen(msg.vals[idx]) {
				return Value{err: ErrIndexOutOfRange}
			}
			elem, _ := sliceIndex(msg.vals[idx], seg.idx)
			if last {
				return elementValue(fs.Type, elem)
			}
			if fs.Type != FieldMessage {
				return Value{err: ErrFieldNotFound}
			}
			em, _ := elem.(*Message)
			schema = fs.Schema
			msg = em
		} else {
			if last {
				if msg == nil || !msg.set[idx] {
					return Value{typ: fs.Type, repeated: fs.Repeated, exists: true, isSet: false}
				}
				return Value{typ: fs.Type, repeated: fs.Repeated, exists: true, isSet: true, val: msg.vals[idx]}
			}
			if fs.Type != FieldMessage {
				return Value{err: ErrFieldNotFound}
			}
			if msg == nil || !msg.set[idx] {
				schema = fs.Schema
				msg = nil
			} else {
				em, _ := msg.vals[idx].(*Message)
				schema = fs.Schema
				msg = em
			}
		}
	}
	return Value{err: ErrFieldNotFound}
}

// Set assigns value at path.
func (m *Message) Set(path string, value any) error {
	var pbuf [8]pathSeg
	segs, err := parsePathBuf(path, pbuf[:0])
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return m.setSelf(value)
	}
	last := segs[len(segs)-1]
	parent, err := m.descend(segs[:len(segs)-1], true)
	if err != nil {
		return err
	}
	idx, fs, ok := parent.schema.lookupName(last.field)
	if !ok {
		return ErrFieldNotFound
	}
	if last.hasIdx {
		return parent.setElement(idx, fs, last.idx, value)
	}
	return parent.setField(idx, fs, value)
}

// Append appends one element to the repeated field at path.
func (m *Message) Append(path string, value any) error {
	var pbuf [8]pathSeg
	segs, err := parsePathBuf(path, pbuf[:0])
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return ErrFieldNotFound
	}
	last := segs[len(segs)-1]
	if last.hasIdx {
		return ErrFieldNotFound
	}
	parent, err := m.descend(segs[:len(segs)-1], true)
	if err != nil {
		return err
	}
	idx, fs, ok := parent.schema.lookupName(last.field)
	if !ok {
		return ErrFieldNotFound
	}
	return parent.appendField(idx, fs, value)
}

// Clear marks the field at path as unset.
func (m *Message) Clear(path string) error {
	var pbuf [8]pathSeg
	segs, err := parsePathBuf(path, pbuf[:0])
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		m.clearAll()
		return nil
	}
	return m.clearPath(segs)
}

// Has reports whether the field at path exists and is set.
func (m *Message) Has(path string) bool {
	v := m.Get(path)
	return v.Exists() && v.IsSet()
}

// SetFields returns the names of currently-set fields in schema declaration order.
func (m *Message) SetFields() []string {
	names := make([]string, 0, len(m.schema.fields))
	for i, fs := range m.schema.fields {
		if m.set[i] {
			names = append(names, fs.Name)
		}
	}
	return names
}

func (m *Message) setSelf(value any) error {
	if value == nil {
		m.clearAll()
		return nil
	}
	src, ok := value.(*Message)
	if !ok {
		return ErrTypeMismatch
	}
	if src == nil {
		return ErrTypeMismatch
	}
	if !schemasEqual(m.schema, src.schema) {
		return ErrTypeMismatch
	}
	m.copyFrom(src)
	return nil
}

func (m *Message) setField(idx int, fs FieldSchema, value any) error {
	if value == nil {
		m.set[idx] = false
		m.vals[idx] = nil
		return nil
	}
	if fs.Repeated {
		v, err := convertToSlice(fs, value)
		if err != nil {
			return err
		}
		m.vals[idx] = v
		m.set[idx] = true
		return nil
	}
	if fs.Type == FieldMessage {
		mm, err := convertMessage(value, fs.Schema)
		if err != nil {
			return err
		}
		m.vals[idx] = mm
		m.set[idx] = true
		return nil
	}
	v, err := convertScalar(fs.Type, value)
	if err != nil {
		return err
	}
	m.vals[idx] = v
	m.set[idx] = true
	return nil
}

func (m *Message) setElement(idx int, fs FieldSchema, i int, value any) error {
	if !fs.Repeated {
		return ErrFieldNotFound
	}
	if !m.set[idx] {
		return ErrIndexOutOfRange
	}
	if i < 0 || i >= sliceLen(m.vals[idx]) {
		return ErrIndexOutOfRange
	}
	if fs.Type == FieldMessage {
		if value == nil {
			m.vals[idx].([]*Message)[i] = nil
			return nil
		}
		mm, err := convertMessage(value, fs.Schema)
		if err != nil {
			return err
		}
		m.vals[idx].([]*Message)[i] = mm
		return nil
	}
	if value == nil {
		return ErrTypeMismatch
	}
	elem, err := convertScalar(fs.Type, value)
	if err != nil {
		return err
	}
	setSliceElem(m.vals[idx], i, elem)
	return nil
}

func (m *Message) appendField(idx int, fs FieldSchema, value any) error {
	if !fs.Repeated {
		return ErrTypeMismatch
	}
	if fs.Type == FieldMessage {
		mm, err := convertMessage(value, fs.Schema)
		if err != nil {
			return err
		}
		if !m.set[idx] {
			m.vals[idx] = []*Message{}
			m.set[idx] = true
		}
		m.vals[idx] = append(m.vals[idx].([]*Message), mm)
		return nil
	}
	elem, err := convertScalar(fs.Type, value)
	if err != nil {
		return err
	}
	if !m.set[idx] {
		m.vals[idx] = makeSlice(fs.Type, 0)
		m.set[idx] = true
	}
	m.vals[idx] = appendToSlice(fs.Type, m.vals[idx], elem)
	return nil
}

func (m *Message) clearPath(segs []pathSeg) error {
	seg := segs[0]
	idx, fs, ok := m.schema.lookupName(seg.field)
	if !ok {
		return ErrFieldNotFound
	}
	if seg.hasIdx {
		if !fs.Repeated {
			return ErrFieldNotFound
		}
		if !m.set[idx] {
			return ErrIndexOutOfRange
		}
		if seg.idx < 0 || seg.idx >= sliceLen(m.vals[idx]) {
			return ErrIndexOutOfRange
		}
		if len(segs) == 1 {
			return m.clearElement(idx, fs, seg.idx)
		}
		if fs.Type != FieldMessage {
			return ErrFieldNotFound
		}
		elem, _ := sliceIndex(m.vals[idx], seg.idx)
		em, _ := elem.(*Message)
		if em == nil {
			return ErrFieldNotFound
		}
		return em.clearPath(segs[1:])
	}
	if len(segs) == 1 {
		m.clearField(idx)
		return nil
	}
	if fs.Type != FieldMessage {
		return ErrFieldNotFound
	}
	if !m.set[idx] {
		return verifyClearPath(fs.Schema, segs[1:])
	}
	em, _ := m.vals[idx].(*Message)
	if em == nil {
		return verifyClearPath(fs.Schema, segs[1:])
	}
	return em.clearPath(segs[1:])
}

func (m *Message) clearField(idx int) {
	m.set[idx] = false
	m.vals[idx] = nil
}

func (m *Message) clearElement(idx int, fs FieldSchema, i int) error {
	if fs.Type == FieldMessage {
		m.vals[idx].([]*Message)[i] = nil
		return nil
	}
	return ErrFieldNotFound
}

func (m *Message) clearAll() {
	for i := range m.set {
		m.set[i] = false
		m.vals[i] = nil
	}
}

func verifyClearPath(s MessageSchema, segs []pathSeg) error {
	for i, seg := range segs {
		_, fs, ok := s.lookupName(seg.field)
		if !ok {
			return ErrFieldNotFound
		}
		if seg.hasIdx {
			if !fs.Repeated {
				return ErrFieldNotFound
			}
			return ErrIndexOutOfRange
		}
		if i == len(segs)-1 {
			return nil
		}
		if fs.Type != FieldMessage {
			return ErrFieldNotFound
		}
		s = fs.Schema
	}
	return nil
}

// descend navigates through intermediate segments, optionally auto-creating
// unset intermediate message fields.
func (m *Message) descend(segs []pathSeg, create bool) (*Message, error) {
	cur := m
	for _, seg := range segs {
		idx, fs, ok := cur.schema.lookupName(seg.field)
		if !ok {
			return nil, ErrFieldNotFound
		}
		if seg.hasIdx {
			if !fs.Repeated {
				return nil, ErrFieldNotFound
			}
			if !cur.set[idx] {
				return nil, ErrIndexOutOfRange
			}
			if seg.idx < 0 || seg.idx >= sliceLen(cur.vals[idx]) {
				return nil, ErrIndexOutOfRange
			}
			if fs.Type != FieldMessage {
				return nil, ErrFieldNotFound
			}
			elem, _ := sliceIndex(cur.vals[idx], seg.idx)
			em, _ := elem.(*Message)
			if em == nil {
				return nil, ErrFieldNotFound
			}
			cur = em
			continue
		}
		if fs.Type != FieldMessage {
			return nil, ErrFieldNotFound
		}
		if !cur.set[idx] {
			if !create {
				return nil, ErrFieldNotFound
			}
			nm := newMessage(fs.Schema)
			cur.vals[idx] = nm
			cur.set[idx] = true
			cur = nm
			continue
		}
		em, _ := cur.vals[idx].(*Message)
		if em == nil {
			if !create {
				return nil, ErrFieldNotFound
			}
			nm := newMessage(fs.Schema)
			cur.vals[idx] = nm
			cur.set[idx] = true
			cur = nm
			continue
		}
		cur = em
	}
	return cur, nil
}
