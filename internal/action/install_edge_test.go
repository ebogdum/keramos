package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createMinimalPackage(t *testing.T, dir string) {
	t.Helper()

	keramosYaml := "apiVersion: keramos/v1\nname: test-app\nversion: 1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}
}

func createPackageWithTemplate(t *testing.T, dir string) {
	t.Helper()
	createMinimalPackage(t, dir)

	os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("name: test\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "svc.yaml"), []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: test\n"), 0o644)
}

func TestInstallDryRunNoKubeCalls(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "dry-test",
		Namespace:   "test-ns",
		DryRun:      "client",
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("dry-run install failed: %v", err)
	}

	// Verify no K8s calls were made (client is nil, would panic if called)
	if "pending-install" != string(rel.Status) {
		t.Errorf("expected status pending-install for dry-run, got %s", rel.Status)
	}
	if "" == rel.Manifest {
		t.Error("expected non-empty manifest")
	}
	if "dry-test" != rel.Name {
		t.Errorf("expected release name dry-test, got %s", rel.Name)
	}
}

func TestInstallDryRunRenderedManifestContent(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "content-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("dry-run install failed: %v", err)
	}

	if !strings.Contains(rel.Manifest, "kind: Service") {
		t.Error("manifest should contain Service kind")
	}
}

func TestInstallEmptyPackageNoTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalPackage(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "empty-pkg",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install empty package failed: %v", err)
	}

	if "" != rel.Manifest {
		t.Errorf("expected empty manifest for package with no templates, got %q", rel.Manifest)
	}
}

func TestInstallInvalidValuesFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "bad-vals",
		Namespace:   "default",
		DryRun:      "client",
		ValueFiles:  []string{"/nonexistent/values.yaml"},
	}

	_, err := Install(nil, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error for invalid values file path")
	}
}

func TestInstallEmptyReleaseName(t *testing.T) {
	opts := &InstallOptions{
		ReleaseName: "",
		DryRun:      "client",
	}

	_, err := Install(nil, "/tmp", opts)
	if nil == err {
		t.Fatal("expected error when release name is empty")
	}
}

func TestInstallDryRunDefaultNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "ns-test",
		Namespace:   "", // empty namespace
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "default" != rel.Namespace {
		t.Errorf("expected default namespace, got %s", rel.Namespace)
	}
}

func TestInstallDryRunWithSetOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "set-test",
		Namespace:   "default",
		DryRun:      "client",
		Sets:        []string{"replicas=5"},
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}

	if 5 != rel.Values["replicas"] {
		t.Errorf("expected replicas=5, got %v", rel.Values["replicas"])
	}
}

func TestInstallDryRunReleaseMeta(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "meta-test",
		Namespace:   "prod",
		DryRun:      "client",
		Description: "test install",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}

	if 1 != rel.Revision {
		t.Errorf("expected revision 1, got %d", rel.Revision)
	}
	if "test-app" != rel.Package.Name {
		t.Errorf("expected package name test-app, got %s", rel.Package.Name)
	}
	if "1.0.0" != rel.Package.Version {
		t.Errorf("expected package version 1.0.0, got %s", rel.Package.Version)
	}
	if "test install" != rel.Info.Description {
		t.Errorf("expected description 'test install', got %s", rel.Info.Description)
	}
}

func TestInstallInvalidPackagePath(t *testing.T) {
	opts := &InstallOptions{
		ReleaseName: "bad-pkg",
		Namespace:   "default",
		DryRun:      "client",
	}

	_, err := Install(nil, "/nonexistent/package/path", opts)
	if nil == err {
		t.Fatal("expected error for nonexistent package path")
	}
}
