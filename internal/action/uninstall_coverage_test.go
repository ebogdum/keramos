package action

import (
	"errors"
	"testing"

	"github.com/ebogdum/keramos/v2/internal/release"
)

func TestUninstallSuccess(t *testing.T) {
	mock := newMockClient("test-ns")

	rel := makeDeployedRelease("uninstall-test", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rel)

	opts := &UninstallOptions{
		ReleaseName: "uninstall-test",
		Namespace:   "test-ns",
	}

	result, err := Uninstall(mock, opts)
	if nil != err {
		t.Fatalf("uninstall failed: %v", err)
	}
	if nil == result {
		t.Fatal("expected result")
	}
	if 0 == len(mock.deletedManifests) {
		t.Error("expected manifests to be deleted")
	}

	// Verify release is removed from storage
	storage := release.NewSecretStorage(mock.clientset, "test-ns")
	_, getErr := storage.Last("uninstall-test")
	if nil == getErr {
		t.Error("expected release to be deleted from storage")
	}
}

func TestUninstallKeepHistory(t *testing.T) {
	mock := newMockClient("test-ns")

	rel := makeDeployedRelease("keep-hist", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rel)

	opts := &UninstallOptions{
		ReleaseName: "keep-hist",
		Namespace:   "test-ns",
		KeepHistory: true,
	}

	result, err := Uninstall(mock, opts)
	if nil != err {
		t.Fatalf("uninstall failed: %v", err)
	}
	if 0 == len(mock.deletedManifests) {
		t.Error("expected manifests to be deleted even with keep-history")
	}

	// Verify release is still in storage
	storage := release.NewSecretStorage(mock.clientset, "test-ns")
	stored, getErr := storage.Last("keep-hist")
	if nil != getErr {
		t.Fatal("expected release to be kept in storage with --keep-history")
	}
	if release.StatusSuperseded != stored.Status {
		t.Errorf("expected status superseded, got %s", stored.Status)
	}
	_ = result
}

func TestUninstallDeleteFailure(t *testing.T) {
	mock := newMockClient("test-ns")

	rel := makeDeployedRelease("del-fail", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rel)

	mock.deleteErr = errors.New("permission denied")

	opts := &UninstallOptions{
		ReleaseName: "del-fail",
		Namespace:   "test-ns",
	}

	result, err := Uninstall(mock, opts)
	if nil == err {
		t.Fatal("expected error on delete failure")
	}
	if release.StatusFailed != result.Status {
		t.Errorf("expected status failed, got %s", result.Status)
	}
}

func TestUninstallReleaseNotFound(t *testing.T) {
	mock := newMockClient("test-ns")

	opts := &UninstallOptions{
		ReleaseName: "nonexistent",
		Namespace:   "test-ns",
	}

	_, err := Uninstall(mock, opts)
	if nil == err {
		t.Fatal("expected error for nonexistent release")
	}
}

func TestUninstallInvalidReleaseName(t *testing.T) {
	mock := newMockClient("test-ns")

	opts := &UninstallOptions{
		ReleaseName: "",
		Namespace:   "test-ns",
	}

	_, err := Uninstall(mock, opts)
	if nil == err {
		t.Fatal("expected error for empty release name")
	}
}

func TestUninstallNamespaceFromClient(t *testing.T) {
	mock := newMockClient("client-ns")

	rel := makeDeployedRelease("ns-uninstall", "client-ns", 1)
	storeRelease(t, mock, "client-ns", rel)

	opts := &UninstallOptions{
		ReleaseName: "ns-uninstall",
		Namespace:   "", // use client namespace
	}

	result, err := Uninstall(mock, opts)
	if nil != err {
		t.Fatalf("uninstall failed: %v", err)
	}
	if 0 == len(mock.deletedManifests) {
		t.Error("expected manifests to be deleted")
	}
	_ = result
}

func TestUninstallWithTimeout(t *testing.T) {
	mock := newMockClient("test-ns")

	rel := makeDeployedRelease("timeout-uninstall", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rel)

	opts := &UninstallOptions{
		ReleaseName: "timeout-uninstall",
		Namespace:   "test-ns",
		Timeout:     30,
	}

	result, err := Uninstall(mock, opts)
	if nil != err {
		t.Fatalf("uninstall failed: %v", err)
	}
	if nil == result {
		t.Fatal("expected result")
	}
}

func TestUninstallMultipleRevisions(t *testing.T) {
	mock := newMockClient("test-ns")

	rev1 := makeDeployedRelease("multi-rev", "test-ns", 1)
	storeRelease(t, mock, "test-ns", rev1)

	rev2 := makeDeployedRelease("multi-rev", "test-ns", 2)
	storeRelease(t, mock, "test-ns", rev2)

	opts := &UninstallOptions{
		ReleaseName: "multi-rev",
		Namespace:   "test-ns",
	}

	_, err := Uninstall(mock, opts)
	if nil != err {
		t.Fatalf("uninstall failed: %v", err)
	}

	// All revisions should be deleted
	storage := release.NewSecretStorage(mock.clientset, "test-ns")
	_, getErr := storage.Last("multi-rev")
	if nil == getErr {
		t.Error("expected all revisions to be deleted")
	}
}
