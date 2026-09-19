package pkg

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
)

func TestLoadPackageMetadataValid(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: keramos/v1
name: my-app
version: 1.0.0
appVersion: "2.0"
description: A test package
type: application
kubeVersion: ">=1.20"
base: ./base
dependencies:
  - name: redis
    version: "7.0.0"
    repository: https://charts.example.com
    condition: redis.enabled
    alias: cache
immutable:
  - .metadata.name
maintainers:
  - name: Alice
    email: alice@example.com
keywords:
  - web
  - api
annotations:
  category: backend
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	meta, err := LoadPackageMetadata(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "keramos/v1" != meta.APIVersion {
		t.Fatalf("expected apiVersion keramos/v1, got %s", meta.APIVersion)
	}
	if "my-app" != meta.Name {
		t.Fatalf("expected name my-app, got %s", meta.Name)
	}
	if "1.0.0" != meta.Version {
		t.Fatalf("expected version 1.0.0, got %s", meta.Version)
	}
	if "2.0" != meta.AppVersion {
		t.Fatalf("expected appVersion 2.0, got %s", meta.AppVersion)
	}
	if "A test package" != meta.Description {
		t.Fatalf("unexpected description: %s", meta.Description)
	}
	if "application" != meta.Type {
		t.Fatalf("expected type application, got %s", meta.Type)
	}
	if ">=1.20" != meta.KubeVersion {
		t.Fatalf("unexpected kubeVersion: %s", meta.KubeVersion)
	}
	if "./base" != meta.Base {
		t.Fatalf("unexpected base: %s", meta.Base)
	}
	if 1 != len(meta.Dependencies) {
		t.Fatalf("expected 1 dependency, got %d", len(meta.Dependencies))
	}
	dep := meta.Dependencies[0]
	if "redis" != dep.Name {
		t.Fatalf("expected dep name redis, got %s", dep.Name)
	}
	if "cache" != dep.Alias {
		t.Fatalf("expected dep alias cache, got %s", dep.Alias)
	}
	if 1 != len(meta.Immutable) {
		t.Fatalf("expected 1 immutable path, got %d", len(meta.Immutable))
	}
	if 1 != len(meta.Maintainers) {
		t.Fatalf("expected 1 maintainer, got %d", len(meta.Maintainers))
	}
	if "Alice" != meta.Maintainers[0].Name {
		t.Fatalf("expected maintainer Alice, got %s", meta.Maintainers[0].Name)
	}
	if 1 != len(meta.Annotations) {
		t.Fatalf("expected 1 annotation, got %d", len(meta.Annotations))
	}
}

func TestLoadPackageMetadataMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPackageMetadata(dir)
	if nil == err {
		t.Fatal("expected error for missing keramos.yaml")
	}
	var he *keramoserr.KeramosError
	if !errors.As(err, &he) {
		t.Fatal("expected KeramosError")
	}
	if keramoserr.ErrPackageInvalid != he.Type {
		t.Fatalf("expected PACKAGE_INVALID, got %s", he.Type)
	}
}

func TestLoadPackageMetadataMissingName(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: keramos/v1
version: 1.0.0
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	_, err := LoadPackageMetadata(dir)
	if nil == err {
		t.Fatal("expected validation error for missing name")
	}
	var he *keramoserr.KeramosError
	if !errors.As(err, &he) {
		t.Fatal("expected KeramosError")
	}
}

func TestLoadPackageMetadataMissingAPIVersion(t *testing.T) {
	dir := t.TempDir()
	content := `name: my-app
version: 1.0.0
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	_, err := LoadPackageMetadata(dir)
	if nil == err {
		t.Fatal("expected validation error for missing apiVersion")
	}
}

func TestLoadPackageMetadataMissingVersion(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: keramos/v1
name: my-app
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	_, err := LoadPackageMetadata(dir)
	if nil == err {
		t.Fatal("expected validation error for missing version")
	}
}

func TestLoadPackageMetadataInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: keramos/v1
  name: bad indent
`
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	_, err := LoadPackageMetadata(dir)
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadValuesValid(t *testing.T) {
	dir := t.TempDir()
	content := `replicas: 3
image:
  repository: nginx
  tag: latest
labels:
  app: web
`
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	vals, err := LoadValues(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 3 != vals["replicas"] {
		t.Fatalf("expected replicas=3, got %v", vals["replicas"])
	}
	imageMap, ok := vals["image"].(Values)
	if !ok {
		t.Fatal("expected image to be a Values map")
	}
	if "nginx" != imageMap["repository"] {
		t.Fatalf("expected image.repository=nginx, got %v", imageMap["repository"])
	}
}

func TestLoadValuesMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadValues(dir)
	if nil == err {
		t.Fatal("expected error for missing values.yaml")
	}
}

func TestLoadValuesInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	content := `key: value
  bad: indent
`
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(content), 0644); nil != err {
		t.Fatal(err)
	}

	_, err := LoadValues(dir)
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadValuesEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(""), 0644); nil != err {
		t.Fatal(err)
	}

	vals, err := LoadValues(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != vals {
		t.Fatalf("expected nil values for empty file, got %v", vals)
	}
}
