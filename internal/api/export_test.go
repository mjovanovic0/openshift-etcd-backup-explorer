package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sigsyaml "sigs.k8s.io/yaml"
)

// fetch performs a GET and returns the status, headers and body.
func fetch(t *testing.T, srv *httptest.Server, path string) (*http.Response, []byte) {
	t.Helper()
	res, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return res, body
}

const pvKind = "v1%2FPersistentVolume"

func TestExportYAMLStream(t *testing.T) {
	srv, id := newTestServer(t)

	res, body := fetch(t, srv, "/api/backups/"+id+"/export?kind="+pvKind+"&format=yaml")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Errorf("content type = %q", ct)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "persistentvolumes.yaml") {
		t.Errorf("content disposition = %q", cd)
	}

	// Every document must parse, and the separator count must match.
	docs := splitYAMLDocs(string(body))
	if len(docs) < 100 {
		t.Fatalf("got %d documents, expected every persistent volume", len(docs))
	}
	for i, doc := range docs {
		var parsed map[string]any
		if err := sigsyaml.Unmarshal([]byte(doc), &parsed); err != nil {
			t.Fatalf("document %d does not parse: %v", i, err)
		}
		if parsed["kind"] != "PersistentVolume" {
			t.Errorf("document %d has kind %v", i, parsed["kind"])
		}
		meta, _ := parsed["metadata"].(map[string]any)
		for _, gone := range []string{"uid", "creationTimestamp", "managedFields"} {
			if _, present := meta[gone]; present {
				t.Errorf("document %d still holds metadata.%s", i, gone)
			}
		}
		if meta["name"] == "" || meta["name"] == nil {
			t.Errorf("document %d lost its name", i)
		}
	}
}

func TestExportKeepsGeneratedFieldsWhenAsked(t *testing.T) {
	srv, id := newTestServer(t)

	_, body := fetch(t, srv, "/api/backups/"+id+"/export?kind="+pvKind+"&format=yaml&clean=false")
	if !bytes.Contains(body, []byte("managedFields:")) {
		t.Error("clean=false should return the objects as stored, with managedFields")
	}
	if !bytes.Contains(body, []byte("creationTimestamp:")) {
		t.Error("clean=false should keep creationTimestamp")
	}
}

func TestExportJSONList(t *testing.T) {
	srv, id := newTestServer(t)

	res, body := fetch(t, srv, "/api/backups/"+id+"/export?kind="+pvKind+"&format=json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var list struct {
		APIVersion string           `json:"apiVersion"`
		Kind       string           `json:"kind"`
		Items      []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if list.Kind != "List" || list.APIVersion != "v1" {
		t.Errorf("wrapper = %s/%s, want v1/List", list.APIVersion, list.Kind)
	}
	if len(list.Items) < 100 {
		t.Fatalf("got %d items", len(list.Items))
	}
	meta, _ := list.Items[0]["metadata"].(map[string]any)
	if _, present := meta["uid"]; present {
		t.Error("exported JSON still holds metadata.uid")
	}
}

func TestExportZipLayout(t *testing.T) {
	srv, id := newTestServer(t)

	res, body := fetch(t, srv, "/api/backups/"+id+"/export?kind="+pvKind+"&format=zip")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("content type = %q", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("response is not a zip: %v", err)
	}
	if len(zr.File) < 100 {
		t.Fatalf("zip holds %d entries", len(zr.File))
	}
	seen := map[string]bool{}
	for _, f := range zr.File {
		if seen[f.Name] {
			t.Errorf("duplicate entry %s, unzipping would lose one", f.Name)
		}
		seen[f.Name] = true

		// Cluster scoped objects sit directly under their kind.
		if !strings.HasPrefix(f.Name, "persistentvolume/") || !strings.HasSuffix(f.Name, ".yaml") {
			t.Errorf("unexpected entry path %q", f.Name)
		}
		if strings.Contains(f.Name, "..") {
			t.Errorf("entry %q escapes the archive root", f.Name)
		}
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(rc)
	rc.Close()
	var parsed map[string]any
	if err := sigsyaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("zip entry does not parse as YAML: %v", err)
	}
	if parsed["kind"] != "PersistentVolume" {
		t.Errorf("zip entry kind = %v", parsed["kind"])
	}
}

func TestExportNamespacedLayoutIncludesNamespace(t *testing.T) {
	srv, id := newTestServer(t)

	_, body := fetch(t, srv,
		"/api/backups/"+id+"/export?kind=v1%2FConfigMap&namespace=openshift-etcd&format=zip")
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "configmap/openshift-etcd/") {
			t.Errorf("namespaced object at %q, want it under its namespace", f.Name)
		}
	}
}

func TestExportRespectsFilters(t *testing.T) {
	srv, id := newTestServer(t)

	// The export must contain exactly what the list would show.
	list := getJSON[listResponse](t, srv,
		"/api/backups/"+id+"/resources?kind=v1%2FConfigMap&namespace=openshift-etcd&status=false&limit=500")

	_, body := fetch(t, srv,
		"/api/backups/"+id+"/export?kind=v1%2FConfigMap&namespace=openshift-etcd&format=json")
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != list.Total {
		t.Errorf("export holds %d objects but the list reports %d", len(out.Items), list.Total)
	}
}

func TestExportErrors(t *testing.T) {
	srv, id := newTestServer(t)

	// A filter matching nothing is an error, not an empty file.
	res, _ := fetch(t, srv, "/api/backups/"+id+"/export?kind=v1%2FPod&q=surely-no-such-object-exists")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("empty export status = %d, want 404", res.StatusCode)
	}

	// The whole keyspace is past the single response limit.
	res, body := fetch(t, srv, "/api/backups/"+id+"/export?kind=*")
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("unbounded export status = %d, want 413", res.StatusCode)
	}
	if !bytes.Contains(body, []byte("narrow the filter")) {
		t.Errorf("the limit error should say what to do: %s", body)
	}

	res, _ = fetch(t, srv, "/api/backups/nope/export?kind=v1%2FPod")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown backup status = %d", res.StatusCode)
	}
}

func TestSingleDownloadCleansByDefault(t *testing.T) {
	srv, id := newTestServer(t)

	list := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=v1%2FPod&limit=1&sort=name")
	rid := list.Items[0].ID

	_, clean := fetch(t, srv, "/api/backups/"+id+"/resources/"+rid+"/download?format=yaml")
	if bytes.Contains(clean, []byte("managedFields:")) {
		t.Error("a download should be cleaned by default")
	}

	_, raw := fetch(t, srv, "/api/backups/"+id+"/resources/"+rid+"/download?format=yaml&clean=false")
	if !bytes.Contains(raw, []byte("managedFields:")) {
		t.Error("clean=false should return the object as stored")
	}
	if len(raw) <= len(clean) {
		t.Errorf("the stored form should be larger: %d vs %d", len(raw), len(clean))
	}

	// The raw etcd value is never rewritten, whatever the clean flag says.
	_, rawValue := fetch(t, srv, "/api/backups/"+id+"/resources/"+rid+"/download?format=raw")
	if !bytes.HasPrefix(rawValue, []byte("k8s\x00")) {
		t.Error("the raw download should be the stored protobuf, untouched")
	}
}

// splitYAMLDocs breaks a multi document stream on its separators.
func splitYAMLDocs(s string) []string {
	var out []string
	for _, part := range strings.Split(s, "\n---\n") {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}
