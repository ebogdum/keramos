package engine

import (
	"reflect"
	"testing"
)

func TestFnUntil(t *testing.T) {
	out, err := fnUntil(5)
	if nil != err {
		t.Fatalf("until: %v", err)
	}
	want := []any{0, 1, 2, 3, 4}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("until(5) = %v, want %v", out, want)
	}
}

func TestFnUntilStep(t *testing.T) {
	out, err := fnUntilStep(0, 10, 2)
	if nil != err {
		t.Fatalf("untilStep: %v", err)
	}
	want := []any{0, 2, 4, 6, 8}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("untilStep(0,10,2) = %v, want %v", out, want)
	}
}

func TestFnRandIntRange(t *testing.T) {
	got, err := fnRandInt(10, 20)
	if nil != err {
		t.Fatalf("randInt: %v", err)
	}
	n := got.(int)
	if n < 10 || n >= 20 {
		t.Errorf("randInt out of range: %d", n)
	}
}

func TestFnConcat(t *testing.T) {
	out, _ := fnConcat([]any{1, 2}, []any{3, 4}, 5)
	want := []any{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("concat = %v, want %v", out, want)
	}
}

func TestFnSlice(t *testing.T) {
	out, _ := fnSlice([]any{1, 2, 3, 4, 5}, 1, 4)
	want := []any{2, 3, 4}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("slice = %v, want %v", out, want)
	}
}

func TestFnPrependAppend(t *testing.T) {
	pre, _ := fnPrepend([]any{2, 3}, 1)
	if !reflect.DeepEqual(pre, []any{1, 2, 3}) {
		t.Errorf("prepend mismatch: %v", pre)
	}
	app, _ := fnAppend([]any{1, 2}, 3, 4)
	if !reflect.DeepEqual(app, []any{1, 2, 3, 4}) {
		t.Errorf("append mismatch: %v", app)
	}
}

func TestFnReverse(t *testing.T) {
	out, _ := fnReverse([]any{1, 2, 3})
	if !reflect.DeepEqual(out, []any{3, 2, 1}) {
		t.Errorf("reverse mismatch: %v", out)
	}
}

func TestFnPluck(t *testing.T) {
	list := []any{
		map[string]any{"name": "a", "v": 1},
		map[string]any{"name": "b", "v": 2},
		map[string]any{"v": 3}, // no name → skipped
	}
	out, _ := fnPluck(list, "name")
	if !reflect.DeepEqual(out, []any{"a", "b"}) {
		t.Errorf("pluck mismatch: %v", out)
	}
}

func TestFnDig(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{"b": map[string]any{"c": 42}},
	}
	hit, _ := fnDig(m, "a", "b", "c", "default")
	if 42 != hit {
		t.Errorf("dig hit = %v, want 42", hit)
	}
	miss, _ := fnDig(m, "a", "x", "c", "default")
	if "default" != miss {
		t.Errorf("dig miss = %v, want default", miss)
	}
}

func TestFnB32(t *testing.T) {
	enc, _ := fnB32Enc("hello")
	dec, _ := fnB32Dec(enc)
	if "hello" != dec {
		t.Errorf("b32 round-trip mismatch: %v", dec)
	}
}

func TestFnFromJSON(t *testing.T) {
	out, err := fnFromJSON(`{"x":1,"y":"z"}`)
	if nil != err {
		t.Fatalf("fromJson: %v", err)
	}
	m := out.(map[string]any)
	if 1.0 != m["x"] || "z" != m["y"] {
		t.Errorf("fromJson = %v", m)
	}
}

func TestFnFromYAML(t *testing.T) {
	out, err := fnFromYAML("x: 1\ny: z\n")
	if nil != err {
		t.Fatalf("fromYaml: %v", err)
	}
	m := out.(map[string]any)
	if 1 != m["x"] || "z" != m["y"] {
		t.Errorf("fromYaml = %v", m)
	}
}

func TestFnNospace(t *testing.T) {
	out, _ := fnNospace("a b\tc\nd")
	if "abcd" != out {
		t.Errorf("nospace = %v", out)
	}
}

func TestFnWrap(t *testing.T) {
	out, _ := fnWrap("the quick brown fox", 10)
	s := out.(string)
	for _, line := range []string{"the quick", "brown fox"} {
		if -1 == indexOf(s, line) {
			t.Errorf("wrap missing %q in %q", line, s)
		}
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
