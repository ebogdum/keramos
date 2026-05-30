package engine

import (
	"strings"
	"testing"
)

// Regression tests for adversarial-review findings H2, H3, H4, H5, M5.

func TestSeqIsCapped(t *testing.T) {
	funcs := NewFuncRegistry()
	fn, ok := funcs.Get("seq")
	if !ok {
		t.Fatal("seq not registered")
	}
	if _, err := fn(int64(1), int64(5_000_000)); nil == err {
		t.Fatal("expected seq to reject an oversized range, got nil error")
	}
	// A small range still works.
	v, err := fn(int64(1), int64(3))
	if nil != err {
		t.Fatalf("small seq erred: %v", err)
	}
	if s, ok := v.([]any); !ok || 3 != len(s) {
		t.Fatalf("expected 3-element slice, got %T %v", v, v)
	}
}

func TestSeqHugeBoundsDoNotOverflowPastCap(t *testing.T) {
	funcs := NewFuncRegistry()
	fn, _ := funcs.Get("seq")
	// start/end that would overflow int subtraction must still be rejected.
	if _, err := fn(int64(-9_000_000_000_000_000_000), int64(9_000_000_000_000_000_000)); nil == err {
		t.Fatal("expected huge seq range to be rejected")
	}
}

func TestSetIsCopyOnWrite(t *testing.T) {
	funcs := NewFuncRegistry()
	ctx := &RenderContext{Values: map[string]any{"foo": map[string]any{"a": 1}}}
	if _, err := SubstituteAll(`${values.foo | set "b" 2}`, ctx, funcs); nil != err {
		t.Fatalf("set erred: %v", err)
	}
	m := ctx.Values["foo"].(map[string]any)
	if _, mutated := m["b"]; mutated {
		t.Fatalf("set mutated shared ctx.Values map: %v", m)
	}
}

func TestUnsetIsCopyOnWrite(t *testing.T) {
	funcs := NewFuncRegistry()
	ctx := &RenderContext{Values: map[string]any{"foo": map[string]any{"a": 1, "b": 2}}}
	if _, err := SubstituteAll(`${values.foo | unset "b"}`, ctx, funcs); nil != err {
		t.Fatalf("unset erred: %v", err)
	}
	m := ctx.Values["foo"].(map[string]any)
	if _, present := m["b"]; !present {
		t.Fatalf("unset deleted from shared ctx.Values map: %v", m)
	}
}

func TestCoalesceSkipsFalsyArgs(t *testing.T) {
	funcs := NewFuncRegistry()
	v, err := SubstituteAll(`${"" | coalesce 0 "fallback"}`, &RenderContext{}, funcs)
	if nil != err {
		t.Fatalf("coalesce erred: %v", err)
	}
	if "fallback" != v {
		t.Fatalf("coalesce returned falsy arg: got %v want fallback", v)
	}
}

func TestMixedContentBraceInsideQuotedArg(t *testing.T) {
	funcs := NewFuncRegistry()
	v, err := SubstituteAll(`x ${"a}b" | upper} y`, &RenderContext{}, funcs)
	if nil != err {
		t.Fatalf("erred: %v", err)
	}
	if "x A}B y" != v {
		t.Fatalf("brace-in-quote mishandled: got %q want %q", v, "x A}B y")
	}
}

func TestMixedContentEscapeStillWorks(t *testing.T) {
	funcs := NewFuncRegistry()
	v, err := SubstituteAll(`a $${LITERAL} b`, &RenderContext{}, funcs)
	if nil != err {
		t.Fatalf("erred: %v", err)
	}
	if "a ${LITERAL} b" != v {
		t.Fatalf("escape mishandled: got %q", v)
	}
}

func TestIncludeStructuredPartialEmitsYAML(t *testing.T) {
	eng := New()
	tmpl := map[string]string{"x.yaml": "data:\n  v: ${include \"p\"}\n"}
	partials := map[string]any{"p": map[string]any{"a": 1, "b": "x"}}
	out, err := eng.Render(tmpl, partials, &RenderContext{Values: map[string]any{}, Release: map[string]any{"name": "r"}})
	if nil != err {
		t.Fatalf("render erred: %v", err)
	}
	if strings.Contains(out, "map[") {
		t.Fatalf("include emitted Go map syntax: %q", out)
	}
	if !strings.Contains(out, "a: 1") {
		t.Fatalf("include did not emit YAML keys: %q", out)
	}
}

// Optional-field ergonomics: missing values omit their key cleanly rather than
// rendering `null` / `[]` / nuking the block.

