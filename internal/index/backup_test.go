package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
)

// realBackupDir reports a real cluster backup to test against, if the checkout
// has one. It is never committed, so most runs use the generated fixture.
func realBackupDir() string {
	// Setting this forces the generated fixture, which is how CI runs and a
	// way to check the suite does not quietly depend on real cluster data.
	if os.Getenv("OEBE_FIXTURE") == "demo" {
		return ""
	}
	const dir = "../../backup"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db") {
			return dir
		}
	}
	return ""
}

// loadSample opens whichever backup the fixture helper provides.
func loadSample(t *testing.T) *Backup {
	t.Helper()
	backups, err := Discover(fixtureDir(t))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("discovered %d backups, want 1", len(backups))
	}
	b := backups[0]
	if err := b.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestDiscoverPairsSnapshotWithArchive(t *testing.T) {
	b := loadSample(t)

	if !strings.HasSuffix(b.SnapshotPath, ".db") {
		t.Errorf("snapshot path = %q", b.SnapshotPath)
	}
	if !strings.HasSuffix(b.StaticPath, ".tar.gz") {
		t.Errorf("static archive was not paired with the snapshot: %q", b.StaticPath)
	}
	// The timestamp in the file name must win over the file modification time,
	// which changes whenever the backup is copied around.
	stamp := stampPattern.FindString(filepath.Base(b.SnapshotPath))
	if stamp == "" {
		t.Fatalf("snapshot name %q carries no timestamp", b.SnapshotPath)
	}
	if got := b.TakenAt.Format("2006-01-02_150405"); got != stamp {
		t.Errorf("takenAt = %s, want %s from the file name", got, stamp)
	}
	if b.Archive() == nil {
		t.Fatal("static resources archive was not loaded")
	}
}

