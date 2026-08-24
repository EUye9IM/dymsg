package dymsg

func (m *Message) deepCopy() *Message {
	nm := &Message{
		schema: m.schema,
		vals:   make([]any, len(m.vals)),
		set:    make([]bool, len(m.set)),
	}
	copy(nm.set, m.set)
	for i := range m.vals {
		if !m.set[i] {
			continue
		}
		nm.vals[i] = deepCopyValue(m.schema.fields[i], m.vals[i])
	}
	return nm
}

func (m *Message) copyFrom(src *Message) {
	m.schema = src.schema
	m.vals = make([]any, len(src.vals))
	m.set = make([]bool, len(src.set))
	copy(m.set, src.set)
	for i := range src.vals {
		if !src.set[i] {
			continue
		}
		m.vals[i] = deepCopyValue(src.schema.fields[i], src.vals[i])
	}
}

func deepCopyValue(fs FieldSchema, val any) any {
	if fs.Repeated {
		switch s := val.(type) {
		case []int32:
			c := make([]int32, len(s))
			copy(c, s)
			return c
		case []int64:
			c := make([]int64, len(s))
			copy(c, s)
			return c
		case []uint32:
			c := make([]uint32, len(s))
			copy(c, s)
			return c
		case []uint64:
			c := make([]uint64, len(s))
			copy(c, s)
			return c
		case []float32:
			c := make([]float32, len(s))
			copy(c, s)
			return c
		case []float64:
			c := make([]float64, len(s))
			copy(c, s)
			return c
		case []bool:
			c := make([]bool, len(s))
			copy(c, s)
			return c
		case []string:
			c := make([]string, len(s))
			copy(c, s)
			return c
		case [][]byte:
			c := make([][]byte, len(s))
			for i, b := range s {
				if b != nil {
					cb := make([]byte, len(b))
					copy(cb, b)
					c[i] = cb
				}
			}
			return c
		case []*Message:
			c := make([]*Message, len(s))
			for i, mm := range s {
				if mm != nil {
					c[i] = mm.deepCopy()
				}
			}
			return c
		}
		return nil
	}
	switch fs.Type {
	case FieldBytes:
		b := val.([]byte)
		c := make([]byte, len(b))
		copy(c, b)
		return c
	case FieldMessage:
		mm, _ := val.(*Message)
		if mm == nil {
			return nil
		}
		return mm.deepCopy()
	default:
		return val
	}
}
