package repo

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchIndexWith_Success(t *testing.T) {
	indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411123
      urls:
        - myapp-1.0.0.keramos.tgz
generated: "2025-01-01T00:00:00Z"
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" != r.URL.Path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(indexContent))
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	idx, err := FetchIndexWith(client, srv.URL)
	if nil != err {
		t.Fatalf("FetchIndexWith failed: %v", err)
	}
	if nil == idx {
		t.Fatal("expected non-nil index")
	}
	entries, ok := idx.Entries["myapp"]
	if !ok {
		t.Fatal("expected myapp in entries")
	}
	if 1 != len(entries) {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if "1.0.0" != entries[0].Version {
		t.Errorf("expected version 1.0.0, got %s", entries[0].Version)
	}
}

func TestFetchIndexWith_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := FetchIndexWith(client, srv.URL)
	if nil == err {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %s", err.Error())
	}
}

func TestFetchIndexWith_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	// Disable sleep for retries
	origSleep := sleepFn
	sleepFn = func(_ time.Duration) {}
	defer func() { sleepFn = origSleep }()

	_, err := FetchIndexWith(client, srv.URL)
	if nil == err {
		t.Fatal("expected error for 500")
	}
}

func TestFetchIndexWith_InvalidYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{{{invalid yaml"))
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := FetchIndexWith(client, srv.URL)
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestFetchIndexWith_EmptyEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("apiVersion: v1\n"))
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	idx, err := FetchIndexWith(client, srv.URL)
	if nil != err {
		t.Fatalf("FetchIndexWith failed: %v", err)
	}
	if nil == idx.Entries {
		t.Fatal("expected non-nil entries map")
	}
	if 0 != len(idx.Entries) {
		t.Errorf("expected 0 entries, got %d", len(idx.Entries))
	}
}

func TestFetchIndexWith_AuthInjected(t *testing.T) {
	// httptest serves plain http; opt into plaintext auth to exercise injection.
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1")
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("apiVersion: v1\nentries: {}\n"))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	store := &CredentialStore{
		Credentials: map[string]Credential{
			host: {Type: AuthBearer, Token: "test-token"},
		},
	}
	client := newTestRepoClient(t, store)

	_, err := FetchIndexWith(client, srv.URL)
	if nil != err {
		t.Fatalf("FetchIndexWith failed: %v", err)
	}
	if "Bearer test-token" != receivedAuth {
		t.Errorf("expected bearer token auth, got %q", receivedAuth)
	}
}

func TestDownloadPackageWith_PackageNotInIndex(t *testing.T) {
	indexContent := `apiVersion: v1
entries:
  other-app:
    - name: other-app
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411
      urls:
        - other-app-1.0.0.keramos.tgz
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil == err {
		t.Fatal("expected error for package not in index")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %s", err.Error())
	}
}

func TestDownloadPackageWith_VersionNotFound(t *testing.T) {
	indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411
      urls:
        - myapp-1.0.0.keramos.tgz
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := DownloadPackageWith(client, srv.URL, "myapp", "2.0.0")
	if nil == err {
		t.Fatal("expected error for version not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %s", err.Error())
	}
}

func TestDownloadPackageWith_NoURLs(t *testing.T) {
	indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411
      urls: []
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil == err {
		t.Fatal("expected error for no URLs")
	}
	if !strings.Contains(err.Error(), "no download URL") {
		t.Errorf("expected 'no download URL' error, got: %s", err.Error())
	}
}

func TestDownloadPackageWith_DownloadFailure(t *testing.T) {
	indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411
      urls:
        - myapp-1.0.0.keramos.tgz
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil == err {
		t.Fatal("expected error for download failure (404)")
	}
}

func TestDownloadPackageWith_AbsoluteURL(t *testing.T) {
	// When a URL is absolute (starts with http), it should be used as-is
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411
      urls:
        - ` + "http://" + r.Host + `/packages/myapp-1.0.0.keramos.tgz
`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		if "/packages/myapp-1.0.0.keramos.tgz" == r.URL.Path {
			// Return a non-empty but not-a-real-archive response (enough for download)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-archive-content"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	path, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil != err {
		t.Fatalf("download failed: %v", err)
	}
	if "" == path {
		t.Error("expected non-empty download path")
	}
}

func TestFetchIndexWith_TrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" != r.URL.Path {
			t.Errorf("expected /index.yaml, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("apiVersion: v1\nentries: {}\n"))
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	_, err := FetchIndexWith(client, srv.URL+"/")
	if nil != err {
		t.Fatalf("FetchIndexWith with trailing slash failed: %v", err)
	}
}

