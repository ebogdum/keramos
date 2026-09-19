package integration

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ebogdum/keramos/v3/internal/pkg"
	"github.com/ebogdum/keramos/v3/internal/repo"
	"gopkg.in/yaml.v3"
)

// testPkg describes a package for use in test repository construction.
type testPkg struct {
	Name         string
	Version      string
	Dependencies []pkg.Dependency
}

// createTestPackage creates a keramos package directory and archives it into a .keramos.tgz.
// Returns the path to the archive file.
func createTestPackage(t *testing.T, dir, name, version string, deps []pkg.Dependency) string {
	t.Helper()

	pkgDir := filepath.Join(dir, fmt.Sprintf("%s-%s", flatName(name), version))
	if err := os.MkdirAll(pkgDir, 0755); nil != err {
		t.Fatalf("failed to create package dir: %v", err)
	}

	meta := pkg.PackageMetadata{
		APIVersion:   "keramos/v1",
		Name:         name,
		Version:      version,
		Description:  fmt.Sprintf("Test package %s", name),
		Dependencies: deps,
	}

	metaData, err := yaml.Marshal(&meta)
	if nil != err {
		t.Fatalf("failed to marshal keramos.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), metaData, 0644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}

	valuesContent := []byte("replicas: 1\n")
	if err := os.WriteFile(filepath.Join(pkgDir, "values.yaml"), valuesContent, 0644); nil != err {
		t.Fatalf("failed to write values.yaml: %v", err)
	}

	templatesDir := filepath.Join(pkgDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); nil != err {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	dummyTemplate := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: dummy\n")
	if err := os.WriteFile(filepath.Join(templatesDir, "dummy.yaml"), dummyTemplate, 0644); nil != err {
		t.Fatalf("failed to write dummy template: %v", err)
	}

	archiveName := repo.ArchiveFileName(name, version)
	archivePath := filepath.Join(dir, archiveName)

	if err := createTgzArchive(pkgDir, archivePath, flatName(name)); nil != err {
		t.Fatalf("failed to create archive: %v", err)
	}

	return archivePath
}

// flatName converts a scoped name like @org/pkg to org-pkg for filesystem use.
func flatName(name string) string {
	if repo.IsScoped(name) {
		scope, base := repo.ScopeAndName(name)
		return scope + "-" + base
	}
	return name
}

// createTgzArchive creates a .keramos.tgz from a source directory with the given prefix.
func createTgzArchive(srcDir, archivePath, prefix string) error {
	outFile, err := os.Create(archivePath)
	if nil != err {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tw := tar.NewWriter(gzWriter)
	defer tw.Close()

	return filepath.Walk(srcDir, func(filePath string, info os.FileInfo, walkErr error) error {
		if nil != walkErr {
			return walkErr
		}

		relPath, err := filepath.Rel(srcDir, filePath)
		if nil != err {
			return err
		}

		if "." == relPath {
			return nil
		}

		archName := filepath.Join(prefix, relPath)

		if info.IsDir() {
			header := &tar.Header{
				Name:     archName + "/",
				Typeflag: tar.TypeDir,
				Mode:     0755,
			}
			return tw.WriteHeader(header)
		}

		data, readErr := os.ReadFile(filePath)
		if nil != readErr {
			return readErr
		}

		header := &tar.Header{
			Name:     archName,
			Size:     int64(len(data)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(header); nil != err {
			return err
		}

		_, err = tw.Write(data)
		return err
	})
}

// createTestRepo sets up an httptest server that serves index.yaml and .keramos.tgz files.
// If requireAuth is true, requests without valid basic auth (testuser/testpass) get 401.
// Returns the server and its URL.
func createTestRepo(t *testing.T, packages []testPkg, requireAuth bool) (*httptest.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()

	// Build archives and index
	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	for _, p := range packages {
		archivePath := createTestPackage(t, tmpDir, p.Name, p.Version, p.Dependencies)
		digest := computeFileDigest(t, archivePath)
		archiveName := filepath.Base(archivePath)

		entry := repo.IndexEntry{
			Name:    p.Name,
			Version: p.Version,
			Digest:  digest,
			URLs:    []string{archiveName},
		}
		idx.Entries[p.Name] = append(idx.Entries[p.Name], entry)
	}

	indexData, err := yaml.Marshal(idx)
	if nil != err {
		t.Fatalf("failed to marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "index.yaml"), indexData, 0644); nil != err {
		t.Fatalf("failed to write index.yaml: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if requireAuth {
			user, pass, ok := r.BasicAuth()
			if !ok || "testuser" != user || "testpass" != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="keramos"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		filePath := filepath.Join(tmpDir, filepath.Clean(r.URL.Path))
		if _, statErr := os.Stat(filePath); nil != statErr {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, filePath)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, server.URL
}

// createTestRepoWithETag is like createTestRepo but also tracks request counts and returns ETags.
func createTestRepoWithETag(t *testing.T, packages []testPkg) (*httptest.Server, string, *atomic.Int64) {
	t.Helper()

	tmpDir := t.TempDir()
	requestCount := &atomic.Int64{}

	idx := &repo.IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]repo.IndexEntry),
	}

	for _, p := range packages {
		archivePath := createTestPackage(t, tmpDir, p.Name, p.Version, p.Dependencies)
		digest := computeFileDigest(t, archivePath)
		archiveName := filepath.Base(archivePath)

		entry := repo.IndexEntry{
			Name:    p.Name,
			Version: p.Version,
			Digest:  digest,
			URLs:    []string{archiveName},
		}
		idx.Entries[p.Name] = append(idx.Entries[p.Name], entry)
	}

	indexData, err := yaml.Marshal(idx)
	if nil != err {
		t.Fatalf("failed to marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "index.yaml"), indexData, 0644); nil != err {
		t.Fatalf("failed to write index.yaml: %v", err)
	}

	// Compute ETag for index
	indexDigest := sha256Hex(indexData)
	etag := fmt.Sprintf(`"%s"`, indexDigest[:16])

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		// Handle If-None-Match for index.yaml
		if strings.HasSuffix(r.URL.Path, "index.yaml") || "/" == r.URL.Path {
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		filePath := filepath.Join(tmpDir, filepath.Clean(r.URL.Path))
		if _, statErr := os.Stat(filePath); nil != statErr {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, filePath)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, server.URL, requestCount
}

// computeFileDigest returns the SHA256 hex digest of a file.
func computeFileDigest(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(path)
	if nil != err {
		t.Fatalf("failed to open file for digest: %v", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); nil != err {
		t.Fatalf("failed to compute digest: %v", err)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// sha256Hex returns the SHA256 hex digest of the given data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// newTestClient creates an AuthenticatedClient with the given credentials.
func newTestClient(t *testing.T, host string, cred *repo.Credential) *repo.AuthenticatedClient {
	t.Helper()

	store := &repo.CredentialStore{
		Credentials: make(map[string]repo.Credential),
	}

	if nil != cred {
		store.Set(host, *cred)
	}

	client, err := repo.NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("failed to create authenticated client: %v", err)
	}

	return client
}

// assertFileExists verifies that a file exists at the given path.
func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); nil != err {
		t.Errorf("expected file to exist at %s, but got error: %v", path, err)
	}
}

// assertFileNotExists verifies that no file exists at the given path.
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); nil == err {
		t.Errorf("expected file to NOT exist at %s, but it does", path)
	}
}

// readYAML reads a YAML file and unmarshals it into a map.
func readYAML(t *testing.T, path string) map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if nil != err {
		t.Fatalf("failed to read YAML file %s: %v", path, err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); nil != err {
		t.Fatalf("failed to parse YAML file %s: %v", path, err)
	}

	return result
}


// extractHostPort extracts the host:port from a URL for credential matching.
func extractHostPort(rawURL string) string {
	// Strip scheme
	host := rawURL
	if idx := strings.Index(host, "://"); -1 != idx {
		host = host[idx+3:]
	}
	// Strip path
	if idx := strings.Index(host, "/"); -1 != idx {
		host = host[:idx]
	}
	return host
}

// newTestCredentialStore creates a CredentialStore in a temp directory.
func newTestCredentialStore(t *testing.T) (*repo.CredentialStore, string) {
	t.Helper()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "credentials.json")

	store := &repo.CredentialStore{
		Credentials: make(map[string]repo.Credential),
	}

	// Write an empty store
	data, err := json.MarshalIndent(store, "", "  ")
	if nil != err {
		t.Fatalf("failed to marshal credential store: %v", err)
	}
	if err := os.WriteFile(storePath, data, 0600); nil != err {
		t.Fatalf("failed to write credential store: %v", err)
	}

	return store, storePath
}
