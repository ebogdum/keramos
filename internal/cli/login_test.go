package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/v3/internal/repo"
)

func setupTestCredDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// os.UserConfigDir on Darwin uses $HOME/Library/Application Support
	// On Linux it uses $XDG_CONFIG_HOME (or $HOME/.config)
	// We set HOME so both platforms resolve to our temp directory.
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Determine the actual config dir that os.UserConfigDir will return
	configDir, err := os.UserConfigDir()
	if nil != err {
		t.Fatalf("UserConfigDir failed: %v", err)
	}

	keramosDir := filepath.Join(configDir, "keramos")
	if err := os.MkdirAll(keramosDir, 0755); nil != err {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	return configDir
}

func readCredentialStore(t *testing.T, dir string) map[string]repo.Credential {
	t.Helper()
	path := filepath.Join(dir, "keramos", "credentials.json")
	data, err := os.ReadFile(path)
	if nil != err {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var store struct {
		Credentials map[string]repo.Credential `json:"credentials"`
	}
	if err := json.Unmarshal(data, &store); nil != err {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	return store.Credentials
}

func TestLoginCommand_BasicAuth(t *testing.T) {
	dir := setupTestCredDir(t)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"login", "registry.example.com", "-u", "myuser", "-p", "mypass"})

	if err := root.Execute(); nil != err {
		t.Fatalf("login command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Login succeeded") {
		t.Errorf("expected success message, got %q", output)
	}

	creds := readCredentialStore(t, dir)
	c, ok := creds["registry.example.com"]
	if !ok {
		t.Fatal("expected credential for registry.example.com")
	}
	if "basic" != string(c.Type) {
		t.Errorf("expected type 'basic', got %q", c.Type)
	}
	if "myuser" != c.Username {
		t.Errorf("expected username 'myuser', got %q", c.Username)
	}
}

func TestLoginCommand_BearerToken(t *testing.T) {
	dir := setupTestCredDir(t)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"login", "api.example.com", "--token", "bearer-tok-123"})

	if err := root.Execute(); nil != err {
		t.Fatalf("login command failed: %v", err)
	}

	creds := readCredentialStore(t, dir)
	c, ok := creds["api.example.com"]
	if !ok {
		t.Fatal("expected credential for api.example.com")
	}
	if "bearer" != string(c.Type) {
		t.Errorf("expected type 'bearer', got %q", c.Type)
	}
	if "bearer-tok-123" != c.Token {
		t.Errorf("expected token 'bearer-tok-123', got %q", c.Token)
	}
}

func TestLoginCommand_APIKey(t *testing.T) {
	dir := setupTestCredDir(t)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"login", "data.example.com", "--api-key", "key-abc"})

	if err := root.Execute(); nil != err {
		t.Fatalf("login command failed: %v", err)
	}

	creds := readCredentialStore(t, dir)
	c, ok := creds["data.example.com"]
	if !ok {
		t.Fatal("expected credential for data.example.com")
	}
	if "apikey" != string(c.Type) {
		t.Errorf("expected type 'apikey', got %q", c.Type)
	}
	if "key-abc" != c.APIKey {
		t.Errorf("expected apiKey 'key-abc', got %q", c.APIKey)
	}
}

func TestLoginCommand_NoCredentials(t *testing.T) {
	setupTestCredDir(t)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"login", "host.com"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error when no credentials provided")
	}
}

func TestLogoutCommand(t *testing.T) {
	dir := setupTestCredDir(t)

	// First login
	root := NewRootCommand()
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"login", "host.com", "-u", "user", "-p", "pass"})
	if err := root.Execute(); nil != err {
		t.Fatalf("login failed: %v", err)
	}

	// Then logout
	root2 := NewRootCommand()
	buf := new(bytes.Buffer)
	root2.SetOut(buf)
	root2.SetArgs([]string{"logout", "host.com"})
	if err := root2.Execute(); nil != err {
		t.Fatalf("logout failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Logout succeeded") {
		t.Errorf("expected success message, got %q", output)
	}

	creds := readCredentialStore(t, dir)
	if _, ok := creds["host.com"]; ok {
		t.Error("expected credential to be removed after logout")
	}
}

func TestLogoutCommand_NotLoggedIn(t *testing.T) {
	setupTestCredDir(t)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"logout", "unknown.com"})

	if err := root.Execute(); nil != err {
		t.Fatalf("logout should not fail for unknown host: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Not logged in") {
		t.Errorf("expected 'Not logged in' message, got %q", output)
	}
}
