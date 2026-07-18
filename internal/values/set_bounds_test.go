package values

import (
	"strings"
	"testing"
)

// TestSetNegativeIndexRejected proves a negative --set array index errors
// instead of panicking (was: index out of range [-1]).
func TestSetNegativeIndexRejected(t *testing.T) {
	m := map[string]any{}
	err := ParseSet(m, "foo[-1]=x")
	if nil == err || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected negative-index error, got %v", err)
	}
}

// TestSetHugeIndexRejected proves an oversized index errors instead of
// allocating an enormous slice (OOM).
func TestSetHugeIndexRejected(t *testing.T) {
	m := map[string]any{}
	err := ParseSet(m, "foo[2000000000]=x")
	if nil == err || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected huge-index error, got %v", err)
	}
}

// TestSetIndexWithinBoundWorks confirms normal indexing still works.
func TestSetIndexWithinBoundWorks(t *testing.T) {
	m := map[string]any{}
	if err := ParseSet(m, "foo[2]=x"); nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	arr, ok := m["foo"].([]any)
	if !ok || 3 != len(arr) || "x" != arr[2] {
		t.Fatalf("unexpected result: %#v", m["foo"])
	}
}

// TestResolverNilDefaultsNoSetPanic proves --set on a nil starting map does not
// panic (was: assignment to entry in nil map, via reuse-values).
func TestResolverNilDefaultsNoSetPanic(t *testing.T) {
	r := NewResolver(nil)
	if err := r.ApplySet("a=1", SourceSet); nil != err {
		t.Fatalf("ApplySet on nil-defaults resolver: %v", err)
	}
	if _, ok := r.Result()["a"]; !ok {
		t.Fatal("value not set")
	}
}
