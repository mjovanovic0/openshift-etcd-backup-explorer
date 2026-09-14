package kube

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripGeneratedRemovesObjectMetadata(t *testing.T) {
	doc := map[string]any{
		"apiVersion": "v1",
		"kind":       "PersistentVolume",
		"metadata": map[string]any{
			"name":              "pv-1",
			"uid":               "abc-123",
			"creationTimestamp": "2026-08-04T12:26:20Z",
			"managedFields":     []any{map[string]any{"manager": "kube-controller-manager"}},
			"resourceVersion":   "2415015032",
			"annotations":       map[string]any{"a": "b"},
			"finalizers":        []any{"kubernetes.io/pv-protection"},
		},
	}
	StripGenerated(doc)

	meta := doc["metadata"].(map[string]any)
	for _, gone := range []string{"uid", "creationTimestamp", "managedFields"} {
		if _, present := meta[gone]; present {
			t.Errorf("metadata.%s survived stripping", gone)
		}
	}
	// Everything else must be left alone.
	for _, kept := range []string{"name", "resourceVersion", "annotations", "finalizers"} {
		if _, present := meta[kept]; !present {
			t.Errorf("metadata.%s was removed but should have been kept", kept)
		}
	}
}

func TestStripGeneratedKeepsReferencesToOtherObjects(t *testing.T) {
	// A uid that identifies a different object is a reference, not generated
	// metadata, and removing it would break the manifest.
	doc := map[string]any{
		"kind":     "PersistentVolume",
		"metadata": map[string]any{"name": "pv-1", "uid": "own-uid"},
		"spec": map[string]any{
			"claimRef": map[string]any{"kind": "PersistentVolumeClaim", "name": "data", "uid": "claim-uid"},
		},
	}
	StripGenerated(doc)

	claimRef := doc["spec"].(map[string]any)["claimRef"].(map[string]any)
	if claimRef["uid"] != "claim-uid" {
		t.Errorf("spec.claimRef.uid = %v, it must survive", claimRef["uid"])
	}
	if _, present := doc["metadata"].(map[string]any)["uid"]; present {
		t.Error("the object's own uid should have been removed")
	}
}

func TestStripGeneratedHandlesNestedTemplates(t *testing.T) {
	doc := map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"name": "web",
			"uid":  "dep-uid",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"labels":            map[string]any{"app": "web"},
					"creationTimestamp": nil,
					"managedFields":     []any{"noise"},
				},
			},
		},
	}
	StripGenerated(doc)

	tmpl := doc["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)
	if _, present := tmpl["creationTimestamp"]; present {
		t.Error("a null creationTimestamp inside a template is an artifact and should go")
	}
	if _, present := tmpl["managedFields"]; present {
		t.Error("managedFields inside a template should go")
	}
	if tmpl["labels"] == nil {
		t.Error("template labels must be kept")
	}
}

func TestStripGeneratedKeepsRealNestedTimestamps(t *testing.T) {
	// Only a null timestamp is an artifact. A real one carries information.
	doc := map[string]any{
		"kind":     "List",
		"metadata": map[string]any{"name": "x"},
		"items": []any{
			map[string]any{"metadata": map[string]any{"creationTimestamp": "2026-01-01T00:00:00Z"}},
		},
	}
	StripGenerated(doc)

	item := doc["items"].([]any)[0].(map[string]any)["metadata"].(map[string]any)
	if item["creationTimestamp"] != "2026-01-01T00:00:00Z" {
		t.Errorf("a real nested timestamp was removed: %v", item["creationTimestamp"])
	}
}

func TestStripGeneratedToleratesOddShapes(t *testing.T) {
	// Must not panic on objects with no metadata or a metadata of the wrong type.
	for _, doc := range []map[string]any{
		{},
		{"metadata": nil},
		{"metadata": "not an object"},
		{"spec": []any{1, 2, 3}},
		{"metadata": map[string]any{}, "spec": map[string]any{"nested": nil}},
	} {
		StripGenerated(doc)
	}
}

func TestCleanLeavesTheOriginalUntouched(t *testing.T) {
	_, value := samplePod(t)
	obj, err := Decode(value)
	if err != nil {
		t.Fatal(err)
	}
	originalYAML := obj.YAML
	if !strings.Contains(originalYAML, "uid:") {
		t.Fatal("the sample should carry a uid before cleaning")
	}

	cleaned, err := Clean(obj)
	if err != nil {
		t.Fatal(err)
	}
	if obj.YAML != originalYAML {
		t.Error("Clean modified the object it was given, the browser must keep showing the stored form")
	}
	if strings.Contains(cleaned.YAML, "managedFields") {
		t.Error("cleaned YAML still holds managedFields")
	}

	var doc map[string]any
	if err := json.Unmarshal(cleaned.JSON, &doc); err != nil {
		t.Fatalf("cleaned JSON does not parse: %v", err)
	}
	meta := doc["metadata"].(map[string]any)
	for _, gone := range []string{"uid", "creationTimestamp", "managedFields"} {
		if _, present := meta[gone]; present {
			t.Errorf("cleaned JSON still holds metadata.%s", gone)
		}
	}
	if meta["name"] != "alertmanager-main-0" {
		t.Errorf("cleaning lost the name: %v", meta["name"])
	}
	// Owner references describe the object's place in the cluster and are not
	// generated boilerplate, so they stay.
	if _, present := meta["ownerReferences"]; !present {
		t.Error("ownerReferences should survive cleaning")
	}
}

func TestRenderWithoutStrippingIsFaithful(t *testing.T) {
	_, value := samplePod(t)
	obj, err := Decode(value)
	if err != nil {
		t.Fatal(err)
	}
	_, yamlOut, err := Render(obj.JSON, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"uid:", "creationTimestamp:", "managedFields:"} {
		if !strings.Contains(yamlOut, want) {
			t.Errorf("rendering without stripping dropped %s", want)
		}
	}
}
