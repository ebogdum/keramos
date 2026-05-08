package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/internal/pkg"
	"github.com/ebogdum/keramos/internal/repo"
	"gopkg.in/yaml.v3"
)

func TestLockFileFreshResolve(t *testing.T) {
	srv, srvURL := createDependencyTestRepo(t, "")
	_ = srv

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	projectDir := t.TempDir()
	meta := pkg.PackageMetadata{
		APIVersion: "keramos/v1",
		Name:       "lockfile-test",
		Version:    "0.1.0",
		Dependencies: []pkg.Dependency{
			{Name: "mid-a", Version: "^1.0.0", Repository: srvURL},
		},
	}
	writeMetadata(t, projectDir, &meta)

	// Verify no lock file exists initially
	lockPath := filepath.Join(projectDir, "keramos.lock")
	assertFileNotExists(t, lockPath)

	// Resolve
	result, err := repo.ResolveTree(meta.Dependencies)
	if nil != err {
		t.Fatalf("resolution failed: %v", err)
	}

	// Save lock file
	lf := buildTestLockFile(result)
	if err := repo.SaveLockFile(lf, projectDir); nil != err {
		t.Fatalf("failed to save lock file: %v", err)
	}

	// Verify lock file now exists
	assertFileExists(t, lockPath)

	// Verify lock file contents
	loaded, err := repo.LoadLockFile(projectDir)
	if nil != err {
		t.Fatalf("failed to load lock file: %v", err)
	}
	if nil == loaded {
		t.Fatal("expected non-nil lock file")
	}
	if 0 == len(loaded.Dependencies) {
		t.Fatal("expected at least one locked dependency")
	}

	// Verify all resolved deps are in the lock file
	lockedByName := make(map[string]*repo.LockedDependency, len(loaded.Dependencies))
	for i := range loaded.Dependencies {
		lockedByName[loaded.Dependencies[i].Name] = &loaded.Dependencies[i]
	}

	for _, rd := range result.Resolved {
		ld, ok := lockedByName[rd.Name]
		if !ok {
			t.Errorf("resolved dep %s not found in lock file", rd.Name)
			continue
		}
		if rd.Version != ld.Version {
			t.Errorf("lock file version mismatch for %s: resolved=%s, locked=%s", rd.Name, rd.Version, ld.Version)
		}
	}
}

func TestLockFileLockedResolve(t *testing.T) {
	// Create a lock file and verify IsLockFileCurrent returns true
	lf := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "mid-a", Version: "1.1.0", Repository: "https://example.com", Digest: "abc123"},
			{Name: "leaf-b", Version: "1.0.0", Repository: "https://example.com", Digest: "def456"},
		},
	}

	deps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: "https://example.com"},
	}

	if !repo.IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be current with matching constraints")
	}

	// Save and reload to verify persistence
	projectDir := t.TempDir()
	if err := repo.SaveLockFile(lf, projectDir); nil != err {
		t.Fatalf("failed to save lock file: %v", err)
	}

	loaded, err := repo.LoadLockFile(projectDir)
	if nil != err {
		t.Fatalf("failed to load lock file: %v", err)
	}

	if !repo.IsLockFileCurrent(loaded, deps) {
		t.Error("expected reloaded lock file to still be current")
	}
}

func TestLockFileStaleness(t *testing.T) {
	lf := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "mid-a", Version: "1.1.0", Repository: "https://example.com"},
		},
	}

	// Original constraint satisfied
	currentDeps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: "https://example.com"},
	}
	if !repo.IsLockFileCurrent(lf, currentDeps) {
		t.Error("expected lock file to be current with ^1.0.0")
	}

	// Change constraint to require 2.x — lock is now stale
	staleDeps := []pkg.Dependency{
		{Name: "mid-a", Version: "^2.0.0", Repository: "https://example.com"},
	}
	if repo.IsLockFileCurrent(lf, staleDeps) {
		t.Error("expected lock file to be stale with ^2.0.0")
	}

	// Add a new dependency not in the lock file — lock is stale
	newDeps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: "https://example.com"},
		{Name: "new-dep", Version: "^1.0.0", Repository: "https://example.com"},
	}
	if repo.IsLockFileCurrent(lf, newDeps) {
		t.Error("expected lock file to be stale when new dependency added")
	}
}

