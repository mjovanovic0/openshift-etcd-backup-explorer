package kube

import (
	"bytes"
	"encoding/json"
	"fmt"

	openshiftapi "github.com/openshift/api"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	sigsyaml "sigs.k8s.io/yaml"
)

// scheme knows every built in Kubernetes type and every OpenShift type, which
// together cover everything the API server writes to etcd in protobuf form.
// Custom resources are stored as JSON and need no generated type.
var (
	scheme     = runtime.NewScheme()
	protoCodec runtime.Serializer
)

func init() {
	if err := kubeGroups.AddToScheme(scheme); err != nil {
		panic(fmt.Sprintf("install kubernetes types: %v", err))
	}
	if err := openshiftapi.InstallKube(scheme); err != nil {
		panic(fmt.Sprintf("install kubernetes types via openshift: %v", err))
	}
	if err := openshiftapi.Install(scheme); err != nil {
		panic(fmt.Sprintf("install openshift types: %v", err))
	}
	protoCodec = protobuf.NewSerializer(scheme, scheme)
}

// Object is a decoded stored value, ready to be shown as YAML or JSON.
type Object struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Encoding   Encoding        `json:"encoding"`
	JSON       json.RawMessage `json:"json"`
	YAML       string          `json:"yaml"`
	// Decoded reports whether the object body was understood. A protobuf value
	// whose type is not registered still yields its type and a clear reason.
	Decoded bool   `json:"decoded"`
	Reason  string `json:"reason,omitempty"`
}

// Decode turns a raw etcd value into indented JSON and YAML.
func Decode(value []byte) (*Object, error) {
	switch DetectEncoding(value) {
	case EncodingJSON:
		return decodeJSON(value)
	case EncodingProtobuf:
		return decodeProtobuf(value)
	default:
		return nil, fmt.Errorf("value is neither JSON nor Kubernetes protobuf (%d bytes)", len(value))
	}
}

func decodeJSON(value []byte) (*Object, error) {
	var generic map[string]any
	if err := json.Unmarshal(value, &generic); err != nil {
		return nil, fmt.Errorf("parse JSON value: %w", err)
	}
	pretty, err := json.MarshalIndent(generic, "", "  ")
	if err != nil {
		return nil, err
	}
	y, err := sigsyaml.JSONToYAML(value)
	if err != nil {
		return nil, fmt.Errorf("convert to YAML: %w", err)
	}
	apiVersion, _ := generic["apiVersion"].(string)
	kind, _ := generic["kind"].(string)
	return &Object{
		APIVersion: apiVersion,
		Kind:       kind,
		Encoding:   EncodingJSON,
		JSON:       pretty,
		YAML:       string(y),
		Decoded:    true,
	}, nil
}

func decodeProtobuf(value []byte) (*Object, error) {
	env, ok := ParseEnvelope(value)
	if !ok {
		return nil, fmt.Errorf("malformed Kubernetes protobuf envelope")
	}
	obj := &Object{APIVersion: env.APIVersion, Kind: env.Kind, Encoding: EncodingProtobuf}

	gv, err := schema.ParseGroupVersion(env.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parse apiVersion %q: %w", env.APIVersion, err)
	}
	gvk := gv.WithKind(env.Kind)

	decoded, _, err := protoCodec.Decode(value, &gvk, nil)
	if err != nil {
		// No generated type is compiled in. This happens for a type that has
		// been removed from the Kubernetes libraries, such as PodSecurityPolicy.
		// ObjectMeta sits at a fixed place inside every stored object, so show
		// that much rather than failing the request.
		obj.Reason = fmt.Sprintf("no Go type is compiled in for %s, so only metadata could be read from the %d byte object body", gvk, len(env.Raw))
		stub := map[string]any{
			"apiVersion": env.APIVersion,
			"kind":       env.Kind,
			"metadata":   PartialMeta(env.Raw),
		}
		pretty, err := json.MarshalIndent(stub, "", "  ")
		if err != nil {
			return nil, err
		}
		y, err := sigsyaml.Marshal(stub)
		if err != nil {
			return nil, err
		}
		obj.JSON, obj.YAML = pretty, string(y)
		return obj, nil
	}

	// The protobuf envelope carries the type but generated structs leave
	// TypeMeta empty, so put it back before serializing.
	decoded.GetObjectKind().SetGroupVersionKind(gvk)

	pretty, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render JSON: %w", err)
	}
	compact, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	y, err := sigsyaml.JSONToYAML(compact)
	if err != nil {
		return nil, fmt.Errorf("convert to YAML: %w", err)
	}
	obj.JSON, obj.YAML, obj.Decoded = pretty, string(y), true
	return obj, nil
}

// KnownKind reports whether a generated type is compiled in for a type.
func KnownKind(apiVersion, kind string) bool {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false
	}
	return scheme.Recognizes(gv.WithKind(kind))
}

// EncodeProtobuf serializes an object the way the API server writes it into
// etcd, as the "k8s\0" envelope around the generated protobuf. It is the
// inverse of Decode and is used to build test and demo snapshots.
func EncodeProtobuf(obj runtime.Object, apiVersion, kind string) ([]byte, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("parse apiVersion %q: %w", apiVersion, err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gv.WithKind(kind))

	var buf bytes.Buffer
	if err := protoCodec.Encode(obj, &buf); err != nil {
		return nil, fmt.Errorf("encode %s/%s: %w", apiVersion, kind, err)
	}
	return buf.Bytes(), nil
}
