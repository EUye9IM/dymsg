package msgcodec

// EncodeProto 将当前消息编码为 Protobuf wire format 字节。
func (m *msgImpl) EncodeProto() ([]byte, error) {
	var b []byte
	for _, rf := range m.schema.numAsc {
		fv := m.values[rf.name]
		if fv == nil || !fv.present {
			continue
		}
		var err error
		b, err = appendProtoField(b, rf, fv)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func appendProtoField(b []byte, rf *runtimeField, fv *fieldValue) ([]byte, error) {
	if rf.typ == FieldMessage {
		if rf.repeated {
			for _, k := range fv.kids {
				pb, err := k.EncodeProto()
				if err != nil {
					return nil, err
				}
				b = appendTag(b, rf.num, wireBytes)
				b = appendVarint(b, uint64(len(pb)))
				b = append(b, pb...)
			}
			return b, nil
		}
		pb, err := fv.child.EncodeProto()
		if err != nil {
			return nil, err
		}
		b = appendTag(b, rf.num, wireBytes)
		b = appendVarint(b, uint64(len(pb)))
		return append(b, pb...), nil
	}
	if rf.repeated {
		if isPacked(rf.typ) {
			var p []byte
			for _, el := range fv.list {
				p, _ = appendScalarVal(p, rf.typ, el)
			}
			b = appendTag(b, rf.num, wireBytes)
			b = appendVarint(b, uint64(len(p)))
			return append(b, p...), nil
		}
		for _, el := range fv.list {
			b = appendTag(b, rf.num, wireBytes)
			b, _ = appendScalarVal(b, rf.typ, el)
		}
		return b, nil
	}
	wt, _ := fieldWire(rf.typ)
	b = appendTag(b, rf.num, wt)
	return appendScalarVal(b, rf.typ, fv.scalar)
}

// EncodeProto 将标量/列表包装编码为裸值(不含字段 key)。
func (v *valueMsg) EncodeProto() ([]byte, error) {
	var b []byte
	if !v.isList {
		return appendScalarVal(b, v.typ, v.scalar)
	}
	if v.typ == FieldMessage {
		for _, mm := range v.msgs {
			pb, err := mm.EncodeProto()
			if err != nil {
				return nil, err
			}
			b = appendVarint(b, uint64(len(pb)))
			b = append(b, pb...)
		}
		return b, nil
	}
	for _, el := range v.list {
		b, _ = appendScalarVal(b, v.typ, el)
	}
	return b, nil
}

// DecodeProto 从 Protobuf wire format 字节解码到当前消息。
func (m *msgImpl) DecodeProto(data []byte) error {
	if len(data) == 0 {
		return ErrTruncated
	}
	pos := 0
	for pos < len(data) {
		key, n, err := readVarint(data[pos:])
		if err != nil {
			return err
		}
		pos += n
		num := key >> 3
		wt := int(key & 7)
		if num == 0 || wt == 3 || wt == 4 || wt == 6 || wt == 7 {
			return ErrMalformedData
		}
		rf := m.schema.byNum[int(num)]
		if rf == nil {
			consumed, err := skipField(data[pos:], wt)
			if err != nil {
				return err
			}
			pos += consumed
			continue
		}
		var err2 error
		if rf.typ == FieldMessage {
			pos, err2 = m.parseMessageField(rf, data, pos, wt)
		} else if isPacked(rf.typ) {
			pos, err2 = m.parsePackedScalar(rf, data, pos, wt)
		} else {
			pos, err2 = m.parseScalarOrBytes(rf, data, pos, wt)
		}
		if err2 != nil {
			return err2
		}
	}
	return nil
}

func (m *msgImpl) parseMessageField(rf *runtimeField, data []byte, pos, wt int) (int, error) {
	if wt != wireBytes {
		return 0, ErrMalformedData
	}
	ln, n, err := readVarint(data[pos:])
	if err != nil {
		return 0, err
	}
	if int(ln) > len(data)-pos-n {
		return 0, ErrTruncated
	}
	body := data[pos+n : pos+n+int(ln)]
	if rf.repeated {
		k := newMessage(rf.child)
		if err := k.DecodeProto(body); err != nil {
			return 0, err
		}
		fv := m.ensure(rf)
		fv.kids = append(fv.kids, k)
		fv.present = true
	} else {
		k := newMessage(rf.child)
		if err := k.DecodeProto(body); err != nil {
			return 0, err
		}
		fv := m.ensure(rf)
		fv.child = k
		fv.present = true
	}
	return pos + n + int(ln), nil
}

func (m *msgImpl) parsePackedScalar(rf *runtimeField, data []byte, pos, wt int) (int, error) {
	if rf.repeated && wt == wireBytes {
		ln, n, err := readVarint(data[pos:])
		if err != nil {
			return 0, err
		}
		if int(ln) > len(data)-pos-n {
			return 0, ErrTruncated
		}
		end := pos + n + int(ln)
		fv := m.ensure(rf)
		for pos+n < end {
			val, c, err := decodeScalarVal(rf.typ, data[pos+n:end])
			if err != nil {
				return 0, err
			}
			fv.list = append(fv.list, val)
			n += c
		}
		fv.present = true
		return end, nil
	}
	if rf.repeated {
		val, c, err := decodeScalarVal(rf.typ, data[pos:])
		if err != nil {
			return 0, err
		}
		fv := m.ensure(rf)
		fv.list = append(fv.list, val)
		fv.present = true
		return pos + c, nil
	}
	targetWt, _ := fieldWire(rf.typ)
	if wt != targetWt {
		return 0, ErrMalformedData
	}
	val, c, err := decodeScalarVal(rf.typ, data[pos:])
	if err != nil {
		return 0, err
	}
	fv := m.ensure(rf)
	fv.scalar = val
	fv.present = true
	return pos + c, nil
}

func (m *msgImpl) parseScalarOrBytes(rf *runtimeField, data []byte, pos, wt int) (int, error) {
	if rf.typ == FieldString || rf.typ == FieldBytes {
		if wt != wireBytes {
			return 0, ErrMalformedData
		}
		if rf.repeated {
			val, c, err := decodeScalarVal(rf.typ, data[pos:])
			if err != nil {
				return 0, err
			}
			fv := m.ensure(rf)
			fv.list = append(fv.list, val)
			fv.present = true
			return pos + c, nil
		}
		val, c, err := decodeScalarVal(rf.typ, data[pos:])
		if err != nil {
			return 0, err
		}
		fv := m.ensure(rf)
		fv.scalar = val
		fv.present = true
		return pos + c, nil
	}
	return m.parsePackedScalar(rf, data, pos, wt)
}
