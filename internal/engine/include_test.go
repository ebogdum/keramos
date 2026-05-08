package engine

import (
	"testing"
)

func TestSimpleInclude(t *testing.T) {
	partials := map[string]any{
		"labels": map[string]any{
			"app":        "myapp",
			"managed-by": "keramos",
		},
	}

	input := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"$include": "labels",
			},
		},
	}

	result, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	metadata := m["metadata"].(map[string]any)
	labels := metadata["labels"].(map[string]any)
	if "myapp" != labels["app"] {
		t.Errorf("expected 'myapp', got %v", labels["app"])
	}
	if "keramos" != labels["managed-by"] {
		t.Errorf("expected 'keramos', got %v", labels["managed-by"])
	}
}

func TestIncludeWithOverrides(t *testing.T) {
	partials := map[string]any{
		"labels": map[string]any{
			"app":        "myapp",
			"managed-by": "keramos",
		},
	}

	input := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"$include":    "labels",
				"extra-label": "custom",
			},
		},
	}

	result, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]any)
	metadata := m["metadata"].(map[string]any)
	labels := metadata["labels"].(map[string]any)
	if "myapp" != labels["app"] {
		t.Errorf("expected 'myapp', got %v", labels["app"])
	}
	if "custom" != labels["extra-label"] {
		t.Errorf("expected 'custom', got %v", labels["extra-label"])
	}
}

func TestIncludeCycleDetection(t *testing.T) {
	partials := map[string]any{
		"a": map[string]any{
			"$include": "b",
		},
		"b": map[string]any{
			"$include": "a",
		},
	}

	input := map[string]any{
		"$include": "a",
	}

	_, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil == err {
		t.Error("expected error for include cycle")
	}
}

func TestIncludeMissingPartial(t *testing.T) {
	partials := map[string]any{}

	input := map[string]any{
		"$include": "nonexistent",
	}

	_, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil == err {
		t.Error("expected error for missing partial")
	}
}

func TestIncludeInSlice(t *testing.T) {
	partials := map[string]any{
		"item": map[string]any{
			"name": "included",
		},
	}

	input := []any{
		map[string]any{"$include": "item"},
		map[string]any{"name": "regular"},
	}

	result, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list := result.([]any)
	first := list[0].(map[string]any)
	if "included" != first["name"] {
		t.Errorf("expected 'included', got %v", first["name"])
	}
}

func TestNestedIncludes(t *testing.T) {
	partials := map[string]any{
		"inner": map[string]any{
			"innerKey": "innerVal",
		},
		"outer": map[string]any{
			"$include": "inner",
			"outerKey": "outerVal",
		},
	}

	input := map[string]any{
		"$include": "outer",
	}

	result, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if "innerVal" != m["innerKey"] {
		t.Errorf("expected 'innerVal', got %v", m["innerKey"])
	}
	if "outerVal" != m["outerKey"] {
		t.Errorf("expected 'outerVal', got %v", m["outerKey"])
	}
}

func TestIncludeNonStringValue(t *testing.T) {
	partials := map[string]any{}

	input := map[string]any{
		"$include": 42,
	}

	_, err := ResolveIncludes(input, partials, make(map[string]bool))
	if nil == err {
		t.Error("expected error for non-string $include value")
	}
}
