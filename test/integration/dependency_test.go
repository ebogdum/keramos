package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/v2/internal/pkg"
	"github.com/ebogdum/keramos/v2/internal/repo"
	"gopkg.in/yaml.v3"
)

func TestDependencyTransitiveResolution(t *testing.T) {
	// leaf-b has no deps, mid-a depends on leaf-b, root depends on mid-a
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "leaf-b", Version: "1.0.0"},
		{Name: "mid-a", Version: "1.0.0", Dependencies: []pkg.Dependency{
			{Name: "leaf-b", Version: "^1.0.0", Repository: "PLACEHOLDER"},
		}},
		{Name: "mid-a", Version: "1.1.0", Dependencies: []pkg.Dependency{
			{Name: "leaf-b", Version: "^1.0.0", Repository: "PLACEHOLDER"},
		}},
	}, false)

	// Fix the placeholder repository URLs in the packages
	// Since the resolver uses FetchIndex via DefaultClient, we need to set up properly
	cred := &repo.Credential{}
	client := newTestClient(t, extractHostPort(serverURL), cred)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	// Rebuild the test repo with correct URLs (transitive deps reference the repo).
	srv, srvURL := createDependencyTestRepo(t, serverURL)
	_ = srv

	client2 := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client2)

	deps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: srvURL},
	}

	result, err := repo.ResolveTree(deps)
	if nil != err {
		t.Fatalf("transitive resolution failed: %v", err)
	}

	if 0 == len(result.Resolved) {
		t.Fatal("expected at least one resolved dependency")
	}

	// Should resolve mid-a and leaf-b
	resolvedNames := make(map[string]string)
	for _, rd := range result.Resolved {
		resolvedNames[rd.Name] = rd.Version
	}

	if _, ok := resolvedNames["mid-a"]; !ok {
		t.Error("expected mid-a in resolved dependencies")
	}
	if _, ok := resolvedNames["leaf-b"]; !ok {
		t.Error("expected leaf-b in resolved dependencies (transitive)")
	}
}

func TestDependencyVersionSelection(t *testing.T) {
	// mid-a has versions 1.0.0 and 1.1.0; ^1.0.0 should pick 1.1.0
	srv, srvURL := createDependencyTestRepo(t, "")
	_ = srv

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	deps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: srvURL},
	}

	result, err := repo.ResolveTree(deps)
	if nil != err {
		t.Fatalf("version selection failed: %v", err)
	}

	for _, rd := range result.Resolved {
		if "mid-a" == rd.Name {
			if "1.1.0" != rd.Version {
				t.Errorf("expected mid-a version 1.1.0 (highest matching ^1.0.0), got %s", rd.Version)
			}
			return
		}
	}

	t.Error("mid-a not found in resolved dependencies")
}

func TestDependencyLockFileGeneration(t *testing.T) {
	srv, srvURL := createDependencyTestRepo(t, "")
	_ = srv

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	// Create a project directory with keramos.yaml
	projectDir := t.TempDir()
	meta := pkg.PackageMetadata{
		APIVersion: "keramos/v1",
		Name:       "test-project",
		Version:    "0.1.0",
		Dependencies: []pkg.Dependency{
			{Name: "mid-a", Version: "^1.0.0", Repository: srvURL},
		},
	}
	writeMetadata(t, projectDir, &meta)

	// Resolve dependencies
	result, err := repo.ResolveTree(meta.Dependencies)
	if nil != err {
		t.Fatalf("resolve failed: %v", err)
	}

	// Build and save lock file
	lf := buildTestLockFile(result)
	if err := repo.SaveLockFile(lf, projectDir); nil != err {
		t.Fatalf("failed to save lock file: %v", err)
	}

	// Verify lock file exists
	lockPath := filepath.Join(projectDir, "keramos.lock")
	assertFileExists(t, lockPath)

	// Reload and verify
	loaded, err := repo.LoadLockFile(projectDir)
	if nil != err {
		t.Fatalf("failed to load lock file: %v", err)
	}
	if nil == loaded {
		t.Fatal("expected non-nil lock file")
	}

	// Should have both mid-a and leaf-b
	lockedNames := make(map[string]bool, len(loaded.Dependencies))
	for _, ld := range loaded.Dependencies {
		lockedNames[ld.Name] = true
	}

	if !lockedNames["mid-a"] {
		t.Error("expected mid-a in lock file")
	}
	if !lockedNames["leaf-b"] {
		t.Error("expected leaf-b in lock file")
	}
}

func TestDependencyLockFileCurrentCheck(t *testing.T) {
	lf := &repo.LockFile{
		APIVersion: "v1",
		Dependencies: []repo.LockedDependency{
			{Name: "mid-a", Version: "1.1.0", Repository: "https://example.com"},
			{Name: "leaf-b", Version: "1.0.0", Repository: "https://example.com"},
		},
	}

	// These constraints should be satisfied by the locked versions
	deps := []pkg.Dependency{
		{Name: "mid-a", Version: "^1.0.0", Repository: "https://example.com"},
	}

	if !repo.IsLockFileCurrent(lf, deps) {
		t.Error("expected lock file to be current")
	}

	// Change the constraint so it's no longer satisfied
	staleDeps := []pkg.Dependency{
		{Name: "mid-a", Version: "^2.0.0", Repository: "https://example.com"},
	}

	if repo.IsLockFileCurrent(lf, staleDeps) {
		t.Error("expected lock file to be stale with ^2.0.0 constraint")
	}
}

