package repo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, store *CredentialStore) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(store)
	if nil != err {
		t.Fatalf("NewAuthenticatedClient failed: %v", err)
	}
	return client
}

func TestAuthenticatedClient_BasicAuthInjection(t *testing.T) {
	// httptest serves plain http; opt into plaintext auth to exercise injection.
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if "alice" != user || "secret" != pass {
			t.Errorf("expected alice:secret, got %s:%s", user, pass)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	store := &CredentialStore{
		Credentials: map[string]Credential{
			host: {Type: AuthBasic, Username: "alice", Password: "secret"},
		},
	}

	client := newTestClient(t, store)
	resp, err := client.Get(srv.URL + "/test")
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthenticatedClient_BearerTokenInjection(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if "Bearer mytoken123" != authHeader {
			t.Errorf("expected 'Bearer mytoken123', got %q", authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	store := &CredentialStore{
		Credentials: map[string]Credential{
			host: {Type: AuthBearer, Token: "mytoken123"},
		},
	}

	client := newTestClient(t, store)
	resp, err := client.Get(srv.URL + "/api")
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthenticatedClient_APIKeyInjection(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if "apikey999" != key {
			t.Errorf("expected 'apikey999', got %q", key)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	store := &CredentialStore{
		Credentials: map[string]Credential{
			host: {Type: AuthAPIKey, APIKey: "apikey999"},
		},
	}

	client := newTestClient(t, store)
	resp, err := client.Get(srv.URL + "/data")
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthenticatedClient_UserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if "keramos/1.2.3" != ua {
			t.Errorf("expected User-Agent 'keramos/1.2.3', got %q", ua)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client := newTestClient(t, store)
	client.SetVersion("1.2.3")

	resp, err := client.Get(srv.URL)
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	resp.Body.Close()
}

func TestAuthenticatedClient_NoCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "" != r.Header.Get("Authorization") {
			t.Error("expected no Authorization header")
		}
		if "" != r.Header.Get("X-API-Key") {
			t.Error("expected no X-API-Key header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client := newTestClient(t, store)

	resp, err := client.Get(srv.URL)
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	resp.Body.Close()
}

func TestAuthenticatedClient_RetryOn500(t *testing.T) {
	// Replace sleepFn so retries don't actually wait
	origSleep := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = origSleep }()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client := newTestClient(t, store)

	resp, err := client.Get(srv.URL)
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		t.Errorf("expected 200 after retries, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if "success" != string(body) {
		t.Errorf("expected body 'success', got %q", string(body))
	}

	if 3 != int(attempts.Load()) {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestAuthenticatedClient_AllRetriesExhausted(t *testing.T) {
	origSleep := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = origSleep }()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client := newTestClient(t, store)

	_, err := client.Get(srv.URL)
	if nil == err {
		t.Fatal("expected error after all retries exhausted")
	}

	if 3 != int(attempts.Load()) {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestAuthenticatedClient_NoRetryOn4xx(t *testing.T) {
	origSleep := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = origSleep }()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	store := &CredentialStore{Credentials: make(map[string]Credential)}
	client := newTestClient(t, store)

	resp, err := client.Get(srv.URL)
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	resp.Body.Close()

	if 1 != int(attempts.Load()) {
		t.Errorf("expected exactly 1 attempt for 4xx, got %d", attempts.Load())
	}

	if 404 != resp.StatusCode {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
