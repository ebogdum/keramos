package values

import (
	"testing"
)

func TestParseSet_SimpleString(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "name=myapp")
	if nil != err {
		t.Fatal(err)
	}
	if "myapp" != vals["name"] {
		t.Errorf("expected name=myapp, got %v", vals["name"])
	}
}

func TestParseSet_DottedPath(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "image.tag=v1.0")
	if nil != err {
		t.Fatal(err)
	}
	image, ok := vals["image"].(map[string]any)
	if !ok {
		t.Fatal("expected image to be map")
	}
	if "v1.0" != image["tag"] {
		t.Errorf("expected image.tag=v1.0, got %v", image["tag"])
	}
}

func TestParseSet_DeepDottedPath(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "a.b.c=deep")
	if nil != err {
		t.Fatal(err)
	}
	a := vals["a"].(map[string]any)
	b := a["b"].(map[string]any)
	if "deep" != b["c"] {
		t.Errorf("expected a.b.c=deep, got %v", b["c"])
	}
}

func TestParseSet_IntegerInference(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "replicas=3")
	if nil != err {
		t.Fatal(err)
	}
	if 3 != vals["replicas"] {
		t.Errorf("expected replicas=3, got %v (type %T)", vals["replicas"], vals["replicas"])
	}
}

func TestParseSet_FloatInference(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "ratio=1.5")
	if nil != err {
		t.Fatal(err)
	}
	if 1.5 != vals["ratio"] {
		t.Errorf("expected ratio=1.5, got %v", vals["ratio"])
	}
}

func TestParseSet_BoolInference(t *testing.T) {
	vals := make(map[string]any)
	_ = ParseSet(vals, "enabled=true")
	_ = ParseSet(vals, "disabled=false")

	if true != vals["enabled"] {
		t.Errorf("expected enabled=true, got %v", vals["enabled"])
	}
	if false != vals["disabled"] {
		t.Errorf("expected disabled=false, got %v", vals["disabled"])
	}
}

func TestParseSet_NullInference(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "val=null")
	if nil != err {
		t.Fatal(err)
	}
	if nil != vals["val"] {
		t.Errorf("expected val=nil, got %v", vals["val"])
	}
}

func TestParseSet_EmptyValue(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "name=")
	if nil != err {
		t.Fatal(err)
	}
	if "" != vals["name"] {
		t.Errorf("expected name='', got %v", vals["name"])
	}
}

func TestParseSet_MissingEquals(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "noequals")
	if nil == err {
		t.Fatal("expected error for missing equals")
	}
}

func TestParseSet_EmptyKey(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "=value")
	if nil == err {
		t.Fatal("expected error for empty key")
	}
}

func TestParseSet_ValueWithEquals(t *testing.T) {
	vals := make(map[string]any)
	err := ParseSet(vals, "key=a=b")
	if nil != err {
		t.Fatal(err)
	}
	if "a=b" != vals["key"] {
		t.Errorf("expected key=a=b, got %v", vals["key"])
	}
}
