package kube

import "time"

// Partial is the metadata that can be recovered from any stored protobuf
// object without a generated type, because ObjectMeta is always field 1 and
// its own layout is fixed.
type Partial struct {
	Name            string            `json:"name,omitempty"`
	GenerateName    string            `json:"generateName,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	UID             string            `json:"uid,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Generation      int64             `json:"generation,omitempty"`
	Created         *time.Time        `json:"creationTimestamp,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	OwnerRefs       []OwnerRef        `json:"ownerReferences,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
}

// PartialMeta recovers ObjectMeta from a protobuf object body.
// ObjectMeta: 1=name 2=generateName 3=namespace 5=uid 6=resourceVersion
// 7=generation 8=creationTimestamp 11=labels 12=annotations
// 13=ownerReferences 14=finalizers. OwnerReference itself is numbered
// 1=kind 3=name 4=uid 5=apiVersion 6=controller.
func PartialMeta(raw []byte) Partial {
	var p Partial
	om := protoField(raw, 1)
	if om == nil {
		return p
	}
	p.Name = string(protoField(om, 1))
	p.GenerateName = string(protoField(om, 2))
	p.Namespace = string(protoField(om, 3))
	p.UID = string(protoField(om, 5))
	p.ResourceVersion = string(protoField(om, 6))
	p.Generation = protoVarint(om, 7)
	if ts := protoField(om, 8); ts != nil {
		if sec := protoVarint(ts, 1); sec > 0 {
			t := time.Unix(sec, 0).UTC()
			p.Created = &t
		}
	}
	p.Labels = protoStringMap(om, 11)
	p.Annotations = protoStringMap(om, 12)
	for _, ref := range protoFields(om, 13) {
		p.OwnerRefs = append(p.OwnerRefs, OwnerRef{
			APIVersion: string(protoField(ref, 5)),
			Kind:       string(protoField(ref, 1)),
			Name:       string(protoField(ref, 3)),
			UID:        string(protoField(ref, 4)),
			Controller: protoVarint(ref, 6) == 1,
		})
	}
	for _, f := range protoFields(om, 14) {
		p.Finalizers = append(p.Finalizers, string(f))
	}
	return p
}

// protoStringMap reads a protobuf map<string,string> field, which is encoded
// as a repeated message with 1=key and 2=value.
func protoStringMap(b []byte, field uint64) map[string]string {
	entries := protoFields(b, field)
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		out[string(protoField(e, 1))] = string(protoField(e, 2))
	}
	return out
}