func TestDependencyConflictDetection(t *testing.T) {
	// conflict-a depends on leaf-b@^1.0.0, conflict-b depends on leaf-b@^2.0.0
	tmpDir := t.TempDir()

	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	// leaf-b versions
	addPackageToIndex(t, tmpDir, idx, "leaf-b", "1.0.0", nil)
	addPackageToIndex(t, tmpDir, idx, "leaf-b", "2.0.0", nil)

	srvURL := serveIndex(t, tmpDir, idx)

	// conflict-a depends on leaf-b@^1.0.0
	addPackageToIndex(t, tmpDir, idx, "conflict-a", "1.0.0", []pkg.Dependency{
		{Name: "leaf-b", Version: "^1.0.0", Repository: srvURL},
	})

	// conflict-b depends on leaf-b@^2.0.0
	addPackageToIndex(t, tmpDir, idx, "conflict-b", "1.0.0", []pkg.Dependency{
		{Name: "leaf-b", Version: "^2.0.0", Repository: srvURL},
	})

	// Rewrite the index with all entries
	indexData, err := yaml.Marshal(idx)
	if nil != err {
		t.Fatalf("failed to marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "index.yaml"), indexData, 0644); nil != err {
		t.Fatalf("failed to write index: %v", err)
	}

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	deps := []pkg.Dependency{
		{Name: "conflict-a", Version: "^1.0.0", Repository: srvURL},
		{Name: "conflict-b", Version: "^1.0.0", Repository: srvURL},
	}

	_, err = repo.ResolveTree(deps)
	if nil == err {
		t.Fatal("expected conflict error, but resolution succeeded")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "conflict") && !strings.Contains(errStr, "CONFLICT") &&
		!strings.Contains(errStr, "no version") {
		t.Errorf("expected conflict-related error, got: %v", err)
	}
}

func TestDependencyMissingPackage(t *testing.T) {
	_, srvURL := createTestRepo(t, []testPkg{
		{Name: "existing", Version: "1.0.0"},
	}, false)

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	deps := []pkg.Dependency{
		{Name: "nonexistent", Version: "^1.0.0", Repository: srvURL},
	}

	_, err := repo.ResolveTree(deps)
	if nil == err {
		t.Fatal("expected error for missing package, but resolution succeeded")
	}

	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention 'nonexistent', got: %v", err)
	}
}

// --- helpers specific to dependency tests ---

// createDependencyTestRepo creates a test repository with a standard dependency chain:
// mid-a@1.0.0 -> leaf-b@^1.0.0
// mid-a@1.1.0 -> leaf-b@^1.0.0
// leaf-b@1.0.0 (no deps)
// leaf-b@2.0.0 (no deps)
func createDependencyTestRepo(t *testing.T, _ string) (*http.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()

	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	// First, create a server to get the URL, then rebuild with correct repo URLs
	// We'll use a two-pass approach: create packages referencing the server URL

	// Create leaf-b packages (no deps)
	addPackageToIndex(t, tmpDir, idx, "leaf-b", "1.0.0", nil)
	addPackageToIndex(t, tmpDir, idx, "leaf-b", "2.0.0", nil)

	// Create a temporary server to get the URL for mid-a's dependencies
	srvURL := serveIndex(t, tmpDir, idx)

	// Now create mid-a packages that depend on leaf-b at this server
	addPackageToIndex(t, tmpDir, idx, "mid-a", "1.0.0", []pkg.Dependency{
		{Name: "leaf-b", Version: "^1.0.0", Repository: srvURL},
	})
	addPackageToIndex(t, tmpDir, idx, "mid-a", "1.1.0", []pkg.Dependency{
		{Name: "leaf-b", Version: "^1.0.0", Repository: srvURL},
	})

	// Rewrite the index
	indexData, err := yaml.Marshal(idx)
	if nil != err {
		t.Fatalf("failed to marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "index.yaml"), indexData, 0644); nil != err {
		t.Fatalf("failed to write index: %v", err)
	}

	return nil, srvURL
}

// addPackageToIndex creates a package archive and adds it to the index.
func addPackageToIndex(t *testing.T, dir string, idx *repo.IndexFile, name, version string, deps []pkg.Dependency) {
	t.Helper()

	archivePath := createTestPackage(t, dir, name, version, deps)
	digest := computeFileDigest(t, archivePath)
	archiveName := filepath.Base(archivePath)

	entry := repo.IndexEntry{
		Name:    name,
		Version: version,
		Digest:  digest,
		URLs:    []string{archiveName},
	}
	idx.Entries[name] = append(idx.Entries[name], entry)
}

// serveIndex writes the index to disk and starts an httptest server serving from that directory.
// Returns the server URL.
func serveIndex(t *testing.T, dir string, idx *repo.IndexFile) string {
	t.Helper()

	indexData, err := yaml.Marshal(idx)
	if nil != err {
		t.Fatalf("failed to marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.yaml"), indexData, 0644); nil != err {
		t.Fatalf("failed to write index: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if _, statErr := os.Stat(filePath); nil != statErr {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filePath)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server.URL
}

// writeMetadata writes a keramos.yaml to the given directory.
func writeMetadata(t *testing.T, dir string, meta *pkg.PackageMetadata) {
	t.Helper()

	data, err := yaml.Marshal(meta)
	if nil != err {
		t.Fatalf("failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), data, 0644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}
}

// buildTestLockFile creates a LockFile from a ResolutionResult.
func buildTestLockFile(result *repo.ResolutionResult) *repo.LockFile {
	deps := make([]repo.LockedDependency, 0, len(result.Resolved))
	for _, rd := range result.Resolved {
		deps = append(deps, repo.LockedDependency(rd))
	}

	return &repo.LockFile{
		APIVersion:   "v1",
		Dependencies: deps,
	}
}
