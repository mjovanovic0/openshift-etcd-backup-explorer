// Package kube understands how the Kubernetes API server stores objects in
// etcd. Values are either JSON (custom resources) or the Kubernetes protobuf
// envelope "k8s\0" followed by a runtime.Unknown message (built in and
// OpenShift types).
package kube

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// ProtoMagic is the prefix the Kubernetes protobuf serializer writes.
var ProtoMagic = []byte{'k', '8', 's', 0}

// Encoding reports how a stored value is serialized.
type Encoding string

const (
	EncodingProtobuf Encoding = "protobuf"
	EncodingJSON     Encoding = "json"
	EncodingUnknown  Encoding = "unknown"
)

// Meta is the small amount of information needed to index an object. It is
// extracted without fully decoding the object, which keeps indexing a large
// snapshot fast and free of generated type dependencies.
type Meta struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	UID        string
	Created    time.Time
	Encoding   Encoding
}

// DetectEncoding classifies a stored value.
func DetectEncoding(v []byte) Encoding {
	switch {
	case bytes.HasPrefix(v, ProtoMagic):
		return EncodingProtobuf
	case len(bytes.TrimLeft(v, " \t\r\n")) > 0 && bytes.TrimLeft(v, " \t\r\n")[0] == '{':
		return EncodingJSON
	default:
		return EncodingUnknown
	}
}

// ExtractMeta pulls type and object metadata out of a stored value.
func ExtractMeta(value []byte) (Meta, bool) {
	switch DetectEncoding(value) {
	case EncodingProtobuf:
		return protoMeta(value)
	case EncodingJSON:
		return jsonMeta(value)
	default:
		return Meta{Encoding: EncodingUnknown}, false
	}
}

// UnknownEnvelope is the decoded runtime.Unknown wrapper: the type of the
// object plus the bytes of the object itself.
type UnknownEnvelope struct {
	APIVersion string
	Kind       string
	Raw        []byte
}

// ParseEnvelope decodes the "k8s\0" runtime.Unknown wrapper:
// 1=TypeMeta{1=apiVersion,2=kind} 2=Raw 3=contentEncoding 4=contentType.
func ParseEnvelope(value []byte) (UnknownEnvelope, bool) {
	if !bytes.HasPrefix(value, ProtoMagic) {
		return UnknownEnvelope{}, false
	}
	body := value[4:]
	var env UnknownEnvelope
	if tm := protoField(body, 1); tm != nil {
		env.APIVersion = string(protoField(tm, 1))
		env.Kind = string(protoField(tm, 2))
	}
	env.Raw = protoField(body, 2)
	return env, env.Kind != ""
}

func protoMeta(value []byte) (Meta, bool) {
	env, ok := ParseEnvelope(value)
	if !ok {
		return Meta{Encoding: EncodingProtobuf}, false
	}
	m := Meta{
		APIVersion: env.APIVersion,
		Kind:       env.Kind,
		Encoding:   EncodingProtobuf,
	}
	// Every generated type carries ObjectMeta as field 1, and inside it
	// 1=name 3=namespace 5=uid 8=creationTimestamp.
	if om := protoField(env.Raw, 1); om != nil {
		m.Name = string(protoField(om, 1))
		m.Namespace = string(protoField(om, 3))
		m.UID = string(protoField(om, 5))
		if ts := protoField(om, 8); ts != nil {
			if sec := protoVarint(ts, 1); sec > 0 {
				m.Created = time.Unix(sec, 0).UTC()
			}
		}
	}
	return m, true
}

type jsonShallow struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		UID               string `json:"uid"`
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
}

func jsonMeta(value []byte) (Meta, bool) {
	var s jsonShallow
	if err := json.Unmarshal(value, &s); err != nil {
		return Meta{Encoding: EncodingJSON}, false
	}
	m := Meta{
		APIVersion: s.APIVersion,
		Kind:       s.Kind,
		Name:       s.Metadata.Name,
		Namespace:  s.Metadata.Namespace,
		UID:        s.Metadata.UID,
		Encoding:   EncodingJSON,
	}
	if s.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, s.Metadata.CreationTimestamp); err == nil {
			m.Created = t.UTC()
		}
	}
	return m, s.Kind != ""
}

// SplitAPIVersion turns "apps/v1" into ("apps", "v1") and "v1" into ("", "v1").
func SplitAPIVersion(apiVersion string) (group, version string) {
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		return apiVersion[:i], apiVersion[i+1:]
	}
	return "", apiVersion
}

// NameFromKey recovers a name and namespace from the etcd key when the object
// itself carries no ObjectMeta, which happens for a few internal API server
// records such as masterleases and service IP ranges.
func NameFromKey(key string) (namespace, name string) {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	if len(parts) == 0 {
		return "", key
	}
	return "", parts[len(parts)-1]
}

// protoField returns the bytes of the first length delimited field with the
// given number, skipping over every other field.
func protoField(b []byte, want uint64) []byte {
	i := 0
	for i < len(b) {
		tag, n := uvarint(b[i:])
		if n <= 0 {
			return nil
		}
		i += n
		field, wire := tag>>3, tag&7
		switch wire {
		case 0:
			_, n := uvarint(b[i:])
			if n <= 0 {
				return nil
			}
			i += n
		case 1:
			i += 8
		case 5:
			i += 4
		case 2:
			l, n := uvarint(b[i:])
			if n <= 0 || i+n+int(l) > len(b) {
				return nil
			}
			i += n
			if field == want {
				return b[i : i+int(l)]
			}
			i += int(l)
		default:
			return nil
		}
	}
	return nil
}

// protoVarint returns the first varint field with the given number.
func protoVarint(b []byte, want uint64) int64 {
	i := 0
	for i < len(b) {
		tag, n := uvarint(b[i:])
		if n <= 0 {
			return 0
		}
		i += n
		field, wire := tag>>3, tag&7
		switch wire {
		case 0:
			v, n := uvarint(b[i:])
			if n <= 0 {
				return 0
			}
			i += n
			if field == want {
				return int64(v)
			}
		case 1:
			i += 8
		case 5:
			i += 4
		case 2:
			l, n := uvarint(b[i:])
			if n <= 0 || i+n+int(l) > len(b) {
				return 0
			}
			i += n + int(l)
		default:
			return 0
		}
	}
	return 0
}

func uvarint(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i := 0; i < len(b) && i < 10; i++ {
		c := b[i]
		if c < 0x80 {
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, -1
}
