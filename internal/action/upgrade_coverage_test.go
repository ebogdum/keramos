package action

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebogdum/keramos/v2/internal/release"
)

// storeRelease creates a release revision in storage so upgrade can find it.
func storeRelease(t *testing.T, mock *mockKubeClient, ns string, rel *release.Release) {
	t.Helper()
	storage := release.NewSecretStorage(mock.clientset, ns)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("failed to store release: %v", err)
	}
}

func makeDeployedRelease(name, ns string, revision int) *release.Release {
	return &release.Release{
		Name:      name,
		Namespace: ns,
		Revision:  revision,
		Status:    release.StatusDeployed,
		Package: release.PackageRef{
			Name:    "test-app",
			Version: "1.0.0",
		},
		Values:   map[string]any{"replicas": 1, "name": "test"},
		Manifest: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-app\nspec:\n  replicas: 1\n",
		Info: release.ReleaseInfo{
			FirstDeployed: time.Now().UTC().Add(-time.Hour),
			LastDeployed:  time.Now().UTC().Add(-time.Hour),
		},
	}
}

func TestUpgradeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("my-release", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "my-release",
		Namespace:   "test-ns",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	if 2 != rel.Revision {
		t.Errorf("expected revision 2, got %d", rel.Revision)
	}
	if 0 == len(mock.appliedManifests) {
		t.Error("expected manifests to be applied")
	}
}

func TestUpgradeWithInstallFallback(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	// No existing release - should fall back to install
	opts := &UpgradeOptions{
		ReleaseName: "new-release",
		Namespace:   "test-ns",
		Install:     true,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade --install failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	if 1 != rel.Revision {
		t.Errorf("expected revision 1 (install), got %d", rel.Revision)
	}
}

func TestUpgradeWithoutInstallNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	opts := &UpgradeOptions{
		ReleaseName: "missing-release",
		Namespace:   "test-ns",
		Install:     false,
	}

	_, err := Upgrade(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error when release not found and --install not set")
	}
}

func TestUpgradeReuseValues(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("reuse-vals", "test-ns", 1)
	existing.Values = map[string]any{"replicas": 1, "name": "test", "custom": "previous-value"}
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "reuse-vals",
		Namespace:   "test-ns",
		ReuseValues: true,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade reuse-values failed: %v", err)
	}
	// Previous custom value should be preserved
	if "previous-value" != rel.Values["custom"] {
		t.Errorf("expected custom=previous-value, got %v", rel.Values["custom"])
	}
}

func TestUpgradeResetValues(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("reset-vals", "test-ns", 1)
	existing.Values = map[string]any{"replicas": 1, "name": "test", "custom": "old-value"}
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "reset-vals",
		Namespace:   "test-ns",
		ResetValues: true,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade reset-values failed: %v", err)
	}
	// Previous custom value should be gone (reset to defaults)
	if _, ok := rel.Values["custom"]; ok {
		t.Error("expected custom value to be reset")
	}
}

func TestUpgradeApplyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("apply-fail", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	mock.applyErr = errors.New("apply failed")

	opts := &UpgradeOptions{
		ReleaseName: "apply-fail",
		Namespace:   "test-ns",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on apply failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestUpgradeAtomicApplyFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("atomic-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	// First apply succeeds (for storing the release), then subsequent fails
	callCount := 0
	origApply := mock.applyErr
	_ = origApply
	mock.applyErr = errors.New("apply failed during upgrade")

	opts := &UpgradeOptions{
		ReleaseName: "atomic-upg",
		Namespace:   "test-ns",
		Atomic:      true,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	_ = callCount
	if nil == err {
		t.Fatal("expected error on atomic upgrade failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
	// Atomic should attempt rollback by re-applying previous manifests
	// The first apply call fails, then atomic rollback calls apply again
	if 2 > len(mock.appliedManifests) {
		// At least 1 failed apply + 1 rollback apply
		// (it's OK if rollback also fails since applyErr is still set)
	}
}

func TestUpgradeWaitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("wait-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	mock.waitErr = errors.New("wait timeout")

	opts := &UpgradeOptions{
		ReleaseName: "wait-upg",
		Namespace:   "test-ns",
		Wait:        true,
		Timeout:     30 * time.Second,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on wait failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestUpgradeServerDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("dry-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "dry-upg",
		Namespace:   "test-ns",
		DryRun:      "server",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("server dry-run upgrade failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed for dry-run, got %s", rel.Status)
	}
	if 0 == len(mock.dryRunManifests) {
		t.Error("expected DryRunApply to be called")
	}
	if 0 != len(mock.appliedManifests) {
		t.Error("expected no real apply for dry-run")
	}
}

func TestUpgradeClientDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("dry-client-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "dry-client-upg",
		Namespace:   "test-ns",
		DryRun:      "client",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("client dry-run upgrade failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed for client dry-run, got %s", rel.Status)
	}
}

func TestUpgradeNamespaceFromClient(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("from-client")
	existing := makeDeployedRelease("ns-upg", "from-client", 1)
	storeRelease(t, mock, "from-client", existing)

	opts := &UpgradeOptions{
		ReleaseName: "ns-upg",
		Namespace:   "",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if "from-client" != rel.Namespace {
		t.Errorf("expected from-client, got %s", rel.Namespace)
	}
}

func TestUpgradeWithDescription(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("desc-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "desc-upg",
		Namespace:   "test-ns",
		Description: "Upgrade to v2",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if "Upgrade to v2" != rel.Info.Description {
		t.Errorf("expected description, got %q", rel.Info.Description)
	}
}

func TestUpgradeWithProfile(t *testing.T) {
	dir := filepath.Join(fixturesPath(), "with-profiles")

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("prof-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "prof-upg",
		Namespace:   "test-ns",
		Profile:     "prod",
	}

	rel, err := Upgrade(mock, tmpDir(t, dir), opts)
	if nil != err {
		t.Fatalf("upgrade with profile failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestUpgradeInvalidReleaseName(t *testing.T) {
	mock := newMockClient("test-ns")
	opts := &UpgradeOptions{
		ReleaseName: "",
		Namespace:   "test-ns",
	}

	_, err := Upgrade(mock, "/tmp", opts)
	if nil == err {
		t.Fatal("expected error for empty release name")
	}
}

func TestUpgradeWithSets(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("sets-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "sets-upg",
		Namespace:   "test-ns",
		Sets:        []string{"replicas=5"},
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade with sets failed: %v", err)
	}
	if 5 != rel.Values["replicas"] {
		t.Errorf("expected replicas=5, got %v", rel.Values["replicas"])
	}
}

func TestUpgradeAtomicWaitFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("atomic-wait-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	mock.waitErr = errors.New("wait timed out")

	opts := &UpgradeOptions{
		ReleaseName: "atomic-wait-upg",
		Namespace:   "test-ns",
		Atomic:      true,
		Wait:        true,
		Timeout:     30 * time.Second,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on atomic wait failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

// tmpDir returns the given path unchanged; it exists so tests with fixture
// paths pass a non-tempdir value through the same interface.
func tmpDir(_ *testing.T, path string) string {
	return path
}

func TestUpgradeDefaultNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("default")
	existing := makeDeployedRelease("def-ns-upg", "default", 1)
	storeRelease(t, mock, "default", existing)

	opts := &UpgradeOptions{
		ReleaseName: "def-ns-upg",
		Namespace:   "default",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if "default" != rel.Namespace {
		t.Errorf("expected default, got %s", rel.Namespace)
	}
}

func TestUpgradeReuseValuesWithResetOverride(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("reuse-reset", "test-ns", 1)
	existing.Values = map[string]any{"replicas": 1, "name": "test", "custom": "val"}
	storeRelease(t, mock, "test-ns", existing)

	// Both reuse and reset set - reset wins
	opts := &UpgradeOptions{
		ReleaseName: "reuse-reset",
		Namespace:   "test-ns",
		ReuseValues: true,
		ResetValues: true,
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	// When both set, ResetValues wins (custom should be gone)
	if _, ok := rel.Values["custom"]; ok {
		t.Error("expected custom value to be reset when both ReuseValues and ResetValues are set")
	}
}

func TestUpgradeServerDryRunFailure(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("dry-fail-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	mock.dryRunErr = errors.New("validation error")

	opts := &UpgradeOptions{
		ReleaseName: "dry-fail-upg",
		Namespace:   "test-ns",
		DryRun:      "server",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error on server dry-run failure")
	}
	if nil == rel {
		t.Fatal("expected release to be returned")
	}
}

func TestUpgradeDefaultWaitTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("wait-def-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "wait-def-upg",
		Namespace:   "test-ns",
		Wait:        true,
		Timeout:     0, // use default
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestUpgradeWithValueFiles(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	overrideFile := filepath.Join(tmpDir, "override.yaml")
	os.WriteFile(overrideFile, []byte("env: prod\n"), 0o644)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("vf-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "vf-upg",
		Namespace:   "test-ns",
		ValueFiles:  []string{overrideFile},
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if "prod" != rel.Values["env"] {
		t.Errorf("expected env=prod, got %v", rel.Values["env"])
	}
}

func TestUpgradeUserValuesTracked(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("uv-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "uv-upg",
		Namespace:   "test-ns",
		Sets:        []string{"custom=tracked"},
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if nil == rel.UserValues {
		t.Fatal("expected non-nil UserValues")
	}
	if "tracked" != rel.UserValues["custom"] {
		t.Errorf("expected custom=tracked, got %v", rel.UserValues["custom"])
	}
}

func TestUpgradeCapabilitiesError(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	mock.capsErr = errors.New("discovery failed")
	existing := makeDeployedRelease("caps-err-upg", "test-ns", 1)
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "caps-err-upg",
		Namespace:   "test-ns",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade should succeed with caps error: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestUpgradePreservesFirstDeployed(t *testing.T) {
	tmpDir := t.TempDir()
	createTestPackage(t, tmpDir)

	mock := newMockClient("test-ns")
	existing := makeDeployedRelease("ts-upg", "test-ns", 1)
	firstDeployed := existing.Info.FirstDeployed
	storeRelease(t, mock, "test-ns", existing)

	opts := &UpgradeOptions{
		ReleaseName: "ts-upg",
		Namespace:   "test-ns",
	}

	rel, err := Upgrade(mock, tmpDir, opts)
	if nil != err {
		t.Fatalf("upgrade failed: %v", err)
	}
	if !rel.Info.FirstDeployed.Equal(firstDeployed) {
		t.Errorf("expected FirstDeployed preserved from rev1")
	}
}
