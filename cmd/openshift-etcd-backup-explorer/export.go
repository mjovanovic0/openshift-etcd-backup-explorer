package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/index"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// runExport writes matching resources to a folder, one manifest per file, or
// to stdout as a single stream.
func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	path := backupFlag(fs)
	kind := fs.String("kind", "", `resource type, either a Kind such as "PersistentVolume" or a full id such as "v1/Pod"`)
	namespace := fs.String("namespace", "", "restrict to one namespace")
	q := fs.String("q", "", "match name, namespace or kind")
	deep := fs.Bool("deep", false, "also search inside the stored object bodies")
	out := fs.String("out", "", `folder to write into, or "-" for a single stream on stdout`)
	format := fs.String("o", "yaml", "output format: yaml or json")
	keep := fs.Bool("keep-generated", false,
		"keep metadata.uid, metadata.creationTimestamp and managedFields, which are removed by default")
	force := fs.Bool("force", false, "write into a folder that already holds files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required, give a folder to write into or - for stdout")
	}
	if *format != "yaml" && *format != "json" {
		return fmt.Errorf("unknown format %q, use yaml or json", *format)
	}

	b, err := loadOne(*path)
	if err != nil {
		return err
	}
	defer b.Close()
	idx := b.Index()

	kindID, err := resolveKind(idx, *kind)
	if err != nil {
		return err
	}
	res, err := idx.List(index.Query{
		KindID: kindID, Namespace: *namespace, Search: *q, Deep: *deep, Sort: "namespace",
	})
	if err != nil {
		return err
	}
	if len(res.Items) == 0 {
		return errors.New("no resources match these filters")
	}

	if *out == "-" {
		return exportStream(idx, res.Items, *format, !*keep)
	}
	return exportFolder(idx, res.Items, *out, *format, !*keep, *force)
}

// exportStream writes every object to stdout as one document stream.
func exportStream(idx *index.Index, items []*index.Resource, format string, strip bool) error {
	for i, item := range items {
		obj, err := renderForExport(idx, item, strip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipped %s: %v\n", item.Key, err)
			continue
		}
		if format == "json" {
			fmt.Println(string(obj.JSON))
			continue
		}
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Printf("# %s %s\n", item.Kind, displayPath(item))
		fmt.Print(obj.YAML)
	}
	return nil
}

// exportFolder writes one file per object under out, laid out as
// kind/namespace/name so the result is browsable.
func exportFolder(idx *index.Index, items []*index.Resource, out, format string, strip, force bool) error {
	if err := checkTargetFolder(out, force); err != nil {
		return err
	}

	used := map[string]int{}
	var written, skipped int
	for _, item := range items {
		obj, err := renderForExport(idx, item, strip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipped %s: %v\n", item.Key, err)
			skipped++
			continue
		}

		rel := relativePath(item, format)
		if n := used[rel]; n > 0 {
			ext := filepath.Ext(rel)
			rel = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(rel, ext), n+1, ext)
		}
		used[relativePath(item, format)]++

		full := filepath.Join(out, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		payload := []byte(obj.YAML)
		if format == "json" {
			payload = obj.JSON
		}
		// The manifests can hold secret data, so keep them readable by the
		// user who exported them and nobody else.
		if err := os.WriteFile(full, payload, 0o600); err != nil {
			return err
		}
		written++
	}

	fmt.Printf("wrote %d manifests to %s\n", written, out)
	if skipped > 0 {
		fmt.Printf("skipped %d resources that could not be decoded\n", skipped)
	}
	if strip {
		fmt.Println("removed metadata.uid, metadata.creationTimestamp and managedFields, pass -keep-generated to keep them")
	}
	return nil
}

// checkTargetFolder refuses to scatter files into a folder that already holds
// something, unless the caller insists.
func checkTargetFolder(out string, force bool) error {
	entries, err := os.ReadDir(out)
	switch {
	case os.IsNotExist(err):
		return os.MkdirAll(out, 0o755)
	case err != nil:
		return err
	case len(entries) > 0 && !force:
		return fmt.Errorf("%s already holds %d entries, pass -force to write into it anyway", out, len(entries))
	}
	return nil
}

func renderForExport(idx *index.Index, item *index.Resource, strip bool) (*kube.Object, error) {
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

func relativePath(r *index.Resource, format string) string {
	parts := []string{safeSegment(strings.ToLower(r.Kind))}
	if r.Namespace != "" {
		parts = append(parts, safeSegment(r.Namespace))
	}
	parts = append(parts, safeSegment(r.Name)+"."+format)
	return filepath.Join(parts...)
}

func displayPath(r *index.Resource) string {
	if r.Namespace == "" {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}

// safeSegment keeps a name usable as a path segment. Object names can hold
// characters such as the colon in an image digest.
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