func TestRepoFile_AddAndRemove(t *testing.T) {
	rf := &RepoFile{}

	if err := rf.Add("stable", "https://repo.example.com"); nil != err {
		t.Fatalf("Add failed: %v", err)
	}
	if 1 != len(rf.Repositories) {
		t.Errorf("expected 1 repo, got %d", len(rf.Repositories))
	}

	// Duplicate add should fail
	if err := rf.Add("stable", "https://other.example.com"); nil == err {
		t.Fatal("expected error for duplicate repo name")
	}

	// Add another
	if err := rf.Add("dev", "https://dev.example.com"); nil != err {
		t.Fatalf("Add dev failed: %v", err)
	}
	if 2 != len(rf.Repositories) {
		t.Errorf("expected 2 repos, got %d", len(rf.Repositories))
	}

	// Remove
	if err := rf.Remove("stable"); nil != err {
		t.Fatalf("Remove failed: %v", err)
	}
	if 1 != len(rf.Repositories) {
		t.Errorf("expected 1 repo after remove, got %d", len(rf.Repositories))
	}
	if "dev" != rf.Repositories[0].Name {
		t.Errorf("expected remaining repo to be dev, got %s", rf.Repositories[0].Name)
	}

	// Remove nonexistent
	if err := rf.Remove("nonexistent"); nil == err {
		t.Fatal("expected error for removing nonexistent repo")
	}
}

func TestMergeIndex_Override(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"app": {
				{Name: "app", Version: "1.0.0", Digest: "old"},
			},
		},
	}
	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"app": {
				{Name: "app", Version: "1.0.0", Digest: "new"},
				{Name: "app", Version: "2.0.0", Digest: "v2"},
			},
		},
	}

	merged := MergeIndex(existing, update)
	entries := merged.Entries["app"]
	if 2 != len(entries) {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	for _, e := range entries {
		if "1.0.0" == e.Version && "new" != e.Digest {
			t.Errorf("expected digest=new for 1.0.0, got %s", e.Digest)
		}
	}
}

func TestMergeIndex_NewPackage(t *testing.T) {
	existing := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"app": {{Name: "app", Version: "1.0.0"}},
		},
	}
	update := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"newapp": {{Name: "newapp", Version: "1.0.0"}},
		},
	}

	merged := MergeIndex(existing, update)
	if _, ok := merged.Entries["app"]; !ok {
		t.Error("expected app in merged")
	}
	if _, ok := merged.Entries["newapp"]; !ok {
		t.Error("expected newapp in merged")
	}
}

func TestSaveAndLoadIndex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "index.yaml")

	idx := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"myapp": {
				{Name: "myapp", Version: "1.0.0", Digest: "abc123", URLs: []string{"myapp-1.0.0.keramos.tgz"}},
				{Name: "myapp", Version: "2.0.0", Digest: "def456", URLs: []string{"myapp-2.0.0.keramos.tgz"}},
			},
		},
	}

	if err := SaveIndex(idx, path); nil != err {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	loaded, err := LoadIndex(path)
	if nil != err {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	entries, ok := loaded.Entries["myapp"]
	if !ok {
		t.Fatal("expected myapp in entries")
	}
	if 2 != len(entries) {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestLoadIndex_NotFound(t *testing.T) {
	_, err := LoadIndex("/nonexistent/index.yaml")
	if nil == err {
		t.Fatal("expected error for nonexistent index")
	}
}

func TestSaveIndex_BadPath(t *testing.T) {
	idx := &IndexFile{APIVersion: "v1", Entries: map[string][]IndexEntry{}}
	err := SaveIndex(idx, "/nonexistent/dir/index.yaml")
	if nil == err {
		t.Fatal("expected error for bad path")
	}
}

func TestDownloadPackageWith_RelativeURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			indexContent := `apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: 806166f1698bd2415adafa8e02c7c2a89d393a60978d0ac27efc9ec3265ab5c5
      urls:
        - packages/myapp-1.0.0.keramos.tgz
`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(indexContent))
			return
		}
		if "/packages/myapp-1.0.0.keramos.tgz" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-archive"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: map[string]Credential{}}
	client := newTestRepoClient(t, store)

	path, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil != err {
		t.Fatalf("download failed: %v", err)
	}
	if "" == path {
		t.Error("expected non-empty download path")
	}
}

func newTestRepoClient(t *testing.T, store *CredentialStore) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("NewAuthenticatedClient failed: %v", err)
	}
	return client
}
