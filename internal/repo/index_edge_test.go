package repo

import (
	"os"
	"testing"
	"time"
)

func TestMergeIndexOverlappingVersions(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"app": {
				{Name: "app", Version: "1.0.0", Digest: "old-digest-1"},
				{Name: "app", Version: "2.0.0", Digest: "old-digest-2"},
			},
		},
	}

	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"app": {
				{Name: "app", Version: "2.0.0", Digest: "new-digest-2"},
				{Name: "app", Version: "3.0.0", Digest: "new-digest-3"},
			},
		},
	}

	merged := MergeIndex(existing, update)
	appEntries := merged.Entries["app"]

	if 3 != len(appEntries) {
		t.Fatalf("expected 3 entries (1.0.0, 2.0.0 updated, 3.0.0 new), got %d", len(appEntries))
	}

	// Verify 2.0.0 was updated
	for _, e := range appEntries {
		if "2.0.0" == e.Version && "new-digest-2" != e.Digest {
			t.Errorf("expected 2.0.0 to have new-digest-2, got %s", e.Digest)
		}
	}

	// Verify 1.0.0 preserved
	found := false
	for _, e := range appEntries {
		if "1.0.0" == e.Version && "old-digest-1" == e.Digest {
			found = true
		}
	}
	if !found {
		t.Error("expected 1.0.0 with old-digest-1 to be preserved")
	}
}

func TestMergeIndexEmptyExisting(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
	}

	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"new-pkg": {
				{Name: "new-pkg", Version: "1.0.0", Digest: "abc"},
			},
		},
	}

	merged := MergeIndex(existing, update)
	if 1 != len(merged.Entries["new-pkg"]) {
		t.Errorf("expected 1 entry for new-pkg, got %d", len(merged.Entries["new-pkg"]))
	}
}

func TestMergeIndexEmptyUpdate(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"pkg": {
				{Name: "pkg", Version: "1.0.0", Digest: "abc"},
			},
		},
	}

	update := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
	}

	merged := MergeIndex(existing, update)
	if 1 != len(merged.Entries["pkg"]) {
		t.Errorf("expected 1 entry preserved, got %d", len(merged.Entries["pkg"]))
	}
}

func TestMergeIndexBothEmpty(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
	}
	update := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
	}

	merged := MergeIndex(existing, update)
	if 0 != len(merged.Entries) {
		t.Errorf("expected 0 entries, got %d", len(merged.Entries))
	}
}

func TestMergeIndexMultiplePackages(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"a": {{Name: "a", Version: "1.0.0", Digest: "a1"}},
			"b": {{Name: "b", Version: "1.0.0", Digest: "b1"}},
		},
	}

	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"b": {{Name: "b", Version: "2.0.0", Digest: "b2"}},
			"c": {{Name: "c", Version: "1.0.0", Digest: "c1"}},
		},
	}

	merged := MergeIndex(existing, update)

	// a: 1 entry preserved
	if 1 != len(merged.Entries["a"]) {
		t.Errorf("expected 1 entry for a, got %d", len(merged.Entries["a"]))
	}
	// b: 2 entries (1.0.0 + 2.0.0)
	if 2 != len(merged.Entries["b"]) {
		t.Errorf("expected 2 entries for b, got %d", len(merged.Entries["b"]))
	}
	// c: 1 new entry
	if 1 != len(merged.Entries["c"]) {
		t.Errorf("expected 1 entry for c, got %d", len(merged.Entries["c"]))
	}
}

func TestGenerateIndexNonexistentDir(t *testing.T) {
	_, err := GenerateIndex("/nonexistent/dir", "")
	if nil == err {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoadIndexInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := tmpDir + "/bad-index.yaml"
	if err := writeFile(badPath, []byte("{{invalid yaml[[")); nil != err {
		t.Fatal(err)
	}

	_, err := LoadIndex(badPath)
	if nil == err {
		t.Fatal("expected error for invalid YAML index")
	}
}

func TestSaveAndLoadIndexRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := tmpDir + "/index.yaml"

	now := time.Now().Truncate(time.Second)
	original := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"pkg-x": {
				{
					Name:    "pkg-x",
					Version: "1.0.0",
					Digest:  "sha256abc",
					URLs:    []string{"https://repo.example.com/pkg-x-1.0.0.keramos.tgz"},
					Created: now,
				},
				{
					Name:    "pkg-x",
					Version: "0.9.0",
					Digest:  "sha256def",
					URLs:    []string{"https://repo.example.com/pkg-x-0.9.0.keramos.tgz"},
					Created: now,
				},
			},
		},
		Generated: now,
	}

	if err := SaveIndex(original, indexPath); nil != err {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	loaded, err := LoadIndex(indexPath)
	if nil != err {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	entries := loaded.Entries["pkg-x"]
	if 2 != len(entries) {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	foundV1 := false
	foundV09 := false
	for _, e := range entries {
		if "1.0.0" == e.Version {
			foundV1 = true
		}
		if "0.9.0" == e.Version {
			foundV09 = true
		}
	}
	if !foundV1 || !foundV09 {
		t.Error("expected both versions to round-trip")
	}
}

func TestLoadIndexNilEntriesHandled(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := tmpDir + "/index.yaml"

	// Write an index with no entries key
	content := "apiVersion: v1\ngenerated: 2024-01-01T00:00:00Z\n"
	if err := writeFile(indexPath, []byte(content)); nil != err {
		t.Fatal(err)
	}

	loaded, err := LoadIndex(indexPath)
	if nil != err {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	if nil == loaded.Entries {
		t.Fatal("expected non-nil Entries map")
	}
	if 0 != len(loaded.Entries) {
		t.Errorf("expected 0 entries, got %d", len(loaded.Entries))
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
