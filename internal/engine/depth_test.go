package engine

import "testing"

func TestDepthExceeds(t *testing.T) {
	shallow := map[string]any{"a": map[string]any{"b": 1}}
	if depthExceeds(shallow, 200) {
		t.Fatal("shallow doc should not exceed depth")
	}
	// build a doc nested 300 deep
	var deep any = 1
	for i := 0; i < 300; i++ {
		deep = map[string]any{"x": deep}
	}
	if !depthExceeds(deep, 200) {
		t.Fatal("300-deep doc should exceed the 200 limit")
	}
}
