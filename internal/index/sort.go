package index

import (
	"sort"
	"strings"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// kindOrder puts the types an operator reaches for first at the top of the
// resource type list, the way the OpenShift console does. Everything else
// follows, ordered by how many objects it has.
var kindOrder = []string{
	"v1/Namespace",
	"v1/Pod",
	"apps/v1/Deployment",
	"apps.openshift.io/v1/DeploymentConfig",
	"apps/v1/ReplicaSet",
	"apps/v1/StatefulSet",
	"apps/v1/DaemonSet",
	"batch/v1/Job",
	"batch/v1/CronJob",
	"v1/Service",
	"v1/Endpoints",
	"discovery.k8s.io/v1/EndpointSlice",
	"v1/ConfigMap",
	"v1/Secret",
	"v1/PersistentVolumeClaim",
	"v1/PersistentVolume",
	"networking.k8s.io/v1/Ingress",
	"route.openshift.io/v1/Route",
	"v1/ServiceAccount",
	"rbac.authorization.k8s.io/v1/Role",
	"rbac.authorization.k8s.io/v1/RoleBinding",
	"rbac.authorization.k8s.io/v1/ClusterRole",
	"rbac.authorization.k8s.io/v1/ClusterRoleBinding",
	"apiextensions.k8s.io/v1/CustomResourceDefinition",
	"apiextensions.k8s.io/v1beta1/CustomResourceDefinition",
	"v1/Node",
	"v1/Event",
}

func sortKinds(kinds []Kind) {
	rank := map[string]int{}
	for i, id := range kindOrder {
		rank[id] = i
	}
	sort.SliceStable(kinds, func(a, b int) bool {
		ra, oka := rank[kinds[a].ID]
		rb, okb := rank[kinds[b].ID]
		switch {
		case oka && okb:
			return ra < rb
		case oka:
			return true
		case okb:
			return false
		}
		if kinds[a].Count != kinds[b].Count {
			return kinds[a].Count > kinds[b].Count
		}
		return kinds[a].Kind < kinds[b].Kind
	})
}

func sortResources(rs []*Resource, field string, desc bool) {
	// missing reports that a resource has no value for the sort field. Such a
	// resource always sorts last, in either direction, so an absent timestamp
	// never displaces real data at the top of the list.
	missing := func(*Resource) bool { return false }
	var less func(a, b *Resource) bool

	switch field {
	case "namespace":
		less = func(a, b *Resource) bool {
			if a.Namespace != b.Namespace {
				return a.Namespace < b.Namespace
			}
			return a.Name < b.Name
		}
	case "kind":
		less = func(a, b *Resource) bool {
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			return a.Name < b.Name
		}
	case "created":
		missing = func(r *Resource) bool { return r.Created == nil }
		less = func(a, b *Resource) bool {
			if a.Created.Equal(*b.Created) {
				return a.Name < b.Name
			}
			return a.Created.Before(*b.Created)
		}
	case "size":
		less = func(a, b *Resource) bool {
			if a.Size != b.Size {
				return a.Size < b.Size
			}
			return a.Name < b.Name
		}
	default: // name
		less = func(a, b *Resource) bool {
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.Namespace < b.Namespace
		}
	}

	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		mi, mj := missing(a), missing(b)
		if mi != mj {
			return mj
		}
		if mi && mj {
			return a.Name < b.Name
		}
		if desc {
			return less(b, a)
		}
		return less(a, b)
	})
}

// pluralize turns a Kind into the plural label used in the type list. The
// stored etcd path is not a reliable source because grouped and core resources
// are laid out differently, so the Kind is pluralized directly.
func pluralize(kind string) string {
	if kind == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(kind, "s") && !strings.HasSuffix(kind, "ss"):
		return kind // Endpoints, DNS, and other already plural kinds
	case strings.HasSuffix(kind, "y") && !hasVowelBefore(kind):
		return kind[:len(kind)-1] + "ies"
	case strings.HasSuffix(kind, "s"), strings.HasSuffix(kind, "x"),
		strings.HasSuffix(kind, "ch"), strings.HasSuffix(kind, "sh"):
		return kind + "es"
	}
	return kind + "s"
}

func hasVowelBefore(s string) bool {
	if len(s) < 2 {
		return false
	}
	return strings.ContainsRune("aeiouAEIOU", rune(s[len(s)-2]))
}

func ownerUIDs(value []byte) []string {
	refs := kube.OwnerRefs(value)
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r.UID != "" {
			out = append(out, r.UID)
		}
	}
	return out
}
