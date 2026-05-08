package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/internal/repo"
	"gopkg.in/yaml.v3"
)

func TestPrivateRepoLoginBasicAuth(t *testing.T) {
	store, _ := newTestCredentialStore(t)

	host := "registry.example.com"
	cred := repo.Credential{
		Type:     repo.AuthBasic,
		Username: "testuser",
		Password: "testpass",
	}

	store.Set(host, cred)

	// Verify the credential is stored
	retrieved, ok := store.Get(host)
	if !ok {
		t.Fatal("expected credential to be stored")
	}
	if repo.AuthBasic != retrieved.Type {
		t.Errorf("expected type %s, got %s", repo.AuthBasic, retrieved.Type)
	}
	if "testuser" != retrieved.Username {
		t.Errorf("expected username 'testuser', got %q", retrieved.Username)
	}
	if "testpass" != retrieved.Password {
		t.Errorf("expected password 'testpass', got %q", retrieved.Password)
	}
}

func TestPrivateRepoLoginBearerToken(t *testing.T) {
	store, _ := newTestCredentialStore(t)

	host := "registry.example.com"
	cred := repo.Credential{
		Type:  repo.AuthBearer,
		Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test",
	}

	store.Set(host, cred)

	retrieved, ok := store.Get(host)
	if !ok {
		t.Fatal("expected credential to be stored")
	}
	if repo.AuthBearer != retrieved.Type {
		t.Errorf("expected type %s, got %s", repo.AuthBearer, retrieved.Type)
	}
	if cred.Token != retrieved.Token {
		t.Errorf("expected token %q, got %q", cred.Token, retrieved.Token)
	}
}

func TestPrivateRepoLoginAPIKey(t *testing.T) {
	store, _ := newTestCredentialStore(t)

	host := "registry.example.com"
	cred := repo.Credential{
		Type:   repo.AuthAPIKey,
		APIKey: "ak_live_abcdef123456",
	}

	store.Set(host, cred)

	retrieved, ok := store.Get(host)
	if !ok {
		t.Fatal("expected credential to be stored")
	}
	if repo.AuthAPIKey != retrieved.Type {
		t.Errorf("expected type %s, got %s", repo.AuthAPIKey, retrieved.Type)
	}
	if cred.APIKey != retrieved.APIKey {
		t.Errorf("expected API key %q, got %q", cred.APIKey, retrieved.APIKey)
	}
}