func TestLockFileUpdateSingle(t *testing.T) {
	// Verify mergeLockFile preserves unrelated entries
	existingLock := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "mid-a", Version: "1.0.0", Repository: "https://example.com", Digest: "aaa"},
			{Name: "leaf-b", Version: "1.0.0", Repository: "https://example.com", Digest: "bbb"},
			{Name: "unrelated", Version: "3.0.0", Repository: "https://example.com", Digest: "ccc"},
		},
	}

	// Save the lock file
	projectDir := t.TempDir()
	if err := repo.SaveLockFile(existingLock, projectDir); nil != err {
		t.Fatalf("failed to save lock file: %v", err)
	}

	// Load it back
	loaded, err := repo.LoadLockFile(projectDir)
	if nil != err {
		t.Fatalf("failed to load lock file: %v", err)
	}

	// Verify all three entries are present
	if 3 != len(loaded.Dependencies) {
		t.Fatalf("expected 3 locked dependencies, got %d", len(loaded.Dependencies))
	}

	lockedByName := make(map[string]*repo.LockedDependency, len(loaded.Dependencies))
	for i := range loaded.Dependencies {
		lockedByName[loaded.Dependencies[i].Name] = &loaded.Dependencies[i]
	}

	// Verify unrelated dependency is preserved
	unrelated, ok := lockedByName["unrelated"]
	if !ok {
		t.Fatal("expected 'unrelated' in lock file")
	}
	if "3.0.0" != unrelated.Version {
		t.Errorf("expected unrelated version 3.0.0, got %s", unrelated.Version)
	}
}

func TestLockFileDigestVerification(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test package
	archivePath := createTestPackage(t, tmpDir, "verified-pkg", "1.0.0", nil)
	correctDigest := computeFileDigest(t, archivePath)

	// Verify with correct digest succeeds
	if err := repo.VerifyDigest(archivePath, correctDigest); nil != err {
		t.Fatalf("expected digest verification to pass: %v", err)
	}

	// Create another archive to tamper with
	tamperedArchivePath := createTestPackage(t, tmpDir, "tampered-pkg", "1.0.0", nil)

	// Verify with wrong digest fails
	wrongDigest := "0000000000000000000000000000000000000000000000000000000000000000"
	err := repo.VerifyDigest(tamperedArchivePath, wrongDigest)
	if nil == err {
		t.Fatal("expected digest verification to fail with wrong digest")
	}

	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected 'mismatch' in error, got: %v", err)
	}

	// The tampered file should have been deleted
	assertFileNotExists(t, tamperedArchivePath)
}

func TestLockFileMissingDependency(t *testing.T) {
	// Lock file with no entry for a declared dependency
	lf := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "existing", Version: "1.0.0"},
		},
	}

	deps := []pkg.Dependency{
		{Name: "existing", Version: "^1.0.0"},
		{Name: "missing", Version: "^1.0.0"},
	}

	if repo.IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be stale when a dependency is missing from it")
	}
}

func TestLockFileEmptyDependencies(t *testing.T) {
	lf := &repo.LockFile{
		APIVersion:   "v1",
		Dependencies: []repo.LockedDependency{},
	}

	// No declared dependencies — lock file is current
	if !repo.IsLockFileCurrent(lf, nil) {
		t.Error("expected empty lock file to be current when no deps declared")
	}

	if !repo.IsLockFileCurrent(lf, []pkg.Dependency{}) {
		t.Error("expected empty lock file to be current when empty deps declared")
	}
}

func TestLockFileNil(t *testing.T) {
	// Nil lock file is never current
	deps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0"},
	}

	if repo.IsLockFileCurrent(nil, deps) {
		t.Error("expected nil lock file to be stale")
	}
}

