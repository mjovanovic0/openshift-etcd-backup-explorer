// Package staticres reads the static_kuberesources tarball that OpenShift
// writes next to an etcd snapshot. It holds the static pod manifests, certs
// and configmaps that a restore needs, which are not part of the keyspace.
package staticres

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// File is one entry in the archive.
type File struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"isDir"`
	Content []byte `json:"-"`
}

// Archive is an extracted static resources tarball held in memory. The archive
// is small, a few hundred kilobytes, so keeping it decompressed is simpler
// than seeking through gzip on every request.
type Archive struct {
	Path  string
	Files []File
	index map[string]int
}

// Open reads and decompresses an archive.
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Read only, so a close failure cannot lose anything.
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = gz.Close() }()

	a := &Archive{Path: path, index: map[string]int{}}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		name := strings.TrimPrefix(h.Name, "./")
		file := File{
			Path:  name,
			Size:  h.Size,
			Mode:  os.FileMode(h.Mode).String(),
			IsDir: h.Typeflag == tar.TypeDir,
		}
		if !file.IsDir {
			// Cap any single file so a surprising archive cannot exhaust memory.
			body, err := io.ReadAll(io.LimitReader(tr, 8<<20))
			if err != nil {
				return nil, err
			}
			file.Content = body
			file.Size = int64(len(body))
		}
		a.index[strings.TrimSuffix(name, "/")] = len(a.Files)
		a.Files = append(a.Files, file)
	}
	sort.Slice(a.Files, func(i, j int) bool { return a.Files[i].Path < a.Files[j].Path })
	a.index = map[string]int{}
	for i, f := range a.Files {
		a.index[strings.TrimSuffix(f.Path, "/")] = i
	}
	return a, nil
}

// Get returns the contents of one file in the archive.
func (a *Archive) Get(path string) (File, bool) {
	i, ok := a.index[strings.TrimSuffix(strings.TrimPrefix(path, "./"), "/")]
	if !ok {
		return File{}, false
	}
	return a.Files[i], true
}
