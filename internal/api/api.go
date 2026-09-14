// Package api serves the browser API over the indexed backups.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/index"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// Server serves the API for a set of discovered backups.
type Server struct {
	backups []*index.Backup
	log     *slog.Logger
}

// New builds a server over already discovered backups.
func New(backups []*index.Backup, log *slog.Logger) *Server {
	return &Server{backups: backups, log: log}
}

// Routes registers every API route.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/backups", s.handle(s.listBackups))
	mux.HandleFunc("GET /api/backups/{id}/info", s.handle(s.backupInfo))
	mux.HandleFunc("GET /api/backups/{id}/kinds", s.handle(s.kinds))
	mux.HandleFunc("GET /api/backups/{id}/namespaces", s.handle(s.namespaces))
	mux.HandleFunc("GET /api/backups/{id}/resources", s.handle(s.resources))
	mux.HandleFunc("GET /api/backups/{id}/resources/{rid}", s.handle(s.resource))
	mux.HandleFunc("GET /api/backups/{id}/resources/{rid}/download", s.download)
	mux.HandleFunc("GET /api/backups/{id}/export", s.export)
	mux.HandleFunc("GET /api/backups/{id}/static", s.handle(s.staticFiles))
	mux.HandleFunc("GET /api/backups/{id}/static/file", s.handle(s.staticFile))
}

// ---- backups ----

type backupDTO struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	TakenAt      time.Time `json:"takenAt"`
	Size         int64     `json:"size"`
	SnapshotPath string    `json:"snapshotPath"`
	StaticPath   string    `json:"staticPath,omitempty"`
	Loaded       bool      `json:"loaded"`
	Resources    int       `json:"resources"`
}

func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) error {
	out := make([]backupDTO, 0, len(s.backups))
	for _, b := range s.backups {
		d := backupDTO{
			ID: b.ID, Label: b.Label, TakenAt: b.TakenAt, Size: b.Size,
			SnapshotPath: b.SnapshotPath, StaticPath: b.StaticPath, Loaded: b.Loaded(),
		}
		if b.Loaded() {
			d.Resources = b.Index().Total()
		}
		out = append(out, d)
	}
	return writeJSON(w, out)
}

type infoDTO struct {
	Backup    backupDTO `json:"backup"`
	Snapshot  any       `json:"snapshot"`
	Resources int       `json:"resources"`
	Kinds     int       `json:"kinds"`
	Namespace int       `json:"namespaces"`
	Static    int       `json:"staticFiles"`
	IndexedIn string    `json:"indexedIn"`
}

func (s *Server) backupInfo(w http.ResponseWriter, r *http.Request) error {
	b, idx, err := s.lookup(r)
	if err != nil {
		return err
	}
	out := infoDTO{
		Backup: backupDTO{
			ID: b.ID, Label: b.Label, TakenAt: b.TakenAt, Size: b.Size,
			SnapshotPath: b.SnapshotPath, StaticPath: b.StaticPath,
			Loaded: true, Resources: idx.Total(),
		},
		Snapshot:  idx.Info,
		Resources: idx.Total(),
		Kinds:     len(idx.Kinds()),
		Namespace: len(idx.Namespaces()),
		IndexedIn: idx.BuiltIn.Round(time.Millisecond).String(),
	}
	if a := b.Archive(); a != nil {
		// Count files only, so this agrees with the archive browser.
		for _, f := range a.Files {
			if !f.IsDir {
				out.Static++
			}
		}
	}
	return writeJSON(w, out)
}

// ---- kinds and namespaces ----

func (s *Server) kinds(w http.ResponseWriter, r *http.Request) error {
	_, idx, err := s.lookup(r)
	if err != nil {
		return err
	}
	// The list always starts with an "everything" row, the way the console does.
	type kindsDTO struct {
		Total int          `json:"total"`
		Items []index.Kind `json:"items"`
	}
	return writeJSON(w, kindsDTO{Total: idx.Total(), Items: idx.Kinds()})
}

func (s *Server) namespaces(w http.ResponseWriter, r *http.Request) error {
	_, idx, err := s.lookup(r)
	if err != nil {
		return err
	}
	return writeJSON(w, idx.Namespaces())
}

// ---- resources ----

type resourceRow struct {
	*index.Resource
	Status *kube.Status `json:"status,omitempty"`
}

type listDTO struct {
	Total  int           `json:"total"`
	Offset int           `json:"offset"`
	Limit  int           `json:"limit"`
	Items  []resourceRow `json:"items"`
}