func TestPrivateRepoFetchIndex(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || "testuser" != user || "testpass" != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		idx := &repo.IndexFile{
			APIVersion: "v1",
			Entries: map[string][]repo.IndexEntry{
				"private-app": {
					{Name: "private-app", Version: "1.0.0", Digest: "abc123"},
				},
			},
		}
		data, _ := yaml.Marshal(idx)
		w.Write(data)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	host := extractHostPort(server.URL)
	cred := &repo.Credential{
		Type:     repo.AuthBasic,
		Username: "testuser",
		Password: "testpass",
	}
	client := newTestClient(t, host, cred)

	idx, err := repo.FetchIndexWith(client, server.URL)
	if nil != err {
		t.Fatalf("expected authenticated fetch to succeed: %v", err)
	}

	entries, ok := idx.Entries["private-app"]
	if !ok {
		t.Fatal("expected private-app in index")
	}
	if 1 != len(entries) {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestPrivateRepoDownloadPackage(t *testing.T) {
	_, serverURL := createTestRepo(t, []testPkg{
		{Name: "private-pkg", Version: "1.0.0"},
	}, true)

	host := extractHostPort(serverURL)

	// Without auth should fail
	noAuthClient := newTestClient(t, host, nil)
	_, err := repo.DownloadPackageWith(noAuthClient, serverURL, "private-pkg", "1.0.0")
	if nil == err {
		t.Fatal("expected download without auth to fail")
	}

	// With auth should succeed
	cred := &repo.Credential{
		Type:     repo.AuthBasic,
		Username: "testuser",
		Password: "testpass",
	}
	authClient := newTestClient(t, host, cred)

	archivePath, err := repo.DownloadPackageWith(authClient, serverURL, "private-pkg", "1.0.0")
	if nil != err {
		t.Fatalf("expected authenticated download to succeed: %v", err)
	}

	assertFileExists(t, archivePath)
}

func TestPrivateRepoLogout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || "testuser" != user || "testpass" != pass {
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

	host := extractHostPort(server.URL)

	// Create a store with credentials
	store := &repo.CredentialStore{
		Credentials: make(map[string]repo.Credential),
	}
	store.Set(host, repo.Credential{
		Type:     repo.AuthBasic,
		Username: "testuser",
		Password: "testpass",
	})

	// Verify fetch works with credentials
	client, err := repo.NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = repo.FetchIndexWith(client, server.URL)
	if nil != err {
		t.Fatalf("expected fetch to succeed with credentials: %v", err)
	}

	// Logout: remove the credential
	store.Remove(host)

	// Verify credential is gone
	_, ok := store.Get(host)
	if ok {
		t.Fatal("expected credential to be removed after logout")
	}

	// Create new client with the now-empty store
	loggedOutClient, err := repo.NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("failed to create logged out client: %v", err)
	}

	// Fetch should fail with 401
	_, err = repo.FetchIndexWith(loggedOutClient, server.URL)
	if nil == err {
		t.Fatal("expected fetch to fail after logout")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error after logout, got: %v", err)
	}
}

func TestPrivateRepoMultipleCredentials(t *testing.T) {
	// Set up two private registries
	handler1 := func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || "user1" != user || "pass1" != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		idx := &repo.IndexFile{APIVersion: "v1", Entries: map[string][]repo.IndexEntry{
			"app-from-reg1": {{Name: "app-from-reg1", Version: "1.0.0"}},
		}}
		data, _ := yaml.Marshal(idx)
		w.Write(data)
	}

	handler2 := func(w http.ResponseWriter, r *http.Request) {
		if "Bearer token-for-reg2" != r.Header.Get("Authorization") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		idx := &repo.IndexFile{APIVersion: "v1", Entries: map[string][]repo.IndexEntry{
			"app-from-reg2": {{Name: "app-from-reg2", Version: "2.0.0"}},
		}}
		data, _ := yaml.Marshal(idx)
		w.Write(data)
	}

	mux1 := http.NewServeMux()
	mux1.HandleFunc("/index.yaml", handler1)
	server1 := httptest.NewServer(mux1)
	defer server1.Close()

	mux2 := http.NewServeMux()
	mux2.HandleFunc("/index.yaml", handler2)
	server2 := httptest.NewServer(mux2)
	defer server2.Close()

	store := &repo.CredentialStore{
		Credentials: make(map[string]repo.Credential),
	}
	store.Set(extractHostPort(server1.URL), repo.Credential{
		Type:     repo.AuthBasic,
		Username: "user1",
		Password: "pass1",
	})
	store.Set(extractHostPort(server2.URL), repo.Credential{
		Type:  repo.AuthBearer,
		Token: "token-for-reg2",
	})

	client, err := repo.NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("failed to create client: %v", err)
	}

	// Fetch from registry 1
	idx1, err := repo.FetchIndexWith(client, server1.URL)
	if nil != err {
		t.Fatalf("failed to fetch from registry 1: %v", err)
	}
	if _, ok := idx1.Entries["app-from-reg1"]; !ok {
		t.Error("expected app-from-reg1 in registry 1 index")
	}

	// Fetch from registry 2
	idx2, err := repo.FetchIndexWith(client, server2.URL)
	if nil != err {
		t.Fatalf("failed to fetch from registry 2: %v", err)
	}
	if _, ok := idx2.Entries["app-from-reg2"]; !ok {
		t.Error("expected app-from-reg2 in registry 2 index")
	}
}

func TestPrivateRepoCredentialOverwrite(t *testing.T) {
	store, _ := newTestCredentialStore(t)
	host := "registry.example.com"

	// Set initial credential
	store.Set(host, repo.Credential{
		Type:     repo.AuthBasic,
		Username: "user1",
		Password: "pass1",
	})

	// Overwrite with new credential
	store.Set(host, repo.Credential{
		Type:  repo.AuthBearer,
		Token: "new-token",
	})

	retrieved, ok := store.Get(host)
	if !ok {
		t.Fatal("expected credential to exist")
	}
	if repo.AuthBearer != retrieved.Type {
		t.Errorf("expected type %s after overwrite, got %s", repo.AuthBearer, retrieved.Type)
	}
	if "new-token" != retrieved.Token {
		t.Errorf("expected token 'new-token', got %q", retrieved.Token)
	}
}
