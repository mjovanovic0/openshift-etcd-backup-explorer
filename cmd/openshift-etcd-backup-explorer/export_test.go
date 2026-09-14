package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeSegment(t *testing.T) {
	cases := map[string]string{
		"simple":                    "simple",
		"sha256:0068fce7f4004bfbed": "sha256_0068fce7f4004bfbed",
		"a/b":                       "a_b",
		`we"ird*name?`:              "we_ird_name_",
		"":                          "unnamed",
	}
	for in, want := range cases {
		if got := safeSegment(in); got != want {
			t.Errorf("safeSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeSegmentNeverEscapesADirectory(t *testing.T) {
	// A crafted object name must not be able to write outside the target.
	for _, name := range []string{"../../etc/passwd", "..\\..\\windows", "a/../../b"} {
		got := safeSegment(name)
		if strings.ContainsAny(got, `/\`) {
			t.Errorf("safeSegment(%q) = %q, which still holds a separator", name, got)
		}
		if filepath.IsAbs(got) {
			t.Errorf("safeSegment(%q) produced an absolute path", name)
		}
	}
}

func TestCheckTargetFolder(t *testing.T) {
	// A folder that does not exist is created.
	fresh := filepath.Join(t.TempDir(), "new", "nested")
	if err := checkTargetFolder(fresh, false); err != nil {
		t.Fatalf("creating a fresh folder: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("folder was not created: %v", err)
	}

	// An empty folder is fine.
	empty := t.TempDir()
	if err := checkTargetFolder(empty, false); err != nil {
		t.Errorf("an empty folder should be accepted: %v", err)
	}

	// A folder holding files is refused unless forced, so an export cannot
	// quietly scatter manifests among someone's work.
	occupied := t.TempDir()
	if err := os.WriteFile(filepath.Join(occupied, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkTargetFolder(occupied, false); err == nil {
		t.Error("a folder with files should be refused without -force")
	}
	if err := checkTargetFolder(occupied, true); err != nil {
		t.Errorf("-force should allow it: %v", err)
	}
}
