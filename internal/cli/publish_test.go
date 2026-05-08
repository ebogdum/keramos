package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/internal/repo"
)

func TestPublishMissingArchive(t *testing.T) {
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"publish", "/nonexistent/archive.keramos.tgz", "--repo", "https://example.com"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for missing archive, got nil")
	}
}

func TestPublishMissingFlags(t *testing.T) {
	// Create a dummy .keramos.tgz file
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test-1.0.0.keramos.tgz")
	if err := os.WriteFile(archivePath, []byte("fake"), 0644); nil != err {
		t.Fatal(err)
	}

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"publish", archivePath})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for missing --repo/--oci flags, got nil")
	}

	if !strings.Contains(err.Error(), "specify --repo or --oci") {
		t.Errorf("expected 'specify --repo or --oci' in error, got: %v", err)
	}
}

func TestPublishInvalidExtension(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(archivePath, []byte("fake"), 0644); nil != err {
		t.Fatal(err)
	}

	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"publish", archivePath, "--repo", "https://example.com"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid extension, got nil")
	}

	if !strings.Contains(err.Error(), ".keramos.tgz") {
		t.Errorf("expected '.keramos.tgz' in error, got: %v", err)
	}
}

func TestPublishValidArchiveExtraction(t *testing.T) {
	// Create a real package and archive it
	srcDir := t.TempDir()
	destDir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: testpkg
version: 1.0.0
description: A test package
`
	if err := os.MkdirAll(filepath.Join(srcDir, "templates"), 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "values.yaml"), []byte("key: value\n"), 0644); nil != err {
		t.Fatal(err)
	}

	archivePath, err := repo.PackageArchive(srcDir, destDir, "")
	if nil != err {
		t.Fatalf("PackageArchive failed: %v", err)
	}

	// Test extraction of metadata from archive
	meta, err := extractArchiveMetadata(archivePath)
	if nil != err {
		t.Fatalf("extractArchiveMetadata failed: %v", err)
	}

	if "testpkg" != meta.Name {
		t.Errorf("expected name 'testpkg', got %q", meta.Name)
	}
	if "1.0.0" != meta.Version {
		t.Errorf("expected version '1.0.0', got %q", meta.Version)
	}
}

func TestPublishScopedArchiveExtraction(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	keramosYAML := `apiVersion: v1
name: "@myorg/redis"
version: 2.0.0
description: Scoped package
`
	if err := os.MkdirAll(filepath.Join(srcDir, "templates"), 0755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "keramos.yaml"), []byte(keramosYAML), 0644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "values.yaml"), []byte("key: value\n"), 0644); nil != err {
		t.Fatal(err)
	}

	archivePath, err := repo.PackageArchive(srcDir, destDir, "")
	if nil != err {
		t.Fatalf("PackageArchive failed: %v", err)
	}

	// Verify scoped archive filename
	expectedName := "@myorg-redis-2.0.0.keramos.tgz"
	if filepath.Base(archivePath) != expectedName {
		t.Errorf("expected archive name %q, got %q", expectedName, filepath.Base(archivePath))
	}

	meta, err := extractArchiveMetadata(archivePath)
	if nil != err {
		t.Fatalf("extractArchiveMetadata failed: %v", err)
	}

	if "@myorg/redis" != meta.Name {
		t.Errorf("expected name '@myorg/redis', got %q", meta.Name)
	}
	if "2.0.0" != meta.Version {
		t.Errorf("expected version '2.0.0', got %q", meta.Version)
	}
}
