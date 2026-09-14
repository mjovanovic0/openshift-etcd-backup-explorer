// Package demo builds a synthetic etcd backup that looks like one taken from a
// small OpenShift cluster. It exists so the test suite, the screenshots and
// anyone trying the tool do not need a real cluster backup, which would carry
// real secrets and real customer names.
package demo

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Stamp is the timestamp the generated backup carries, in the shape OpenShift
// uses for its file names. It is fixed so the fixture is reproducible.
const Stamp = "2026-02-17_081500"

// TakenAt is Stamp as a time.
var TakenAt = time.Date(2026, 2, 17, 8, 15, 0, 0, time.UTC)

// Write generates a snapshot and its static resources archive in dir and
// returns the path of the snapshot.
func Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	snapshotPath := filepath.Join(dir, "snapshot_"+Stamp+".db")
	if err := writeSnapshot(snapshotPath); err != nil {
		return "", err
	}
	if err := writeStaticArchive(filepath.Join(dir, "static_kuberesources_"+Stamp+".tar.gz")); err != nil {
		return "", err
	}
	return snapshotPath, nil
}

// record is one key and its stored value, ready to be written into the
// keyspace.
type record struct {
	key     string
	value   []byte
	deleted bool
}

func writeSnapshot(path string) (err error) {
	records, err := buildCluster()
	if err != nil {
		return err
	}

	// Remove any earlier file so the fixture is rebuilt from scratch.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return err
	}
	// bolt flushes on Close, so a failure there means the fixture on disk is
	// not what was written.
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return db.Update(func(tx *bolt.Tx) error {
		// etcd creates these buckets whether or not they hold anything.
		for _, name := range []string{"alarm", "auth", "authRoles", "authUsers", "cluster", "key", "lease", "meta", "members", "members_removed"} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return err
			}
		}

		keys := tx.Bucket([]byte("key"))
		rev := int64(1000)
		for _, r := range records {
			rev++
			// An object that was updated leaves its earlier revision behind,
			// which is what makes a real snapshot larger than its live set.
			if rev%17 == 0 {
				if err := keys.Put(revisionKey(rev, 0, false), marshalKeyValue(r.key, r.value, rev, rev)); err != nil {
					return err
				}
				rev++
			}
			if err := keys.Put(revisionKey(rev, 0, r.deleted), marshalKeyValue(r.key, r.value, rev, rev)); err != nil {
				return err
			}
		}

		cluster := tx.Bucket([]byte("cluster"))
		if err := cluster.Put([]byte("clusterVersion"), []byte("3.6.0")); err != nil {
			return err
		}

		meta := tx.Bucket([]byte("meta"))
		idx := make([]byte, 8)
		binary.BigEndian.PutUint64(idx, uint64(rev)+4096)
		if err := meta.Put([]byte("consistent_index"), idx); err != nil {
			return err
		}
		if err := meta.Put([]byte("finishedCompactRev"), revisionKey(900, 0, false)); err != nil {
			return err
		}
		if err := meta.Put([]byte("confState"), []byte(`{"voters":[1,2,3],"auto_leave":false}`)); err != nil {
			return err
		}

		members := tx.Bucket([]byte("members"))
		for i, name := range []string{"master-0.demo.example.com", "master-1.demo.example.com", "master-2.demo.example.com"} {
			m := map[string]any{
				"id":         2000000000000000000 + i,
				"name":       name,
				"peerURLs":   []string{fmt.Sprintf("https://192.0.2.%d:2380", 10+i)},
				"clientURLs": []string{fmt.Sprintf("https://192.0.2.%d:2379", 10+i)},
			}
			body, err := json.Marshal(m)
			if err != nil {
				return err
			}
			if err := members.Put([]byte(fmt.Sprintf("%016x", 2000000000000000000+i)), body); err != nil {
				return err
			}
		}

		// A couple of expired leases, as a real cluster always has.
		leases := tx.Bucket([]byte("lease"))
		for i := 0; i < 4; i++ {
			id := make([]byte, 8)
			binary.BigEndian.PutUint64(id, uint64(0x3419000000000000+i))
			if err := leases.Put(id, []byte{0x08, 0xb9, 0xcb, 0x86, 0x10, 0xbc, 0xa3, 0x05}); err != nil {
				return err
			}
		}
		return nil
	})
}

// revisionKey builds the bbolt key etcd uses: 8 bytes of main revision, a '_'
// separator, 8 bytes of sub revision, and a trailing 't' for a deletion.
func revisionKey(main, sub int64, tombstone bool) []byte {
	b := make([]byte, 17, 18)
	binary.BigEndian.PutUint64(b[0:8], uint64(main))
	b[8] = '_'
	binary.BigEndian.PutUint64(b[9:17], uint64(sub))
	if tombstone {
		b = append(b, 't')
	}
	return b
}

// marshalKeyValue encodes mvccpb.KeyValue:
// 1=key 2=create_revision 3=mod_revision 4=version 5=value.
func marshalKeyValue(key string, value []byte, createRev, modRev int64) []byte {
	var out []byte
	out = appendBytesField(out, 1, []byte(key))
	out = appendVarintField(out, 2, uint64(createRev))
	out = appendVarintField(out, 3, uint64(modRev))
	out = appendVarintField(out, 4, 1)
	out = appendBytesField(out, 5, value)
	return out
}

// Protobuf wire types, the low three bits of a field tag.
const (
	wireVarint = 0
	wireBytes  = 2
)

func appendBytesField(dst []byte, field uint64, value []byte) []byte {
	dst = appendVarint(dst, field<<3|wireBytes)
	dst = appendVarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendVarintField(dst []byte, field, value uint64) []byte {
	dst = appendVarint(dst, field<<3|wireVarint)
	return appendVarint(dst, value)
}

func appendVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}
