package action

import (
	"errors"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/internal/release"
)

// failingStorage implements release.Storage; Update always errors so we can
// assert that callers surface a dropped status write instead of swallowing it.
type failingStorage struct{ updateErr error }

func (f *failingStorage) Create(*release.Release) error              { return nil }
func (f *failingStorage) Update(*release.Release) error              { return f.updateErr }
func (f *failingStorage) Get(string, int) (*release.Release, error)  { return nil, nil }
func (f *failingStorage) List(string) ([]*release.Release, error)    { return nil, nil }
func (f *failingStorage) Last(string) (*release.Release, error)      { return nil, nil }
func (f *failingStorage) History(string) ([]*release.Release, error) { return nil, nil }
func (f *failingStorage) Delete(string, int) error                   { return nil }

func TestMarkFailedSurfacesDroppedStatusWrite(t *testing.T) {
	st := &failingStorage{updateErr: errors.New("apiserver unreachable")}
	rel := &release.Release{Name: "r", Status: release.StatusDeployed}
	sec := markFailed(st, rel)
	if release.StatusFailed != rel.Status {
		t.Fatalf("expected status set to failed, got %s", rel.Status)
	}
	if 0 == len(sec) {
		t.Fatal("expected a secondary error describing the dropped status write")
	}
	if !strings.Contains(sec[0], "not persisted") {
		t.Fatalf("unexpected secondary message: %q", sec[0])
	}
}

func TestCombineFailureWrapsCauseAndSecondary(t *testing.T) {
	cause := errors.New("apply failed")
	out := combineFailure(cause, []string{"atomic rollback incomplete: boom"})
	if nil == out {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(out, cause) {
		t.Fatal("combined error must unwrap to the original cause")
	}
	if !strings.Contains(out.Error(), "rollback incomplete") {
		t.Fatalf("combined error should mention the secondary failure: %q", out.Error())
	}
	// No secondary -> return the cause unchanged.
	if combineFailure(cause, nil) != cause {
		t.Fatal("combineFailure with no secondary must return the cause unchanged")
	}
}

func TestAtomicRollbackUpgradeReturnsErrorOnReapplyFailure(t *testing.T) {
	mock := &mockKubeClient{applyErr: errors.New("reapply refused")}
	st := &failingStorage{updateErr: errors.New("update refused")}
	prev := &release.Release{Name: "r", Revision: 1, Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}
	failed := &release.Release{Name: "r", Revision: 2, Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}
	err := atomicRollbackUpgrade(mock, st, prev, failed)
	if nil == err {
		t.Fatal("expected atomicRollbackUpgrade to report its own failure, got nil")
	}
	if !strings.Contains(err.Error(), "rollback incomplete") {
		t.Fatalf("expected 'rollback incomplete', got: %v", err)
	}
	if !strings.Contains(err.Error(), "re-apply") {
		t.Fatalf("expected re-apply failure surfaced, got: %v", err)
	}
}

func TestAtomicRollbackUpgradeCleanWhenAllSucceed(t *testing.T) {
	mock := &mockKubeClient{}
	st := &failingStorage{updateErr: nil}
	prev := &release.Release{Name: "r", Revision: 1, Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}
	failed := &release.Release{Name: "r", Revision: 2, Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}
	if err := atomicRollbackUpgrade(mock, st, prev, failed); nil != err {
		t.Fatalf("expected clean rollback, got: %v", err)
	}
}
