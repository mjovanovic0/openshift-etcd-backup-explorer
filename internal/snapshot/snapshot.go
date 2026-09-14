// Package snapshot reads an etcd v3 snapshot file (a bbolt database) without
// starting an etcd server. It exposes the newest revision of every live key
// plus the cluster metadata that etcd keeps alongside the keyspace.
package snapshot

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Snapshot is an open, read-only handle on a snapshot file.
type Snapshot struct {
	Path string
	Size int64

	db *bolt.DB
}

// Entry is the newest surviving revision of one etcd key.
type Entry struct {
	Key       string
	RevKey    []byte // bbolt key, used to read the value back on demand
	ModRev    int64
	CreateRev int64
	Version   int64
	Size      int
}

// Member is one etcd cluster member as recorded in the snapshot.
type Member struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	PeerURLs   []string `json:"peerURLs"`
	ClientURLs []string `json:"clientURLs"`
	Removed    bool     `json:"removed"`
}

// Info describes the snapshot itself: where it came from and what etcd recorded.
type Info struct {
	File           string    `json:"file"`
	Size           int64     `json:"size"`
	ModTime        time.Time `json:"modTime"`
	ClusterVersion string    `json:"clusterVersion"`
	Revision       int64     `json:"revision"`
	CompactRev     int64     `json:"compactRevision"`
	ConsistentIdx  uint64    `json:"consistentIndex"`
	TotalKeys      int       `json:"totalKeys"`
	LiveKeys       int       `json:"liveKeys"`
	Tombstones     int       `json:"tombstones"`
	Leases         int       `json:"leases"`
	Members        []Member  `json:"members"`
}

// Open opens a snapshot file read-only. bbolt is opened with ReadOnly so the
// file on disk is never modified, which matters for a backup.
func Open(path string) (*Snapshot, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0400, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open snapshot %s: %w", path, err)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("key")) == nil {
			return fmt.Errorf("no %q bucket: this does not look like an etcd v3 snapshot", "key")
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Snapshot{Path: path, Size: st.Size(), db: db}, nil
}

func (s *Snapshot) Close() error { return s.db.Close() }

// Scan walks the keyspace and calls fn for the newest live revision of every
// key. Tombstoned keys are reported as deleted so callers can skip them.
// The value slice is only valid for the duration of the call.
func (s *Snapshot) Scan(fn func(e Entry, value []byte) error) error {
	type latest struct {
		e       Entry
		deleted bool
	}
	// Revision keys sort ascending by (main, sub), so a forward walk sees the
	// newest revision of each key last and simply overwrites earlier ones.
	newest := map[string]*latest{}
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("key")).Cursor()
		for rk, rv := c.First(); rk != nil; rk, rv = c.Next() {
			kv, ok := parseKeyValue(rv)
			if !ok {
				continue
			}
			deleted := isTombstone(rk)
			cur, seen := newest[string(kv.key)]
			if seen && kv.modRev < cur.e.ModRev {
				continue
			}
			e := Entry{
				Key:       string(kv.key),
				RevKey:    append([]byte(nil), rk...),
				ModRev:    kv.modRev,
				CreateRev: kv.createRev,
				Version:   kv.version,
				Size:      len(kv.value),
			}
			if seen {
				cur.e, cur.deleted = e, deleted
			} else {
				newest[string(kv.key)] = &latest{e: e, deleted: deleted}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(newest))
	for k, l := range newest {
		if !l.deleted {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		for _, k := range keys {
			e := newest[k].e
			raw := b.Get(e.RevKey)
			if raw == nil {
				continue
			}
			kv, ok := parseKeyValue(raw)
			if !ok {
				continue
			}
			if err := fn(e, kv.value); err != nil {
				return err
			}
		}
		return nil
	})
}

// Value reads back the value stored at a revision key.
func (s *Snapshot) Value(revKey []byte) ([]byte, error) {
	var out []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte("key")).Get(revKey)
		if raw == nil {
			return fmt.Errorf("revision key not found")
		}
		kv, ok := parseKeyValue(raw)
		if !ok {
			return fmt.Errorf("malformed mvcc record")
		}
		out = append([]byte(nil), kv.value...)
		return nil
	})
	return out, err
}

