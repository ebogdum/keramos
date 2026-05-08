package engine

import (
	"strings"
	"testing"
)

func TestEvaluateExpressionEmpty(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// Empty expression resolves to nil (empty path resolves against values)
	result, err := EvaluateExpression("", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty string path lookup returns nil
	if nil != result {
		t.Errorf("expected nil for empty expression, got %v", result)
	}
}

func TestEvaluateExpressionWhitespaceOnly(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// Whitespace-only trims to empty, resolves same as empty
	result, err := EvaluateExpression("   ", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for whitespace-only expression, got %v", result)
	}
}

func TestSubstituteAllEmptyExpression(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// ${} contains empty expression, resolves to nil
	result, err := SubstituteAll("${}", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single expression returning nil preserves nil
	if nil != result {
		t.Errorf("expected nil for ${}, got %v", result)
	}
}

func TestSubstituteAllWhitespaceExpression(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// ${ } whitespace expression resolves to nil
	result, err := SubstituteAll("${ }", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for ${ }, got %v", result)
	}
}

func TestEvaluateExpressionDeepUndefinedPath(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// values.a doesn't exist, so values.a.b.c.d.e should return nil (not error)
	result, err := EvaluateExpression("values.a.b.c.d.e", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for deeply nested undefined path, got %v", result)
	}
}

func TestEvaluateExpressionResolvesToNil(t *testing.T) {
	ctx := &RenderContext{
		Values: map[string]any{
			"nullValue": nil,
		},
	}
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values.nullValue", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestSubstituteAllMultipleSubstitutions(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll("${values.name}-${values.image.tag}-${values.image.repository}", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if "myapp-v1.0-nginx" != s {
		t.Errorf("expected 'myapp-v1.0-nginx', got %s", s)
	}
}

func TestEvaluateExpressionUnknownFunctionInPipeline(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	_, err := EvaluateExpression("values.name | nonexistentFunc", ctx, funcs)
	if nil == err {
		t.Error("expected error for unknown function in pipeline")
	}
}

func TestEvaluateExpressionDefaultMissingArg(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// default() requires a fallback argument
	_, err := EvaluateExpression("values.name | default", ctx, funcs)
	if nil == err {
		t.Error("expected error for default without argument")
	}
}

func TestEvaluateExpressionLongFunctionChain(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// Chain: upper -> lower -> upper -> lower -> trim
	result, err := EvaluateExpression("values.name | upper | lower | upper | lower | trim", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "myapp" != result {
		t.Errorf("expected 'myapp', got %v", result)
	}
}

func TestSubstituteAllNilNodePassthrough(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll(nil, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil passthrough, got %v", result)
	}
}

func TestSubstituteAllIntPassthrough(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll(42, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 42 != result {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestSubstituteAllBoolPassthrough(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := SubstituteAll(true, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if true != result {
		t.Errorf("expected true, got %v", result)
	}
}

func TestEvaluateExpressionNilValuesMap(t *testing.T) {
	ctx := &RenderContext{
		Values: nil,
	}
	funcs := NewFuncRegistry()

	_, err := EvaluateExpression("values.name", ctx, funcs)
	if nil == err {
		t.Error("expected error for nil values map")
	}
}

func TestSplitPipelineEmptyInput(t *testing.T) {
	segments, err := splitPipeline("")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single empty segment
	if 1 != len(segments) {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
}

func TestParseFuncCallMalformedMissingCloseParen(t *testing.T) {
	_, _, err := parseFuncCall("default('arg'")
	if nil == err {
		t.Error("expected error for malformed function call missing close paren")
	}
}

func TestParseFuncCallNoArgs(t *testing.T) {
	name, args, err := parseFuncCall("upper")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "upper" != name {
		t.Errorf("expected 'upper', got %s", name)
	}
	if nil != args {
		t.Errorf("expected nil args, got %v", args)
	}
}

func TestParseFuncCallEmptyParens(t *testing.T) {
	name, args, err := parseFuncCall("myFunc()")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "myFunc" != name {
		t.Errorf("expected 'myFunc', got %s", name)
	}
	if nil != args {
		t.Errorf("expected nil args for empty parens, got %v", args)
	}
}

func TestSubstituteAllNestedMapWithExpressions(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := map[string]any{
		"outer": map[string]any{
			"inner": "${values.name}",
		},
	}
	result, err := SubstituteAll(input, ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	outer := m["outer"].(map[string]any)
	if "myapp" != outer["inner"] {
		t.Errorf("expected 'myapp', got %v", outer["inner"])
	}
}

func TestSubstituteAllSliceWithError(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := []any{"${values.name | nonexistent}"}
	_, err := SubstituteAll(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for unknown function in slice element")
	}
}

func TestSubstituteAllMapWithError(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	input := map[string]any{
		"key": "${values.name | nonexistent}",
	}
	_, err := SubstituteAll(input, ctx, funcs)
	if nil == err {
		t.Error("expected error for unknown function in map value")
	}
}

func TestEvaluateExpressionCapabilitiesNamespace(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("capabilities.kubeVersion", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "1.28" != result {
		t.Errorf("expected '1.28', got %v", result)
	}
}

func TestEvaluateExpressionNamespaceOnlyReturnsMap(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	result, err := EvaluateExpression("values", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if "myapp" != m["name"] {
		t.Errorf("expected 'myapp' in returned map")
	}
}

func TestEvaluateExpressionMixedContentWithNilValue(t *testing.T) {
	ctx := testContext()
	funcs := NewFuncRegistry()

	// values.nonexistent resolves to nil, which formats as "<nil>" in mixed content
	result, err := SubstituteAll("prefix-${values.nonexistent}-suffix", ctx, funcs)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if !strings.Contains(s, "prefix-") {
		t.Errorf("expected prefix in result, got %s", s)
	}
}
