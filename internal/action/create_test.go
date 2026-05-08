package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_ScaffoldsCorrectly(t *testing.T) {
	dir := t.TempDir()
	name := "myapp"

	if err := Create(name, dir); nil != err {
		t.Fatalf("Create returned error: %v", err)
	}

	expectedFiles := []string{
		"keramos.yaml",
		"values.yaml",
		"templates/deployment.yaml",
		"templates/service.yaml",
		"templates/_helpers.yaml",
		"templates/notes.yaml",
		".keramosignore",
	}

	for _, f := range expectedFiles {
		fullPath := filepath.Join(dir, name, f)
		if _, err := os.Stat(fullPath); nil != err {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}
}

func TestCreate_KeramosYAMLContent(t *testing.T) {
	dir := t.TempDir()
	name := "testpkg"

	if err := Create(name, dir); nil != err {
		t.Fatalf("Create returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, name, "keramos.yaml"))
	if nil != err {
		t.Fatalf("failed to read keramos.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "apiVersion: keramos/v1") {
		t.Error("keramos.yaml missing apiVersion: keramos/v1")
	}
	if !strings.Contains(content, "name: testpkg") {
		t.Error("keramos.yaml missing name")
	}
	if !strings.Contains(content, "version: 0.1.0") {
		t.Error("keramos.yaml missing version")
	}
}

func TestCreate_ErrorsIfDirectoryExists(t *testing.T) {
	dir := t.TempDir()
	name := "existing"

	mkDir(t, filepath.Join(dir, name))

	err := Create(name, dir)
	if nil == err {
		t.Fatal("expected error when directory exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %s", err.Error())
	}
}

func TestCreate_PackagePassesLint(t *testing.T) {
	dir := t.TempDir()
	name := "lintable"

	if err := Create(name, dir); nil != err {
		t.Fatalf("Create returned error: %v", err)
	}

	pkgPath := filepath.Join(dir, name)
	result, err := Lint(pkgPath, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("lint error: [%s] %s: %s", msg.Severity, msg.File, msg.Message)
		}
		t.Fatal("created package did not pass lint")
	}
}

func TestCreate_PackageRendersWithTemplate(t *testing.T) {
	dir := t.TempDir()
	name := "renderable"

	if err := Create(name, dir); nil != err {
		t.Fatalf("Create returned error: %v", err)
	}

	pkgPath := filepath.Join(dir, name)

	// Use the lint render path to verify templates render
	result, err := Lint(pkgPath, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("render error: [%s] %s: %s", msg.Severity, msg.File, msg.Message)
		}
		t.Fatal("created package templates did not render")
	}
}
