package engine

import (
	"reflect"
	"testing"
)

func TestFnIntCasts(t *testing.T) {
	if v, _ := fnInt("42"); 42 != v {
		t.Errorf("int(\"42\") = %v", v)
	}
	if v, _ := fnInt64("42"); int64(42) != v {
		t.Errorf("int64(\"42\") = %v", v)
	}
	if v, _ := fnFloat64("1.5"); 1.5 != v {
		t.Errorf("float64(\"1.5\") = %v", v)
	}
}

func TestFnURLQueryEscape(t *testing.T) {
	got, _ := fnURLQueryEscape("a b&c=1")
	if "a+b%26c%3D1" != got {
		t.Errorf("urlquery = %q", got)
	}
}

func TestFnFromYAMLArray(t *testing.T) {
	yaml := `name: a
---
name: b
---
name: c
`
	out, err := fnFromYAMLArray(yaml)
	if nil != err {
		t.Fatalf("fromYamlArray: %v", err)
	}
	list := out.([]any)
	if 3 != len(list) {
		t.Fatalf("expected 3 docs, got %d", len(list))
	}
	if "a" != list[0].(map[string]any)["name"] {
		t.Errorf("doc 0 = %v", list[0])
	}
}

func TestFnSeq(t *testing.T) {
	cases := []struct {
		val  any
		args []any
		want []any
	}{
		{5, nil, []any{1, 2, 3, 4, 5}},
		{1, []any{3}, []any{1, 2, 3}},
		{1, []any{2, 7}, []any{1, 3, 5, 7}},
		{10, []any{-2, 4}, []any{10, 8, 6, 4}},
	}
	for _, c := range cases {
		got, err := fnSeq(c.val, c.args...)
		if nil != err {
			t.Errorf("seq(%v, %v): %v", c.val, c.args, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("seq(%v, %v) = %v, want %v", c.val, c.args, got, c.want)
		}
	}
}

func TestFnSplitN(t *testing.T) {
	out, _ := fnSplitN("a:b:c:d", ":", 2)
	want := []any{"a", "b:c:d"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("splitn = %v", out)
	}
}

func TestFnWithout(t *testing.T) {
	out, _ := fnWithout([]any{1, 2, 3, 4}, 2, 4)
	if !reflect.DeepEqual(out, []any{1, 3}) {
		t.Errorf("without = %v", out)
	}
}

func TestFnInitialRest(t *testing.T) {
	in := []any{"a", "b", "c"}
	if init, _ := fnInitial(in); !reflect.DeepEqual(init, []any{"a", "b"}) {
		t.Errorf("initial = %v", init)
	}
	if rest, _ := fnRest(in); !reflect.DeepEqual(rest, []any{"b", "c"}) {
		t.Errorf("rest = %v", rest)
	}
	if init, _ := fnInitial([]any{}); !reflect.DeepEqual(init, []any{}) {
		t.Errorf("initial empty = %v", init)
	}
}

func TestFnTuple(t *testing.T) {
	out, _ := fnTuple(1, 2, 3)
	if !reflect.DeepEqual(out, []any{1, 2, 3}) {
		t.Errorf("tuple = %v", out)
	}
}

func TestFnRegexQuoteMeta(t *testing.T) {
	out, _ := fnRegexQuoteMeta(`a.b*c`)
	if `a\.b\*c` != out {
		t.Errorf("regexQuoteMeta = %v", out)
	}
}
