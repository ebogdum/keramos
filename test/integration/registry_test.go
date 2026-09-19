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

	"github.com/ebogdum/keramos/v2/internal/repo"
	"gopkg.in/yaml.v3"
)

func TestRegistryUnauthenticatedFetchFails(t *testing.T) {
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "myapp", Version: "1.0.0"},
	}, true)

	// Create client without credentials
	client := newTestClient(t, extractHostPort(serverURL), nil)

	_, err := repo.FetchIndexWith(client, serverURL)
	if nil == err {
		t.Fatal("expected unauthenticated fetch to fail, but it succeeded")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

func TestRegistryAuthenticatedFetchSucceeds(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1") // httptest serves plain http
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "myapp", Version: "1.0.0"},
		{Name: "myapp", Version: "2.0.0"},
	}, true)

	cred := &repo.Credential{
		Type:     repo.AuthBasic,
		Username: "testuser",
		Password: "testpass",
	}
	client := newTestClient(t, extractHostPort(serverURL), cred)

	idx, err := repo.FetchIndexWith(client, serverURL)
	if nil != err {
		t.Fatalf("expected authenticated fetch to succeed, got: %v", err)
	}

	entries, ok := idx.Entries["myapp"]
	if !ok {
		t.Fatal("expected myapp in index entries")
	}
	if 2 != len(entries) {
		t.Errorf("expected 2 entries for myapp, got %d", len(entries))
	}
}

