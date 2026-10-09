package engine

import (
	"strings"
	"testing"
)

func TestLetBindsValuesForItsBody(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values:  map[string]any{"storages": []any{" a:", "b", " c:"}},
		Release: map[string]any{"name": "rel"},
	}
	template := `$let:
  names:
    $each: ${values.storages}
    $as: item
    $yield:
      $if: ${item | hasPrefix ' '}
      $then: ${item | trim | trimSuffix ':'}
$in:
  kind: ConfigMap
  data:
    storages: ${names | join ','}
`
	docs, err := e.RenderFile("cm.yaml", template, map[string]any{}, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected one document, got %d", len(docs))
	}
	if !strings.Contains(docs[0], "storages: a,c") {
		t.Errorf("rendered %q, want storages: a,c", docs[0])
	}
}

func TestFormatEachBuildsLinesPerItem(t *testing.T) {
	ctx := &RenderContext{Values: map[string]any{"config": map[string]any{"a": "1", "b": "2"}}}
	got, err := EvaluateExpression("values.config | formatEach '%v = %v;' 'key' 'value' | join ''", ctx, NewFuncRegistry())
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "a = 1;b = 2;" != got {
		t.Errorf("formatEach produced %q", got)
	}
}

func TestTextDirectiveJoinsItems(t *testing.T) {
	e := New()
	ctx := &RenderContext{Values: map[string]any{"ports": []any{80, 443}}}
	docs, err := e.RenderFile("cm.yaml", `data:
  list:
    $text:
      - "ports:"
      - $each: ${values.ports}
        $as: port
        $yield: "  - ${port}"
`, map[string]any{}, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"ports:", "- 80", "- 443"} {
		if !strings.Contains(docs[0], want) {
			t.Errorf("rendered %q, want it to contain %q", docs[0], want)
		}
	}
}
