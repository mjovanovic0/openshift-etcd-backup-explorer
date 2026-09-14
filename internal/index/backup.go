package index

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/snapshot"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/staticres"
)

// stampPattern matches the timestamp OpenShift puts in backup file names,
// for example snapshot_2026-09-11_092812.db.
var stampPattern = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})_(\d{2})(\d{2})(\d{2})`)

// Backup is one snapshot plus the static resources archive taken with it.
type Backup struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	SnapshotPath string    `json:"snapshotPath"`
	StaticPath   string    `json:"staticPath,omitempty"`
	TakenAt      time.Time `json:"takenAt"`
	Size         int64     `json:"size"`

	snap    *snapshot.Snapshot
	archive *staticres.Archive
	idx     *Index
}

// Discover finds the backups under a path. The path may be a directory holding
// one or more snapshots, or a single snapshot file.
func Discover(path string) ([]*Backup, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var snapshots []string
	var statics []string
	if st.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			full := filepath.Join(path, e.Name())
			switch {
			case strings.HasSuffix(e.Name(), ".db"):
				snapshots = append(snapshots, full)
			case strings.HasSuffix(e.Name(), ".tar.gz"):
				statics = append(statics, full)
			}
		}
	} else {
		snapshots = append(snapshots, path)
		// Look for the matching archive beside the snapshot.
		dir := filepath.Dir(path)
		if m := stampPattern.FindString(filepath.Base(path)); m != "" {
			matches, _ := filepath.Glob(filepath.Join(dir, "*"+m+"*.tar.gz"))
			statics = append(statics, matches...)
		}
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no etcd snapshot (*.db) found in %s", path)
	}

	var out []*Backup
	for _, s := range snapshots {
		fi, err := os.Stat(s)
		if err != nil {
			return nil, err
		}
		b := &Backup{
			ID:           strings.TrimSuffix(filepath.Base(s), ".db"),
			SnapshotPath: s,
			Size:         fi.Size(),
			TakenAt:      fi.ModTime(),
		}
		stamp := stampPattern.FindStringSubmatch(filepath.Base(s))
		if stamp != nil {
			if t, err := time.Parse("2006-01-02 150405", stamp[1]+" "+stamp[2]+stamp[3]+stamp[4]); err == nil {
				b.TakenAt = t
			}
			// Pair the archive by its timestamp.
			for _, a := range statics {
				if strings.Contains(filepath.Base(a), stamp[0]) {
					b.StaticPath = a
					break
				}
			}
		}
		if b.StaticPath == "" && len(statics) == 1 && len(snapshots) == 1 {
			b.StaticPath = statics[0]
		}
		b.Label = b.TakenAt.Format("2006-01-02 15:04:05") + " (etcd)"
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TakenAt.After(out[j].TakenAt) })
	return out, nil
}

// Load opens the snapshot, indexes it and reads the static resources archive.
func (b *Backup) Load() error {
	snap, err := snapshot.Open(b.SnapshotPath)
	if err != nil {
		return err
	}
	idx, err := Build(snap)
	if err != nil {
		snap.Close()
		return err
	}
	b.snap, b.idx = snap, idx

	if b.StaticPath != "" {
		archive, err := staticres.Open(b.StaticPath)
		if err != nil {
			// A missing or damaged archive must not stop the keyspace browser.
			return nil
		}
		b.archive = archive
	}
	return nil
}

// Close releases the snapshot file.
func (b *Backup) Close() error {
	if b.snap != nil {
		return b.snap.Close()
	}
	return nil
}

// Index returns the built index, or nil when the backup is not loaded.
func (b *Backup) Index() *Index { return b.idx }

// Archive returns the static resources archive, or nil when there is none.
func (b *Backup) Archive() *staticres.Archive { return b.archive }

// Loaded reports whether the backup has been indexed.
func (b *Backup) Loaded() bool { return b.idx != nil }
