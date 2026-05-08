package repo

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ebogdum/keramos/internal/pkg"
)

// setupTestRepoServer creates a test HTTP server serving a repository with
// the given packages. Each package is a map entry: name -> []version info.
// Returns the server URL and a cleanup function.
func setupTestRepoServer(t *testing.T, packages map[string][]testPackageVersion) (*httptest.Server, func()) {
	t.Helper()

	repoDir := t.TempDir()

	for name, versions := range packages {
		for _, ver := range versions {
			pkgDir := filepath.Join(repoDir, fmt.Sprintf("%s-%s", name, ver.version))
			if err := os.MkdirAll(pkgDir, 0755); nil != err {
				t.Fatal(err)
			}

			keramosYAML := fmt.Sprintf("apiVersion: v1\nname: %s\nversion: %s\n", name, ver.version)
			if 0 < len(ver.deps) {
				keramosYAML += "dependencies:\n"
				for _, d := range ver.deps {
					keramosYAML += fmt.Sprintf("  - name: %s\n    version: %q\n    repository: REPO_URL\n", d.name, d.version)
				}
			}

			if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
				t.Fatal(err)
			}

			archivePath, err := PackageArchive(pkgDir, repoDir, "")
			if nil != err {
				t.Fatal(err)
			}
			_ = archivePath
		}
	}

	idx, err := GenerateIndex(repoDir, "")
	if nil != err {
		t.Fatal(err)
	}

	if err := SaveIndex(idx, filepath.Join(repoDir, "index.yaml")); nil != err {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(repoDir)))

	// Patch keramos.yaml files to use the actual server URL in dependency repository fields
	// We need to regenerate archives with correct URLs
	for name, versions := range packages {
		for _, ver := range versions {
			if 0 == len(ver.deps) {
				continue
			}

			pkgDir := filepath.Join(repoDir, fmt.Sprintf("%s-%s", name, ver.version))
			keramosYAML := fmt.Sprintf("apiVersion: v1\nname: %s\nversion: %s\n", name, ver.version)
			keramosYAML += "dependencies:\n"
			for _, d := range ver.deps {
				keramosYAML += fmt.Sprintf("  - name: %s\n    version: %q\n    repository: %s\n", d.name, d.version, srv.URL)
			}

			if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
				t.Fatal(err)
			}

			// Remove old archive and recreate
			oldArchive := filepath.Join(repoDir, fmt.Sprintf("%s-%s.keramos.tgz", name, ver.version))
			os.Remove(oldArchive)

			if _, err := PackageArchive(pkgDir, repoDir, ""); nil != err {
				t.Fatal(err)
			}
		}
	}

	// Regenerate index with updated archives
	idx, err = GenerateIndex(repoDir, "")
	if nil != err {
		t.Fatal(err)
	}
	if err := SaveIndex(idx, filepath.Join(repoDir, "index.yaml")); nil != err {
		t.Fatal(err)
	}

	// Set up an authenticated client that doesn't need credentials
	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client, clientErr := NewAuthenticatedClient(store)
	if nil != clientErr {
		t.Fatal(clientErr)
	}
	SetDefaultClient(client)

	cleanup := func() {
		srv.Close()
		SetDefaultClient(nil)
	}

	return srv, cleanup
}

type testPackageVersion struct {
	version string
	deps    []testDep
}

type testDep struct {
	name    string
	version string
}

func TestResolveTreeDirectDeps(t *testing.T) {
	srv, cleanup := setupTestRepoServer(t, map[string][]testPackageVersion{
		"lib-a": {{version: "1.0.0"}, {version: "1.2.0"}},
		"lib-b": {{version: "2.0.0"}, {version: "2.1.0"}},
	})
	defer cleanup()

	deps := []pkg.Dependency{
		{Name: "lib-a", Version: "^1.0.0", Repository: srv.URL},
		{Name: "lib-b", Version: ">=2.0.0", Repository: srv.URL},
	}

	result, err := ResolveTree(deps)
	if nil != err {
		t.Fatalf("ResolveTree failed: %v", err)
	}

	if 2 != len(result.Resolved) {
		t.Fatalf("expected 2 resolved deps, got %d", len(result.Resolved))
	}

	resolved := make(map[string]string, len(result.Resolved))
	for _, rd := range result.Resolved {
		resolved[rd.Name] = rd.Version
	}

	if "1.2.0" != resolved["lib-a"] {
		t.Errorf("expected lib-a@1.2.0, got %s", resolved["lib-a"])
	}
	if "2.1.0" != resolved["lib-b"] {
		t.Errorf("expected lib-b@2.1.0, got %s", resolved["lib-b"])
	}
}

func TestResolveTreeTransitiveDeps(t *testing.T) {
	srv, cleanup := setupTestRepoServer(t, map[string][]testPackageVersion{
		"lib-a": {{
			version: "1.0.0",
			deps:    []testDep{{name: "lib-b", version: "^2.0.0"}},
		}},
		"lib-b": {{version: "2.0.0"}, {version: "2.3.0"}},
	})
	defer cleanup()

	deps := []pkg.Dependency{
		{Name: "lib-a", Version: "^1.0.0", Repository: srv.URL},
	}

	result, err := ResolveTree(deps)
	if nil != err {
		t.Fatalf("ResolveTree failed: %v", err)
	}

	if 2 != len(result.Resolved) {
		t.Fatalf("expected 2 resolved deps (lib-a + lib-b), got %d", len(result.Resolved))
	}

	resolved := make(map[string]string, len(result.Resolved))
	for _, rd := range result.Resolved {
		resolved[rd.Name] = rd.Version
	}

	if "1.0.0" != resolved["lib-a"] {
		t.Errorf("expected lib-a@1.0.0, got %s", resolved["lib-a"])
	}
	if "2.3.0" != resolved["lib-b"] {
		t.Errorf("expected lib-b@2.3.0 (highest matching ^2.0.0), got %s", resolved["lib-b"])
	}
}

