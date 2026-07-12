package repo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthNotSentOverPlaintextByDefault proves registry credentials are NOT
// transmitted over plaintext HTTP unless the operator explicitly opts in,
// closing the on-path credential-theft vector. Mirrors the OCI path's
// KERAMOS_ALLOW_PLAINTEXT_AUTH suppression.
func TestAuthNotSentOverPlaintextByDefault(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	store := &CredentialStore{
		Credentials: map[string]Credential{
			host: {Type: AuthBasic, Username: "alice", Password: "secret"},
		},
	}

	// Default: env unset -> credentials must be withheld over http.
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "")
	client := newTestClient(t, store)
	resp, err := client.Get(srv.URL + "/index.yaml")
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}
	resp.Body.Close()

	if "" != gotAuth {
		t.Fatalf("credentials leaked over plaintext HTTP: Authorization=%q", gotAuth)
	}
}
