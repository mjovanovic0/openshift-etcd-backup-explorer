package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/index"
)

const backupDir = "../../backup"

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	if _, err := os.Stat(backupDir); err != nil {
		t.Skip("no sample backup in ./backup, skipping")
	}
	backups, err := index.Discover(backupDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if err := backups[0].Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { backups[0].Close() })

	mux := http.NewServeMux()
	New(backups, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, backups[0].ID
}

// getJSON fetches a path and decodes it, failing the test on any error status.
func getJSON[T any](t *testing.T, srv *httptest.Server, path string) T {
	t.Helper()
	res, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("GET %s: status %d, body %s", path, res.StatusCode, body)
	}
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("GET %s: decode response: %v", path, err)
	}
	return out
}

func TestListBackups(t *testing.T) {
	srv, id := newTestServer(t)

	type backupDTO struct {
		ID        string `json:"id"`
		Label     string `json:"label"`
		Loaded    bool   `json:"loaded"`
		Resources int    `json:"resources"`
	}
	got := getJSON[[]backupDTO](t, srv, "/api/backups")
	if len(got) != 1 {
		t.Fatalf("got %d backups, want 1", len(got))
	}
	if got[0].ID != id || !got[0].Loaded || got[0].Resources == 0 {
		t.Errorf("unexpected backup: %+v", got[0])
	}
}

func TestKindsAndNamespaces(t *testing.T) {
	srv, id := newTestServer(t)

	kinds := getJSON[struct {
		Total int `json:"total"`
		Items []struct {
			ID         string `json:"id"`
			Label      string `json:"label"`
			Count      int    `json:"count"`
			Namespaced bool   `json:"namespaced"`
		} `json:"items"`
	}](t, srv, "/api/backups/"+id+"/kinds")

	if kinds.Total == 0 || len(kinds.Items) == 0 {
		t.Fatal("kinds response is empty")
	}
	// The counts of every type must add up to the reported total.
	sum := 0
	for _, k := range kinds.Items {
		if k.ID == "" || k.Label == "" {
			t.Errorf("kind with no id or label: %+v", k)
		}
		sum += k.Count
	}
	if sum != kinds.Total {
		t.Errorf("kind counts sum to %d, but total is %d", sum, kinds.Total)
	}

	namespaces := getJSON[[]struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}](t, srv, "/api/backups/"+id+"/namespaces")
	if len(namespaces) == 0 {
		t.Fatal("namespaces response is empty")
	}
	for _, ns := range namespaces {
		if ns.Name == "" || ns.Count == 0 {
			t.Errorf("unexpected namespace entry: %+v", ns)
		}
	}
}

type listResponse struct {
	Total int `json:"total"`
	Items []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Kind      string `json:"kind"`
		Status    *struct {
			Text string `json:"text"`
			Tone string `json:"tone"`
		} `json:"status"`
	} `json:"items"`
}

func TestResourcesListCarriesStatus(t *testing.T) {
	srv, id := newTestServer(t)

	got := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=v1/Pod&limit=20&sort=name")
	if got.Total == 0 || len(got.Items) != 20 {
		t.Fatalf("got %d of %d pods, want a page of 20", len(got.Items), got.Total)
	}
	// Every pod has a phase, so the status column must be populated.
	for _, r := range got.Items {
		if r.Kind != "Pod" {
			t.Errorf("kind filter leaked a %s", r.Kind)
		}
		if r.Status == nil || r.Status.Text == "" {
			t.Errorf("pod %s/%s has no status", r.Namespace, r.Name)
		}
	}

	// Asking for no status must skip the decode work and omit the field.
	plain := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=v1/Pod&limit=5&status=false")
	for _, r := range plain.Items {
		if r.Status != nil {
			t.Errorf("status was returned even though it was switched off: %+v", r.Status)
		}
	}
}

