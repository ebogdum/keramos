package engine

import (
	"testing"
)

func TestIfTruthy(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if": "${values.enabled}",
		"$then": map[string]any{
			"replicas": "${values.replicas}",
		},
		"$else": map[string]any{
			"replicas": 1,
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if "${values.replicas}" != m["replicas"] {
		t.Errorf("expected ${values.replicas}, got %v", m["replicas"])
	}
}

func TestIfFalsy(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"enabled": false,
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if": "${values.enabled}",
		"$then": map[string]any{
			"replicas": 3,
		},
		"$else": map[string]any{
			"replicas": 1,
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if 1 != m["replicas"] {
		t.Errorf("expected 1, got %v", m["replicas"])
	}
}

func TestIfWithoutElse(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"enabled": false,
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if": "${values.enabled}",
		"$then": map[string]any{
			"replicas": 3,
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for falsy without $else, got %v", result)
	}
}

func TestDocumentLevelIf(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"ingress": map[string]any{
				"enabled": true,
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":        "${values.ingress.enabled}",
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "Ingress",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if "Ingress" != m["kind"] {
		t.Errorf("expected 'Ingress', got %v", m["kind"])
	}
	// $if should be removed
	if _, has := m["$if"]; has {
		t.Error("$if key should have been removed")
	}
}

func TestDocumentLevelIfFalsy(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"ingress": map[string]any{
				"enabled": false,
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if":        "${values.ingress.enabled}",
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "Ingress",
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for falsy document-level $if, got %v", result)
	}
}

func TestEachOverList(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each": "${values.ports}",
		"$as":   "port",
		"$yield": map[string]any{
			"name":          "${port.name}",
			"containerPort": "${port.number}",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected list, got %T: %v", result, result)
	}
	if 2 != len(list) {
		t.Fatalf("expected 2 items, got %d", len(list))
	}

	first := list[0].(map[string]any)
	if "http" != first["name"] {
		t.Errorf("expected 'http', got %v", first["name"])
	}
	if 80 != first["containerPort"] {
		t.Errorf("expected 80, got %v", first["containerPort"])
	}
}

func TestEachOverMap(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"annotations": map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each": "${values.annotations}",
		"$as":   "ann",
		"$yield": map[string]any{
			"annotation": "${ann.key}: ${ann.value}",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	if 2 != len(list) {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
}

func TestEachOverNil(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each":  "${values.items}",
		"$yield": map[string]any{"item": "${$item}"},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}
	if 0 != len(list) {
		t.Errorf("expected empty list for nil iterable, got %d", len(list))
	}
}

func TestSwitchMatching(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"service": map[string]any{
				"type": "LoadBalancer",
				"loadBalancerIP": "10.0.0.1",
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$switch": "${values.service.type}",
		"$cases": map[string]any{
			"ClusterIP": map[string]any{
				"clusterIP": "None",
			},
			"LoadBalancer": map[string]any{
				"loadBalancerIP": "${values.service.loadBalancerIP}",
			},
		},
		"$default": map[string]any{},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if "${values.service.loadBalancerIP}" != m["loadBalancerIP"] {
		t.Errorf("expected loadBalancerIP expression, got %v", m["loadBalancerIP"])
	}
}

func TestSwitchDefault(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"service": map[string]any{
				"type": "NodePort",
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$switch": "${values.service.type}",
		"$cases": map[string]any{
			"ClusterIP": map[string]any{
				"clusterIP": "None",
			},
		},
		"$default": map[string]any{
			"fallback": true,
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if true != m["fallback"] {
		t.Errorf("expected fallback=true, got %v", m["fallback"])
	}
}

func TestSwitchNoMatch(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"service": map[string]any{
				"type": "NodePort",
			},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$switch": "${values.service.type}",
		"$cases": map[string]any{
			"ClusterIP": map[string]any{
				"clusterIP": "None",
			},
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for no match without default, got %v", result)
	}
}

func TestNestedControlFlow(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"enabled": true,
			"items":   []any{"a", "b"},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$if": "${values.enabled}",
		"$then": map[string]any{
			"$each":  "${values.items}",
			"$yield": "${$item}",
		},
	}

	result, err := ProcessControlFlow(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := result.([]any)
	if !ok {
		t.Fatalf("expected list, got %T: %v", result, result)
	}
	if 2 != len(list) {
		t.Errorf("expected 2 items, got %d", len(list))
	}
}

func TestEachMissingYield(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"items": []any{"a"},
		},
	}
	funcs := NewFuncRegistry()

	input := map[string]any{
		"$each": "${values.items}",
	}

	_, err := ProcessControlFlow(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for missing $yield")
	}
}
