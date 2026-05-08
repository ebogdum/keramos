package engine

import (
	"strings"
	"testing"
)

func TestRenderEmptyTemplatesMap(t *testing.T) {
	e := New()
	ctx := testContext()

	result, err := e.Render(map[string]string{}, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" != result {
		t.Errorf("expected empty result for empty templates map, got %q", result)
	}
}

func TestRenderTemplateWithOnlyComments(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `# This is a comment
# Another comment line
`

	docs, err := e.RenderFile("comments.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// YAML comments result in nil documents, which are skipped
	if 0 != len(docs) {
		t.Errorf("expected 0 documents for comment-only file, got %d", len(docs))
	}
}

func TestRenderEmptyYAMLDocument(t *testing.T) {
	e := New()
	ctx := testContext()

	content := "---\n"

	docs, err := e.RenderFile("empty.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// An empty YAML document (just ---) parses to nil, which should be skipped
	if 0 != len(docs) {
		t.Errorf("expected 0 documents for empty YAML doc, got %d", len(docs))
	}
}

func TestRenderMultiDocWithConditionalFalsy(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"service":  map[string]any{"enabled": true},
			"ingress":  map[string]any{"enabled": false},
			"name":     "myapp",
		},
	}

	content := `apiVersion: v1
kind: Service
metadata:
  name: ${values.name}
---
$if: ${values.ingress.enabled}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${values.name}-ingress
`

	docs, err := e.RenderFile("multi.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document (ingress removed), got %d", len(docs))
	}
	if !strings.Contains(docs[0], "kind: Service") {
		t.Error("expected Service in output")
	}
}

func TestRenderNilDocument(t *testing.T) {
	e := New()
	ctx := testContext()

	// renderDocument with nil input
	result, err := e.renderDocument(nil, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != result {
		t.Errorf("expected nil for nil document, got %v", result)
	}
}

func TestCleanDollarKeysNested(t *testing.T) {
	// cleanDollarKeys strips ONLY keramos's own control-flow keys ($if,
	// $each, $switch, $include, $cases, ...). Other $-prefixed keys
	// (JSON Schema $ref / $schema / $defs, CRD extensions, ad-hoc
	// names) MUST be preserved.
	input := map[string]any{
		"keep":     "value",
		"$comment": "preserved",       // not a directive — kept
		"$if":      "${values.flag}",  // directive — stripped
		"nested": map[string]any{
			"keep2":   "value2",
			"$ref":    "#/$defs/foo",  // JSON Schema — kept
			"$switch": "${values.x}",  // directive — stripped
		},
		"list": []any{
			map[string]any{
				"ok":      true,
				"$schema": "x", // JSON Schema — kept
			},
		},
	}

	result := cleanDollarKeys(input)
	m := result.(map[string]any)

	if _, ok := m["$if"]; ok {
		t.Error("$if (keramos directive) should be removed")
	}
	if _, ok := m["$comment"]; !ok {
		t.Error("$comment (non-directive) should be preserved")
	}
	if "value" != m["keep"] {
		t.Errorf("expected 'value', got %v", m["keep"])
	}

	nested := m["nested"].(map[string]any)
	if _, ok := nested["$switch"]; ok {
		t.Error("nested $switch (keramos directive) should be removed")
	}
	if _, ok := nested["$ref"]; !ok {
		t.Error("nested $ref (not a directive) should be preserved")
	}
	if "value2" != nested["keep2"] {
		t.Error("nested keep2 should be preserved")
	}

	list := m["list"].([]any)
	item := list[0].(map[string]any)
	if _, ok := item["$schema"]; !ok {
		t.Error("$schema in list item (not a directive) should be preserved")
	}
	if true != item["ok"] {
		t.Error("ok should be preserved")
	}
}

func TestCleanDollarKeysScalar(t *testing.T) {
	result := cleanDollarKeys("hello")
	if "hello" != result {
		t.Errorf("expected 'hello', got %v", result)
	}
}

func TestCleanDollarKeysNil(t *testing.T) {
	result := cleanDollarKeys(nil)
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestSplitYAMLDocumentsMultiple(t *testing.T) {
	content := "a: 1\n---\nb: 2\n---\nc: 3\n"
	docs, err := splitYAMLDocuments(content)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 3 != len(docs) {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}
}

func TestSplitYAMLDocumentsEmpty(t *testing.T) {
	docs, err := splitYAMLDocuments("")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(docs) {
		t.Errorf("expected 0 docs for empty content, got %d", len(docs))
	}
}

func TestMarshalYAMLSimple(t *testing.T) {
	input := map[string]any{
		"key": "value",
	}
	result, err := marshalYAML(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "key: value") {
		t.Errorf("expected YAML output, got %q", result)
	}
}

func TestRenderWithIncludeError(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    $include: nonexistent_partial
`

	_, err := e.RenderFile("cm.yaml", content, nil, ctx)
	if nil == err {
		t.Error("expected error for missing partial in include")
	}
}

func TestRenderWithExpressionError(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `apiVersion: v1
kind: Service
metadata:
  name: ${values.name | unknownFunction}
`

	_, err := e.RenderFile("svc.yaml", content, nil, ctx)
	if nil == err {
		t.Error("expected error for unknown function in expression")
	}
}
