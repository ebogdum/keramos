package layer

import (
	"testing"

	"github.com/ebogdum/keramos/internal/maputil"
)

func TestDeepMerge_BothNil(t *testing.T) {
	result := DeepMerge(nil, nil)
	if nil != result {
		t.Errorf("expected nil for both nil inputs, got %v", result)
	}
}

func TestDeepMerge_EmptyMaps(t *testing.T) {
	result := DeepMerge(map[string]any{}, map[string]any{})
	if nil == result {
		t.Fatal("expected non-nil result")
	}
	if 0 != len(result) {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestDeepMerge_SrcMapOverridesDstNonMap(t *testing.T) {
	dst := map[string]any{
		"key": "string-value",
	}
	src := map[string]any{
		"key": map[string]any{"nested": true},
	}
	result := DeepMerge(dst, src)
	m, ok := result["key"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result["key"])
	}
	if true != m["nested"] {
		t.Error("expected nested=true")
	}
}

func TestDeepMerge_DstMapOverriddenBySrcScalar(t *testing.T) {
	dst := map[string]any{
		"key": map[string]any{"nested": true},
	}
	src := map[string]any{
		"key": "replaced-with-string",
	}
	result := DeepMerge(dst, src)
	s, ok := result["key"].(string)
	if !ok {
		t.Fatalf("expected string, got %T", result["key"])
	}
	if "replaced-with-string" != s {
		t.Errorf("expected 'replaced-with-string', got %s", s)
	}
}

func TestDeepMerge_DeeplyNested(t *testing.T) {
	dst := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "original",
				},
			},
		},
	}
	src := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"e": "added",
				},
			},
		},
	}
	result := DeepMerge(dst, src)
	a := result["a"].(map[string]any)
	b := a["b"].(map[string]any)
	c := b["c"].(map[string]any)

	if "original" != c["d"] {
		t.Errorf("expected d=original preserved, got %v", c["d"])
	}
	if "added" != c["e"] {
		t.Errorf("expected e=added, got %v", c["e"])
	}
}

func TestDeepMerge_NilValueInSrc(t *testing.T) {
	dst := map[string]any{
		"key": "value",
	}
	src := map[string]any{
		"key": nil,
	}
	result := DeepMerge(dst, src)
	if nil != result["key"] {
		t.Errorf("expected nil (src overrides dst), got %v", result["key"])
	}
}

func TestDeepMerge_NilValueInDst(t *testing.T) {
	dst := map[string]any{
		"key": nil,
	}
	src := map[string]any{
		"key": "value",
	}
	result := DeepMerge(dst, src)
	if "value" != result["key"] {
		t.Errorf("expected 'value', got %v", result["key"])
	}
}

func TestCopyMap_Nil(t *testing.T) {
	result := maputil.CopyMap(nil)
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestCopyMap_DoesNotShareReferences(t *testing.T) {
	original := map[string]any{
		"nested": map[string]any{
			"key": "value",
		},
	}
	copied := maputil.CopyMap(original)
	nested := copied["nested"].(map[string]any)
	nested["key"] = "modified"

	originalNested := original["nested"].(map[string]any)
	if "modified" == originalNested["key"] {
		t.Error("CopyMap shares references with original")
	}
}

func TestDeepCopyValue_Slice(t *testing.T) {
	original := []any{1, "two", map[string]any{"three": 3}}
	copied := maputil.DeepCopyValue(original)
	copiedSlice := copied.([]any)

	if 3 != len(copiedSlice) {
		t.Fatalf("expected 3 items, got %d", len(copiedSlice))
	}

	// Modify copy, check original not affected
	inner := copiedSlice[2].(map[string]any)
	inner["three"] = 999
	origInner := original[2].(map[string]any)
	if 999 == origInner["three"] {
		t.Error("DeepCopyValue shares references for slice elements")
	}
}

func TestDeepCopyValue_Scalar(t *testing.T) {
	result := maputil.DeepCopyValue(42)
	if 42 != result {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestDeepCopyValue_Nil(t *testing.T) {
	result := maputil.DeepCopyValue(nil)
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToMap_NilReturnsNil(t *testing.T) {
	result := maputil.ToMap(nil)
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToMap_NonMapReturnsNil(t *testing.T) {
	result := maputil.ToMap("not a map")
	if nil != result {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToMap_MapReturnsMap(t *testing.T) {
	m := map[string]any{"key": "val"}
	result := maputil.ToMap(m)
	if nil == result {
		t.Fatal("expected map, got nil")
	}
	if "val" != result["key"] {
		t.Errorf("expected key=val, got %v", result["key"])
	}
}
