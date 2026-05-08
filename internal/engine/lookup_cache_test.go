package engine

import "testing"

// TestLookupCacheHits verifies a single render that calls lookup() with
// identical arguments hits the cluster only once. C1 regression test.
func TestLookupCacheHits(t *testing.T) {
	calls := 0
	ctx := &RenderContext{
		Lookup: func(_, _, _, _ string) (map[string]any, error) {
			calls++
			return map[string]any{"x": 1}, nil
		},
	}
	fn := makeLookupFunc(ctx)
	for i := 0; i < 5; i++ {
		_, err := fn("v1", "ConfigMap", "default", "demo")
		if nil != err {
			t.Fatalf("lookup: %v", err)
		}
	}
	if 1 != calls {
		t.Errorf("expected 1 cluster call, got %d", calls)
	}
}

func TestLookupCacheKeyDistinct(t *testing.T) {
	calls := 0
	ctx := &RenderContext{
		Lookup: func(_, _, _, _ string) (map[string]any, error) {
			calls++
			return map[string]any{}, nil
		},
	}
	fn := makeLookupFunc(ctx)
	_, _ = fn("v1", "ConfigMap", "default", "a")
	_, _ = fn("v1", "ConfigMap", "default", "b")
	if 2 != calls {
		t.Errorf("expected 2 distinct calls, got %d", calls)
	}
}
