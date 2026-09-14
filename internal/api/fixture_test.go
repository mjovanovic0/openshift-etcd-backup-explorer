package api

import (
	"os"
	"testing"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/demo"
)

// fixtureDir returns a backup to test against, preferring a real one in
// ./backup and otherwise generating a synthetic cluster.
func fixtureDir(t *testing.T) string {
	t.Helper()
	// Setting OEBE_FIXTURE=demo forces the generated fixture, which is how CI
	// runs the suite.
	if os.Getenv("OEBE_FIXTURE") != "demo" {
		if _, err := os.Stat("../../backup"); err == nil {
			return "../../backup"
		}
	}
	dir := t.TempDir()
	if _, err := demo.Write(dir); err != nil {
		t.Fatalf("generate the demo backup: %v", err)
	}
	return dir
}
