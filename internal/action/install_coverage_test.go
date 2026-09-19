package action

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebogdum/keramos/v2/internal/release"
)

func createTestPackage(t *testing.T, dir string) {
	t.Helper()
	keramosYaml := "apiVersion: keramos/v1\nname: test-app\nversion: 1.0.0\nappVersion: \"2.0\"\n"
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte(keramosYaml), 0o644)
	os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("replicas: 1\nname: test\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "deployment.yaml"),
		[]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-app\nspec:\n  replicas: 1\n"), 0o644)
}

func createTestPackageWithHooks(t *testing.T, dir string) {
	t.Helper()
	createTestPackage(t, dir)
	os.MkdirAll(filepath.Join(dir, "hooks"), 0o755)
	hookContent := "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: pre-install-hook\n  namespace: test-ns\n"
	os.WriteFile(filepath.Join(dir, "hooks", "pre-install.yaml"), []byte(hookContent), 0o644)
}

func TestInstallSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "my-release",
		Namespace:   "test-ns",
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}

	if release.StatusDeployed != rel.Status {
		t.Errorf("expected status deployed, got %s", rel.Status)
	}
	if "my-release" != rel.Name {
		t.Errorf("expected name my-release, got %s", rel.Name)
	}
	if "test-ns" != rel.Namespace {
		t.Errorf("expected namespace test-ns, got %s", rel.Namespace)
	}
	if 1 != rel.Revision {
		t.Errorf("expected revision 1, got %d", rel.Revision)
	}
	if 0 == len(mock.appliedManifests) {
		t.Error("expected manifests to be applied")
	}

	// Verify release is stored (can be retrieved)
	storage := release.NewSecretStorage(mock.clientset, "test-ns")
	stored, getErr := storage.Last("my-release")
	if nil != getErr {
		t.Fatalf("failed to get stored release: %v", getErr)
	}
	if release.StatusDeployed != stored.Status {
		t.Errorf("stored release status should be deployed, got %s", stored.Status)
	}
}

func TestInstallWithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackageWithHooks(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "hooks-release",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install with hooks failed: %v", err)
	}

	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	// Hooks should have been applied (pre-install hook manifest + main manifest)
	if 0 == len(mock.appliedManifests) {
		t.Error("expected manifests applied")
	}
}

func TestInstallApplyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.applyErr = errors.New("connection refused")

	opts := &InstallOptions{
		ReleaseName: "fail-release",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on apply failure")
	}
	if nil == rel {
		t.Fatal("expected release to be returned even on failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestInstallWaitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.waitErr = errors.New("timeout waiting for readiness")

	opts := &InstallOptions{
		ReleaseName: "wait-fail",
		Namespace:   "test-ns",
		Wait:        true,
		Timeout:     30 * time.Second,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on wait failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestInstallAtomicApplyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.applyErr = errors.New("apply failed")

	opts := &InstallOptions{
		ReleaseName: "atomic-fail",
		Namespace:   "test-ns",
		Atomic:      true,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on atomic apply failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
	// Atomic should trigger cleanup (delete manifests)
	if 0 == len(mock.deletedManifests) {
		t.Error("expected atomic cleanup to delete manifests")
	}
}

func TestInstallAtomicWaitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.waitErr = errors.New("wait timed out")

	opts := &InstallOptions{
		ReleaseName: "atomic-wait-fail",
		Namespace:   "test-ns",
		Atomic:      true,
		Wait:        true,
		Timeout:     30 * time.Second,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on atomic wait failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
	if 0 == len(mock.deletedManifests) {
		t.Error("expected atomic cleanup to delete manifests")
	}
}

func TestInstallNamespaceCreation(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("new-ns")
	opts := &InstallOptions{
		ReleaseName: "ns-create",
		Namespace:   "new-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestInstallNamespaceCreationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("bad-ns")
	mock.createNSErr = errors.New("forbidden")

	opts := &InstallOptions{
		ReleaseName:     "ns-fail",
		Namespace:       "bad-ns",
		CreateNamespace: true,
	}

	_, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on namespace creation failure")
	}
}

func TestInstallServerDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "server-dry",
		Namespace:   "test-ns",
		DryRun:      "server",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("server dry-run failed: %v", err)
	}
	if release.StatusPendingInstall != rel.Status {
		t.Errorf("expected pending-install, got %s", rel.Status)
	}
	if 0 == len(mock.dryRunManifests) {
		t.Error("expected DryRunApply to be called")
	}
	// Real apply should NOT be called
	if 0 != len(mock.appliedManifests) {
		t.Error("expected no real apply for dry-run")
	}
}

func TestInstallServerDryRunFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.dryRunErr = errors.New("validation failed")

	opts := &InstallOptions{
		ReleaseName: "server-dry-fail",
		Namespace:   "test-ns",
		DryRun:      "server",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on server dry-run failure")
	}
	if nil == rel {
		t.Fatal("expected release to be returned")
	}
}