func TestRegistryDownloadPackage(t *testing.T) {
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "myapp", Version: "1.0.0"},
	}, false)

	client := newTestClient(t, extractHostPort(serverURL), nil)

	// Set as default client so DownloadPackageWith works
	archivePath, err := repo.DownloadPackageWith(client, serverURL, "myapp", "1.0.0")
	if nil != err {
		t.Fatalf("failed to download package: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	assertFileExists(t, archivePath)

	// Verify the downloaded archive has correct digest
	downloadedDigest := computeFileDigest(t, archivePath)
	if "" == downloadedDigest {
		t.Fatal("expected non-empty digest for downloaded package")
	}

	// Fetch index to compare digest
	idx, err := repo.FetchIndexWith(client, serverURL)
	if nil != err {
		t.Fatalf("failed to fetch index: %v", err)
	}

	entries := idx.Entries["myapp"]
	if downloadedDigest != entries[0].Digest {
		t.Errorf("digest mismatch: index=%s, downloaded=%s", entries[0].Digest, downloadedDigest)
	}
}

func TestRegistryPublishPackage(t *testing.T) {
	tmpDir := t.TempDir()
	var receivedArchive []byte

	// Create a publish endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/packages", func(w http.ResponseWriter, r *http.Request) {
		if "POST" != r.Method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || "testuser" != user || "testpass" != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if err := r.ParseMultipartForm(32 << 20); nil != err {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("package")
		if nil != err {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if nil != err {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		receivedArchive = data
		w.WriteHeader(http.StatusCreated)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create a package to publish
	archivePath := createTestPackage(t, tmpDir, "publish-test", "1.0.0", nil)
	archiveData, err := os.ReadFile(archivePath)
	if nil != err {
		t.Fatalf("failed to read archive: %v", err)
	}

	// Publish via multipart POST
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
	req.SetBasicAuth("testuser", "testpass")

	resp, err := http.DefaultClient.Do(req)
	if nil != err {
		t.Fatalf("failed to publish package: %v", err)
	}
	defer resp.Body.Close()

	if http.StatusCreated != resp.StatusCode {
		t.Errorf("expected 201 Created, got %d", resp.StatusCode)
	}

	if 0 == len(receivedArchive) {
		t.Fatal("server did not receive archive data")
	}

	if len(archiveData) != len(receivedArchive) {
		t.Errorf("archive size mismatch: sent=%d, received=%d", len(archiveData), len(receivedArchive))
	}
}

func TestRegistrySearch(t *testing.T) {
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "redis", Version: "1.0.0"},
		{Name: "redis", Version: "2.0.0"},
		{Name: "postgres", Version: "1.0.0"},
		{Name: "nginx", Version: "3.0.0"},
	}, false)

	client := newTestClient(t, extractHostPort(serverURL), nil)

	idx, err := repo.FetchIndexWith(client, serverURL)
	if nil != err {
		t.Fatalf("failed to fetch index: %v", err)
	}

	// Search for "redis" by checking entries
	keyword := "redis"
	var matches []repo.IndexEntry
	for name, entries := range idx.Entries {
		if strings.Contains(name, keyword) {
			matches = append(matches, entries...)
		}
	}

	if 2 != len(matches) {
		t.Errorf("expected 2 search results for %q, got %d", keyword, len(matches))
	}

	// Search for "postgres"
	keyword = "postgres"
	matches = nil
	for name, entries := range idx.Entries {
		if strings.Contains(name, keyword) {
			matches = append(matches, entries...)
		}
	}

	if 1 != len(matches) {
		t.Errorf("expected 1 search result for %q, got %d", keyword, len(matches))
	}

	// Search for non-existent
	keyword = "nonexistent"
	matches = nil
	for name, entries := range idx.Entries {
		if strings.Contains(name, keyword) {
			matches = append(matches, entries...)
		}
	}

	if 0 != len(matches) {
		t.Errorf("expected 0 search results for %q, got %d", keyword, len(matches))
	}
}

func TestRegistryCacheIntegration(t *testing.T) {
	packages := []testPkg{
		{Name: "cached-app", Version: "1.0.0"},
	}

	_, serverURL, requestCount := createTestRepoWithETag(t, packages)

	// Create cache in temp dir
	cacheDir := t.TempDir()
	cache := &repo.IndexCache{
		CacheDir: cacheDir,
		TTL:      5 * 60 * 1e9, // 5 minutes in nanoseconds (time.Duration)
	}

	client := newTestClient(t, extractHostPort(serverURL), nil)

	// First fetch: should hit the server
	idx, err := repo.FetchIndexWith(client, serverURL)
	if nil != err {
		t.Fatalf("first fetch failed: %v", err)
	}

	firstRequestCount := requestCount.Load()
	if 0 == firstRequestCount {
		t.Fatal("expected at least one request to the server")
	}

	// Store in cache
	if err := cache.Put(serverURL, idx, ""); nil != err {
		t.Fatalf("failed to cache index: %v", err)
	}

	// Second fetch from cache: should NOT hit server
	cachedIdx, _, hit := cache.Get(serverURL)
	if !hit {
		t.Fatal("expected cache hit on second request")
	}

	if nil == cachedIdx {
		t.Fatal("cached index is nil")
	}

	// Verify cached data matches
	cachedEntries, ok := cachedIdx.Entries["cached-app"]
	if !ok {
		t.Fatal("expected cached-app in cached index")
	}
	if 1 != len(cachedEntries) {
		t.Errorf("expected 1 cached entry, got %d", len(cachedEntries))
	}

	// Server request count should not have increased
	secondRequestCount := requestCount.Load()
	if secondRequestCount != firstRequestCount {
		t.Errorf("expected no additional server requests after cache hit, but count went from %d to %d",
			firstRequestCount, secondRequestCount)
	}
}

func TestRegistryIndexGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple packages
	createTestPackage(t, tmpDir, "app-a", "1.0.0", nil)
	createTestPackage(t, tmpDir, "app-a", "1.1.0", nil)
	createTestPackage(t, tmpDir, "app-b", "2.0.0", nil)

	idx, err := repo.GenerateIndex(tmpDir, "https://example.com/packages")
	if nil != err {
		t.Fatalf("failed to generate index: %v", err)
	}

	if "v1" != idx.APIVersion {
		t.Errorf("expected apiVersion v1, got %s", idx.APIVersion)
	}

	entriesA, ok := idx.Entries["app-a"]
	if !ok {
		t.Fatal("expected app-a in index")
	}
	if 2 != len(entriesA) {
		t.Errorf("expected 2 entries for app-a, got %d", len(entriesA))
	}

	entriesB, ok := idx.Entries["app-b"]
	if !ok {
		t.Fatal("expected app-b in index")
	}
	if 1 != len(entriesB) {
		t.Errorf("expected 1 entry for app-b, got %d", len(entriesB))
	}

	// Verify digests are populated
	for _, e := range entriesA {
		if "" == e.Digest {
			t.Errorf("expected non-empty digest for %s@%s", e.Name, e.Version)
		}
	}

	// Save and reload
	indexPath := filepath.Join(tmpDir, "index.yaml")
	if err := repo.SaveIndex(idx, indexPath); nil != err {
		t.Fatalf("failed to save index: %v", err)
	}

	reloaded, err := repo.LoadIndex(indexPath)
	if nil != err {
		t.Fatalf("failed to reload index: %v", err)
	}

	if len(idx.Entries) != len(reloaded.Entries) {
		t.Errorf("reloaded index has %d entries, expected %d", len(reloaded.Entries), len(idx.Entries))
	}
}

