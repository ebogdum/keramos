package repo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecCredentialHelper(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()

	// Create a mock credential helper script
	scriptPath := filepath.Join(dir, "docker-credential-mock")
	script := `#!/bin/sh
read HOST
echo '{"Username":"testuser","Secret":"testpass"}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); nil != err {
		t.Fatal(err)
	}

	// Add to PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)

	cred, err := execCredentialHelper("mock", "registry.example.com")
	if nil != err {
		t.Fatalf("execCredentialHelper failed: %v", err)
	}

	if "testuser" != cred.Username {
		t.Errorf("expected username 'testuser', got %q", cred.Username)
	}
	if "testpass" != cred.Password {
		t.Errorf("expected password 'testpass', got %q", cred.Password)
	}
	if AuthBasic != cred.Type {
		t.Errorf("expected type %q, got %q", AuthBasic, cred.Type)
	}
}

func TestExecCredentialHelperNotFound(t *testing.T) {
	_, err := execCredentialHelper("nonexistent-helper-xyz", "registry.example.com")
	if nil == err {
		t.Fatal("expected error for non-existent helper")
	}
}

func TestMatchCredHelper(t *testing.T) {
	helpers := map[string]string{
		"registry.example.com": "ecr",
		"*.gcr.io":             "gcr",
		"ghcr.io":              "gh",
	}

	tests := []struct {
		host     string
		expected string
	}{
		{"registry.example.com", "ecr"},
		{"us.gcr.io", "gcr"},
		{"eu.gcr.io", "gcr"},
		{"ghcr.io", "gh"},
		{"unknown.example.com", ""},
		{"docker.io", ""},
	}

	for _, tt := range tests {
		result := matchCredHelper(helpers, tt.host)
		if tt.expected != result {
			t.Errorf("matchCredHelper(%q): expected %q, got %q", tt.host, tt.expected, result)
		}
	}
}

func TestGetForHostWithCache(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()

	// Create a mock helper that counts invocations via a file
	counterFile := filepath.Join(dir, "counter")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); nil != err {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "docker-credential-counting")
	script := `#!/bin/sh
read HOST
# Increment counter
COUNT=$(cat "` + counterFile + `")
COUNT=$((COUNT + 1))
echo "$COUNT" > "` + counterFile + `"
echo '{"Username":"user","Secret":"pass"}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); nil != err {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)

	store := &CredentialStore{
		Credentials: make(map[string]Credential),
		CredHelpers: map[string]string{
			"cached.example.com": "counting",
		},
	}

	// First call should exec the helper
	cred, ok := store.GetForHost("cached.example.com")
	if !ok {
		t.Fatal("expected credential to be returned")
	}
	if "user" != cred.Username {
		t.Errorf("expected username 'user', got %q", cred.Username)
	}

	// Second call should use cache (counter should still be 1)
	cred2, ok2 := store.GetForHost("cached.example.com")
	if !ok2 {
		t.Fatal("expected cached credential to be returned")
	}
	if "user" != cred2.Username {
		t.Errorf("expected cached username 'user', got %q", cred2.Username)
	}

	counterData, err := os.ReadFile(counterFile)
	if nil != err {
		t.Fatal(err)
	}
	if "1\n" != string(counterData) && "1" != string(counterData) {
		t.Errorf("expected helper to be called once, counter file contains: %q", string(counterData))
	}
}

func TestGetForHostExactMatchPrecedence(t *testing.T) {
	store := &CredentialStore{
		Credentials: map[string]Credential{
			"registry.example.com": {
				Type:  AuthBearer,
				Token: "direct-token",
			},
		},
		CredHelpers: map[string]string{
			"registry.example.com": "some-helper",
		},
	}

	cred, ok := store.GetForHost("registry.example.com")
	if !ok {
		t.Fatal("expected credential")
	}
	if AuthBearer != cred.Type {
		t.Errorf("expected bearer type from direct credentials, got %q", cred.Type)
	}
	if "direct-token" != cred.Token {
		t.Errorf("expected direct-token, got %q", cred.Token)
	}
}