func TestOmitEmptyDropsKeyForMissingValue(t *testing.T) {
	eng := New()
	tmpl := map[string]string{"cm.yaml": `kind: ConfigMap
metadata:
  name: x
spec:
  required: present
  optional: ${values.missing | omitempty}
`}
	out, err := eng.Render(tmpl, nil, &RenderContext{Values: map[string]any{}, Release: map[string]any{"name": "r"}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "optional") {
		t.Errorf("expected optional key omitted, got:\n%s", out)
	}
	if strings.Contains(out, "null") {
		t.Errorf("expected no null in output, got:\n%s", out)
	}
	if !strings.Contains(out, "required: present") {
		t.Errorf("required field should remain:\n%s", out)
	}
}

func TestFieldLevelIfOmitsOnlyItsKey(t *testing.T) {
	eng := New()
	tmpl := map[string]string{"cm.yaml": `kind: ConfigMap
metadata:
  name: x
spec:
  keep: yes
  maybe:
    $if: ${values.enabled}
    $then: there
`}
	out, err := eng.Render(tmpl, nil, &RenderContext{Values: map[string]any{"enabled": false}, Release: map[string]any{"name": "r"}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "maybe") {
		t.Errorf("expected 'maybe' key omitted when $if false, got:\n%s", out)
	}
	if !strings.Contains(out, "keep:") {
		t.Errorf("sibling field must survive:\n%s", out)
	}
}

func TestEachOverMissingOmitsField(t *testing.T) {
	eng := New()
	tmpl := map[string]string{"cm.yaml": `kind: ConfigMap
metadata:
  name: x
spec:
  ports:
    $each: ${values.ports}
    $as: p
    $yield: ${$p}
`}
	out, err := eng.Render(tmpl, nil, &RenderContext{Values: map[string]any{}, Release: map[string]any{"name": "r"}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "ports") {
		t.Errorf("expected 'ports' omitted when collection missing, got:\n%s", out)
	}
}

func TestOmitDoesNotLeakThroughPipeline(t *testing.T) {
	funcs := NewFuncRegistry()
	v, err := SubstituteAll(`${values.missing | omitempty | upper}`, &RenderContext{Values: map[string]any{}}, funcs)
	if nil != err {
		t.Fatalf("erred: %v", err)
	}
	if !isOmit(v) {
		t.Fatalf("omit must propagate through downstream funcs, got %#v", v)
	}
}

func TestOmitInMixedContentRendersEmpty(t *testing.T) {
	funcs := NewFuncRegistry()
	v, err := SubstituteAll(`pre-${values.missing | omitempty}-post`, &RenderContext{Values: map[string]any{}}, funcs)
	if nil != err {
		t.Fatalf("erred: %v", err)
	}
	if "pre--post" != v {
		t.Fatalf("omit in mixed content should render empty, got %q", v)
	}
}

func TestOmitNeverMarshalsAsEmptyMap(t *testing.T) {
	eng := New()
	tmpl := map[string]string{"cm.yaml": "a: ${values.x | omitempty | upper}\nb: keep\n"}
	out, err := eng.Render(tmpl, nil, &RenderContext{Values: map[string]any{}, Release: map[string]any{"name": "r"}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "{}") || strings.Contains(out, "omitSentinel") {
		t.Fatalf("omit leaked into output:\n%s", out)
	}
	if strings.Contains(out, "a:") {
		t.Fatalf("key 'a' should be omitted:\n%s", out)
	}
}

func TestSetDeepCopiesNestedSlices(t *testing.T) {
	funcs := NewFuncRegistry()
	ctx := &RenderContext{Values: map[string]any{
		"foo": map[string]any{"list": []any{map[string]any{"a": 1}}},
	}}
	out, err := SubstituteAll(`${values.foo | set "b" 2}`, ctx, funcs)
	if nil != err {
		t.Fatalf("erred: %v", err)
	}
	// Mutate the returned copy's nested slice element.
	m := out.(map[string]any)
	m["list"].([]any)[0].(map[string]any)["a"] = 999
	// The shared context must be unaffected.
	orig := ctx.Values["foo"].(map[string]any)["list"].([]any)[0].(map[string]any)
	if 1 != orig["a"] {
		t.Fatalf("nested slice element aliased to shared context: %v", orig)
	}
}

func TestIncludeRecursionGuard(t *testing.T) {
	eng := New()
	// A partial that includes itself must hit the depth guard, not hang.
	tmpl := map[string]string{"x.yaml": "v: ${include \"loop\"}\n"}
	partials := map[string]any{"loop": `v: ${include "loop"}`}
	_, err := eng.Render(tmpl, partials, &RenderContext{Values: map[string]any{}, Release: map[string]any{"name": "r"}})
	if nil == err {
		t.Fatal("expected recursion-depth error from self-including partial")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected depth error, got: %v", err)
	}
}
