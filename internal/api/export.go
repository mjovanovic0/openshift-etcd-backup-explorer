package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/index"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// exportLimit guards against a request that would walk the whole keyspace and
// build a single enormous response.
const exportLimit = 20000

// export writes every resource matching the current filters as one YAML
// stream, one JSON list, or a zip holding a file per resource. It takes the
// same filter parameters as the resource list, so what you export is exactly
// what you are looking at.
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	_, idx, err := s.lookup(r)
	if err != nil {
		writeError(w, err, s.log)
		return
	}

	q := r.URL.Query()
	res, err := idx.List(index.Query{
		KindID:    q.Get("kind"),
		Namespace: q.Get("namespace"),
		Search:    q.Get("q"),
		Deep:      q.Get("deep") == "true",
		Sort:      "namespace",
	})
	if err != nil {
		writeError(w, err, s.log)
		return
	}
	if len(res.Items) == 0 {
		writeError(w, statusError{http.StatusNotFound, "no resources match these filters"}, s.log)
		return
	}
	if len(res.Items) > exportLimit {
		writeError(w, statusError{http.StatusRequestEntityTooLarge, fmt.Sprintf(
			"%d resources match, which is more than the %d this can export at once, narrow the filter first",
			len(res.Items), exportLimit)}, s.log)
		return
	}

	// Stripping is on unless the caller asks for the object as stored.
	strip := q.Get("clean") != "false"
	base := exportName(idx, q.Get("kind"), q.Get("namespace"))

	switch q.Get("format") {
	case "zip":
		s.exportZip(w, idx, res.Items, strip, base)
	case "json":
		s.exportJSON(w, idx, res.Items, strip, base)
	default:
		s.exportYAML(w, idx, res.Items, strip, base)
	}
}

// exportYAML writes the objects as one multi document stream, which is what
// kubectl apply reads and what is useful on the clipboard.
func (s *Server) exportYAML(w http.ResponseWriter, idx *index.Index, items []*index.Resource, strip bool, base string) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+base+".yaml\"")

	for i, item := range items {
		obj, err := s.renderOne(idx, item, strip)
		if err != nil {
			fmt.Fprintf(w, "# %s %s: %v\n", item.Kind, resourcePath(item), err)
			continue
		}
		if i > 0 {
			io.WriteString(w, "---\n")
		}
		fmt.Fprintf(w, "# %s %s\n", item.Kind, resourcePath(item))
		io.WriteString(w, obj.YAML)
		if !strings.HasSuffix(obj.YAML, "\n") {
			io.WriteString(w, "\n")
		}
	}
}

// exportJSON writes a v1 List, the shape kubectl produces for a collection.
func (s *Server) exportJSON(w http.ResponseWriter, idx *index.Index, items []*index.Resource, strip bool, base string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+base+".json\"")

	list := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		obj, err := s.renderOne(idx, item, strip)
		if err != nil {
			continue
		}
		list = append(list, obj.JSON)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{"apiVersion": "v1", "kind": "List", "items": list})
}

// exportZip writes one file per resource, laid out by kind and namespace so
// unpacking it gives a folder tree rather than a heap of files.
func (s *Server) exportZip(w http.ResponseWriter, idx *index.Index, items []*index.Resource, strip bool, base string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+base+".zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	used := map[string]int{}
	for _, item := range items {
		obj, err := s.renderOne(idx, item, strip)
		if err != nil {
			s.log.Warn("export skipped a resource", "key", item.Key, "error", err)
			continue
		}
		// Two objects can land on the same path, for example the same name in
		// the same namespace under different API groups. Keep both.
		entry := zipEntryName(item)
		name := entry
		if n := used[entry]; n > 0 {
			ext := path.Ext(entry)
			name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(entry, ext), n+1, ext)
		}
		used[entry]++

		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if item.Created != nil {
			header.Modified = *item.Created
		} else {
			header.Modified = time.Now()
		}
		f, err := zw.CreateHeader(header)
		if err != nil {
			s.log.Error("export could not write a zip entry", "name", name, "error", err)
			return
		}
		io.WriteString(f, obj.YAML)
	}
}

// renderOne reads and decodes a single resource for export.
func (s *Server) renderOne(idx *index.Index, item *index.Resource, strip bool) (*kube.Object, error) {
	body, err := idx.Value(item)
	if err != nil {
		return nil, err
	}
	obj, err := kube.Decode(body)
	if err != nil {
		return nil, err
	}
	if !strip {
		return obj, nil
	}
	return kube.Clean(obj)
}

// resourcePath names a resource the way kubectl does, as namespace/name.
func resourcePath(r *index.Resource) string {
	if r.Namespace == "" {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}

// zipEntryName lays an object out as kind/namespace/name.yaml, with cluster
// scoped objects sitting directly under their kind.
func zipEntryName(r *index.Resource) string {
	parts := []string{safeSegment(strings.ToLower(r.Kind))}
	if r.Namespace != "" {
		parts = append(parts, safeSegment(r.Namespace))
	}
	parts = append(parts, safeSegment(r.Name)+".yaml")
	return path.Join(parts...)
}

// exportName builds the download file name from the filters in use.
func exportName(idx *index.Index, kindID, namespace string) string {
	parts := []string{"etcd-backup"}
	if kindID != "" && kindID != "*" {
		for _, k := range idx.Kinds() {
			if k.ID == kindID {
				parts = append(parts, strings.ToLower(k.Label))
				break
			}
		}
	} else {
		parts = append(parts, "all-resources")
	}
	if namespace != "" && namespace != "*" {
		parts = append(parts, namespace)
	}
	return safeSegment(strings.Join(parts, "-"))
}

// safeSegment keeps a name usable as a path segment on every platform.
// Object names can hold characters such as the colon in an image digest.
func safeSegment(s string) string {
	if s == "" {
		return "unnamed"
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 0x20 {
			return '_'
		}
		return r
	}, s)
}
