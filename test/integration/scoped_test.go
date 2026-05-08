package integration

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/internal/pkg"
	"github.com/ebogdum/keramos/internal/repo"
)

func TestScopedPackageCreation(t *testing.T) {
	tmpDir := t.TempDir()
	scopedName := "@myorg/redis"

	archivePath := createTestPackage(t, tmpDir, scopedName, "1.0.0", nil)
	assertFileExists(t, archivePath)

	// Verify archive naming: @myorg-redis-1.0.0.keramos.tgz
	expectedName := repo.ArchiveFileName(scopedName, "1.0.0")
	if expectedName != filepath.Base(archivePath) {
		t.Errorf("expected archive name %q, got %q", expectedName, filepath.Base(archivePath))
	}

	// Verify the archive contains valid metadata
	extractDir := t.TempDir()
	if err := repo.ExtractArchive(archivePath, extractDir); nil != err {
		t.Fatalf("failed to extract scoped archive: %v", err)
	}

	// Find the extracted keramos.yaml
	entries, err := os.ReadDir(extractDir)
	if nil != err {
		t.Fatalf("failed to read extract dir: %v", err)
	}

	var foundMeta bool
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(extractDir, entry.Name(), "keramos.yaml")
		if _, statErr := os.Stat(metaPath); nil == statErr {
			meta, loadErr := pkg.LoadPackageMetadata(filepath.Join(extractDir, entry.Name()))
			if nil != loadErr {
				t.Fatalf("failed to load metadata from extracted archive: %v", loadErr)
			}
			if scopedName != meta.Name {
				t.Errorf("expected package name %q, got %q", scopedName, meta.Name)
			}
			foundMeta = true
			break
		}
	}

	if !foundMeta {
		t.Fatal("expected to find keramos.yaml in extracted scoped archive")
	}
}

