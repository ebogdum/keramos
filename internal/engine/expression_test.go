package engine

import (
	"testing"
)

func testContext() *RenderContext {
	return &RenderContext{
		Values: map[string]any{
			"name":     "myapp",
			"replicas": 3,
			"enabled":  true,
			"image": map[string]any{
				"repository": "nginx",
				"tag":        "v1.0",
			},
			"ports": []any{
				map[string]any{"name": "http", "number": 80},
				map[string]any{"name": "https", "number": 443},
			},
			"labels": map[string]any{
				"app":     "myapp",
				"version": "1.0",
			},
		},
		Package: map[string]any{
			"name":       "mypackage",
			"version":    "0.1.0",
			"appVersion": "1.0.0",
		},
		Release: map[string]any{
			"name":      "my-release",
			"namespace": "default",
		},
		Capabilities: map[string]any{
			"kubeVersion": "1.28",
		},
	}
}

func TestEvaluateExpressionSimplePath(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.name", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "myapp" != result {
		t.Errorf("expected 'myapp', got %v", result)
	}
}

func TestEvaluateExpressionNestedPath(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.image.tag", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "v1.0" != result {
		t.Errorf("expected 'v1.0', got %v", result)
	}
}

func TestEvaluateExpressionWithFunction(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.name | upper", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "MYAPP" != result {
		t.Errorf("expected 'MYAPP', got %v", result)
	}
}

func TestEvaluateExpressionFuncWithArgs(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// missing key → nil → default kicks in
	result, err := EvaluateExpression("values.tag | default('latest')", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "latest" != result {
		t.Errorf("expected 'latest', got %v", result)
	}
}

func TestEvaluateExpressionChainedFunctions(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.name | lower | quote", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if `"myapp"` != result {
		t.Errorf("expected '\"myapp\"', got %v", result)
	}
}

func TestEvaluateExpressionTypePreservation(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.replicas", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	intVal, ok := result.(int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	if 3 != intVal {
		t.Errorf("expected 3, got %d", intVal)
	}
}

func TestEvaluateExpressionPackageNamespace(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("package.name", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "mypackage" != result {
		t.Errorf("expected 'mypackage', got %v", result)
	}
}

func TestEvaluateExpressionReleaseNamespace(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("release.name", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "my-release" != result {
		t.Errorf("expected 'my-release', got %v", result)
	}
}

func TestEvaluateExpressionUnknownFunction(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	_, err := EvaluateExpression("values.name | nonexistent", ctx, funcs)
	if nil == err {
		t.Error("expected error for unknown function")
	}
}

func TestSubstituteAllSingleExpression(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll("${values.replicas}", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	intVal, ok := result.(int)
	if !ok {
		t.Fatalf("expected int type preservation, got %T: %v", result, result)
	}
	if 3 != intVal {
		t.Errorf("expected 3, got %d", intVal)
	}
}

func TestSubstituteAllMixedString(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll("${values.name}-svc", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string for mixed content, got %T", result)
	}
	if "myapp-svc" != s {
		t.Errorf("expected 'myapp-svc', got %s", s)
	}
}

func TestSubstituteAllMap(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := map[string]any{
		"name":     "${values.name}",
		"replicas": "${values.replicas}",
	}
	result, err := SubstituteAll(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if "myapp" != m["name"] {
		t.Errorf("expected 'myapp', got %v", m["name"])
	}
	if 3 != m["replicas"] {
		t.Errorf("expected 3, got %v", m["replicas"])
	}
}

func TestSubstituteAllSlice(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := []any{"${values.name}", "${values.replicas}"}
	result, err := SubstituteAll(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	list := result.([]any)
	if "myapp" != list[0] {
		t.Errorf("expected 'myapp', got %v", list[0])
	}
	if 3 != list[1] {
		t.Errorf("expected 3, got %v", list[1])
	}
}

func TestSubstituteAllUndefinedVarDefault(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll("${values.nonexistent | default('fallback')}", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "fallback" != result {
		t.Errorf("expected 'fallback', got %v", result)
	}
}

func TestSplitPipeline(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"values.name", 1},
		{"values.name | upper", 2},
		{"values.name | upper | quote", 3},
		{"values.tag | default('latest')", 2},
		{"values.name | replace('a', 'b')", 2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			segments, err := splitPipeline(tt.input)
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expected != len(segments) {
				t.Errorf("expected %d segments, got %d: %v", tt.expected, len(segments), segments)
			}
		})
	}
}

func TestParseFuncCall(t *testing.T) {
	tests := []struct {
		input    string
		name     string
		argCount int
	}{
		{"upper", "upper", 0},
		{"default('latest')", "default", 1},
		{"replace('old', 'new')", "replace", 2},
		{"indent(4)", "indent", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, args, err := parseFuncCall(tt.input)
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.name != name {
				t.Errorf("expected name %q, got %q", tt.name, name)
			}
			if tt.argCount != len(args) {
				t.Errorf("expected %d args, got %d: %v", tt.argCount, len(args), args)
			}
		})
	}
}

func TestEvaluateExpressionBoolPreservation(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.enabled", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	b, ok := result.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T", result)
	}
	if !b {
		t.Error("expected true")
	}
}
