package action

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLint_ValidSimplePackage(t *testing.T) {
	result, err := Lint("../../test/fixtures/simple", nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("unexpected error: [%s] %s: %s", msg.Severity, msg.File, msg.Message)
		}
	}
}

func TestLint_ValidWithBasePackage(t *testing.T) {
	result, err := Lint("../../test/fixtures/with-base/overlay", nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("unexpected error: [%s] %s: %s", msg.Severity, msg.File, msg.Message)
		}
	}
}

func TestLint_ValidWithProfilesPackage(t *testing.T) {
	result, err := Lint("../../test/fixtures/with-profiles", nil, nil, "staging", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("unexpected error: [%s] %s: %s", msg.Severity, msg.File, msg.Message)
		}
	}
}

func TestLint_MissingKeramosYAML(t *testing.T) {
	dir := t.TempDir()
	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected errors for missing keramos.yaml")
	}

	found := false
	for _, msg := range result.Errors {
		if "keramos.yaml" == msg.File {
			found = true
		}
	}
	if !found {
		t.Error("expected error referencing keramos.yaml")
	}
}

func TestLint_InvalidAPIVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: invalid/v2\nname: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for wrong apiVersion")
	}

	found := false
	for _, msg := range result.Errors {
		if "keramos.yaml" == msg.File && contains(msg.Message, "keramos/v1") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about apiVersion keramos/v1")
	}
}

func TestLint_InvalidSemver(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: not-a-version\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for invalid semver")
	}

	found := false
	for _, msg := range result.Errors {
		if contains(msg.Message, "semver") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about semver")
	}
}

func TestLint_InvalidValuesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	writeFile(t, dir, "values.yaml", "{{invalid yaml")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for invalid values.yaml")
	}
}

func TestLint_InvalidSchema(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	writeFile(t, dir, "values.schema.json", "not valid json{{{")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for invalid schema")
	}
}

func TestLint_EmptyTemplatesWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if 0 == len(result.Warnings) {
		t.Fatal("expected warning for empty templates/")
	}
}

func TestLint_StrictFailsOnWarnings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", true)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	// strict mode doesn't change LintResult, it's up to the CLI to interpret
	if 0 == len(result.Warnings) {
		t.Fatal("expected warnings present")
	}
}

func TestLint_MissingProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "nonexistent", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for missing profile")
	}
}

func TestLint_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected errors for missing name and version")
	}
	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(result.Errors))
	}
}

// --- helpers ---

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, name)
	if err := os.WriteFile(fullPath, []byte(content), 0644); nil != err {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); nil != err {
		t.Fatalf("failed to create dir %s: %v", path, err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
