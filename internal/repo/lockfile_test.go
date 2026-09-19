package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ebogdum/keramos/v3/internal/pkg"
)

func TestLockFileSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{
		APIVersion: "v1",
		Dependencies: []LockedDependency{
			{
				Name:         "redis",
				Version:      "5.2.1",
				Repository:   "https://example.com/repo",
				Digest:       "abc123",
				Dependencies: []string{"common"},
			},
			{
				Name:       "common",
				Version:    "2.1.0",
				Repository: "https://example.com/repo",
				Digest:     "def456",
			},
		},
	}

	if err := SaveLockFile(lf, dir); nil != err {
		t.Fatalf("SaveLockFile failed: %v", err)
	}

	// Verify file exists
	lockPath := filepath.Join(dir, "keramos.lock")
	if _, err := os.Stat(lockPath); nil != err {
		t.Fatalf("keramos.lock not created: %v", err)
	}

	loaded, err := LoadLockFile(dir)
	if nil != err {
		t.Fatalf("LoadLockFile failed: %v", err)
	}

	if nil == loaded {
		t.Fatal("LoadLockFile returned nil")
	}

	if "v1" != loaded.APIVersion {
		t.Errorf("expected apiVersion v1, got %s", loaded.APIVersion)
	}

	if 2 != len(loaded.Dependencies) {
		t.Fatalf("expected 2 dependencies, got %d", len(loaded.Dependencies))
	}

	redis := loaded.Dependencies[0]
	if "redis" != redis.Name {
		t.Errorf("expected name redis, got %s", redis.Name)
	}
	if "5.2.1" != redis.Version {
		t.Errorf("expected version 5.2.1, got %s", redis.Version)
	}
	if "abc123" != redis.Digest {
		t.Errorf("expected digest abc123, got %s", redis.Digest)
	}
	if 1 != len(redis.Dependencies) {
		t.Fatalf("expected 1 sub-dependency, got %d", len(redis.Dependencies))
	}
	if "common" != redis.Dependencies[0] {
		t.Errorf("expected sub-dep common, got %s", redis.Dependencies[0])
	}
}

func TestLoadLockFileMissing(t *testing.T) {
	dir := t.TempDir()

	lf, err := LoadLockFile(dir)
	if nil != err {
		t.Fatalf("LoadLockFile failed for missing file: %v", err)
	}

	if nil != lf {
		t.Errorf("expected nil for missing lock file, got %+v", lf)
	}
}

func TestIsLockFileCurrentAllSatisfied(t *testing.T) {
	lf := &LockFile{
		APIVersion: "v1",
		Dependencies: []LockedDependency{
			{Name: "redis", Version: "5.2.1", Repository: "https://example.com/repo"},
			{Name: "auth", Version: "3.0.0", Repository: "https://example.com/repo"},
		},
	}

	deps := []pkg.Dependency{
		{Name: "redis", Version: "^5.0.0", Repository: "https://example.com/repo"},
		{Name: "auth", Version: ">=3.0.0", Repository: "https://example.com/repo"},
	}

	if !IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be current")
	}
}

func TestIsLockFileCurrentStale(t *testing.T) {
	lf := &LockFile{
		APIVersion: "v1",
		Dependencies: []LockedDependency{
			{Name: "redis", Version: "4.0.0", Repository: "https://example.com/repo"},
		},
	}

	deps := []pkg.Dependency{
		{Name: "redis", Version: "^5.0.0", Repository: "https://example.com/repo"},
	}

	if IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be stale (4.0.0 does not satisfy ^5.0.0)")
	}
}

func TestIsLockFileCurrentMissingDep(t *testing.T) {
	lf := &LockFile{
		APIVersion: "v1",
		Dependencies: []LockedDependency{
			{Name: "redis", Version: "5.0.0", Repository: "https://example.com/repo"},
		},
	}

	deps := []pkg.Dependency{
		{Name: "redis", Version: "^5.0.0", Repository: "https://example.com/repo"},
		{Name: "auth", Version: "^1.0.0", Repository: "https://example.com/repo"},
	}

	if IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be stale (auth not locked)")
	}
}

func TestIsLockFileCurrentNilLockFile(t *testing.T) {
	deps := []pkg.Dependency{
		{Name: "redis", Version: "^5.0.0", Repository: "https://example.com/repo"},
	}

	if IsLockFileCurrent(nil, deps) {
		t.Error("expected nil lock file to be not current")
	}
}

func TestBuildLockFile(t *testing.T) {
	result := &ResolutionResult{
		Resolved: []ResolvedDep{
			{
				Name:         "lib-a",
				Version:      "1.2.0",
				Repository:   "https://example.com",
				Digest:       "aaa",
				Dependencies: []string{"lib-b"},
			},
			{
				Name:       "lib-b",
				Version:    "2.0.0",
				Repository: "https://example.com",
				Digest:     "bbb",
			},
		},
	}

	lf := buildLockFile(result)

	if "v1" != lf.APIVersion {
		t.Errorf("expected apiVersion v1, got %s", lf.APIVersion)
	}
	if 2 != len(lf.Dependencies) {
		t.Fatalf("expected 2 locked deps, got %d", len(lf.Dependencies))
	}
	if "lib-a" != lf.Dependencies[0].Name {
		t.Errorf("expected first dep lib-a, got %s", lf.Dependencies[0].Name)
	}
	if "aaa" != lf.Dependencies[0].Digest {
		t.Errorf("expected digest aaa, got %s", lf.Dependencies[0].Digest)
	}
}
