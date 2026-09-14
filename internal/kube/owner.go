package kube

import "encoding/json"

// OwnerRef is one entry of metadata.ownerReferences.
type OwnerRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller bool   `json:"controller"`
}

// OwnerRefs reads metadata.ownerReferences out of a stored value in either
// encoding.
func OwnerRefs(value []byte) []OwnerRef {
	switch DetectEncoding(value) {
	case EncodingProtobuf:
		env, ok := ParseEnvelope(value)
		if !ok {
			return nil
		}
		om := protoField(env.Raw, 1)
		if om == nil {
			return nil
		}
		// ObjectMeta field 13 is repeated OwnerReference. Its fields are not
		// numbered in declaration order: 1=kind 3=name 4=uid 5=apiVersion
		// 6=controller.
		var out []OwnerRef
		for _, ref := range protoFields(om, 13) {
			out = append(out, OwnerRef{
				APIVersion: string(protoField(ref, 5)),
				Kind:       string(protoField(ref, 1)),
				Name:       string(protoField(ref, 3)),
				UID:        string(protoField(ref, 4)),
				Controller: protoVarint(ref, 6) == 1,
			})
		}
		return out

	case EncodingJSON:
		var s struct {
			Metadata struct {
				OwnerReferences []OwnerRef `json:"ownerReferences"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(value, &s); err != nil {
			return nil
		}
		return s.Metadata.OwnerReferences
	}
	return nil
}

// protoFields returns every length delimited field with the given number.
func protoFields(b []byte, want uint64) [][]byte {
	var out [][]byte
	i := 0
	for i < len(b) {
		tag, n := uvarint(b[i:])
		if n <= 0 {
			return out
		}
		i += n
		field, wire := tag>>3, tag&7
		switch wire {
		case 0:
			_, n := uvarint(b[i:])
			if n <= 0 {
				return out
			}
			i += n
		case 1:
			i += 8
		case 5:
			i += 4
		case 2:
			l, n := uvarint(b[i:])
			if n <= 0 || i+n+int(l) > len(b) {
				return out
			}
			i += n
			if field == want {
				out = append(out, b[i:i+int(l)])
			}
			i += int(l)
		default:
			return out
		}
	}
	return out
}
