package action

import "testing"

func TestNewResourcesOnlyNilSnapshot(t *testing.T) {
	got := newResourcesOnly(nil, "kind: Foo", nil)
	if nil != got {
		t.Errorf("expected nil when snapshot is nil, got %v", got)
	}
}

func TestNewResourcesOnlyExcludesPreExisting(t *testing.T) {
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: a
  namespace: ns
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: b
  namespace: ns
`
	preExisting := map[string]bool{
		"v1|ConfigMap|ns|a": true,
	}
	got := newResourcesOnly(nil, manifest, preExisting)
	if !got["v1|ConfigMap|ns|b"] {
		t.Errorf("expected new resource b included, got %v", got)
	}
	if got["v1|ConfigMap|ns|a"] {
		t.Errorf("expected pre-existing resource a excluded, got %v", got)
	}
}
