package values

import "testing"

// TestProvenancePrunesOnTypeChange proves the trace has no stale leaves when a
// value changes shape between contributions.
func TestProvenancePrunesOnTypeChange(t *testing.T) {
	// scalar -> map: default foo=5, then override foo:{a:1}
	r := NewResolver(map[string]any{"foo": 5})
	r.ApplyMap(map[string]any{"foo": map[string]any{"a": 1}}, SourceValueFile, "override.yaml")
	p := r.Trace().Provenance()
	if _, stale := p["foo"]; stale {
		t.Fatalf("stale scalar leaf 'foo' should be pruned after it became a map: %v", p)
	}
	if _, ok := p["foo.a"]; !ok {
		t.Fatalf("expected foo.a in provenance: %v", p)
	}

	// map -> scalar via --set: default foo:{a:1}, then --set foo=5
	r2 := NewResolver(map[string]any{"foo": map[string]any{"a": 1}})
	if err := r2.ApplySet("foo=5", SourceSet); nil != err {
		t.Fatal(err)
	}
	p2 := r2.Trace().Provenance()
	if _, stale := p2["foo.a"]; stale {
		t.Fatalf("stale sub-leaf 'foo.a' should be pruned after foo became scalar: %v", p2)
	}
	if _, ok := p2["foo"]; !ok {
		t.Fatalf("expected foo in provenance: %v", p2)
	}
}
