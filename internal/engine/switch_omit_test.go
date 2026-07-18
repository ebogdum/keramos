package engine

import (
	"strings"
	"testing"
)

// TestSwitchNoMatchOmitsField proves an unmatched $switch with no $default
// drops its map field instead of emitting `key: null`.
func TestSwitchNoMatchOmitsField(t *testing.T) {
	doc := map[string]any{
		"kind": "ConfigMap",
		"policy": map[string]any{
			"$switch": "staging",
			"$cases":  map[string]any{"prod": "high"},
		},
	}
	ctx := &RenderContext{Values: map[string]any{}}
	e := New()
	out, err := e.renderDocument(doc, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	m, _ := out.(map[string]any)
	if _, present := m["policy"]; present {
		t.Fatalf("unmatched $switch should drop the field, got: %#v", m["policy"])
	}
	// And the marshalled form must not carry a null.
	y, _ := marshalYAML(out)
	if strings.Contains(y, "policy:") {
		t.Fatalf("expected no 'policy:' key in output:\n%s", y)
	}
}
