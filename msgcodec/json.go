package msgcodec

import (
	"bytes"
	"encoding/json"
)

// EncodeJSON 将当前消息编码为 JSON 字节。
func (m *msgImpl) EncodeJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	first := true
	for _, rf := range m.schema.decl {
		fv := m.values[rf.name]
		if fv == nil || !fv.present {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(rf.name)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := m.encodeJSONField(rf, fv)
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (m *msgImpl) encodeJSONField(rf *runtimeField, fv *fieldValue) ([]byte, error) {
	if rf.typ == FieldMessage {
		if rf.repeated {
			var b bytes.Buffer
			b.WriteByte('[')
			for i, k := range fv.kids {
				if i > 0 {
					b.WriteByte(',')
				}
				vb, err := k.EncodeJSON()
				if err != nil {
					return nil, err
				}
				b.Write(vb)
			}
			b.WriteByte(']')
			return b.Bytes(), nil
		}
		return fv.child.EncodeJSON()
	}
	if rf.repeated {
		return json.Marshal(fv.list)
	}
	return json.Marshal(fv.scalar)
}

// EncodeJSON 将标量/列表包装编码为 JSON。
func (v *valueMsg) EncodeJSON() ([]byte, error) {
	if !v.isList {
		return json.Marshal(v.scalar)
	}
	if v.typ == FieldMessage {
		var b bytes.Buffer
		b.WriteByte('[')
		for i, mm := range v.msgs {
			if i > 0 {
				b.WriteByte(',')
			}
			vb, err := mm.EncodeJSON()
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	}
	return json.Marshal(v.list)
}

// DecodeJSON 从 JSON 字节解码到当前消息。
func (m *msgImpl) DecodeJSON(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return ErrTruncated
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ErrMalformedData
	}
	for _, rf := range m.schema.decl {
		r, ok := raw[rf.name]
		if !ok || string(r) == "null" {
			continue
		}
		if err := m.decodeJSONField(rf, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *msgImpl) decodeJSONField(rf *runtimeField, r json.RawMessage) error {
	fv := m.ensure(rf)
	if rf.typ == FieldMessage {
		if rf.repeated {
			var arr []json.RawMessage
			if err := json.Unmarshal(r, &arr); err != nil {
				return ErrMalformedData
			}
			kids := make([]*msgImpl, 0, len(arr))
			for _, el := range arr {
				if string(el) == "null" {
					continue
				}
				k := newMessage(rf.child)
				if err := k.DecodeJSON(el); err != nil {
					return err
				}
				kids = append(kids, k)
			}
			fv.kids = kids
			fv.present = true
			return nil
		}
		k := newMessage(rf.child)
		if err := k.DecodeJSON(r); err != nil {
			return err
		}
		fv.child = k
		fv.present = true
		return nil
	}
	if rf.repeated {
		var arr []json.RawMessage
		if err := json.Unmarshal(r, &arr); err != nil {
			return ErrMalformedData
		}
		list := make([]any, 0, len(arr))
		for _, el := range arr {
			v, err := decodeScalarJSON(el, rf.typ)
			if err != nil {
				return ErrMalformedData
			}
			list = append(list, v)
		}
		fv.list = list
		fv.present = true
		return nil
	}
	v, err := decodeScalarJSON(r, rf.typ)
	if err != nil {
		return ErrMalformedData
	}
	fv.scalar = v
	fv.present = true
	return nil
}

func decodeScalarJSON(r json.RawMessage, ft FieldType) (any, error) {
	switch ft {
	case FieldString:
		var s string
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, ErrMalformedData
		}
		return s, nil
	case FieldBytes:
		var d []byte
		if err := json.Unmarshal(r, &d); err != nil {
			return nil, ErrMalformedData
		}
		return d, nil
	case FieldBool:
		var b bool
		if err := json.Unmarshal(r, &b); err != nil {
			return nil, ErrMalformedData
		}
		return b, nil
	case FieldInt32, FieldInt64, FieldUint32, FieldUint64, FieldFloat, FieldDouble:
		dec := json.NewDecoder(bytes.NewReader(r))
		dec.UseNumber()
		var num json.Number
		if err := dec.Decode(&num); err != nil {
			return nil, ErrMalformedData
		}
		return convertTo(num.String(), ft)
	}
	return nil, ErrMalformedData
}
