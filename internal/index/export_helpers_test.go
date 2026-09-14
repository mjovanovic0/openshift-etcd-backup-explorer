package index

import "testing"

// TestQueryIsStableForExport checks the property the export relies on: asking
// for every match returns the same set the paged list walks through.
func TestQueryIsStableForExport(t *testing.T) {
	idx := loadSample(t).Index()

	q := Query{KindID: "v1/PersistentVolume", Sort: "namespace"}
	all, err := idx.List(q)
	if err != nil {
		t.Fatal(err)
	}
	if all.Total == 0 {
		t.Skip("no persistent volumes in the sample")
	}
	if len(all.Items) != all.Total {
		t.Fatalf("an unlimited query returned %d of %d items", len(all.Items), all.Total)
	}

	// Walking the same query in pages must visit exactly the same objects.
	seen := map[string]bool{}
	for offset := 0; offset < all.Total; offset += 25 {
		page, err := idx.List(Query{KindID: q.KindID, Sort: q.Sort, Offset: offset, Limit: 25})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page.Items {
			if seen[r.ID] {
				t.Errorf("paging visited %s twice", r.Name)
			}
			seen[r.ID] = true
		}
	}
	if len(seen) != all.Total {
		t.Errorf("paging visited %d objects, the unpaged query returned %d", len(seen), all.Total)
	}
}
