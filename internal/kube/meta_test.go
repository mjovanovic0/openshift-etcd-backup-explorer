package kube

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// samplePod is encoded with the real Kubernetes protobuf serializer, so these
// tests check the hand written parsers against the exact bytes an API server
// writes into etcd.
func samplePod(t *testing.T) (*corev1.Pod, []byte) {
	t.Helper()
	created := metav1.NewTime(time.Date(2026, 3, 28, 11, 7, 24, 0, time.UTC))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "alertmanager-main-0",
			Namespace:         "openshift-monitoring",
			UID:               "02e21cb2-6e18-4ea3-ba8c-93293df52607",
			ResourceVersion:   "2454426080",
			Generation:        1,
			CreationTimestamp: created,
			Labels:            map[string]string{"app": "alertmanager", "statefulset.kubernetes.io/pod-name": "alertmanager-main-0"},
			Annotations:       map[string]string{"openshift.io/scc": "nonroot"},
			Finalizers:        []string{"example.com/cleanup"},
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager:    "kube-controller-manager",
				Operation:  metav1.ManagedFieldsOperationUpdate,
				APIVersion: "v1",
				FieldsType: "FieldsV1",
			}},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       "alertmanager-main",
				UID:        "7b1b0f2e-0000-4000-8000-000000000001",
				Controller: boolPtr(true),
			}},
		},
		Spec:   corev1.PodSpec{NodeName: "master-0"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	var buf bytes.Buffer
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	pod.GetObjectKind().SetGroupVersionKind(gvk)
	if err := protoCodec.Encode(pod, &buf); err != nil {
		t.Fatalf("encode pod as protobuf: %v", err)
	}
	return pod, buf.Bytes()
}

func boolPtr(b bool) *bool { return &b }

