package engine

import (
	"testing"
)

func TestIfWithStringFalse(t *testing.T) {
	// The literal strings "false"/"FALSE"/"False"/"0"/"no" are treated
	// as falsy by isTruthy. Without this, a value round-tripped through
	// a string-only source (env var, `--set foo=false`) would render
	// the truthy branch — a real foot-gun. The docs
	// (docs/templates/expressions.md) state this behaviour explicitly.
	ctx := &RenderContext{
		Values: map[string]any{
			"condition": "false",
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.condition}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' (string %q is falsy), got %v", "false", result)
	}
}

func TestIfWithEmptyString(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"condition": "",
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.condition}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' for empty string, got %v", result)
	}
}

func TestIfWithZero(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"condition": 0,
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.condition}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' for zero, got %v", result)
	}
}

func TestIfWithEmptyList(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"condition": []any{},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.condition}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' for empty list, got %v", result)
	}
}

func TestIfWithEmptyMap(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"condition": map[string]any{},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.condition}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' for empty map, got %v", result)
	}
}

func TestIfWithNilCondition(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":   "${values.missing}",
		"$then": "yes",
		"$else": "no",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "no" != result {
		t.Errorf("expected 'no' for nil condition, got %v", result)
	}
}

func TestEachOverEmptyList(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"items": []any{},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each":  "${values.items}",
		"$yield": "${$item}",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if 0 != len(list) {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestEachOverEmptyMap(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"labels": map[string]any{},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each":  "${values.labels}",
		"$as":    "label",
		"$yield": "${label.key}=${label.value}",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if 0 != len(list) {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestEachOverNonIterable(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"count": 42,
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each":  "${values.count}",
		"$yield": "${$item}",
	}

	_, err := ProcessControlFlow(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for $each over non-iterable (int)")
	}
}

func TestEachOverStringNonIterable(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"name": "hello",
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each":  "${values.name}",
		"$yield": "${$item}",
	}

	_, err := ProcessControlFlow(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for $each over non-iterable (string)")
	}
}

func TestSwitchNoMatchNoDefault(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"mode": "unknown",
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$switch": "${values.mode}",
		"$cases": map[string]any{
			"a": "resultA",
			"b": "resultB",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// An unmatched switch with no $default drops the field/document — it must
	// omit (matching $if), which reads as nil-or-omit-sentinel here.
	if nil != result && !isOmit(result) {
		t.Errorf("expected omit for unmatched switch without default, got %v", result)
	}
}

func TestSwitchMissingCasesMap(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"mode": "a",
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$switch": "${values.mode}",
		// missing $cases
	}

	_, err := ProcessControlFlow(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for $switch without $cases")
	}
}

func TestNestedIfInsideEach(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"items": []any{
				map[string]any{"name": "a", "enabled": true},
				map[string]any{"name": "b", "enabled": false},
				map[string]any{"name": "c", "enabled": true},
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each": "${values.items}",
		"$as":   "item",
		"$yield": map[string]any{
			"$if":   "${item.enabled}",
			"$then": "${item.name}",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	// A $if without $else inside $each now OMITS the disabled element rather
	// than emitting a nil hole, so iteration doubles as a filter.
	if 2 != len(list) {
		t.Fatalf("expected 2 items (disabled filtered out), got %d: %v", len(list), list)
	}
	if "a" != list[0] {
		t.Errorf("expected 'a', got %v", list[0])
	}
	if "c" != list[1] {
		t.Errorf("expected 'c', got %v", list[1])
	}
}

func TestNestedEachInsideEach(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"groups": []any{
				map[string]any{
					"name":  "g1",
					"items": []any{"x", "y"},
				},
				map[string]any{
					"name":  "g2",
					"items": []any{"z"},
				},
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each": "${values.groups}",
		"$as":   "group",
		"$yield": map[string]any{
			"$each":  "${group.items}",
			"$yield": "${group.name}-${$item}",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	// outer flattens inner: g1-x, g1-y, g2-z
	if 3 != len(list) {
		t.Fatalf("expected 3 items (flattened), got %d: %v", len(list), list)
	}
	if "g1-x" != list[0] {
		t.Errorf("expected 'g1-x', got %v", list[0])
	}
	if "g1-y" != list[1] {
		t.Errorf("expected 'g1-y', got %v", list[1])
	}
	if "g2-z" != list[2] {
		t.Errorf("expected 'g2-z', got %v", list[2])
	}
}

func TestProcessControlFlowSlice(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"enabled": true,
		},
	}
	funcs := NewFuncRegistry()

	input := []any{
		map[string]any{
			"$if":   "${values.enabled}",
			"$then": "included",
		},
		"static",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if 2 != len(list) {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if "included" != list[0] {
		t.Errorf("expected 'included', got %v", list[0])
	}
	if "static" != list[1] {
		t.Errorf("expected 'static', got %v", list[1])
	}
}

func TestProcessControlFlowScalarPassthrough(t *testing.T) {
	funcs := NewFuncRegistry()

	result, err := ProcessControlFlow("hello", &RenderContext{}, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "hello" != result {
		t.Errorf("expected 'hello', got %v", result)
	}
}

func TestProcessControlFlowNilPassthrough(t *testing.T) {
	funcs := NewFuncRegistry()

	result, err := ProcessControlFlow(nil, &RenderContext{}, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestEachWithDefaultAsVariable(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"items": []any{"alpha", "beta"},
		},
	}
	funcs := NewFuncRegistry()

	// No $as specified, should default to $item
	input := map[string]any{
		"$each":  "${values.items}",
		"$yield": "${$item}",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if 2 != len(list) {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if "alpha" != list[0] {
		t.Errorf("expected 'alpha', got %v", list[0])
	}
	if "beta" != list[1] {
		t.Errorf("expected 'beta', got %v", list[1])
	}
}

func TestDeepClone(t *testing.T) {
	original := map[string]any{
		"a": map[string]any{
			"b": []any{1, 2, 3},
		},
		"c": "string",
	}

	cloned := deepClone(original)
	clonedMap := cloned.(map[string]any)
	inner := clonedMap["a"].(map[string]any)
	innerList := inner["b"].([]any)

	// Modify clone, verify original not affected
	innerList[0] = 999
	originalInner := original["a"].(map[string]any)
	originalList := originalInner["b"].([]any)
	if 999 == originalList[0] {
		t.Error("deepClone did not create independent copy")
	}
}

func TestDeepCloneNil(t *testing.T) {
	result := deepClone(nil)
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestDeepCloneScalar(t *testing.T) {
	result := deepClone(42)
	if 42 != result {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestScopeWithVarPreservesContext(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"original": "value",
		},
		Package: map[string]any{
			"name": "pkg",
		},
		Release: map[string]any{
			"name": "rel",
		},
		Capabilities: map[string]any{
			"kubeVersion": "1.28",
		},
		Files: map[string][]byte{
			"file.txt": []byte("data"),
		},
	}

	scoped := scopeWithVar(ctx, "newVar", "newValue")

	if "value" != scoped.Values["original"] {
		t.Error("original value not preserved")
	}
	if "newValue" != scoped.Values["newVar"] {
		t.Error("new variable not set")
	}
	if "pkg" != scoped.Package["name"] {
		t.Error("package not preserved")
	}
	if "rel" != scoped.Release["name"] {
		t.Error("release not preserved")
	}
	if "1.28" != scoped.Capabilities["kubeVersion"] {
		t.Error("capabilities not preserved")
	}
}
