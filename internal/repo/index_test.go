package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateIndex(t *testing.T) {
	pkgDir := t.TempDir()
	archiveDir := t.TempDir()

	createTestPackage(t, pkgDir)

	archivePath, err := PackageArchive(pkgDir, archiveDir, "")
	if nil != err {
		t.Fatalf("PackageArchive failed: %v", err)
	}

	_ = archivePath

	idx, err := GenerateIndex(archiveDir, "https://example.com/packages")
	if nil != err {
		t.Fatalf("GenerateIndex failed: %v", err)
	}

	if "v1" != idx.APIVersion {
		t.Errorf("expected APIVersion v1, got %s", idx.APIVersion)
	}

	entries, ok := idx.Entries["testpkg"]
	if !ok {
		t.Fatal("expected testpkg in index entries")
	}

	if 1 != len(entries) {
		t.Fatalf("expected 1 entry for testpkg, got %d", len(entries))
	}

	entry := entries[0]
	if "testpkg" != entry.Name {
		t.Errorf("expected name testpkg, got %s", entry.Name)
	}
	if "1.0.0" != entry.Version {
		t.Errorf("expected version 1.0.0, got %s", entry.Version)
	}
	if "" == entry.Digest {
		t.Error("expected non-empty digest")
	}
	if 0 == len(entry.URLs) {
		t.Error("expected at least one URL")
	}
	if "https://example.com/packages/testpkg-1.0.0.keramos.tgz" != entry.URLs[0] {
		t.Errorf("unexpected URL: %s", entry.URLs[0])
	}
}

func TestGenerateIndexNoBaseURL(t *testing.T) {
	pkgDir := t.TempDir()
	archiveDir := t.TempDir()

	createTestPackage(t, pkgDir)

	if _, err := PackageArchive(pkgDir, archiveDir, ""); nil != err {
		t.Fatalf("PackageArchive failed: %v", err)
	}

	idx, err := GenerateIndex(archiveDir, "")
	if nil != err {
		t.Fatalf("GenerateIndex failed: %v", err)
	}

	entries := idx.Entries["testpkg"]
	if "testpkg-1.0.0.keramos.tgz" != entries[0].URLs[0] {
		t.Errorf("expected relative URL, got %s", entries[0].URLs[0])
	}
}

func TestLoadAndSaveIndex(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.yaml")

	original := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"mypkg": {
				{
					Name:    "mypkg",
					Version: "1.0.0",
					Digest:  "abc123",
					URLs:    []string{"https://example.com/mypkg-1.0.0.keramos.tgz"},
					Created: time.Now().Truncate(time.Second),
				},
			},
		},
		Generated: time.Now().Truncate(time.Second),
	}

	if err := SaveIndex(original, indexPath); nil != err {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if nil != err {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	if "v1" != loaded.APIVersion {
		t.Errorf("expected APIVersion v1, got %s", loaded.APIVersion)
	}

	entries, ok := loaded.Entries["mypkg"]
	if !ok {
		t.Fatal("expected mypkg in loaded entries")
	}
	if 1 != len(entries) {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if "abc123" != entries[0].Digest {
		t.Errorf("expected digest abc123, got %s", entries[0].Digest)
	}
}

func TestLoadIndexNotFound(t *testing.T) {
	_, err := LoadIndex("/nonexistent/index.yaml")
	if nil == err {
		t.Error("expected error for nonexistent index file")
	}
}

func TestMergeIndex(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"pkg-a": {
				{Name: "pkg-a", Version: "1.0.0", Digest: "old-digest"},
				{Name: "pkg-a", Version: "0.9.0", Digest: "older-digest"},
			},
			"pkg-b": {
				{Name: "pkg-b", Version: "2.0.0", Digest: "b-digest"},
			},
		},
	}

	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"pkg-a": {
				{Name: "pkg-a", Version: "1.0.0", Digest: "new-digest"},
				{Name: "pkg-a", Version: "1.1.0", Digest: "newer-digest"},
			},
			"pkg-c": {
				{Name: "pkg-c", Version: "3.0.0", Digest: "c-digest"},
			},
		},
	}

	merged := MergeIndex(existing, update)

	// pkg-a should have 3 entries: 1.0.0 (updated), 0.9.0 (kept), 1.1.0 (new)
	pkgA := merged.Entries["pkg-a"]
	if 3 != len(pkgA) {
		t.Fatalf("expected 3 entries for pkg-a, got %d", len(pkgA))
	}

	// Check that 1.0.0 was updated with new digest
	foundUpdated := false
	for _, e := range pkgA {
		if "1.0.0" == e.Version && "new-digest" == e.Digest {
			foundUpdated = true
			break
		}
	}
	if !foundUpdated {
		t.Error("expected pkg-a 1.0.0 to be updated with new-digest")
	}

	// pkg-b should still exist
	if _, ok := merged.Entries["pkg-b"]; !ok {
		t.Error("expected pkg-b to still be in merged index")
	}

	// pkg-c should be added
	pkgC, ok := merged.Entries["pkg-c"]
	if !ok {
		t.Fatal("expected pkg-c in merged index")
	}
	if 1 != len(pkgC) {
		t.Errorf("expected 1 entry for pkg-c, got %d", len(pkgC))
	}
}

func TestGenerateIndexEmptyDir(t *testing.T) {
	dir := t.TempDir()

	idx, err := GenerateIndex(dir, "")
	if nil != err {
		t.Fatalf("GenerateIndex failed on empty dir: %v", err)
	}

	if 0 != len(idx.Entries) {
		t.Errorf("expected 0 entries for empty dir, got %d", len(idx.Entries))
	}
}

func TestGenerateIndexMultipleVersions(t *testing.T) {
	pkgDir := t.TempDir()
	archiveDir := t.TempDir()

	createTestPackage(t, pkgDir)

	// Create two versions
	if _, err := PackageArchive(pkgDir, archiveDir, "1.0.0"); nil != err {
		t.Fatalf("PackageArchive v1 failed: %v", err)
	}
	if _, err := PackageArchive(pkgDir, archiveDir, "2.0.0"); nil != err {
		t.Fatalf("PackageArchive v2 failed: %v", err)
	}

	idx, err := GenerateIndex(archiveDir, "")
	if nil != err {
		t.Fatalf("GenerateIndex failed: %v", err)
	}

	entries := idx.Entries["testpkg"]
	if 2 != len(entries) {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Should be sorted version descending
	if entries[0].Version < entries[1].Version {
		t.Error("expected entries sorted by version descending")
	}
}

func TestSaveIndexCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "index.yaml")

	idx := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
		Generated:  time.Now(),
	}

	// SaveIndex writes to a file — the parent directory must exist.
	// Create the parent directory manually since SaveIndex doesn't create parents.
	if err := os.MkdirAll(filepath.Dir(nested), 0755); nil != err {
		t.Fatal(err)
	}

	if err := SaveIndex(idx, nested); nil != err {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	if _, err := os.Stat(nested); nil != err {
		t.Errorf("expected index file at %s: %v", nested, err)
	}
}
