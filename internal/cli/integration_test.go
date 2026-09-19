package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/v2/internal/repo"
)

// --- verifyInstalledDigests ---

func TestVerifyInstalledDigests_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)

	// No keramos.lock exists, should succeed silently
	err := verifyInstalledDigests(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyInstalledDigests_EmptyLockFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte("apiVersion: v1\ndependencies: []\n"), 0o644)

	err := verifyInstalledDigests(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- printLayerTree ---

func TestPrintDependencyTree_NoDeps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: myapp\nversion: 1.0.0\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	err := printLayerTree(root, dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "myapp@1.0.0") {
		t.Errorf("expected package name and version in output, got %q", out)
	}
}

func TestPrintDependencyTree_WithDeps(t *testing.T) {
	dir := t.TempDir()
	keramosYaml := `apiVersion: keramos/v1
name: myapp
version: 1.0.0
dependencies:
  - name: lib-a
    version: "^1.0.0"
    repository: http://example.com
  - name: lib-b
    version: "^2.0.0"
    repository: http://example.com
`
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644)

	lockYaml := `apiVersion: v1
generated: 2024-01-01T00:00:00Z
dependencies:
  - name: lib-a
    version: "1.2.0"
    repository: http://example.com
    digest: sha256:abc123
  - name: lib-b
    version: "2.1.0"
    repository: http://example.com
    digest: sha256:def456
`
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte(lockYaml), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	err := printLayerTree(root, dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "myapp@1.0.0") {
		t.Error("expected root package info")
	}
	if !strings.Contains(out, "lib-a@1.2.0") {
		t.Error("expected lib-a dependency")
	}
	if !strings.Contains(out, "lib-b@2.1.0") {
		t.Error("expected lib-b dependency")
	}
}

func TestPrintDependencyTree_UnresolvedDep(t *testing.T) {
	dir := t.TempDir()
	keramosYaml := `apiVersion: keramos/v1
name: myapp
version: 1.0.0
dependencies:
  - name: missing-dep
    version: "^1.0.0"
    repository: http://example.com
`
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644)

	lockYaml := `apiVersion: v1
generated: 2024-01-01T00:00:00Z
dependencies: []
`
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte(lockYaml), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	err := printLayerTree(root, dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "not resolved") {
		t.Error("expected 'not resolved' for missing dep")
	}
}

func TestPrintDependencyTree_WithTransitiveDeps(t *testing.T) {
	dir := t.TempDir()
	keramosYaml := `apiVersion: keramos/v1
name: myapp
version: 1.0.0
dependencies:
  - name: lib-a
    version: "^1.0.0"
    repository: http://example.com
`
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644)

	lockYaml := `apiVersion: v1
generated: 2024-01-01T00:00:00Z
dependencies:
  - name: lib-a
    version: "1.0.0"
    repository: http://example.com
    digest: sha256:abc
    dependencies:
      - lib-b
  - name: lib-b
    version: "2.0.0"
    repository: http://example.com
    digest: sha256:def
`
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte(lockYaml), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	err := printLayerTree(root, dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "lib-a@1.0.0") {
		t.Error("expected lib-a")
	}
	if !strings.Contains(out, "lib-b@2.0.0") {
		t.Error("expected lib-b as transitive dep")
	}
}

func TestPrintDependencyTree_DeduplicatedDeps(t *testing.T) {
	dir := t.TempDir()
	keramosYaml := `apiVersion: keramos/v1
name: myapp
version: 1.0.0
dependencies:
  - name: lib-a
    version: "^1.0.0"
    repository: http://example.com
  - name: lib-b
    version: "^2.0.0"
    repository: http://example.com
`
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644)

	lockYaml := `apiVersion: v1
generated: 2024-01-01T00:00:00Z
dependencies:
  - name: lib-a
    version: "1.0.0"
    repository: http://example.com
    digest: sha256:abc
    dependencies:
      - lib-c
  - name: lib-b
    version: "2.0.0"
    repository: http://example.com
    digest: sha256:def
    dependencies:
      - lib-c
  - name: lib-c
    version: "3.0.0"
    repository: http://example.com
    digest: sha256:ghi
`
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte(lockYaml), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	err := printLayerTree(root, dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "deduped") {
		t.Error("expected deduped indicator for shared dependency")
	}
}

// --- keyringDir ---

func TestKeyringDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := keyringDir()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" == dir {
		t.Fatal("expected non-empty dir")
	}
	if !strings.Contains(dir, "keyring") {
		t.Errorf("expected 'keyring' in path, got %q", dir)
	}
}

// --- verifyArchiveSignature ---

func TestVerifyArchiveSignature_NoProvFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	archivePath := filepath.Join(tmpDir, "test.keramos.tgz")
	os.WriteFile(archivePath, []byte("fake"), 0o644)

	err := verifyArchiveSignature(archivePath)
	if nil == err {
		t.Fatal("expected error for missing .prov file")
	}
	if !strings.Contains(err.Error(), "no provenance file") {
		t.Errorf("expected 'no provenance file' in error, got: %v", err)
	}
}