func TestResolveTreeCycleDetection(t *testing.T) {
	// Create packages that form a cycle: lib-a -> lib-b -> lib-a
	srv, cleanup := setupTestRepoServer(t, map[string][]testPackageVersion{
		"lib-a": {{
			version: "1.0.0",
			deps:    []testDep{{name: "lib-b", version: "^1.0.0"}},
		}},
		"lib-b": {{
			version: "1.0.0",
			deps:    []testDep{{name: "lib-a", version: "^1.0.0"}},
		}},
	})
	defer cleanup()

	deps := []pkg.Dependency{
		{Name: "lib-a", Version: "^1.0.0", Repository: srv.URL},
	}

	_, err := ResolveTree(deps)
	if nil == err {
		t.Fatal("expected cycle detection error, got nil")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "cycle") {
		t.Errorf("expected error mentioning cycle, got: %s", errMsg)
	}
}

func TestResolveTreeVersionSelection(t *testing.T) {
	// Both lib-a and lib-b depend on common — should pick highest compatible version
	srv, cleanup := setupTestRepoServer(t, map[string][]testPackageVersion{
		"lib-a": {{
			version: "1.0.0",
			deps:    []testDep{{name: "common", version: "^2.0.0"}},
		}},
		"lib-b": {{
			version: "1.0.0",
			deps:    []testDep{{name: "common", version: ">=2.1.0"}},
		}},
		"common": {
			{version: "2.0.0"},
			{version: "2.1.0"},
			{version: "2.5.0"},
		},
	})
	defer cleanup()

	deps := []pkg.Dependency{
		{Name: "lib-a", Version: "^1.0.0", Repository: srv.URL},
		{Name: "lib-b", Version: "^1.0.0", Repository: srv.URL},
	}

	result, err := ResolveTree(deps)
	if nil != err {
		t.Fatalf("ResolveTree failed: %v", err)
	}

	resolved := make(map[string]string, len(result.Resolved))
	for _, rd := range result.Resolved {
		resolved[rd.Name] = rd.Version
	}

	// common should be 2.5.0 — highest satisfying both ^2.0.0 and >=2.1.0
	if "2.5.0" != resolved["common"] {
		t.Errorf("expected common@2.5.0, got %s", resolved["common"])
	}
}

func TestResolveTreeConflictDetection(t *testing.T) {
	// lib-a requires common ^1.0.0, lib-b requires common ^2.0.0 — conflict
	srv, cleanup := setupTestRepoServer(t, map[string][]testPackageVersion{
		"lib-a": {{
			version: "1.0.0",
			deps:    []testDep{{name: "common", version: "^1.0.0"}},
		}},
		"lib-b": {{
			version: "1.0.0",
			deps:    []testDep{{name: "common", version: "^2.0.0"}},
		}},
		"common": {
			{version: "1.0.0"},
			{version: "1.5.0"},
			{version: "2.0.0"},
			{version: "2.5.0"},
		},
	})
	defer cleanup()

	deps := []pkg.Dependency{
		{Name: "lib-a", Version: "^1.0.0", Repository: srv.URL},
		{Name: "lib-b", Version: "^1.0.0", Repository: srv.URL},
	}

	_, err := ResolveTree(deps)
	if nil == err {
		t.Fatal("expected conflict error, got nil")
	}

	errMsg := err.Error()
	if !containsSubstring(errMsg, "conflict") && !containsSubstring(errMsg, "CONFLICT") && !containsSubstring(errMsg, "no version") {
		t.Errorf("expected error mentioning conflict, got: %s", errMsg)
	}
}

func TestExtractMetadataFromArchive(t *testing.T) {
	dir := t.TempDir()

	pkgDir := filepath.Join(dir, "test-pkg")
	if err := os.MkdirAll(pkgDir, 0755); nil != err {
		t.Fatal(err)
	}

	keramosYAML := `apiVersion: v1
name: test-pkg
version: 3.2.1
description: A test package
dependencies:
  - name: dep-a
    version: "^1.0.0"
    repository: https://example.com/repo
`
	if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}

	archivePath, err := PackageArchive(pkgDir, dir, "")
	if nil != err {
		t.Fatal(err)
	}

	meta, err := extractMetadataFromArchive(archivePath)
	if nil != err {
		t.Fatalf("extractMetadataFromArchive failed: %v", err)
	}

	if "test-pkg" != meta.Name {
		t.Errorf("expected name test-pkg, got %s", meta.Name)
	}
	if "3.2.1" != meta.Version {
		t.Errorf("expected version 3.2.1, got %s", meta.Version)
	}
	if 1 != len(meta.Dependencies) {
		t.Fatalf("expected 1 dependency, got %d", len(meta.Dependencies))
	}
	if "dep-a" != meta.Dependencies[0].Name {
		t.Errorf("expected dep name dep-a, got %s", meta.Dependencies[0].Name)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	sLen := len(s)
	subLen := len(substr)
	for i := 0; i <= sLen-subLen; i++ {
		if s[i:i+subLen] == substr {
			return true
		}
	}
	return false
}
