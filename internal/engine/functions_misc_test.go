package engine

import (
	"reflect"
	"testing"
)

func TestFnDictStructured(t *testing.T) {
	out, err := fnDict("a", 1, "b", []any{2, 3}, "c", map[string]any{"k": "v"})
	if nil != err {
		t.Fatalf("dict: %v", err)
	}
	m := out.(map[string]any)
	if 1 != m["a"] {
		t.Errorf("dict a = %v", m["a"])
	}
	if !reflect.DeepEqual(m["b"], []any{2, 3}) {
		t.Errorf("dict b = %v", m["b"])
	}
	if !reflect.DeepEqual(m["c"], map[string]any{"k": "v"}) {
		t.Errorf("dict c = %v", m["c"])
	}
}

func TestFnGetReturnsNilForMissing(t *testing.T) {
	out, _ := fnGet(map[string]any{"x": 1}, "y")
	if nil != out {
		t.Errorf("get missing = %v, want nil", out)
	}
}

func TestFnPrintfNumeric(t *testing.T) {
	out, err := fnPrintf("port=%d", 8080)
	if nil != err {
		t.Fatalf("printf: %v", err)
	}
	if "port=8080" != out {
		t.Errorf("printf = %v", out)
	}
}

func TestFnSemverCompareVersionFirst(t *testing.T) {
	// keramos pipeline form: ${"1.2.3" | semverCompare ">=1.0.0"}
	out, err := fnSemverCompare("1.2.3", ">=1.0.0")
	if nil != err {
		t.Fatalf("semverCompare: %v", err)
	}
	if true != out {
		t.Errorf("semverCompare = %v, want true", out)
	}
}

func TestFnMergeJSON(t *testing.T) {
	dst := map[string]any{"a": 1, "b": 2}
	out, err := fnMerge(dst, `{"b": 99, "c": 3}`)
	if nil != err {
		t.Fatalf("merge: %v", err)
	}
	m := out.(map[string]any)
	// merge: dst wins on existing non-zero
	if 2 != m["b"] {
		t.Errorf("merge b = %v, want 2", m["b"])
	}
	if 3.0 != m["c"] {
		t.Errorf("merge c = %v", m["c"])
	}
}

func TestFnMergeOverwriteJSON(t *testing.T) {
	dst := map[string]any{"a": 1, "b": 2}
	out, err := fnMergeOverwrite(dst, `{"b": 99}`)
	if nil != err {
		t.Fatalf("mergeOverwrite: %v", err)
	}
	m := out.(map[string]any)
	if 99.0 != m["b"] {
		t.Errorf("mergeOverwrite b = %v, want 99", m["b"])
	}
}

func TestFnPickOmit(t *testing.T) {
	src := map[string]any{"a": 1, "b": 2, "c": 3}
	picked, _ := fnPick(src, "a", "c")
	if !reflect.DeepEqual(picked, map[string]any{"a": 1, "c": 3}) {
		t.Errorf("pick = %v", picked)
	}
	omitted, _ := fnOmit(src, "b")
	if !reflect.DeepEqual(omitted, map[string]any{"a": 1, "c": 3}) {
		t.Errorf("omit = %v", omitted)
	}
}

func TestFnFail(t *testing.T) {
	if _, err := fnFail("custom message"); nil == err {
		t.Fatal("fail should always return error")
	}
}

func TestFnKindOf(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"hello", "string"},
		{42, "int"},
		{3.14, "float64"},
		{true, "bool"},
		{map[string]any{"x": 1}, "map"},
		{[]any{1, 2}, "slice"},
		{nil, "invalid"},
	}
	for _, c := range cases {
		got, _ := fnKindOf(c.in)
		if got != c.want {
			t.Errorf("kindOf(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
