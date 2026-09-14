package demo

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// TestWriteProducesAReadableBackup checks the fixture through the same reader
// the tool uses, so a broken generator fails here rather than confusing every
// other test.
func TestWriteProducesAReadableBackup(t *testing.T) {
	dir := t.TempDir()
	path, err := Write(dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasSuffix(path, ".db") {
		t.Errorf("snapshot path = %q", path)
	}
}

func TestClusterUsesOnlyReservedNames(t *testing.T) {
	records, err := buildCluster()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 50 {
		t.Fatalf("the fixture holds only %d objects", len(records))
	}

	// The point of this package is that a published screenshot, a CI log or a
	// bug report never carries a real cluster's names. Rather than listing
	// names to avoid, which would itself put them in the repository, assert
	// that every name in the fixture comes from a reserved space:
	// example.com is reserved for documentation by RFC 2606, and 192.0.2.0/24
	// is TEST-NET-1 from RFC 5737.
	host := regexp.MustCompile(`\b[a-z0-9][a-z0-9.-]*\.(com|net|org|io|local)\b`)
	ipv4 := regexp.MustCompile(`\b(\d{1,3}\.){3}\d{1,3}\b`)

	allowedNamespace := map[string]bool{"openshift-gitops": true}
	for _, ns := range namespaces {
		allowedNamespace[ns] = true
	}

	for _, r := range records {
		body := string(r.value)

		for _, h := range host.FindAllString(body, -1) {
			if isAPIDomain(h) {
				// An API group name such as rbac.authorization.k8s.io is part
				// of the object, not a hostname from someone's cluster.
				continue
			}
			if !strings.HasSuffix(h, "example.com") {
				t.Errorf("%s carries hostname %q, which is not under example.com", r.key, h)
			}
		}

		for _, ip := range ipv4.FindAllString(body, -1) {
			switch {
			case strings.HasPrefix(ip, "192.0.2."): // RFC 5737 documentation range
			case strings.HasPrefix(ip, "10.128."): // the usual cluster pod network
			case strings.HasPrefix(ip, "172.30."): // the usual cluster service network
			default:
				t.Errorf("%s carries address %q, which is not a reserved example range", r.key, ip)
			}
		}
	}

	// Namespaces are the names most visible in a screenshot, so pin them.
	for _, r := range records {
		meta, _ := kube.ExtractMeta(r.value)
		if meta.Namespace != "" && !allowedNamespace[meta.Namespace] {
			t.Errorf("%s lives in namespace %q, which is not one the fixture declares", r.key, meta.Namespace)
		}
	}
}

func TestClusterCoversBothStorageEncodings(t *testing.T) {
	records, err := buildCluster()
	if err != nil {
		t.Fatal(err)
	}
	var proto, jsonEncoded int
	for _, r := range records {
		switch {
		case strings.HasPrefix(string(r.value), "k8s\x00"):
			proto++
		case strings.HasPrefix(string(r.value), "{"):
			jsonEncoded++
		}
	}
	// A snapshot that exercised only one encoding would leave half the
	// decoder untested.
	if proto < 20 {
		t.Errorf("only %d protobuf objects", proto)
	}
	if jsonEncoded < 3 {
		t.Errorf("only %d JSON objects, custom resources are stored as JSON", jsonEncoded)
	}
}

// apiDomains are the suffixes of Kubernetes and ecosystem API groups, which
// appear inside every stored object and are not cluster hostnames.
var apiDomains = []string{
	"k8s.io",
	"kubernetes.io",
	"openshift.io",
	"argoproj.io",
	"coreos.com",
	"bitnami.com",
	"svc",
	"local",
}

func isAPIDomain(host string) bool {
	for _, suffix := range apiDomains {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}
