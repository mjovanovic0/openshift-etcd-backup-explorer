package index

import (
	"testing"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/demo"
)

// fixtureDir returns a backup to test against. A real backup in ./backup is
// used when one is there, because testing against real cluster data catches
// what a fixture cannot. Otherwise a synthetic one is generated, so the suite
// runs anywhere, including in CI.
func fixtureDir(t *testing.T) string {
	t.Helper()
	if dir := realBackupDir(); dir != "" {
		return dir
	}
	dir := t.TempDir()
	if _, err := demo.Write(dir); err != nil {
		t.Fatalf("generate the demo backup: %v", err)
	}
	return dir
}
