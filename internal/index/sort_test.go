package index

import (
	"testing"
	"time"
)

func TestPluralize(t *testing.T) {
	cases := map[string]string{
		"Pod":                      "Pods",
		"Deployment":               "Deployments",
		"ReplicaSet":               "ReplicaSets",
		"Ingress":                  "Ingresses",
		"CustomResourceDefinition": "CustomResourceDefinitions",
		"Endpoints":                "Endpoints",
		"NetworkPolicy":            "NetworkPolicies",
		"Gateway":                  "Gateways",
		"APIService":               "APIServices",
		"":                         "",
	}
	for in, want := range cases {
		if got := pluralize(in); got != want {
			t.Errorf("pluralize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortKindsPutsCommonTypesFirst(t *testing.T) {
	kinds := []Kind{
		{ID: "argoproj.io/v1alpha1/Application", Kind: "Application", Count: 385},
		{ID: "v1/Pod", Kind: "Pod", Count: 622},
		{ID: "v1/Event", Kind: "Event", Count: 5185},
		{ID: "bitnami.com/v1alpha1/SealedSecret", Kind: "SealedSecret", Count: 136},
		{ID: "v1/Namespace", Kind: "Namespace", Count: 213},
	}
	sortKinds(kinds)

	// Namespace and Pod are in the curated order, so they lead even though
	// Event and Application hold far more objects.
	if kinds[0].ID != "v1/Namespace" {
		t.Errorf("first kind = %s, want v1/Namespace", kinds[0].ID)
	}
	if kinds[1].ID != "v1/Pod" {
		t.Errorf("second kind = %s, want v1/Pod", kinds[1].ID)
	}
	if kinds[2].ID != "v1/Event" {
		t.Errorf("third kind = %s, want v1/Event", kinds[2].ID)
	}
	// The remaining types that are not curated fall back to count order.
	if kinds[3].ID != "argoproj.io/v1alpha1/Application" {
		t.Errorf("fourth kind = %s, want the most numerous custom resource", kinds[3].ID)
	}
}

func TestSortResources(t *testing.T) {
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	build := func() []*Resource {
		return []*Resource{
			{Name: "beta", Namespace: "z", Kind: "Pod", Size: 10, Created: &newer},
			{Name: "alpha", Namespace: "a", Kind: "Service", Size: 30, Created: &older},
			{Name: "gamma", Namespace: "m", Kind: "ConfigMap", Size: 20, Created: nil},
		}
	}

	rs := build()
	sortResources(rs, "name", false)
	if rs[0].Name != "alpha" || rs[2].Name != "gamma" {
		t.Errorf("name order = %s,%s,%s", rs[0].Name, rs[1].Name, rs[2].Name)
	}

	rs = build()
	sortResources(rs, "name", true)
	if rs[0].Name != "gamma" {
		t.Errorf("descending name order starts with %s, want gamma", rs[0].Name)
	}

	rs = build()
	sortResources(rs, "namespace", false)
	if rs[0].Namespace != "a" {
		t.Errorf("namespace order starts with %q", rs[0].Namespace)
	}

	rs = build()
	sortResources(rs, "size", false)
	if rs[0].Size != 10 || rs[2].Size != 30 {
		t.Errorf("size order = %d,%d,%d", rs[0].Size, rs[1].Size, rs[2].Size)
	}

	// An object with no creation timestamp must sort last rather than crash.
	rs = build()
	sortResources(rs, "created", false)
	if rs[0].Name != "alpha" {
		t.Errorf("created order starts with %s, want alpha (oldest)", rs[0].Name)
	}
	if rs[2].Name != "gamma" {
		t.Errorf("object with no timestamp = %s at the end, want gamma", rs[2].Name)
	}

	rs = build()
	sortResources(rs, "created", true)
	if rs[0].Name != "beta" {
		t.Errorf("descending created order starts with %s, want beta (newest)", rs[0].Name)
	}
}

func TestEncodeDecodeID(t *testing.T) {
	for _, key := range []string{
		"/kubernetes.io/pods/openshift-monitoring/alertmanager-main-0",
		"/openshift.io/images/sha256:0068fce7f4004bfbed363cd80a27f670",
		"",
	} {
		id := encodeID(key)
		back, err := DecodeID(id)
		if err != nil {
			t.Fatalf("DecodeID(%q): %v", id, err)
		}
		if back != key {
			t.Errorf("round trip changed the key: %q -> %q", key, back)
		}
	}
}
