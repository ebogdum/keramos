package layer

import (
	"testing"
)

func TestDeepMerge_NilDst(t *testing.T) {
	src := map[string]any{"a": 1}
	result := DeepMerge(nil, src)
	if 1 != result["a"] {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestDeepMerge_NilSrc(t *testing.T) {
	dst := map[string]any{"a": 1}
	result := DeepMerge(dst, nil)
	if 1 != result["a"] {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestDeepMerge_SrcOverridesDst(t *testing.T) {
	dst := map[string]any{"a": 1, "b": 2}
	src := map[string]any{"b": 20, "c": 30}
	result := DeepMerge(dst, src)

	if 1 != result["a"] {
		t.Errorf("expected a=1, got %v", result["a"])
	}
	if 20 != result["b"] {
		t.Errorf("expected b=20, got %v", result["b"])
	}
	if 30 != result["c"] {
		t.Errorf("expected c=30, got %v", result["c"])
	}
}

func TestDeepMerge_RecursiveMaps(t *testing.T) {
	dst := map[string]any{
		"image": map[string]any{
			"repository": "nginx",
			"tag":        "stable",
		},
	}
	src := map[string]any{
		"image": map[string]any{
			"tag": "latest",
		},
	}
	result := DeepMerge(dst, src)

	image, ok := result["image"].(map[string]any)
	if !ok {
		t.Fatal("expected image to be map")
	}
	if "nginx" != image["repository"] {
		t.Errorf("expected repository=nginx, got %v", image["repository"])
	}
	if "latest" != image["tag"] {
		t.Errorf("expected tag=latest, got %v", image["tag"])
	}
}

func TestDeepMerge_DoesNotMutateDst(t *testing.T) {
	dst := map[string]any{"a": 1}
	src := map[string]any{"a": 2}
	_ = DeepMerge(dst, src)

	if 1 != dst["a"] {
		t.Errorf("DeepMerge mutated dst: expected a=1, got %v", dst["a"])
	}
}

func TestDeepMerge_SliceReplaced(t *testing.T) {
	dst := map[string]any{"ports": []any{80}}
	src := map[string]any{"ports": []any{443, 8080}}
	result := DeepMerge(dst, src)

	ports, ok := result["ports"].([]any)
	if !ok {
		t.Fatal("expected ports to be slice")
	}
	if 2 != len(ports) {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}