func TestInstallWithValuesAndSets(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	overrideFile := filepath.Join(tmpDir, "custom.yaml")
	os.WriteFile(overrideFile, []byte("env: staging\n"), 0o644)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "vals-test",
		Namespace:   "test-ns",
		ValueFiles:  []string{overrideFile},
		Sets:        []string{"replicas=3"},
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "staging" != rel.Values["env"] {
		t.Errorf("expected env=staging, got %v", rel.Values["env"])
	}
	if 3 != rel.Values["replicas"] {
		t.Errorf("expected replicas=3, got %v", rel.Values["replicas"])
	}
}

func TestInstallWithProfile(t *testing.T) {
	dir := filepath.Join(fixturesPath(), "with-profiles")

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "profile-test",
		Namespace:   "test-ns",
		Profile:     "prod",
	}

	rel, err := Install(mock, dir, opts)
	if nil != err {
		t.Fatalf("install with profile failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	if 5 != rel.Values["replicas"] {
		t.Errorf("expected replicas=5 from prod profile, got %v", rel.Values["replicas"])
	}
}

func TestInstallWithWaitSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "wait-ok",
		Namespace:   "test-ns",
		Wait:        true,
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install with wait failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestInstallNamespaceFromClient(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("client-ns")
	opts := &InstallOptions{
		ReleaseName: "ns-from-client",
		Namespace:   "", // empty, should use client namespace
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "client-ns" != rel.Namespace {
		t.Errorf("expected client-ns, got %s", rel.Namespace)
	}
}

func TestInstallManifestContent(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "manifest-check",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if !strings.Contains(rel.Manifest, "Deployment") {
		t.Error("manifest should contain Deployment")
	}
	if 0 == len(mock.appliedManifests) {
		t.Error("expected manifests to be applied")
	}
	if !strings.Contains(mock.appliedManifests[0], "Deployment") {
		t.Error("applied manifest should contain Deployment")
	}
}

func TestInstallPackageMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "meta-check",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "test-app" != rel.Package.Name {
		t.Errorf("expected package name test-app, got %s", rel.Package.Name)
	}
	if "1.0.0" != rel.Package.Version {
		t.Errorf("expected version 1.0.0, got %s", rel.Package.Version)
	}
	if "2.0" != rel.Package.AppVersion {
		t.Errorf("expected appVersion 2.0, got %s", rel.Package.AppVersion)
	}
}

func TestInstallWithDescription(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "desc-check",
		Namespace:   "test-ns",
		Description: "Initial deployment",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "Initial deployment" != rel.Info.Description {
		t.Errorf("expected description 'Initial deployment', got %q", rel.Info.Description)
	}
}

func TestInstallDefaultTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	// Wait enabled but no timeout set - should use default
	opts := &InstallOptions{
		ReleaseName: "timeout-default",
		Namespace:   "test-ns",
		Wait:        true,
		Timeout:     0,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestInstallServerDryRunNilClient(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "server-dry-nil",
		Namespace:   "test-ns",
		DryRun:      "server",
	}

	_, err := Install(nil, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error for server dry-run with nil client")
	}
}

func TestInstallUserValuesTracked(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "user-tracked",
		Namespace:   "test-ns",
		Sets:        []string{"custom=myval"},
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if nil == rel.UserValues {
		t.Fatal("expected non-nil UserValues")
	}
	if "myval" != rel.UserValues["custom"] {
		t.Errorf("expected custom=myval in UserValues, got %v", rel.UserValues["custom"])
	}
}

func TestInstallWithHooksPreInstallFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackageWithHooks(t, tmpDir)

	// Create mock that fails on first apply (hook manifest)
	callCount := 0
	mock := newMockClient("test-ns")
	// We need hooks to actually get parsed and executed. The hook is a Job.
	// ExecuteHooks calls client.ApplyManifests on the hook manifest.
	// If that fails, the install should fail with hook error.
	// Let's make ApplyManifests fail - this will fail during hook execution.
	mock.applyErr = errors.New("hook apply failed")

	opts := &InstallOptions{
		ReleaseName: "hook-fail",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	_ = callCount
	if nil == err {
		t.Fatal("expected error on hook apply failure")
	}
	if nil == rel {
		t.Fatal("expected release to be returned")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestInstallWithHooksPreInstallFailureAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackageWithHooks(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.applyErr = errors.New("hook apply failed")

	opts := &InstallOptions{
		ReleaseName: "hook-fail-atomic",
		Namespace:   "test-ns",
		Atomic:      true,
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on atomic hook failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestInstallCapabilitiesError(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.capsErr = errors.New("discovery failed")

	opts := &InstallOptions{
		ReleaseName: "caps-err",
		Namespace:   "test-ns",
	}

	// Should succeed even if capabilities fail (falls back to empty caps)
	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install should succeed even with caps error: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestInstallWithNotesExtracted(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	notesContent := "message: |\n  Thank you for installing.\n"
	os.WriteFile(filepath.Join(tmpDir, "templates", "notes.yaml"), []byte(notesContent), 0o644)

	mock := newMockClient("test-ns")
	opts := &InstallOptions{
		ReleaseName: "notes-test",
		Namespace:   "test-ns",
	}

	rel, err := Install(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "" == rel.Notes {
		t.Error("expected non-empty notes")
	}
	if strings.Contains(rel.Manifest, "message:") {
		t.Error("notes should be extracted from manifest")
	}
}
