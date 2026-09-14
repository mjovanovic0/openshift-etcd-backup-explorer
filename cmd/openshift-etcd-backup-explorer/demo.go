package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/demo"
)

// runDemo writes a synthetic backup so the tool can be tried without a real
// one. A real backup holds real secrets, so this is also what the screenshots
// and the test suite use.
func runDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	out := fs.String("out", "./demo-backup", "folder to write the generated backup into")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := demo.Write(*out)
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*out)

	fmt.Printf("wrote a synthetic backup to %s\n", abs)
	fmt.Printf("  snapshot         %s\n", filepath.Base(path))
	fmt.Printf("  static resources static_kuberesources_%s.tar.gz\n", demo.Stamp)
	fmt.Printf("\nserve it with:\n  openshift-etcd-backup-explorer serve -backup %s -open\n", *out)
	return nil
}
