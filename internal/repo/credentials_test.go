package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCredentialStore_SetGetRemove(t *testing.T) {
	store := &CredentialStore{
		Credentials: make(map[string]Credential),
	}

	cred := Credential{Type: AuthBasic, Username: "alice", Password: "secret"}
	store.Set("example.com", cred)

	got, ok := store.Get("example.com")
	if !ok {
		t.Fatal("expected credential to exist")
	}
	if "alice" != got.Username {
		t.Errorf("expected username 'alice', got %q", got.Username)
	}
	if "secret" != got.Password {
		t.Errorf("expected password 'secret', got %q", got.Password)
	}

	store.Remove("example.com")

	_, ok = store.Get("example.com")
	if ok {
		t.Fatal("expected credential to be removed")
	}
}

func TestCredentialStore_GetForHost_ExactMatch(t *testing.T) {
	store := &CredentialStore{
		Credentials: map[string]Credential{
			"registry.io": {Type: AuthBearer, Token: "tok123"},
		},
	}

	got, ok := store.GetForHost("registry.io")
	if !ok {
		t.Fatal("expected credential for registry.io")
	}
	if AuthBearer != got.Type {
		t.Errorf("expected AuthBearer, got %q", got.Type)
	}
	if "tok123" != got.Token {
		t.Errorf("expected token 'tok123', got %q", got.Token)
	}
}

func TestCredentialStore_GetForHost_NoMatch(t *testing.T) {
	store := &CredentialStore{
		Credentials: make(map[string]Credential),
	}

	_, ok := store.GetForHost("unknown.io")
	if ok {
		t.Fatal("expected no credential for unknown host")
	}
}

func TestCredentialStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "credentials.json")

	store := &CredentialStore{
		Credentials: map[string]Credential{
			"host1.com": {Type: AuthBasic, Username: "u1", Password: "p1"},
			"host2.com": {Type: AuthAPIKey, APIKey: "key456"},
		},
		path: path,
	}

	if err := store.Save(); nil != err {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file permissions (skip on Windows)
	if "windows" != runtime.GOOS {
		info, err := os.Stat(path)
		if nil != err {
			t.Fatalf("Stat failed: %v", err)
		}
		perm := info.Mode().Perm()
		if 0600 != perm {
			t.Errorf("expected permissions 0600, got %04o", perm)
		}
	}

	// Read back and verify contents
	data, err := os.ReadFile(path)
	if nil != err {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded CredentialStore
	if err := json.Unmarshal(data, &loaded); nil != err {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if 2 != len(loaded.Credentials) {
		t.Fatalf("expected 2 credentials, got %d", len(loaded.Credentials))
	}

	c1, ok := loaded.Credentials["host1.com"]
	if !ok {
		t.Fatal("expected credential for host1.com")
	}
	if "u1" != c1.Username {
		t.Errorf("expected username 'u1', got %q", c1.Username)
	}

	c2, ok := loaded.Credentials["host2.com"]
	if !ok {
		t.Fatal("expected credential for host2.com")
	}
	if "key456" != c2.APIKey {
		t.Errorf("expected apiKey 'key456', got %q", c2.APIKey)
	}
}

func TestMigrateOCICredentials(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir, err := os.UserConfigDir()
	if nil != err {
		t.Fatalf("UserConfigDir failed: %v", err)
	}

	keramosDir := filepath.Join(configDir, "keramos")
	if err := os.MkdirAll(keramosDir, 0755); nil != err {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	legacy := map[string]ociCredential{
		"ghcr.io":   {Username: "ghuser", Password: "ghpass"},
		"docker.io": {Username: "dkuser", Password: "dkpass"},
	}
	data, err := json.Marshal(legacy)
	if nil != err {
		t.Fatalf("Marshal failed: %v", err)
	}

	legacyPath := filepath.Join(keramosDir, "oci-credentials.json")
	if err2 := os.WriteFile(legacyPath, data, 0600); nil != err2 {
		t.Fatalf("WriteFile failed: %v", err2)
	}

	store := &CredentialStore{
		Credentials: make(map[string]Credential),
		path:        filepath.Join(keramosDir, "credentials.json"),
	}

	if err := migrateOCICredentials(store); nil != err {
		t.Fatalf("migration failed: %v", err)
	}

	if 2 != len(store.Credentials) {
		t.Fatalf("expected 2 migrated credentials, got %d", len(store.Credentials))
	}

	ghcr, ok := store.Credentials["ghcr.io"]
	if !ok {
		t.Fatal("expected credential for ghcr.io")
	}
	if AuthBasic != ghcr.Type {
		t.Errorf("expected AuthBasic, got %q", ghcr.Type)
	}
	if "ghuser" != ghcr.Username {
		t.Errorf("expected username 'ghuser', got %q", ghcr.Username)
	}
}

func TestCredentialStore_OverwriteExisting(t *testing.T) {
	store := &CredentialStore{
		Credentials: map[string]Credential{
			"host.com": {Type: AuthBasic, Username: "old", Password: "oldpass"},
		},
	}

	store.Set("host.com", Credential{Type: AuthBearer, Token: "newtoken"})

	got, ok := store.Get("host.com")
	if !ok {
		t.Fatal("expected credential to exist")
	}
	if AuthBearer != got.Type {
		t.Errorf("expected AuthBearer, got %q", got.Type)
	}
	if "newtoken" != got.Token {
		t.Errorf("expected token 'newtoken', got %q", got.Token)
	}
}

func TestCredentialStore_GetForHost_NilCredHelpers(t *testing.T) {
	store := &CredentialStore{
		Credentials: make(map[string]Credential),
		CredHelpers: nil,
	}

	_, ok := store.GetForHost("any.host")
	if ok {
		t.Fatal("expected no credential when credHelpers is nil")
	}
}