func TestResourceDetail(t *testing.T) {
	srv, id := newTestServer(t)

	list := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=v1/ConfigMap&q=etcd-pod&limit=1")
	if len(list.Items) == 0 {
		t.Fatal("no configmap to inspect")
	}

	detail := getJSON[struct {
		Resource struct {
			Name     string `json:"name"`
			Key      string `json:"key"`
			Encoding string `json:"encoding"`
		} `json:"resource"`
		Object struct {
			Kind    string          `json:"kind"`
			YAML    string          `json:"yaml"`
			JSON    json.RawMessage `json:"json"`
			Decoded bool            `json:"decoded"`
		} `json:"object"`
		Details []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"details"`
	}](t, srv, "/api/backups/"+id+"/resources/"+list.Items[0].ID)

	if detail.Object.Kind != "ConfigMap" || !detail.Object.Decoded {
		t.Errorf("unexpected object: %+v", detail.Object)
	}
	if !strings.Contains(detail.Object.YAML, "kind: ConfigMap") {
		t.Error("YAML does not carry the kind")
	}
	if len(detail.Object.JSON) == 0 {
		t.Error("JSON view is empty")
	}
	if len(detail.Details) == 0 {
		t.Error("details tab has no fields")
	}
	// The etcd key must be surfaced, since that is what makes this a backup
	// browser rather than a cluster browser.
	var sawKey bool
	for _, f := range detail.Details {
		if f.Label == "etcd key" && strings.HasPrefix(f.Value, "/") {
			sawKey = true
		}
	}
	if !sawKey {
		t.Error("details tab does not show the etcd key")
	}
}

func TestDownloadFormats(t *testing.T) {
	srv, id := newTestServer(t)

	list := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=v1/ConfigMap&q=etcd-pod&limit=1")
	rid := list.Items[0].ID

	for _, tc := range []struct {
		format      string
		wantType    string
		wantSuffix  string
		wantContent string
	}{
		{"yaml", "application/yaml", ".yaml", "kind: ConfigMap"},
		{"json", "application/json", ".json", `"kind": "ConfigMap"`},
		{"raw", "application/octet-stream", ".bin", ""},
	} {
		res, err := srv.Client().Get(srv.URL + "/api/backups/" + id + "/resources/" + rid + "/download?format=" + tc.format)
		if err != nil {
			t.Fatalf("%s: %v", tc.format, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", tc.format, res.StatusCode)
			continue
		}
		if ct := res.Header.Get("Content-Type"); ct != tc.wantType {
			t.Errorf("%s: content type %q, want %q", tc.format, ct, tc.wantType)
		}
		cd := res.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "attachment") || !strings.Contains(cd, tc.wantSuffix) {
			t.Errorf("%s: content disposition %q", tc.format, cd)
		}
		if tc.wantContent != "" && !strings.Contains(string(body), tc.wantContent) {
			t.Errorf("%s: body does not contain %q", tc.format, tc.wantContent)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body", tc.format)
		}
	}
}

func TestStaticResources(t *testing.T) {
	srv, id := newTestServer(t)

	files := getJSON[[]struct {
		Path  string `json:"path"`
		IsDir bool   `json:"isDir"`
		Size  int64  `json:"size"`
	}](t, srv, "/api/backups/"+id+"/static")
	if len(files) == 0 {
		t.Fatal("archive listing is empty")
	}

	var target string
	for _, f := range files {
		if strings.HasSuffix(f.Path, "configmaps/etcd-pod/pod.yaml") {
			target = f.Path
		}
	}
	if target == "" {
		t.Fatal("expected pod.yaml in the archive listing")
	}

	file := getJSON[struct {
		Path   string `json:"path"`
		Text   string `json:"text"`
		Binary bool   `json:"binary"`
	}](t, srv, "/api/backups/"+id+"/static/file?path="+target)
	if file.Binary || !strings.Contains(file.Text, "kind: Pod") {
		t.Errorf("unexpected file content: binary=%v len=%d", file.Binary, len(file.Text))
	}
}

func TestErrorResponses(t *testing.T) {
	srv, id := newTestServer(t)

	cases := []struct {
		path string
		want int
	}{
		{"/api/backups/nope/kinds", http.StatusNotFound},
		{"/api/backups/" + id + "/resources/not-a-real-id", http.StatusNotFound},
		{"/api/backups/" + id + "/static/file?path=nope", http.StatusNotFound},
	}
	for _, c := range cases {
		res, err := srv.Client().Get(srv.URL + c.path)
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		if res.StatusCode != c.want {
			t.Errorf("GET %s: status %d, want %d", c.path, res.StatusCode, c.want)
		}
		// Errors must be JSON so the UI can show the reason.
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.Error == "" {
			t.Errorf("GET %s: error body is not usable JSON: %s", c.path, body)
		}
	}
}

func TestInvalidQueryParamsFallBackToDefaults(t *testing.T) {
	srv, id := newTestServer(t)

	// A nonsense limit or offset must not fail the request.
	got := getJSON[listResponse](t, srv,
		"/api/backups/"+id+"/resources?kind=v1/Pod&limit=abc&offset=-5&sort=nonsense")
	if len(got.Items) == 0 {
		t.Error("invalid parameters produced no rows")
	}
	// The limit is capped, so a huge request cannot return the whole keyspace.
	big := getJSON[listResponse](t, srv, "/api/backups/"+id+"/resources?kind=*&limit=100000")
	if len(big.Items) > 500 {
		t.Errorf("returned %d rows, the limit should cap at 500", len(big.Items))
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:         "0 B",
		512:       "512 B",
		1024:      "1.0 KiB",
		1536:      "1.5 KiB",
		1048576:   "1.0 MiB",
		389767200: "371.7 MiB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
