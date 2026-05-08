package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListDependenciesNoDeps(t *testing.T) {
	dir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: nodeps
version: 1.0.0
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}

	deps, err := ListDependencies(dir)
	if nil != err {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	if 0 != len(deps) {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}
}

func TestListDependenciesWithDeps(t *testing.T) {
	dir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: withdeps
version: 1.0.0
dependencies:
  - name: redis
    version: ">=1.0.0"
    repository: https://example.com/repo
  - name: postgres
    version: "^2.0.0"
    repository: https://example.com/repo
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}

	deps, err := ListDependencies(dir)
	if nil != err {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	if 2 != len(deps) {
		t.Fatalf("expected 2 dependencies, got %d", len(deps))
	}

	// Both should be missing since packages/ doesn't exist
	for _, d := range deps {
		if "missing" != d.Status {
			t.Errorf("expected status missing for %s, got %s", d.Name, d.Status)
		}
		if "" != d.Installed {
			t.Errorf("expected empty installed version for %s, got %s", d.Name, d.Installed)
		}
	}
}

func TestListDependenciesInstalledOk(t *testing.T) {
	dir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: withdeps
version: 1.0.0
dependencies:
  - name: redis
    version: ">=1.0.0"
    repository: https://example.com/repo
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}

	// Create an installed dependency
	redisDir := filepath.Join(dir, "packages", "redis", "redis")
	if err := os.MkdirAll(redisDir, 0755); nil != err {
		t.Fatal(err)
	}

	redisYAML := `apiVersion: v1
name: redis
version: 1.5.0
`
	if err := os.WriteFile(filepath.Join(redisDir, "keramos.yaml"), []byte(redisYAML), 0644); nil != err {
		t.Fatal(err)
	}

	deps, err := ListDependencies(dir)
	if nil != err {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	if 1 != len(deps) {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}

	d := deps[0]
	if "redis" != d.Name {
		t.Errorf("expected name redis, got %s", d.Name)
	}
	if "1.5.0" != d.Installed {
		t.Errorf("expected installed 1.5.0, got %s", d.Installed)
	}
	if "ok" != d.Status {
		t.Errorf("expected status ok, got %s", d.Status)
	}
}

func TestListDependenciesOutdated(t *testing.T) {
	dir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: withdeps
version: 1.0.0
dependencies:
  - name: redis
    version: ">=2.0.0"
    repository: https://example.com/repo
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}

	// Create an installed dependency that doesn't match the constraint
	redisDir := filepath.Join(dir, "packages", "redis", "redis")
	if err := os.MkdirAll(redisDir, 0755); nil != err {
		t.Fatal(err)
	}

	redisYAML := `apiVersion: v1
name: redis
version: 1.5.0
`
	if err := os.WriteFile(filepath.Join(redisDir, "keramos.yaml"), []byte(redisYAML), 0644); nil != err {
		t.Fatal(err)
	}

	deps, err := ListDependencies(dir)
	if nil != err {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	if 1 != len(deps) {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}

	d := deps[0]
	if "outdated" != d.Status {
		t.Errorf("expected status outdated, got %s", d.Status)
	}
}

func TestDetermineDepStatus(t *testing.T) {
	tests := []struct {
		constraint string
		installed  string
		expected   string
	}{
		{">=1.0.0", "1.5.0", "ok"},
		{">=2.0.0", "1.5.0", "outdated"},
		{"^1.0.0", "1.9.9", "ok"},
		{"^1.0.0", "2.0.0", "outdated"},
		{"invalid", "1.0.0", "ok"},   // invalid constraint => default ok
		{">=1.0.0", "invalid", "ok"}, // invalid version => default ok
	}

	for _, tc := range tests {
		result := determineDepStatus(tc.constraint, tc.installed)
		if tc.expected != result {
			t.Errorf("determineDepStatus(%q, %q) = %q, want %q", tc.constraint, tc.installed, result, tc.expected)
		}
	}
}
