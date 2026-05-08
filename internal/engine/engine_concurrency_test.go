package engine

import (
	"sync"
	"testing"
)

// TestParallelRendersDoNotRace exercises the per-render registry by running
// many concurrent renders with different lookup contexts. The race detector
// must not fire — A9's regression test.
func TestParallelRendersDoNotRace(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"pod.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${Release.name}\ndata:\n  v: ${Values.x}\n",
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := &RenderContext{
				Values:  map[string]any{"x": i},
				Package: map[string]any{"name": "p"},
				Release: map[string]any{"name": "r"},
				Lookup: func(_, _, _, _ string) (map[string]any, error) {
					return map[string]any{"goroutine": i}, nil
				},
			}
			if _, err := eng.Render(templates, nil, ctx); nil != err {
				t.Errorf("render: %v", err)
			}
		}()
	}
	wg.Wait()
}