// --- verifyInstalledSignatures ---

func TestVerifyInstalledSignatures_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)

	err := verifyInstalledSignatures(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyInstalledSignatures_NoPackagesDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte("apiVersion: v1\ndependencies: []\n"), 0o644)

	err := verifyInstalledSignatures(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- matchesKeywordBroad ---

func TestMatchesKeywordBroad(t *testing.T) {
	entries := []repo.IndexEntry{
		{Description: "A Redis cache package"},
	}

	// Matches by name
	if !matchesKeywordBroad("redis", entries, "redis") {
		t.Error("expected match by name")
	}

	// Matches by description
	if !matchesKeywordBroad("other-name", entries, "cache") {
		t.Error("expected match by description")
	}

	// No match
	if matchesKeywordBroad("other-name", entries, "postgres") {
		t.Error("expected no match")
	}
}

// --- contains (test_cmd.go helper) ---

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello", "hello", true},
		{"hello", "xyz", false},
		{"hi", "hello", false},
		{"", "x", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		result := contains(tt.s, tt.substr)
		if tt.want != result {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.want)
		}
	}
}

// --- Debug command with hooks fixture ---

func TestDebugCommand_WithBase(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-base", "overlay")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"debug", "--trace", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("debug command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== RENDERED OUTPUT ===") {
		t.Error("expected rendered output section")
	}
}

// --- Template command with base ---

func TestTemplateCommand_WithBasePackage(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-base", "overlay")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"template", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("template command failed: %v", err)
	}
}

// --- Install with notes ---

func TestInstallCommand_DryRunWithNotes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: notes-test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("name: notes-test\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "templates", "notes.yaml"), []byte("message: |\n  Thanks for installing!\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "notes-rel", dir, "--dry-run", "client"})

	if err := root.Execute(); nil != err {
		t.Fatalf("install dry-run failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NOTES:") {
		t.Errorf("expected NOTES in output, got %q", out)
	}
	if !strings.Contains(out, "Thanks for installing!") {
		t.Errorf("expected notes content in output, got %q", out)
	}
}

// --- Install generate-name with resolve ---

func TestInstallCommand_GenerateNameFromPackage(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "--generate-name", "--dry-run", "client", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("install --generate-name failed: %v", err)
	}
}

// --- Install with profile ---

func TestInstallCommand_DryRunWithProfile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "profile-rel", dir, "--dry-run", "client", "--profile", "prod"})

	if err := root.Execute(); nil != err {
		t.Fatalf("install --profile failed: %v", err)
	}
}

// --- Install with set and values ---

func TestInstallCommand_DryRunWithSets(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "set-rel", dir, "--dry-run", "client", "--set", "name=custom"})

	if err := root.Execute(); nil != err {
		t.Fatalf("install --set failed: %v", err)
	}
}

// --- Lint with errors and warnings display ---

func TestLintCommand_ErrorsWithFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: not-semver\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", dir})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid semver")
	}
	out := buf.String()
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("expected [ERROR] in output, got %q", out)
	}
	if !strings.Contains(out, "keramos.yaml") {
		t.Errorf("expected file reference in error output, got %q", out)
	}
}

// --- Dependency list command ---

func TestDependencyListCommand_NoDeps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dependency", "list", dir})

	err := root.Execute()
	if nil != err {
		t.Fatalf("dep list failed: %v", err)
	}
}

// --- verifyInstalledDigests with deps ---

func TestVerifyInstalledDigests_WithMismatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)

	lockYaml := `apiVersion: v1
generated: 2024-01-01T00:00:00Z
dependencies:
  - name: lib-a
    version: "1.0.0"
    repository: http://example.com
    digest: sha256:abc
`
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte(lockYaml), 0o644)

	// No packages dir - CheckInstalledVersion will return "" so no mismatch
	err := verifyInstalledDigests(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- verifyInstalledSignatures with packages dir but no archives ---

func TestVerifyInstalledSignatures_EmptyPackagesDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte("apiVersion: v1\ndependencies: []\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "packages"), 0o755)

	err := verifyInstalledSignatures(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyInstalledSignatures_WithNonArchiveFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte("apiVersion: v1\ndependencies: []\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "packages"), 0o755)
	// Non-archive file should be skipped
	os.WriteFile(filepath.Join(dir, "packages", "readme.txt"), []byte("info"), 0o644)

	err := verifyInstalledSignatures(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyInstalledSignatures_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "keramos.lock"), []byte("apiVersion: v1\ndependencies: []\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "packages", "subdir"), 0o755)

	err := verifyInstalledSignatures(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Lint with values file ---

func TestLintCommand_WithValuesFile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "override.yaml")
	os.WriteFile(overridePath, []byte("name: custom\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", "-f", overridePath, dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("lint command failed: %v", err)
	}
}
