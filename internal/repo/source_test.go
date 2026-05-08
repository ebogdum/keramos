package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSource_Local(t *testing.T) {
	tests := []struct {
		input    string
		wantType SourceType
		wantURL  string
		wantSub  string
	}{
		{"../base", SourceLocal, "../base", ""},
		{"./lib/common", SourceLocal, "./lib/common", ""},
		{"/absolute/path", SourceLocal, "/absolute/path", ""},
	}

	for _, tt := range tests {
		srcType, url, sub := ParseSource(tt.input)
		if tt.wantType != srcType {
			t.Errorf("ParseSource(%q) type = %d, want %d", tt.input, srcType, tt.wantType)
		}
		if tt.wantURL != url {
			t.Errorf("ParseSource(%q) url = %q, want %q", tt.input, url, tt.wantURL)
		}
		if tt.wantSub != sub {
			t.Errorf("ParseSource(%q) sub = %q, want %q", tt.input, sub, tt.wantSub)
		}
	}
}

func TestParseSource_Registry(t *testing.T) {
	tests := []struct {
		input   string
		wantURL string
	}{
		{"https://registry.example.com", "https://registry.example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://charts.bitnami.com/bitnami", "https://charts.bitnami.com/bitnami"},
	}

	for _, tt := range tests {
		srcType, url, sub := ParseSource(tt.input)
		if SourceRegistry != srcType {
			t.Errorf("ParseSource(%q) type = %d, want SourceRegistry", tt.input, srcType)
		}
		if tt.wantURL != url {
			t.Errorf("ParseSource(%q) url = %q, want %q", tt.input, url, tt.wantURL)
		}
		if "" != sub {
			t.Errorf("ParseSource(%q) sub = %q, want empty", tt.input, sub)
		}
	}
}

func TestParseSource_Git(t *testing.T) {
	tests := []struct {
		input   string
		wantURL string
		wantSub string
	}{
		{
			"git::https://github.com/myorg/configs.git",
			"https://github.com/myorg/configs.git",
			"",
		},
		{
			"git::https://github.com/myorg/configs.git//keramos-packages/service",
			"https://github.com/myorg/configs.git",
			"keramos-packages/service",
		},
		{
			"git::git@github.com:org/repo.git",
			"git@github.com:org/repo.git",
			"",
		},
		{
			"git::https://github.com/org/repo.git//deep/nested/path",
			"https://github.com/org/repo.git",
			"deep/nested/path",
		},
	}

	for _, tt := range tests {
		srcType, url, sub := ParseSource(tt.input)
		if SourceGit != srcType {
			t.Errorf("ParseSource(%q) type = %d, want SourceGit", tt.input, srcType)
		}
		if tt.wantURL != url {
			t.Errorf("ParseSource(%q) url = %q, want %q", tt.input, url, tt.wantURL)
		}
		if tt.wantSub != sub {
			t.Errorf("ParseSource(%q) sub = %q, want %q", tt.input, sub, tt.wantSub)
		}
	}
}

func TestIsCommitSHA(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"a1b2c3d4e5f6", true},
		{"abc1234", true},
		{"ABCDEF1234567890", true},
		{"main", false},
		{"v1.0.0", false},
		{"abc12", false},
		{"ghijkl", false},
	}

	for _, tt := range tests {
		got := isCommitSHA(tt.input)
		if tt.want != got {
			t.Errorf("isCommitSHA(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolveLocalSource(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); nil != err {
		t.Fatal(err)
	}

	got, err := resolveLocalSource("sub", tmpDir)
	if nil != err {
		t.Fatalf("resolveLocalSource(sub) error: %v", err)
	}
	if subDir != got {
		t.Errorf("resolveLocalSource(sub) = %q, want %q", got, subDir)
	}

	got, err = resolveLocalSource(subDir, "/some/other/base")
	if nil != err {
		t.Fatalf("resolveLocalSource(abs) error: %v", err)
	}
	if subDir != got {
		t.Errorf("resolveLocalSource(abs) = %q, want %q", got, subDir)
	}

	_, err = resolveLocalSource("nonexistent", tmpDir)
	if nil == err {
		t.Error("expected error for non-existent local source")
	}
}

func TestFetchGitSource_LocalRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(repoDir, 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0644); nil != err {
		t.Fatal(err)
	}

	runTestGit(t, repoDir, "init")
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "init")

	result, err := FetchGitSource(repoDir, "", "", cacheDir)
	if nil != err {
		t.Fatalf("FetchGitSource error: %v", err)
	}

	if !fileExists(filepath.Join(result, "keramos.yaml")) {
		t.Error("expected keramos.yaml in fetched git source")
	}
}

func TestFetchGitSource_WithSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	cacheDir := filepath.Join(tmpDir, "cache")
	pkgDir := filepath.Join(repoDir, "packages", "myapp")

	if err := os.MkdirAll(pkgDir, 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: myapp\nversion: 1.0.0\n"), 0644); nil != err {
		t.Fatal(err)
	}

	runTestGit(t, repoDir, "init")
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "init")

	result, err := FetchGitSource(repoDir, "", "packages/myapp", cacheDir)
	if nil != err {
		t.Fatalf("FetchGitSource with subdir error: %v", err)
	}

	if !fileExists(filepath.Join(result, "keramos.yaml")) {
		t.Error("expected keramos.yaml in subdir of fetched git source")
	}
}

func TestFetchGitSource_InvalidSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(repoDir, 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0644); nil != err {
		t.Fatal(err)
	}

	runTestGit(t, repoDir, "init")
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "init")

	_, err := FetchGitSource(repoDir, "", "nonexistent/path", cacheDir)
	if nil == err {
		t.Error("expected error for invalid subdir")
	}
}

func TestFetchGitSource_TagCheckout(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	cacheDir := filepath.Join(tmpDir, "cache")

	if err := os.MkdirAll(repoDir, 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0644); nil != err {
		t.Fatal(err)
	}

	runTestGit(t, repoDir, "init")
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "v1")
	runTestGit(t, repoDir, "tag", "v1.0.0")

	if err := os.WriteFile(filepath.Join(repoDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 2.0.0\n"), 0644); nil != err {
		t.Fatal(err)
	}
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "v2")

	result, err := FetchGitSource(repoDir, "v1.0.0", "", cacheDir)
	if nil != err {
		t.Fatalf("FetchGitSource tag checkout error: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(result, "keramos.yaml"))
	if nil != readErr {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(data), "version: 1.0.0") {
		t.Errorf("expected version 1.0.0 at tag v1.0.0, got: %s", string(data))
	}
}

func TestGitResolveCommit(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(repoDir, 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello"), 0644); nil != err {
		t.Fatal(err)
	}

	runTestGit(t, repoDir, "init")
	runTestGit(t, repoDir, "add", ".")
	runTestGit(t, repoDir, "commit", "-m", "init")

	commit, err := GitResolveCommit(repoDir)
	if nil != err {
		t.Fatalf("GitResolveCommit error: %v", err)
	}

	if 40 != len(commit) {
		t.Errorf("expected 40-char SHA, got %d chars: %s", len(commit), commit)
	}

	if !isCommitSHA(commit) {
		t.Errorf("expected valid hex SHA, got: %s", commit)
	}
}

func TestHashString(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	h3 := hashString("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different inputs should produce different hashes")
	}
	if 64 != len(h1) {
		t.Errorf("expected 64-char hex hash, got %d", len(h1))
	}
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	output, err := cmd.CombinedOutput()
	if nil != err {
		t.Fatalf("git %v failed: %s\n%s", args, err, string(output))
	}
}
