package engine

import (
	"reflect"
	"strings"
	"testing"
)

func helmCtx() *RenderContext {
	return &RenderContext{
		Values: map[string]any{
			"env":      "prod",
			"replicas": 3,
			"enabled":  true,
			"name":     "my-app",
			"labels":   map[string]any{"tier": "backend"},
			"extra":    map[string]any{"team": "core"},
			"ports":    []any{80, 443},
		},
		Package: map[string]any{"name": "pkg"},
		Release: map[string]any{"name": "rel", "namespace": "ns"},
	}
}

func evalExpr(t *testing.T, expr string) any {
	t.Helper()
	got, err := EvaluateExpression(expr, helmCtx(), NewFuncRegistry())
	if nil != err {
		t.Fatalf("EvaluateExpression(%q): %v", expr, err)
	}
	return got
}

func TestComparisonFunctions(t *testing.T) {
	cases := map[string]any{
		`eq values.env "prod"`:   true,
		`eq values.env "dev"`:    false,
		`ne values.env "dev"`:    true,
		`lt values.replicas 5`:   true,
		`le values.replicas 3`:   true,
		`gt values.replicas 5`:   false,
		`ge values.replicas 3`:   true,
		`not values.enabled`:     false,
		`and values.enabled 1`:   int64(1),
		`or values.missing "fb"`: "fb",
	}
	for expr, want := range cases {
		if got := evalExpr(t, expr); !reflect.DeepEqual(want, got) {
			t.Errorf("%s = %v (%T), want %v", expr, got, got, want)
		}
	}
}

func TestNestedCallArguments(t *testing.T) {
	got := evalExpr(t, `and values.enabled (eq values.env "prod")`)
	if true != got {
		t.Errorf("nested eq inside and = %v, want true", got)
	}
	got = evalExpr(t, `(eq values.env "prod") | ternary "yes" "no"`)
	if "yes" != got {
		t.Errorf("parenthesised pipeline input = %v, want yes", got)
	}
	got = evalExpr(t, `printf "%s-%s" values.name values.env`)
	if "my-app-prod" != got {
		t.Errorf("printf with path arguments = %v, want my-app-prod", got)
	}
}

func TestBridgedSprigFunctions(t *testing.T) {
	if got := evalExpr(t, `"hello" | b64enc`); "aGVsbG8=" != got {
		t.Errorf("b64enc = %v", got)
	}
	if got := evalExpr(t, `"aGVsbG8=" | b64dec`); "hello" != got {
		t.Errorf("b64dec = %v", got)
	}
	if got := evalExpr(t, `"xxhixx" | trimAll "x"`); "hi" != got {
		t.Errorf("trimAll = %v", got)
	}
	if got := evalExpr(t, `"a,b" | splitList ","`); !reflect.DeepEqual([]any{"a", "b"}, got) {
		t.Errorf("splitList = %v (%T)", got, got)
	}
	if got := evalExpr(t, `"42" | atoi`); 42 != got {
		t.Errorf("atoi = %v", got)
	}
}

func TestIndexAndList(t *testing.T) {
	if got := evalExpr(t, `values.labels | index "tier"`); "backend" != got {
		t.Errorf("index map = %v", got)
	}
	if got := evalExpr(t, `values.ports | index 1`); 443 != got {
		t.Errorf("index list = %v", got)
	}
	got := evalExpr(t, `list "a" "b"`)
	if !reflect.DeepEqual([]any{"a", "b"}, got) {
		t.Errorf("list = %v", got)
	}
}

func TestMergeIntoMap(t *testing.T) {
	input := map[string]any{
		"app":    "static",
		"$merge": "${values.labels}",
	}
	out, err := renderTree(input)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	m := out.(map[string]any)
	if "static" != m["app"] || "backend" != m["tier"] {
		t.Errorf("merged map = %v", m)
	}
}

func TestMergeListOfMaps(t *testing.T) {
	input := map[string]any{
		"app": "static",
		"$merge": []any{
			map[string]any{"$if": "${values.enabled}", "$then": map[string]any{"a": 1}},
			map[string]any{"$if": "${values.missing}", "$then": map[string]any{"b": 2}},
			"${values.extra}",
		},
	}
	out, err := renderTree(input)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	m := out.(map[string]any)
	if 1 != m["a"] {
		t.Errorf("conditional merge dropped a: %v", m)
	}
	if _, present := m["b"]; present {
		t.Errorf("false branch merged b: %v", m)
	}
	if "core" != m["team"] {
		t.Errorf("expression merge missing team: %v", m)
	}
}

func TestMergeSplicesIntoList(t *testing.T) {
	input := map[string]any{
		"items": []any{
			"first",
			map[string]any{"$merge": "${values.ports}"},
			map[string]any{"$merge": map[string]any{"$if": "${values.missing}", "$then": []any{"never"}}},
		},
	}
	out, err := renderTree(input)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	items := out.(map[string]any)["items"].([]any)
	want := []any{"first", 80, 443}
	if !reflect.DeepEqual(want, items) {
		t.Errorf("spliced list = %v, want %v", items, want)
	}
}

func TestSubstitutionInMapKeys(t *testing.T) {
	input := map[string]any{"${values.name}": "yes"}
	out, err := renderTree(input)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if "yes" != out.(map[string]any)["my-app"] {
		t.Errorf("key substitution produced %v", out)
	}
}

func TestIncludeWithBoundContext(t *testing.T) {
	e := New()
	partials := map[string]any{
		"greeting": map[string]any{"who": "${$ctx.who}"},
	}
	ctx := helmCtx()
	docs, err := e.RenderFile("t.yaml", `body: ${include "greeting" (dict "who" "world") | fromYaml}`, partials, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected one document, got %d", len(docs))
	}
	if want := "who: world"; !strings.Contains(docs[0], want) {
		t.Errorf("rendered %q, want it to contain %q", docs[0], want)
	}
}

func TestMergeAcceptsYAMLText(t *testing.T) {
	input := map[string]any{
		"app":    "static",
		"$merge": "tier: backend\nteam: core\n",
	}
	out, err := renderTree(input)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	m := out.(map[string]any)
	if "backend" != m["tier"] || "core" != m["team"] {
		t.Errorf("merged text = %v", m)
	}
}

func TestMissingPathSegmentsResolveToNil(t *testing.T) {
	got, err := EvaluateExpression("values.missing.deeper.key", helmCtx(), NewFuncRegistry())
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != got {
		t.Errorf("expected nil, got %v", got)
	}
	got, err = EvaluateExpression("values.missing | get 'key'", helmCtx(), NewFuncRegistry())
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != got {
		t.Errorf("expected nil from get, got %v", got)
	}
}

func TestSprigResultsAreUsableByOtherFunctions(t *testing.T) {
	got, err := EvaluateExpression("'a,b' | splitList ',' | append 'c'", helmCtx(), NewFuncRegistry())
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []any{"a", "b", "c"}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("chained sprig result = %v (%T), want %v", got, got, want)
	}
}

func renderTree(input any) (any, error) {
	e := New()
	ctx := helmCtx()
	return e.renderDocument(input, map[string]any{}, ctx)
}
