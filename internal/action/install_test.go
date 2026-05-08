package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallDryRun(t *testing.T) {
	// Create a temporary package structure
	tmpDir := t.TempDir()

	// Create keramos.yaml
	keramosYaml := `apiVersion: v1
name: test-app
version: 1.0.0
appVersion: "2.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte(keramosYaml), 0644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}

	// Create values.yaml
	valuesYaml := `replicas: 1
image:
  repository: nginx
  tag: latest
`
	if err := os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesYaml), 0644); nil != err {
		t.Fatalf("failed to write values.yaml: %v", err)
	}

	// Create templates directory
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); nil != err {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Create a simple template (no expressions, just plain YAML)
	deployTmpl := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
`
	if err := os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte(deployTmpl), 0644); nil != err {
		t.Fatalf("failed to write template: %v", err)
	}

	opts := &InstallOptions{
		ReleaseName: "my-release",
		Namespace:   "test-ns",
		DryRun:      "client",
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("dry-run install failed: %v", err)
	}

	if "my-release" != rel.Name {
		t.Errorf("expected release name my-release, got %s", rel.Name)
	}
	if "test-ns" != rel.Namespace {
		t.Errorf("expected namespace test-ns, got %s", rel.Namespace)
	}
	if 1 != rel.Revision {
		t.Errorf("expected revision 1, got %d", rel.Revision)
	}
	if "pending-install" != string(rel.Status) {
		t.Errorf("expected status pending-install for dry-run, got %s", rel.Status)
	}
	if "test-app" != rel.Package.Name {
		t.Errorf("expected package name test-app, got %s", rel.Package.Name)
	}
	if "1.0.0" != rel.Package.Version {
		t.Errorf("expected package version 1.0.0, got %s", rel.Package.Version)
	}
	if "" == rel.Manifest {
		t.Error("expected non-empty manifest")
	}
	if !strings.Contains(rel.Manifest, "Deployment") {
		t.Error("manifest should contain Deployment")
	}
}

func TestInstallDryRunWithValueOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	keramosYaml := `apiVersion: v1
name: override-test
version: 2.0.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte(keramosYaml), 0644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}

	valuesYaml := `replicas: 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte(valuesYaml), 0644); nil != err {
		t.Fatalf("failed to write values.yaml: %v", err)
	}

	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); nil != err {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	deployTmpl := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`
	if err := os.WriteFile(filepath.Join(templatesDir, "cm.yaml"), []byte(deployTmpl), 0644); nil != err {
		t.Fatalf("failed to write template: %v", err)
	}

	// Write an override values file
	overrideFile := filepath.Join(tmpDir, "override.yaml")
	if err := os.WriteFile(overrideFile, []byte("replicas: 5\n"), 0644); nil != err {
		t.Fatalf("failed to write override file: %v", err)
	}

	opts := &InstallOptions{
		ReleaseName: "override-release",
		Namespace:   "default",
		DryRun:      "client",
		ValueFiles:  []string{overrideFile},
		Sets:        []string{"replicas=10"},
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("dry-run install with overrides failed: %v", err)
	}

	// --set should take highest precedence
	replicas, ok := rel.Values["replicas"]
	if !ok {
		t.Fatal("expected replicas in values")
	}
	if 10 != replicas {
		t.Errorf("expected replicas=10 from --set, got %v", replicas)
	}
}

func TestInstallRequiresReleaseName(t *testing.T) {
	opts := &InstallOptions{
		ReleaseName: "",
		DryRun:      "client",
	}

	_, err := Install(nil, "/nonexistent", opts)
	if nil == err {
		t.Fatal("expected error when release name is empty")
	}
}

func TestJoinDocs(t *testing.T) {
	tests := []struct {
		name     string
		docs     []string
		expected string
	}{
		{"empty", nil, ""},
		{"single", []string{"doc1"}, "doc1"},
		{"multiple", []string{"doc1\n", "doc2\n"}, "doc1\n---\ndoc2\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinDocs(tt.docs)
			if tt.expected != result {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