func TestDetectEncoding(t *testing.T) {
	_, proto := samplePod(t)
	cases := []struct {
		name  string
		value []byte
		want  Encoding
	}{
		{"protobuf", proto, EncodingProtobuf},
		{"json", []byte(`{"kind":"Application"}`), EncodingJSON},
		{"json with leading space", []byte("  \n{\"kind\":\"X\"}"), EncodingJSON},
		{"neither", []byte("compact_rev_key"), EncodingUnknown},
		{"empty", nil, EncodingUnknown},
	}
	for _, c := range cases {
		if got := DetectEncoding(c.value); got != c.want {
			t.Errorf("%s: DetectEncoding = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExtractMetaProtobuf(t *testing.T) {
	pod, value := samplePod(t)

	got, ok := ExtractMeta(value)
	if !ok {
		t.Fatal("ExtractMeta reported failure on a valid protobuf object")
	}
	if got.Kind != "Pod" || got.APIVersion != "v1" {
		t.Errorf("type = %s/%s, want v1/Pod", got.APIVersion, got.Kind)
	}
	if got.Name != pod.Name {
		t.Errorf("name = %q, want %q", got.Name, pod.Name)
	}
	if got.Namespace != pod.Namespace {
		t.Errorf("namespace = %q, want %q", got.Namespace, pod.Namespace)
	}
	if got.UID != string(pod.UID) {
		t.Errorf("uid = %q, want %q", got.UID, pod.UID)
	}
	if !got.Created.Equal(pod.CreationTimestamp.Time) {
		t.Errorf("created = %s, want %s", got.Created, pod.CreationTimestamp.Time)
	}
	if got.Encoding != EncodingProtobuf {
		t.Errorf("encoding = %q, want protobuf", got.Encoding)
	}
}

func TestExtractMetaJSON(t *testing.T) {
	value := []byte(`{
	  "apiVersion": "argoproj.io/v1alpha1",
	  "kind": "Application",
	  "metadata": {
	    "name": "carbon-run",
	    "namespace": "argocd",
	    "uid": "aaaabbbb-cccc-dddd-eeee-ffff00001111",
	    "creationTimestamp": "2026-01-02T03:04:05Z"
	  }
	}`)
	got, ok := ExtractMeta(value)
	if !ok {
		t.Fatal("ExtractMeta reported failure on a valid JSON object")
	}
	if got.Kind != "Application" || got.APIVersion != "argoproj.io/v1alpha1" {
		t.Errorf("type = %s/%s", got.APIVersion, got.Kind)
	}
	if got.Name != "carbon-run" || got.Namespace != "argocd" {
		t.Errorf("name/namespace = %q/%q", got.Name, got.Namespace)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if !got.Created.Equal(want) {
		t.Errorf("created = %s, want %s", got.Created, want)
	}
	if got.Encoding != EncodingJSON {
		t.Errorf("encoding = %q, want json", got.Encoding)
	}
}

func TestExtractMetaRejectsGarbage(t *testing.T) {
	for _, value := range [][]byte{
		[]byte("compact_rev_key"),
		{'k', '8', 's', 0},                   // envelope with no body
		{'k', '8', 's', 0, 0xff, 0xff, 0xff}, // unparseable body
		[]byte(`{"kind":`),                   // truncated JSON
	} {
		if _, ok := ExtractMeta(value); ok {
			t.Errorf("ExtractMeta(%q) reported success, want failure", value)
		}
	}
}

func TestOwnerRefsProtobuf(t *testing.T) {
	_, value := samplePod(t)
	refs := OwnerRefs(value)
	if len(refs) != 1 {
		t.Fatalf("got %d owner references, want 1", len(refs))
	}
	got := refs[0]
	if got.Kind != "StatefulSet" || got.Name != "alertmanager-main" {
		t.Errorf("owner = %s/%s", got.Kind, got.Name)
	}
	if got.UID != "7b1b0f2e-0000-4000-8000-000000000001" {
		t.Errorf("owner uid = %q", got.UID)
	}
	if !got.Controller {
		t.Error("owner should be marked as the controller")
	}
}

func TestOwnerRefsJSON(t *testing.T) {
	value := []byte(`{"kind":"Pod","metadata":{"ownerReferences":[
	  {"apiVersion":"apps/v1","kind":"ReplicaSet","name":"web-1","uid":"u-1","controller":true}]}}`)
	refs := OwnerRefs(value)
	if len(refs) != 1 || refs[0].Name != "web-1" || !refs[0].Controller {
		t.Fatalf("unexpected owner references: %+v", refs)
	}
}

func TestPartialMetaRecoversMetadata(t *testing.T) {
	pod, value := samplePod(t)
	env, ok := ParseEnvelope(value)
	if !ok {
		t.Fatal("ParseEnvelope failed")
	}
	p := PartialMeta(env.Raw)

	if p.Name != pod.Name || p.Namespace != pod.Namespace {
		t.Errorf("name/namespace = %q/%q", p.Name, p.Namespace)
	}
	if p.ResourceVersion != pod.ResourceVersion {
		t.Errorf("resourceVersion = %q, want %q", p.ResourceVersion, pod.ResourceVersion)
	}
	if p.Generation != pod.Generation {
		t.Errorf("generation = %d, want %d", p.Generation, pod.Generation)
	}
	if len(p.Labels) != len(pod.Labels) {
		t.Errorf("labels = %v, want %d entries", p.Labels, len(pod.Labels))
	}
	if p.Labels["app"] != "alertmanager" {
		t.Errorf("label app = %q, want alertmanager", p.Labels["app"])
	}
	if p.Annotations["openshift.io/scc"] != "nonroot" {
		t.Errorf("annotations = %v", p.Annotations)
	}
	if len(p.Finalizers) != 1 || p.Finalizers[0] != "example.com/cleanup" {
		t.Errorf("finalizers = %v", p.Finalizers)
	}
	if len(p.OwnerRefs) != 1 {
		t.Errorf("owner references = %v", p.OwnerRefs)
	}
	if p.Created == nil || !p.Created.Equal(pod.CreationTimestamp.Time) {
		t.Errorf("created = %v", p.Created)
	}
}

func TestDecodeProtobufRoundTrip(t *testing.T) {
	pod, value := samplePod(t)

	obj, err := Decode(value)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !obj.Decoded {
		t.Fatalf("object was not decoded: %s", obj.Reason)
	}
	if obj.Kind != "Pod" || obj.APIVersion != "v1" {
		t.Errorf("type = %s/%s", obj.APIVersion, obj.Kind)
	}

	// The JSON must carry the type back, which the generated structs drop.
	var back corev1.Pod
	if err := json.Unmarshal(obj.JSON, &back); err != nil {
		t.Fatalf("re-parse decoded JSON: %v", err)
	}
	if back.Kind != "Pod" || back.APIVersion != "v1" {
		t.Errorf("decoded JSON lost its type: %s/%s", back.APIVersion, back.Kind)
	}
	if back.Name != pod.Name || back.Status.Phase != corev1.PodRunning {
		t.Errorf("round trip lost data: name=%q phase=%q", back.Name, back.Status.Phase)
	}
	if !bytes.Contains([]byte(obj.YAML), []byte("name: alertmanager-main-0")) {
		t.Error("YAML output does not contain the object name")
	}
}

func TestDecodeJSONPassesThrough(t *testing.T) {
	value := []byte(`{"apiVersion":"argoproj.io/v1alpha1","kind":"Application","spec":{"project":"default"}}`)
	obj, err := Decode(value)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !obj.Decoded || obj.Kind != "Application" {
		t.Errorf("unexpected result: %+v", obj)
	}
	if !bytes.Contains([]byte(obj.YAML), []byte("project: default")) {
		t.Errorf("YAML lost the spec: %s", obj.YAML)
	}
}

func TestDecodeRejectsUnknownEncoding(t *testing.T) {
	if _, err := Decode([]byte("not an object")); err == nil {
		t.Error("Decode should fail on a value that is neither JSON nor protobuf")
	}
}

func TestSplitAPIVersion(t *testing.T) {
	cases := map[string][2]string{
		"v1":                    {"", "v1"},
		"apps/v1":               {"apps", "v1"},
		"route.openshift.io/v1": {"route.openshift.io", "v1"},
		"":                      {"", ""},
	}
	for in, want := range cases {
		g, v := SplitAPIVersion(in)
		if g != want[0] || v != want[1] {
			t.Errorf("SplitAPIVersion(%q) = %q,%q want %q,%q", in, g, v, want[0], want[1])
		}
	}
}

func TestKnownKind(t *testing.T) {
	if !KnownKind("v1", "Pod") {
		t.Error("v1/Pod should be compiled in")
	}
	if !KnownKind("route.openshift.io/v1", "Route") {
		t.Error("OpenShift Route should be compiled in")
	}
	if !KnownKind("discovery.k8s.io/v1", "EndpointSlice") {
		t.Error("EndpointSlice should be compiled in")
	}
	if KnownKind("argoproj.io/v1alpha1", "Application") {
		t.Error("a custom resource should not report a compiled in type")
	}
}
