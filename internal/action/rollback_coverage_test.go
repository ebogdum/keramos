package action

import (
	"errors"
	"testing"
	"time"

	"github.com/ebogdum/keramos/v3/internal/release"
)

func TestRollbackToSpecificRevision(t *testing.T) {
	mock := newMockClient("test-ns")

	// Create rev 1 and rev 2
	rev1 := makeDeployedRelease("rb-test", "test-ns", 1)
	rev1.Manifest = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: rev1-config\n"
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-test", "test-ns", 2)
	rev2.Manifest = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: rev2-config\n"
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-test",
		Namespace:   "test-ns",
		Revision:    1, // rollback to rev 1
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	if 3 != rel.Revision {
		t.Errorf("expected revision 3 (new rev), got %d", rel.Revision)
	}
	// Should have applied rev1's manifest
	if 0 == len(mock.appliedManifests) {
		t.Error("expected manifests to be applied")
	}
	if mock.appliedManifests[0] != rev1.Manifest {
		t.Error("expected rev1 manifest to be applied")
	}
}

func TestRollbackToPrevious(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-prev", "test-ns", 1)
	rev1.Manifest = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: prev-config\n"
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-prev", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-prev",
		Namespace:   "test-ns",
		Revision:    0, // 0 means previous
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
	if 3 != rel.Revision {
		t.Errorf("expected revision 3, got %d", rel.Revision)
	}
}

func TestRollbackApplyFailure(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-fail", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-fail", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	mock.applyErr = errors.New("apply failed")

	opts := &RollbackOptions{
		ReleaseName: "rb-fail",
		Namespace:   "test-ns",
		Revision:    1,
	}

	rel, err := Rollback(mock, opts)
	if nil == err {
		t.Fatal("expected error on apply failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestRollbackWithWaitFailure(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-wait", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-wait", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	mock.waitErr = errors.New("wait timed out")

	opts := &RollbackOptions{
		ReleaseName: "rb-wait",
		Namespace:   "test-ns",
		Revision:    1,
		Wait:        true,
		Timeout:     30 * time.Second,
	}

	rel, err := Rollback(mock, opts)
	if nil == err {
		t.Fatal("expected error on wait failure")
	}
	if release.StatusFailed != rel.Status {
		t.Errorf("expected status failed, got %s", rel.Status)
	}
}

func TestRollbackReleaseNotFound(t *testing.T) {
	mock := newMockClient("test-ns")

	opts := &RollbackOptions{
		ReleaseName: "nonexistent",
		Namespace:   "test-ns",
		Revision:    1,
	}

	_, err := Rollback(mock, opts)
	if nil == err {
		t.Fatal("expected error for nonexistent release")
	}
}

func TestRollbackInvalidReleaseName(t *testing.T) {
	mock := newMockClient("test-ns")

	opts := &RollbackOptions{
		ReleaseName: "",
		Namespace:   "test-ns",
	}

	_, err := Rollback(mock, opts)
	if nil == err {
		t.Fatal("expected error for empty release name")
	}
}

func TestRollbackNoRevisionsToRollTo(t *testing.T) {
	mock := newMockClient("test-ns")

	// Only rev 1 exists, rollback to previous (rev 0) should fail
	rev1 := makeDeployedRelease("rb-noprev", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	opts := &RollbackOptions{
		ReleaseName: "rb-noprev",
		Namespace:   "test-ns",
		Revision:    0, // previous = rev 0, invalid
	}

	_, err := Rollback(mock, opts)
	if nil == err {
		t.Fatal("expected error when no previous revision")
	}
}

func TestRollbackNamespaceFromClient(t *testing.T) {
	mock := newMockClient("client-ns")

	rev1 := makeDeployedRelease("rb-ns", "client-ns", 1)
	storeRelease(t, mock, "client-ns", rev1)

	rev2 := makeDeployedRelease("rb-ns", "client-ns", 2)
	storeRelease(t, mock, "client-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-ns",
		Namespace:   "", // use client namespace
		Revision:    1,
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if "client-ns" != rel.Namespace {
		t.Errorf("expected client-ns, got %s", rel.Namespace)
	}
}

func TestRollbackCustomDescription(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-desc", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-desc", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-desc",
		Namespace:   "test-ns",
		Revision:    1,
		Description: "Emergency rollback",
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if "Emergency rollback" != rel.Info.Description {
		t.Errorf("expected custom description, got %q", rel.Info.Description)
	}
}

func TestRollbackDefaultDescription(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-defdesc", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-defdesc", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-defdesc",
		Namespace:   "test-ns",
		Revision:    1,
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if "Rollback to 1" != rel.Info.Description {
		t.Errorf("expected 'Rollback to 1', got %q", rel.Info.Description)
	}
}

func TestRollbackWithWaitSuccess(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-wait-ok", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-wait-ok", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-wait-ok",
		Namespace:   "test-ns",
		Revision:    1,
		Wait:        true,
		Timeout:     5 * time.Minute,
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestRollbackTargetWithHooks(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-hooks", "test-ns", 1)
	rev1.Hooks = []release.HookResult{
		{Name: "pre-install-hook", Kind: "Job", Status: "succeeded"},
	}
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-hooks", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-hooks",
		Namespace:   "test-ns",
		Revision:    1,
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}

func TestRollbackDefaultTimeout(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("rb-deftimeout", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("rb-deftimeout", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &RollbackOptions{
		ReleaseName: "rb-deftimeout",
		Namespace:   "test-ns",
		Revision:    1,
		Wait:        true,
		Timeout:     0, // should use default
	}

	rel, err := Rollback(mock, opts)
	if nil != err {
		t.Fatalf("rollback failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}
}