func TestScopedPackagePublish(t *testing.T) {
	tmpDir := t.TempDir()
	scopedName := "@myorg/redis"
	var receivedName string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/packages", func(w http.ResponseWriter, r *http.Request) {
		if "POST" != r.Method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(32 << 20); nil != err {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, header, err := r.FormFile("package")
		if nil != err {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedName = header.Filename

		w.WriteHeader(http.StatusCreated)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	archivePath := createTestPackage(t, tmpDir, scopedName, "1.0.0", nil)
	archiveData, err := os.ReadFile(archivePath)
	if nil != err {
		t.Fatalf("failed to read archive: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("package", filepath.Base(archivePath))
	if nil != err {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(archiveData); nil != err {
		t.Fatalf("failed to write to form: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", server.URL+"/v1/packages", &body)
	if nil != err {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if nil != err {
		t.Fatalf("failed to publish scoped package: %v", err)
	}
	defer resp.Body.Close()

	if http.StatusCreated != resp.StatusCode {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	expectedFilename := repo.ArchiveFileName(scopedName, "1.0.0")
	if expectedFilename != receivedName {
		t.Errorf("expected server to receive filename %q, got %q", expectedFilename, receivedName)
	}
}

func TestScopedDependencyResolution(t *testing.T) {
	tmpDir := t.TempDir()
	scopedName := "@myorg/redis"

	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	addPackageToIndex(t, tmpDir, idx, scopedName, "1.0.0", nil)
	addPackageToIndex(t, tmpDir, idx, scopedName, "1.1.0", nil)

	srvURL := serveIndex(t, tmpDir, idx)

	client := newTestClient(t, extractHostPort(srvURL), nil)
	repo.SetDefaultClient(client)
	defer repo.SetDefaultClient(nil)

	deps := []pkg.Dependency{
		{Name: scopedName, Version: "^1.0.0", Repository: srvURL},
	}

	result, err := repo.ResolveTree(deps)
	if nil != err {
		t.Fatalf("scoped dependency resolution failed: %v", err)
	}

	if 0 == len(result.Resolved) {
		t.Fatal("expected at least one resolved dependency")
	}

	found := false
	for _, rd := range result.Resolved {
		if scopedName == rd.Name {
			found = true
			if "1.1.0" != rd.Version {
				t.Errorf("expected version 1.1.0, got %s", rd.Version)
			}
			break
		}
	}

	if !found {
		t.Errorf("expected %s in resolved dependencies", scopedName)
	}
}

func TestScopedPackageInstallPath(t *testing.T) {
	scopedName := "@myorg/redis"

	// Verify PackageDir returns the correct path
	expectedDir := filepath.Join("packages", "@myorg", "redis")
	actualDir := repo.PackageDir(scopedName)

	if expectedDir != actualDir {
		t.Errorf("expected package dir %q, got %q", expectedDir, actualDir)
	}

	// Verify unscoped stays flat
	unscopedDir := repo.PackageDir("redis")
	expectedUnscoped := filepath.Join("packages", "redis")
	if expectedUnscoped != unscopedDir {
		t.Errorf("expected unscoped dir %q, got %q", expectedUnscoped, unscopedDir)
	}
}

func TestScopedPackageDownload(t *testing.T) {
	tmpDir := t.TempDir()
	scopedName := "@myorg/redis"

	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	addPackageToIndex(t, tmpDir, idx, scopedName, "1.0.0", nil)
	srvURL := serveIndex(t, tmpDir, idx)

	client := newTestClient(t, extractHostPort(srvURL), nil)

	archivePath, err := repo.DownloadPackageWith(client, srvURL, scopedName, "1.0.0")
	if nil != err {
		t.Fatalf("failed to download scoped package: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	assertFileExists(t, archivePath)

	// Extract and verify the metadata
	extractDir := t.TempDir()
	if err := repo.ExtractArchive(archivePath, extractDir); nil != err {
		t.Fatalf("failed to extract: %v", err)
	}

	// Walk to find keramos.yaml
	entries, err := os.ReadDir(extractDir)
	if nil != err {
		t.Fatalf("failed to read extract dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, loadErr := pkg.LoadPackageMetadata(filepath.Join(extractDir, entry.Name()))
		if nil != loadErr {
			continue
		}
		if scopedName != meta.Name {
			t.Errorf("expected package name %q in downloaded archive, got %q", scopedName, meta.Name)
		}
		return
	}

	t.Fatal("did not find valid metadata in downloaded scoped package")
}

func TestScopedNameValidation(t *testing.T) {
	validNames := []string{
		"myapp",
		"@myorg/myapp",
		"@my-org/my.app",
		"@org123/pkg",
	}

	for _, name := range validNames {
		if err := repo.ValidateScopedName(name); nil != err {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}

	invalidNames := []string{
		"",
		"@/foo",
		"@ORG/foo",
		"@org/",
	}

	for _, name := range invalidNames {
		if err := repo.ValidateScopedName(name); nil == err {
			t.Errorf("expected %q to be invalid, but validation passed", name)
		}
	}
}

func TestScopedPackageInIndex(t *testing.T) {
	tmpDir := t.TempDir()
	scopedName := "@myorg/redis"

	// Create a scoped package and generate an index from the directory
	createTestPackage(t, tmpDir, scopedName, "1.0.0", nil)
	createTestPackage(t, tmpDir, scopedName, "2.0.0", nil)

	idx, err := repo.GenerateIndex(tmpDir, "https://example.com/packages")
	if nil != err {
		t.Fatalf("failed to generate index: %v", err)
	}

	entries, ok := idx.Entries[scopedName]
	if !ok {
		// Check if it got indexed under a different key
		t.Logf("index keys: %v", keysOf(idx.Entries))
		t.Fatal("expected scoped package in index")
	}

	if 2 != len(entries) {
		t.Errorf("expected 2 entries for scoped package, got %d", len(entries))
	}

	for _, entry := range entries {
		if !strings.Contains(entry.URLs[0], "@myorg") {
			t.Logf("note: scoped package URL format: %s", entry.URLs[0])
		}
	}
}

// keysOf returns the keys of a map for diagnostic output.
func keysOf(m map[string][]repo.IndexEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestScopedSearchInIndex(t *testing.T) {
	_, srvURL := createTestRepo(t, []testPkg{
		{Name: "@myorg/redis", Version: "1.0.0"},
		{Name: "@myorg/postgres", Version: "1.0.0"},
		{Name: "nginx", Version: "1.0.0"},
	}, false)

	client := newTestClient(t, extractHostPort(srvURL), nil)

	idx, err := repo.FetchIndexWith(client, srvURL)
	if nil != err {
		t.Fatalf("failed to fetch index: %v", err)
	}

	// Search for @myorg scope
	var scopedCount int
	for name := range idx.Entries {
		if strings.HasPrefix(name, "@myorg/") {
			scopedCount++
		}
	}

	if 2 != scopedCount {
		t.Errorf("expected 2 @myorg packages, found %d", scopedCount)
	}
}

// unused import guard — ensure io is used
var _ = io.Discard