func TestDiscoverAcceptsASingleSnapshotFile(t *testing.T) {
	dir := fixtureDir(t)
	found, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := found[0].SnapshotPath

	backups, err := Discover(snapshot)
	if err != nil {
		t.Fatalf("Discover on a single file: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("got %d backups, want 1", len(backups))
	}
	// The archive beside the file must still be found by its timestamp.
	stamp := stampPattern.FindString(filepath.Base(snapshot))
	if !strings.Contains(backups[0].StaticPath, stamp) {
		t.Errorf("static archive = %q, want the one matching %s", backups[0].StaticPath, stamp)
	}
}

func TestDiscoverRejectsAPathWithNoSnapshot(t *testing.T) {
	if _, err := Discover(t.TempDir()); err == nil {
		t.Error("Discover should fail when no snapshot is present")
	}
	if _, err := Discover("/does/not/exist"); err == nil {
		t.Error("Discover should fail on a missing path")
	}
}

func TestIndexCoversTheWholeKeyspace(t *testing.T) {
	idx := loadSample(t).Index()

	if idx.Total() < 50 {
		t.Errorf("indexed %d objects, expected the full keyspace", idx.Total())
	}
	// Every live key is either a Kubernetes object or one of etcd's own
	// records, so the index should account for nearly all of them.
	if diff := idx.Info.LiveKeys - idx.Total(); diff < 0 || diff > 10 {
		t.Errorf("index holds %d of %d live keys, difference %d is larger than expected",
			idx.Total(), idx.Info.LiveKeys, diff)
	}
	if idx.Info.ClusterVersion == "" {
		t.Error("cluster version was not read from the snapshot")
	}
	if len(idx.Info.Members) == 0 {
		t.Error("no etcd members were read from the snapshot")
	}
	if len(idx.Kinds()) < 10 {
		t.Errorf("found %d kinds, expected the full set", len(idx.Kinds()))
	}
	if len(idx.Namespaces()) < 5 {
		t.Errorf("found %d namespaces, expected the full set", len(idx.Namespaces()))
	}
}

func TestEveryIndexedObjectHasANameAndType(t *testing.T) {
	idx := loadSample(t).Index()

	res, err := idx.List(Query{KindID: "*", Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Items {
		if r.Name == "" {
			t.Errorf("object at %s has no name", r.Key)
		}
		if r.Kind == "" {
			t.Errorf("object at %s has no kind", r.Key)
		}
		if r.ID == "" {
			t.Errorf("object at %s has no id", r.Key)
		}
	}
}

// TestEveryKindDecodes is the guarantee that matters most: an operator must be
// able to open any object in the backup and read it.
func TestEveryKindDecodes(t *testing.T) {
	idx := loadSample(t).Index()

	var undecoded []string
	for _, k := range idx.Kinds() {
		res, err := idx.List(Query{KindID: k.ID, Limit: 1, Sort: "name"})
		if err != nil || len(res.Items) == 0 {
			t.Errorf("no sample object for %s", k.ID)
			continue
		}
		r := res.Items[0]
		body, err := idx.Value(r)
		if err != nil {
			t.Errorf("read %s: %v", r.Key, err)
			continue
		}
		obj, err := kube.Decode(body)
		if err != nil {
			t.Errorf("decode %s (%s): %v", k.ID, r.Key, err)
			continue
		}
		if obj.YAML == "" {
			t.Errorf("%s produced no YAML", k.ID)
		}
		if !obj.Decoded {
			undecoded = append(undecoded, k.ID)
			// A type with no compiled in Go struct must still yield metadata.
			if !strings.Contains(obj.YAML, "metadata:") {
				t.Errorf("%s fell back without metadata: %s", k.ID, obj.Reason)
			}
		}
	}
	// PodSecurityPolicy was removed from the Kubernetes libraries, so no Go
	// type exists for it anywhere. Anything beyond that is a real gap.
	for _, id := range undecoded {
		if !strings.Contains(id, "PodSecurityPolicy") {
			t.Errorf("%s could not be fully decoded", id)
		}
	}
	t.Logf("%d of %d kinds decode fully", len(idx.Kinds())-len(undecoded), len(idx.Kinds()))
}

func TestListFiltersAndPages(t *testing.T) {
	idx := loadSample(t).Index()

	all, err := idx.List(Query{KindID: "v1/Pod", Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total < 20 {
		t.Fatalf("only %d pods, expected many more", all.Total)
	}

	// Paging must not change the total and must not repeat or skip a row.
	page1, _ := idx.List(Query{KindID: "v1/Pod", Sort: "name", Limit: 10, Offset: 0})
	page2, _ := idx.List(Query{KindID: "v1/Pod", Sort: "name", Limit: 10, Offset: 10})
	if page1.Total != all.Total || page2.Total != all.Total {
		t.Errorf("totals differ across pages: %d, %d, %d", all.Total, page1.Total, page2.Total)
	}
	if len(page1.Items) != 10 || len(page2.Items) != 10 {
		t.Fatalf("page sizes = %d, %d, want 10 each", len(page1.Items), len(page2.Items))
	}
	for i := range page1.Items {
		if page1.Items[i].ID != all.Items[i].ID {
			t.Errorf("row %d of page 1 does not match the unpaged list", i)
		}
		if page2.Items[i].ID != all.Items[10+i].ID {
			t.Errorf("row %d of page 2 does not match the unpaged list", i)
		}
	}

	// An offset past the end must return an empty page, not an error.
	beyond, err := idx.List(Query{KindID: "v1/Pod", Offset: all.Total + 500, Limit: 10})
	if err != nil {
		t.Fatalf("offset past the end: %v", err)
	}
	if len(beyond.Items) != 0 || beyond.Total != all.Total {
		t.Errorf("offset past the end returned %d items, total %d", len(beyond.Items), beyond.Total)
	}

	// A namespace filter must restrict the rows and the total together.
	ns, err := idx.List(Query{KindID: "v1/Pod", Namespace: "openshift-etcd", Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Total == 0 || ns.Total >= all.Total {
		t.Errorf("namespace filter gave %d of %d pods", ns.Total, all.Total)
	}
	for _, r := range ns.Items {
		if r.Namespace != "openshift-etcd" {
			t.Errorf("namespace filter leaked %s/%s", r.Namespace, r.Name)
		}
	}
}

func TestSearchMatchesNamesAndContents(t *testing.T) {
	idx := loadSample(t).Index()

	byName, err := idx.List(Query{KindID: "v1/ConfigMap", Search: "etcd-pod", Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	if byName.Total == 0 {
		t.Fatal("name search found nothing")
	}
	for _, r := range byName.Items {
		if !strings.Contains(strings.ToLower(r.Name), "etcd-pod") &&
			!strings.Contains(strings.ToLower(r.Namespace), "etcd-pod") {
			t.Errorf("name search returned an unrelated object %s/%s", r.Namespace, r.Name)
		}
	}

	// A content search must find strictly more, because it also matches
	// objects that merely mention the term.
	deep, err := idx.List(Query{KindID: "v1/ConfigMap", Search: "etcd-pod", Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if deep.Total < byName.Total {
		t.Errorf("content search found %d, fewer than the %d name matches", deep.Total, byName.Total)
	}

	// Searching is case insensitive.
	upper, _ := idx.List(Query{KindID: "v1/ConfigMap", Search: "ETCD-POD"})
	if upper.Total != byName.Total {
		t.Errorf("case changed the result count: %d vs %d", upper.Total, byName.Total)
	}
}

func TestGetAndValueRoundTrip(t *testing.T) {
	idx := loadSample(t).Index()

	res, err := idx.List(Query{KindID: "v1/Pod", Limit: 1, Sort: "name"})
	if err != nil || len(res.Items) == 0 {
		t.Fatalf("no pod to test with: %v", err)
	}
	want := res.Items[0]

	got, ok := idx.Get(want.ID)
	if !ok {
		t.Fatalf("Get(%q) missed an indexed object", want.ID)
	}
	if got.Key != want.Key {
		t.Errorf("Get returned %q, want %q", got.Key, want.Key)
	}
	if _, ok := idx.Get("bm90LWEtcmVhbC1pZA"); ok {
		t.Error("Get returned an object for an unknown id")
	}

	body, err := idx.Value(got)
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if len(body) != got.Size {
		t.Errorf("read %d bytes, index recorded %d", len(body), got.Size)
	}
}

func TestRelatedResolvesOwners(t *testing.T) {
	idx := loadSample(t).Index()

	// A pod created by a controller has an owner, which Related must resolve
	// through the owner reference UID.
	res, err := idx.List(Query{KindID: "v1/Pod", Namespace: "openshift-monitoring", Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for _, r := range res.Items {
		body, err := idx.Value(r)
		if err != nil {
			t.Fatal(err)
		}
		owners := kube.OwnerRefs(body)
		if len(owners) == 0 {
			continue
		}
		// The owner reference itself must be complete.
		for _, o := range owners {
			if o.Kind == "" || o.Name == "" || o.UID == "" {
				t.Errorf("incomplete owner reference on %s: %+v", r.Name, o)
			}
		}
		related := idx.Related(r, body)
		var foundOwner, foundNamespace bool
		for _, rel := range related {
			for _, o := range owners {
				if rel.UID == o.UID {
					foundOwner = true
				}
			}
			if rel.Kind == "Namespace" && rel.Name == r.Namespace {
				foundNamespace = true
			}
		}
		if !foundOwner {
			t.Errorf("Related(%s) did not include its owner %s/%s", r.Name, owners[0].Kind, owners[0].Name)
		}
		if !foundNamespace {
			t.Errorf("Related(%s) did not include its namespace", r.Name)
		}
		checked++
		if checked >= 3 {
			break
		}
	}
	if checked == 0 {
		t.Skip("no owned pods in this namespace")
	}
}

func TestArchiveReadsFiles(t *testing.T) {
	a := loadSample(t).Archive()
	if a == nil {
		t.Skip("no archive")
	}

	const known = "static-pod-resources/etcd-pod-58/configmaps/etcd-pod/pod.yaml"
	f, ok := a.Get(known)
	if !ok {
		t.Fatalf("%s was not found in the archive", known)
	}
	if !strings.Contains(string(f.Content), "kind: Pod") {
		t.Error("archive file content does not look like a pod manifest")
	}
	if f.Size != int64(len(f.Content)) {
		t.Errorf("size %d does not match content length %d", f.Size, len(f.Content))
	}
	if _, ok := a.Get("not/in/the/archive"); ok {
		t.Error("Get returned a file that is not in the archive")
	}
}
