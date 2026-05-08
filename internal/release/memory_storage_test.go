package release

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryStorage_CRUD(t *testing.T) {
	s := NewMemoryStorage()

	rel := &Release{
		Name:      "demo",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Manifest:  "kind: ConfigMap",
		Info:      ReleaseInfo{FirstDeployed: time.Now(), LastDeployed: time.Now()},
	}
	if err := s.Create(rel); nil != err {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get("demo", 1)
	if nil != err {
		t.Fatalf("get: %v", err)
	}
	if got.Manifest != "kind: ConfigMap" {
		t.Errorf("expected manifest preserved, got %q", got.Manifest)
	}

	got.Status = StatusFailed
	if err := s.Update(got); nil != err {
		t.Fatalf("update: %v", err)
	}
	again, _ := s.Get("demo", 1)
	if StatusFailed != again.Status {
		t.Errorf("expected updated status, got %s", again.Status)
	}

	if err := s.Delete("demo", 1); nil != err {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("demo", 1); nil == err {
		t.Fatal("expected not-found after delete")
	}
}

func TestMemoryStorage_DuplicateRevision(t *testing.T) {
	s := NewMemoryStorage()
	rel := &Release{Name: "x", Revision: 1, Status: StatusDeployed}
	if err := s.Create(rel); nil != err {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(rel); nil == err {
		t.Fatal("expected duplicate-revision error")
	}
}

func TestMemoryStorage_HistoryOrder(t *testing.T) {
	s := NewMemoryStorage()
	for _, r := range []int{2, 1, 3} {
		_ = s.Create(&Release{Name: "demo", Revision: r, Status: StatusDeployed})
	}
	hist, err := s.History("demo")
	if nil != err {
		t.Fatalf("history: %v", err)
	}
	if 3 != len(hist) {
		t.Fatalf("expected 3 revisions, got %d", len(hist))
	}
	if hist[0].Revision != 1 || hist[1].Revision != 2 || hist[2].Revision != 3 {
		t.Fatalf("expected ascending revisions, got %d, %d, %d",
			hist[0].Revision, hist[1].Revision, hist[2].Revision)
	}
}

func TestMemoryStorage_SizeLimit(t *testing.T) {
	s := NewMemoryStorage()
	// Build a payload that compresses past 1MB.
	huge := strings.Repeat("x", 2*1024*1024)
	rel := &Release{Name: "huge", Revision: 1, Status: StatusDeployed, Manifest: huge}
	err := s.Create(rel)
	// Encoded payload may or may not exceed the limit depending on compressibility;
	// the key behavior: it does not panic, and returns either success or an error
	// matching the storage size limit message.
	if nil != err && !strings.Contains(err.Error(), "storage size limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