func TestLockFileRoundTrip(t *testing.T) {
	projectDir := t.TempDir()

	original := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{
				Name:         "app-a",
				Version:      "1.2.3",
				Repository:   "https://example.com/repo",
				Digest:       "sha256abc",
				Dependencies: []string{"lib-b", "lib-c"},
			},
			{
				Name:       "lib-b",
				Version:    "2.0.0",
				Repository: "https://example.com/repo",
				Digest:     "sha256def",
			},
		},
	}

	if err := repo.SaveLockFile(original, projectDir); nil != err {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := repo.LoadLockFile(projectDir)
	if nil != err {
		t.Fatalf("failed to load: %v", err)
	}

	if len(original.Dependencies) != len(loaded.Dependencies) {
		t.Fatalf("dependency count mismatch: original=%d, loaded=%d",
			len(original.Dependencies), len(loaded.Dependencies))
	}

	for i, orig := range original.Dependencies {
		got := loaded.Dependencies[i]
		if orig.Name != got.Name {
			t.Errorf("dep[%d] name: expected %s, got %s", i, orig.Name, got.Name)
		}
		if orig.Version != got.Version {
			t.Errorf("dep[%d] version: expected %s, got %s", i, orig.Version, got.Version)
		}
		if orig.Repository != got.Repository {
			t.Errorf("dep[%d] repository: expected %s, got %s", i, orig.Repository, got.Repository)
		}
		if orig.Digest != got.Digest {
			t.Errorf("dep[%d] digest: expected %s, got %s", i, orig.Digest, got.Digest)
		}
	}

	// Verify first dep's transitive dependencies
	if 2 != len(loaded.Dependencies[0].Dependencies) {
		t.Errorf("expected 2 transitive deps for app-a, got %d", len(loaded.Dependencies[0].Dependencies))
	}
}

func TestLockFileNonexistentDir(t *testing.T) {
	// LoadLockFile from a non-existent directory should return nil, nil
	lf, err := repo.LoadLockFile("/nonexistent/path/that/does/not/exist")
	if nil != err {
		t.Fatalf("expected no error for non-existent lock file, got: %v", err)
	}
	if nil != lf {
		t.Error("expected nil lock file for non-existent path")
	}
}

func TestLockFileInvalidYAML(t *testing.T) {
	projectDir := t.TempDir()

	// Write invalid YAML
	lockPath := filepath.Join(projectDir, "keramos.lock")
	if err := os.WriteFile(lockPath, []byte("{{invalid yaml"), 0644); nil != err {
		t.Fatalf("failed to write invalid lock file: %v", err)
	}

	_, err := repo.LoadLockFile(projectDir)
	if nil == err {
		t.Error("expected error when loading invalid YAML lock file")
	}
}

func TestLockFileAPIVersion(t *testing.T) {
	projectDir := t.TempDir()

	lf := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "test", Version: "1.0.0"},
		},
	}

	if err := repo.SaveLockFile(lf, projectDir); nil != err {
		t.Fatalf("failed to save: %v", err)
	}

	// Read raw YAML to verify apiVersion field
	data := readYAML(t, filepath.Join(projectDir, "keramos.lock"))
	apiVersion, ok := data["apiVersion"].(string)
	if !ok {
		t.Fatal("expected apiVersion string in lock file")
	}
	if "v1" != apiVersion {
		t.Errorf("expected apiVersion 'v1', got %q", apiVersion)
	}
}

func TestLockFileConstraintSatisfaction(t *testing.T) {
	tests := []struct {
		name       string
		locked     string
		constraint string
		current    bool
	}{
		{"exact match", "1.0.0", "1.0.0", true},
		{"caret range match", "1.5.0", "^1.0.0", true},
		{"caret range too low", "0.9.0", "^1.0.0", false},
		{"caret range major bump", "2.0.0", "^1.0.0", false},
		{"tilde match", "1.0.5", "~1.0.0", true},
		{"tilde too high", "1.1.0", "~1.0.0", false},
		{"greater than", "2.0.0", ">=1.0.0", true},
		{"greater than not met", "0.5.0", ">=1.0.0", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lf := &repo.LockFile{
				APIVersion: "v1",
				Dependencies: []repo.LockedDependency{
					{Name: "test-pkg", Version: tc.locked},
				},
			}

			deps := []pkg.Dependency{
				{Name: "test-pkg", Version: tc.constraint},
			}

			result := repo.IsLockFileCurrent(lf, deps)
			if tc.current != result {
				t.Errorf("constraint %s with locked %s: expected current=%v, got %v",
					tc.constraint, tc.locked, tc.current, result)
			}
		})
	}
}

// unused import guard
var _ = yaml.Marshal