func (s *Server) resources(w http.ResponseWriter, r *http.Request) error {
	_, idx, err := s.lookup(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	limit := intParam(q.Get("limit"), 50)
	if limit > 500 {
		limit = 500
	}
	query := index.Query{
		KindID:    q.Get("kind"),
		Namespace: q.Get("namespace"),
		Search:    q.Get("q"),
		Deep:      q.Get("deep") == "true",
		Sort:      q.Get("sort"),
		Desc:      q.Get("order") == "desc",
		Offset:    intParam(q.Get("offset"), 0),
		Limit:     limit,
	}
	res, err := idx.List(query)
	if err != nil {
		return err
	}

	out := listDTO{Total: res.Total, Offset: query.Offset, Limit: limit, Items: make([]resourceRow, 0, len(res.Items))}
	withStatus := q.Get("status") != "false"
	for _, item := range res.Items {
		row := resourceRow{Resource: item}
		// The status column needs the object body, so it is only decoded for
		// the rows actually being returned.
		if withStatus {
			if body, err := idx.Value(item); err == nil {
				if obj, err := kube.Decode(body); err == nil {
					st := kube.Summarize(item.Kind, obj.JSON)
					if st.Text != "" {
						row.Status = &st
					}
				}
			}
		}
		out.Items = append(out.Items, row)
	}
	return writeJSON(w, out)
}

type detailDTO struct {
	Resource *index.Resource   `json:"resource"`
	Status   *kube.Status      `json:"status,omitempty"`
	Object   *kube.Object      `json:"object"`
	Owners   []kube.OwnerRef   `json:"owners,omitempty"`
	Related  []*index.Resource `json:"related,omitempty"`
	Details  []detailField     `json:"details"`
}

type detailField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func (s *Server) resource(w http.ResponseWriter, r *http.Request) error {
	_, idx, err := s.lookup(r)
	if err != nil {
		return err
	}
	res, ok := idx.Get(r.PathValue("rid"))
	if !ok {
		return statusError{http.StatusNotFound, "resource not found in this backup"}
	}
	body, err := idx.Value(res)
	if err != nil {
		return fmt.Errorf("read object body: %w", err)
	}
	obj, err := kube.Decode(body)
	if err != nil {
		return fmt.Errorf("decode object: %w", err)
	}

	out := detailDTO{Resource: res, Object: obj, Owners: kube.OwnerRefs(body)}
	if st := kube.Summarize(res.Kind, obj.JSON); st.Text != "" {
		out.Status = &st
	}
	if r.URL.Query().Get("related") != "false" {
		out.Related = idx.Related(res, body)
	}
	out.Details = buildDetails(res, obj)
	return writeJSON(w, out)
}

// buildDetails fills the Details tab with the facts an operator wants without
// reading the whole manifest.
func buildDetails(res *index.Resource, obj *kube.Object) []detailField {
	fields := []detailField{
		{"Name", res.Name},
		{"Kind", res.Kind},
		{"API version", res.APIVersion},
	}
	if res.Namespace != "" {
		fields = append(fields, detailField{"Namespace", res.Namespace})
	} else {
		fields = append(fields, detailField{"Scope", "Cluster"})
	}
	if res.UID != "" {
		fields = append(fields, detailField{"UID", res.UID})
	}
	if res.Created != nil {
		fields = append(fields, detailField{"Created", res.Created.Format(time.RFC3339)})
	}
	fields = append(fields,
		detailField{"etcd key", res.Key},
		detailField{"Mod revision", strconv.FormatInt(res.ModRev, 10)},
		detailField{"Stored size", humanBytes(int64(res.Size))},
		detailField{"Stored encoding", string(res.Encoding)},
	)

	// Labels and annotations, read straight from the decoded object.
	var doc struct {
		Metadata struct {
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			Finalizers  []string          `json:"finalizers"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(obj.JSON, &doc); err == nil {
		if n := len(doc.Metadata.Labels); n > 0 {
			fields = append(fields, detailField{"Labels", strconv.Itoa(n)})
			for _, k := range sortedKeys(doc.Metadata.Labels) {
				fields = append(fields, detailField{"  " + k, doc.Metadata.Labels[k]})
			}
		}
		if n := len(doc.Metadata.Annotations); n > 0 {
			fields = append(fields, detailField{"Annotations", strconv.Itoa(n)})
		}
		if len(doc.Metadata.Finalizers) > 0 {
			fields = append(fields, detailField{"Finalizers", strings.Join(doc.Metadata.Finalizers, ", ")})
		}
	}
	return fields
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// download serves one object as a downloadable YAML or JSON file.
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	_, idx, err := s.lookup(r)
	if err != nil {
		writeError(w, err, s.log)
		return
	}
	res, ok := idx.Get(r.PathValue("rid"))
	if !ok {
		writeError(w, statusError{http.StatusNotFound, "resource not found"}, s.log)
		return
	}
	body, err := idx.Value(res)
	if err != nil {
		writeError(w, err, s.log)
		return
	}
	obj, err := kube.Decode(body)
	if err != nil {
		writeError(w, err, s.log)
		return
	}
	// The raw value is the object exactly as etcd holds it, so cleaning only
	// applies to the decoded forms.
	format := r.URL.Query().Get("format")
	if format != "raw" && r.URL.Query().Get("clean") != "false" {
		cleaned, err := kube.Clean(obj)
		if err != nil {
			writeError(w, err, s.log)
			return
		}
		obj = cleaned
	}

	name := res.Name
	if res.Namespace != "" {
		name = res.Namespace + "_" + name
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.ToLower(res.Kind)+"_"+name)

	var payload []byte
	switch format {
	case "json":
		payload, name = obj.JSON, name+".json"
		w.Header().Set("Content-Type", "application/json")
	case "raw":
		payload, name = body, name+".bin"
		w.Header().Set("Content-Type", "application/octet-stream")
	default:
		payload, name = []byte(obj.YAML), name+".yaml"
		w.Header().Set("Content-Type", "application/yaml")
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	_, _ = w.Write(payload)
}

// ---- static resources ----

func (s *Server) staticFiles(w http.ResponseWriter, r *http.Request) error {
	b, _, err := s.lookup(r)
	if err != nil {
		return err
	}
	a := b.Archive()
	if a == nil {
		return writeJSON(w, []any{})
	}
	return writeJSON(w, a.Files)
}

func (s *Server) staticFile(w http.ResponseWriter, r *http.Request) error {
	b, _, err := s.lookup(r)
	if err != nil {
		return err
	}
	a := b.Archive()
	if a == nil {
		return statusError{http.StatusNotFound, "this backup has no static resources archive"}
	}
	f, ok := a.Get(r.URL.Query().Get("path"))
	if !ok {
		return statusError{http.StatusNotFound, "file not found in the archive"}
	}
	type fileDTO struct {
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		Mode   string `json:"mode"`
		Text   string `json:"text"`
		Binary bool   `json:"binary"`
	}
	out := fileDTO{Path: f.Path, Size: f.Size, Mode: f.Mode}
	if isText(f.Content) {
		out.Text = string(f.Content)
	} else {
		out.Binary = true
		out.Text = fmt.Sprintf("(binary file, %s)", humanBytes(f.Size))
	}
	return writeJSON(w, out)
}

// isText reports whether content can be shown in the viewer. Certificates and
// manifests are text, keys and DER blobs are not.
func isText(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	limit := min(len(b), 4096)
	for _, c := range b[:limit] {
		if c == 0 {
			return false
		}
	}
	return true
}

// ---- helpers ----

func (s *Server) lookup(r *http.Request) (*index.Backup, *index.Index, error) {
	id := r.PathValue("id")
	for _, b := range s.backups {
		if b.ID != id {
			continue
		}
		if !b.Loaded() {
			return nil, nil, statusError{http.StatusServiceUnavailable, "backup is still loading"}
		}
		return b, b.Index(), nil
	}
	return nil, nil, statusError{http.StatusNotFound, "unknown backup " + id}
}

type statusError struct {
	code    int
	message string
}

func (e statusError) Error() string { return e.message }

func (s *Server) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeError(w, err, s.log)
		}
	}
}

func writeError(w http.ResponseWriter, err error, log *slog.Logger) {
	code := http.StatusInternalServerError
	var se statusError
	if ok := asStatusError(err, &se); ok {
		code = se.code
	} else {
		log.Error("request failed", "error", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func asStatusError(err error, out *statusError) bool {
	return errors.As(err, out)
}

func writeJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

func intParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	for _, u := range units {
		v /= unit
		if v < unit {
			return fmt.Sprintf("%.1f %s", v, u)
		}
	}
	return fmt.Sprintf("%.1f PiB", v)
}