// Info collects the snapshot level metadata shown in the Backup Info panel.
func (s *Snapshot) Info() (Info, error) {
	st, err := os.Stat(s.Path)
	if err != nil {
		return Info{}, err
	}
	info := Info{File: s.Path, Size: st.Size(), ModTime: st.ModTime()}

	err = s.db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte("cluster")); b != nil {
			info.ClusterVersion = string(b.Get([]byte("clusterVersion")))
		}
		if b := tx.Bucket([]byte("meta")); b != nil {
			if v := b.Get([]byte("consistent_index")); len(v) == 8 {
				info.ConsistentIdx = binary.BigEndian.Uint64(v)
			}
			if v := b.Get([]byte("finishedCompactRev")); len(v) >= 8 {
				info.CompactRev = int64(binary.BigEndian.Uint64(v[:8]))
			}
		}
		if b := tx.Bucket([]byte("lease")); b != nil {
			info.Leases = b.Stats().KeyN
		}
		if b := tx.Bucket([]byte("members")); b != nil {
			if err := b.ForEach(func(k, v []byte) error {
				var m struct {
					ID         json.Number `json:"id"`
					Name       string      `json:"name"`
					PeerURLs   []string    `json:"peerURLs"`
					ClientURLs []string    `json:"clientURLs"`
				}
				if err := json.Unmarshal(v, &m); err != nil {
					return nil
				}
				info.Members = append(info.Members, Member{
					ID: m.ID.String(), Name: m.Name, PeerURLs: m.PeerURLs, ClientURLs: m.ClientURLs,
				})
				return nil
			}); err != nil {
				return err
			}
		}
		if b := tx.Bucket([]byte("members_removed")); b != nil {
			if err := b.ForEach(func(k, v []byte) error {
				info.Members = append(info.Members, Member{ID: string(k), Name: "(removed)", Removed: true})
				return nil
			}); err != nil {
				return err
			}
		}

		b := tx.Bucket([]byte("key"))
		c := b.Cursor()
		seen := map[string]bool{}
		for rk, rv := c.First(); rk != nil; rk, rv = c.Next() {
			info.TotalKeys++
			if isTombstone(rk) {
				info.Tombstones++
			}
			if kv, ok := parseKeyValue(rv); ok {
				if kv.modRev > info.Revision {
					info.Revision = kv.modRev
				}
				seen[string(kv.key)] = true
			}
		}
		info.LiveKeys = len(seen)
		return nil
	})
	sort.Slice(info.Members, func(i, j int) bool { return info.Members[i].Name < info.Members[j].Name })
	return info, err
}

// A revision key is 8 bytes of main revision, a '_' separator and 8 bytes of
// sub revision. etcd appends 't' to mark the key as deleted at that revision.
func isTombstone(revKey []byte) bool {
	return len(revKey) == 18 && revKey[17] == 't'
}

type mvccKV struct {
	key       []byte
	value     []byte
	createRev int64
	modRev    int64
	version   int64
}

// parseKeyValue decodes mvccpb.KeyValue. It is hand written so that reading a
// snapshot does not pull in the etcd server and gRPC dependency tree:
// 1=key 2=create_revision 3=mod_revision 4=version 5=value 6=lease.
func parseKeyValue(b []byte) (mvccKV, bool) {
	var kv mvccKV
	i := 0
	for i < len(b) {
		tag, n := uvarint(b[i:])
		if n <= 0 {
			return kv, false
		}
		i += n
		field, wire := tag>>3, tag&7
		switch wire {
		case 0:
			v, n := uvarint(b[i:])
			if n <= 0 {
				return kv, false
			}
			i += n
			switch field {
			case 2:
				kv.createRev = int64(v)
			case 3:
				kv.modRev = int64(v)
			case 4:
				kv.version = int64(v)
			}
		case 2:
			l, n := uvarint(b[i:])
			if n <= 0 || i+n+int(l) > len(b) {
				return kv, false
			}
			i += n
			switch field {
			case 1:
				kv.key = b[i : i+int(l)]
			case 5:
				kv.value = b[i : i+int(l)]
			}
			i += int(l)
		case 1:
			i += 8
		case 5:
			i += 4
		default:
			return kv, false
		}
	}
	return kv, kv.key != nil
}

func uvarint(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i := 0; i < len(b) && i < 10; i++ {
		c := b[i]
		if c < 0x80 {
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, -1
}
