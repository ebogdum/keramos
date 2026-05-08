package values

import (
	"testing"
)

func TestParseSet_VeryDeepPath(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "a.b.c.d.e.f=deep")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	a := vals["a"].(map[string]any)
	b := a["b"].(map[string]any)
	c := b["c"].(map[string]any)
	d := c["d"].(map[string]any)
	e := d["e"].(map[string]any)
	if "deep" != e["f"] {
		t.Errorf("expected a.b.c.d.e.f=deep, got %v", e["f"])
	}
}

func TestParseSet_OverrideMapWithString(t *testing.T) {
	vals := map[string]any{
		"image": map[string]any{
			"repository": "nginx",
			"tag":        "latest",
		},
	}

	// Override the entire image map with a string
	err := ParseSet(vals, "image=myimage:latest")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "myimage:latest" != vals["image"] {
		t.Errorf("expected image=myimage:latest, got %v", vals["image"])
	}
}

func TestParseSet_OverrideStringWithNestedPath(t *testing.T) {
	vals := map[string]any{
		"image": "nginx",
	}

	// image is a string, but we're setting image.tag -- should create nested map
	err := ParseSet(vals, "image.tag=v2")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	imageMap, ok := vals["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image to be map after nested set, got %T", vals["image"])
	}
	if "v2" != imageMap["tag"] {
		t.Errorf("expected image.tag=v2, got %v", imageMap["tag"])
	}
}

func TestParseSet_NilInference(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "key=nil")
	if nil != err {
		t.Fatal(err)
	}
	if nil != vals["key"] {
		t.Errorf("expected nil for 'nil' value, got %v", vals["key"])
	}
}

func TestParseSet_MultipleEqualsInValue(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "annotation=app=myapp,version=v1")
	if nil != err {
		t.Fatal(err)
	}
	if "app=myapp,version=v1" != vals["annotation"] {
		t.Errorf("expected full string with equals, got %v", vals["annotation"])
	}
}

func TestParseSet_EmptySegmentLeading(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, ".foo=bar")
	if nil == err {
		t.Fatal("expected error for leading dot (empty segment)")
	}
}

func TestParseSet_EmptySegmentTrailing(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "foo.=bar")
	if nil == err {
		t.Fatal("expected error for trailing dot (empty segment)")
	}
}

func TestParseSet_EmptySegmentMiddle(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "foo..bar=baz")
	if nil == err {
		t.Fatal("expected error for consecutive dots (empty segment)")
	}
}

func TestInferType_Table(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{"null", nil},
		{"nil", nil},
		{"true", true},
		{"false", false},
		{"42", int(42)},
		{"3.14", float64(3.14)},
		{"hello", "hello"},
		{"", ""},
		{"0", int(0)},
		{"0.0", float64(0.0)},
		{"-1", int(-1)},
		{"1e10", float64(1e10)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := inferType(tt.input)
			if result != tt.expected {
				t.Errorf("inferType(%q) = %v (%T), expected %v (%T)", tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}