func TestRegistryIndexMerge(t *testing.T) {
	existing := &repo.IndexFile{
		APIVersion: "v1",
		Entries: map[string][]repo.IndexEntry{
			"app-a": {
				{Name: "app-a", Version: "1.0.0", Digest: "aaa"},
			},
		},
	}

	update := &repo.IndexFile{
		APIVersion: "v1",
		Entries: map[string][]repo.IndexEntry{
			"app-a": {
				{Name: "app-a", Version: "1.0.0", Digest: "bbb"}, // override
				{Name: "app-a", Version: "2.0.0", Digest: "ccc"}, // new
			},
			"app-b": {
				{Name: "app-b", Version: "1.0.0", Digest: "ddd"},
			},
		},
	}

	merged := repo.MergeIndex(existing, update)

	entriesA := merged.Entries["app-a"]
	if 2 != len(entriesA) {
		t.Fatalf("expected 2 entries for app-a, got %d", len(entriesA))
	}

	// Verify 1.0.0 was overridden
	var v1Entry *repo.IndexEntry
	for i := range entriesA {
		if "1.0.0" == entriesA[i].Version {
			v1Entry = &entriesA[i]
			break
		}
	}
	if nil == v1Entry {
		t.Fatal("expected app-a 1.0.0 in merged index")
	}
	if "bbb" != v1Entry.Digest {
		t.Errorf("expected overridden digest 'bbb', got %q", v1Entry.Digest)
	}

	// Verify app-b was added
	entriesB := merged.Entries["app-b"]
	if 1 != len(entriesB) {
		t.Errorf("expected 1 entry for app-b, got %d", len(entriesB))
	}
}

func TestRegistryBearerAuth(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1") // httptest serves plain http
	expectedToken := "my-secret-token-12345"

	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if "Bearer "+expectedToken != auth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		idx := &repo.IndexFile{
			APIVersion: "v1",
			Entries:    make(map[string][]repo.IndexEntry),
		}
		data, _ := yaml.Marshal(idx)
		w.Write(data)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cred := &repo.Credential{
		Type:  repo.AuthBearer,
		Token: expectedToken,
	}
	client := newTestClient(t, extractHostPort(server.URL), cred)

	idx, err := repo.FetchIndexWith(client, server.URL)
	if nil != err {
		t.Fatalf("expected bearer auth to succeed, got: %v", err)
	}

	if nil == idx {
		t.Fatal("expected non-nil index")
	}
}

func TestRegistryAPIKeyAuth(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1") // httptest serves plain http
	expectedKey := "ak_test_123456789"

	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		if expectedKey != r.Header.Get("X-API-Key") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		idx := &repo.IndexFile{
			APIVersion: "v1",
			Entries:    make(map[string][]repo.IndexEntry),
		}
		data, _ := yaml.Marshal(idx)
		w.Write(data)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cred := &repo.Credential{
		Type:   repo.AuthAPIKey,
		APIKey: expectedKey,
	}
	client := newTestClient(t, extractHostPort(server.URL), cred)

	idx, err := repo.FetchIndexWith(client, server.URL)
	if nil != err {
		t.Fatalf("expected API key auth to succeed, got: %v", err)
	}

	if nil == idx {
		t.Fatal("expected non-nil index")
	}
}
