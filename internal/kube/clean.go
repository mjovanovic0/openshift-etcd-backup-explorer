package kube

import (
	"encoding/json"
	"fmt"

	sigsyaml "sigs.k8s.io/yaml"
)

// StripGenerated removes the fields the API server fills in by itself. A
// manifest copied out of a backup is usually meant to be read or applied
// somewhere else, and these fields describe the object's life in the old
// cluster rather than what it is.
//
// From the object's own metadata it removes uid, creationTimestamp and
// managedFields. Inside any nested metadata, such as a Deployment's pod
// template, it removes managedFields and a creationTimestamp that is null,
// because the serializer emits that for an empty timestamp and it is noise.
func StripGenerated(doc map[string]any) {
	meta, ok := doc["metadata"].(map[string]any)
	if ok {
		delete(meta, "uid")
		delete(meta, "creationTimestamp")
		delete(meta, "managedFields")
	}
	// Walk the rest of the object for template metadata.
	for key, value := range doc {
		if key == "metadata" {
			continue
		}
		stripNested(value)
	}
}

func stripNested(value any) {
	switch v := value.(type) {
	case map[string]any:
		if meta, ok := v["metadata"].(map[string]any); ok {
			delete(meta, "managedFields")
			// Only a null timestamp is an artifact. A real one inside a
			// nested object is left alone.
			if ts, present := meta["creationTimestamp"]; present && ts == nil {
				delete(meta, "creationTimestamp")
			}
		}
		for _, child := range v {
			stripNested(child)
		}
	case []any:
		for _, child := range v {
			stripNested(child)
		}
	}
}

// Render turns a decoded object back into indented JSON and YAML, optionally
// without the generated fields.
func Render(source json.RawMessage, strip bool) (jsonOut []byte, yamlOut string, err error) {
	var doc map[string]any
	if err := json.Unmarshal(source, &doc); err != nil {
		return nil, "", fmt.Errorf("parse object: %w", err)
	}
	if strip {
		StripGenerated(doc)
	}
	jsonOut, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", err
	}
	compact, err := json.Marshal(doc)
	if err != nil {
		return nil, "", err
	}
	y, err := sigsyaml.JSONToYAML(compact)
	if err != nil {
		return nil, "", fmt.Errorf("convert to YAML: %w", err)
	}
	return jsonOut, string(y), nil
}

// Clean returns a copy of an object with the generated fields removed. The
// original is left untouched, because the browser shows the object exactly as
// the backup holds it and only the copied or exported form is cleaned.
func Clean(o *Object) (*Object, error) {
	jsonOut, yamlOut, err := Render(o.JSON, true)
	if err != nil {
		return nil, err
	}
	clone := *o
	clone.JSON, clone.YAML = jsonOut, yamlOut
	return &clone, nil
}
