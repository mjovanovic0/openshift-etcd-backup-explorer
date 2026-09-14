// Package index builds a queryable in-memory view of a snapshot. Only object
// metadata is held in memory, object bodies stay in the file and are read back
// on demand.
package index

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/snapshot"
)

// Resource is one indexed object.
type Resource struct {
	ID         string        `json:"id"`
	Key        string        `json:"key"`
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	KindID     string        `json:"kindId"`
	Group      string        `json:"group"`
	Version    string        `json:"version"`
	Namespace  string        `json:"namespace"`
	Name       string        `json:"name"`
	UID        string        `json:"uid"`
	Created    *time.Time    `json:"createdAt"`
	Encoding   kube.Encoding `json:"encoding"`
	Size       int           `json:"size"`
	ModRev     int64         `json:"modRevision"`

	revKey []byte
}

// Kind groups resources of the same type for the resource type list.
type Kind struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	APIVersion string `json:"apiVersion"`
	Group      string `json:"group"`
	Version    string `json:"version"`
	Count      int    `json:"count"`
	Namespaced bool   `json:"namespaced"`
	BuiltIn    bool   `json:"builtIn"`
}

// Namespace is one namespace with the number of objects it holds.
type Namespace struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Index is an immutable snapshot index, safe for concurrent reads.
type Index struct {
	snap       *snapshot.Snapshot
	resources  []*Resource
	byID       map[string]*Resource
	byUID      map[string]*Resource
	kinds      []Kind
	namespaces []Namespace
	Info       snapshot.Info
	BuiltIn    time.Duration
}

// Build scans the snapshot once and indexes every live object.
func Build(snap *snapshot.Snapshot) (*Index, error) {
	start := time.Now()
	info, err := snap.Info()
	if err != nil {
		return nil, fmt.Errorf("read snapshot metadata: %w", err)
	}

	idx := &Index{snap: snap, byID: map[string]*Resource{}, byUID: map[string]*Resource{}, Info: info}
	kindCount := map[string]*Kind{}
	nsCount := map[string]int{}

	err = snap.Scan(func(e snapshot.Entry, value []byte) error {
		if len(value) == 0 {
			return nil
		}
		meta, ok := kube.ExtractMeta(value)
		if !ok && meta.Kind == "" {
			// Not a Kubernetes object, for example etcd's own compact_rev_key.
			return nil
		}
		if meta.Name == "" {
			// A few internal API server records carry no ObjectMeta.
			_, meta.Name = kube.NameFromKey(e.Key)
		}
		group, version := kube.SplitAPIVersion(meta.APIVersion)
		kindID := kindIDOf(meta.APIVersion, meta.Kind)

		r := &Resource{
			ID:         encodeID(e.Key),
			Key:        e.Key,
			APIVersion: meta.APIVersion,
			Kind:       meta.Kind,
			KindID:     kindID,
			Group:      group,
			Version:    version,
			Namespace:  meta.Namespace,
			Name:       meta.Name,
			UID:        meta.UID,
			Encoding:   meta.Encoding,
			Size:       e.Size,
			ModRev:     e.ModRev,
			revKey:     e.RevKey,
		}
		if !meta.Created.IsZero() {
			t := meta.Created
			r.Created = &t
		}

		idx.resources = append(idx.resources, r)
		idx.byID[r.ID] = r
		if r.UID != "" {
			idx.byUID[r.UID] = r
		}

		k := kindCount[kindID]
		if k == nil {
			k = &Kind{
				ID:         kindID,
				Kind:       meta.Kind,
				Label:      pluralize(meta.Kind),
				APIVersion: meta.APIVersion,
				Group:      group,
				Version:    version,
				BuiltIn:    kube.KnownKind(meta.APIVersion, meta.Kind),
			}
			kindCount[kindID] = k
		}
		k.Count++
		if r.Namespace != "" {
			k.Namespaced = true
			nsCount[r.Namespace]++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan snapshot: %w", err)
	}

	for _, k := range kindCount {
		idx.kinds = append(idx.kinds, *k)
	}
	sortKinds(idx.kinds)

	for n, c := range nsCount {
		idx.namespaces = append(idx.namespaces, Namespace{Name: n, Count: c})
	}
	sort.Slice(idx.namespaces, func(i, j int) bool { return idx.namespaces[i].Name < idx.namespaces[j].Name })

	idx.BuiltIn = time.Since(start)
	return idx, nil
}

func (i *Index) Kinds() []Kind           { return i.kinds }
func (i *Index) Namespaces() []Namespace { return i.namespaces }
func (i *Index) Total() int              { return len(i.resources) }

// Get returns one indexed resource.
func (i *Index) Get(id string) (*Resource, bool) {
	r, ok := i.byID[id]
	return r, ok
}

// Value reads the stored object body for a resource.
func (i *Index) Value(r *Resource) ([]byte, error) { return i.snap.Value(r.revKey) }

// Query describes a resource list request.
type Query struct {
	KindID    string
	Namespace string
	Search    string // matches name, namespace and kind
	Deep      bool   // also match the stored object body
	Sort      string // name, namespace, created, kind, size
	Desc      bool
	Offset    int
	Limit     int
}

// Result is a page of resources.
type Result struct {
	Total int         `json:"total"`
	Items []*Resource `json:"items"`
}

// List filters, sorts and pages the index.
func (i *Index) List(q Query) (Result, error) {
	needle := strings.ToLower(strings.TrimSpace(q.Search))
	var matched []*Resource

	for _, r := range i.resources {
		if q.KindID != "" && q.KindID != "*" && r.KindID != q.KindID {
			continue
		}
		if q.Namespace != "" && q.Namespace != "*" && r.Namespace != q.Namespace {
			continue
		}
		if needle != "" && !i.matches(r, needle, q.Deep) {
			continue
		}
		matched = append(matched, r)
	}

	sortResources(matched, q.Sort, q.Desc)

	total := len(matched)
	if q.Offset > total {
		q.Offset = total
	}
	end := total
	if q.Limit > 0 && q.Offset+q.Limit < total {
		end = q.Offset + q.Limit
	}
	return Result{Total: total, Items: matched[q.Offset:end]}, nil
}

func (i *Index) matches(r *Resource, needle string, deep bool) bool {
	if strings.Contains(strings.ToLower(r.Name), needle) ||
		strings.Contains(strings.ToLower(r.Namespace), needle) ||
		strings.Contains(strings.ToLower(r.Kind), needle) {
		return true
	}
	if !deep {
		return false
	}
	// Both JSON and protobuf keep strings as plain bytes, so a raw substring
	// search over the stored value finds content hits in either encoding
	// without paying to decode the object.
	body, err := i.snap.Value(r.revKey)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(body)), needle)
}

// Related finds resources connected to r: its owners, the objects it owns and
// its namespace peers of the same kind.
func (i *Index) Related(r *Resource, body []byte) []*Resource {
	seen := map[string]bool{r.ID: true}
	var out []*Resource

	add := func(x *Resource) {
		if x != nil && !seen[x.ID] {
			seen[x.ID] = true
			out = append(out, x)
		}
	}

	// Owners, resolved through the ownerReferences UID.
	for _, uid := range ownerUIDs(body) {
		add(i.byUID[uid])
	}
	// Objects that name this resource as their owner.
	if r.UID != "" {
		for _, cand := range i.resources {
			if cand.Namespace != r.Namespace {
				continue
			}
			b, err := i.snap.Value(cand.revKey)
			if err != nil {
				continue
			}
			for _, uid := range ownerUIDs(b) {
				if uid == r.UID {
					add(cand)
					break
				}
			}
		}
	}
	// The namespace itself.
	if r.Namespace != "" {
		for _, cand := range i.resources {
			if cand.Kind == "Namespace" && cand.Name == r.Namespace {
				add(cand)
				break
			}
		}
	}
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

func encodeID(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// DecodeID turns a resource id back into its etcd key.
func DecodeID(id string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	return string(b), err
}

func kindIDOf(apiVersion, kind string) string {
	if apiVersion == "" {
		return kind
	}
	return apiVersion + "/" + kind
}
