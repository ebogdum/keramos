package engine

import (
	"reflect"
	"testing"
)

func TestRegexMatch(t *testing.T) {
	out, err := fnRegexMatch("foo123", "^foo[0-9]+$")
	if nil != err {
		t.Fatalf("regexMatch: %v", err)
	}
	if true != out {
		t.Errorf("regexMatch = %v, want true", out)
	}
}

func TestRegexFindAll(t *testing.T) {
	out, err := fnRegexFindAll("a1 b2 c3", `[a-z][0-9]`)
	if nil != err {
		t.Fatalf("regexFindAll: %v", err)
	}
	want := []any{"a1", "b2", "c3"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("regexFindAll = %v, want %v", out, want)
	}
}

func TestRegexReplaceAll(t *testing.T) {
	out, err := fnRegexReplaceAll("hello world", `\s+`, "-")
	if nil != err {
		t.Fatalf("regexReplaceAll: %v", err)
	}
	if "hello-world" != out {
		t.Errorf("regexReplaceAll = %v", out)
	}
}

func TestRegexSplit(t *testing.T) {
	out, _ := fnRegexSplit("a, b , c", `\s*,\s*`)
	want := []any{"a", "b", "c"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("regexSplit = %v, want %v", out, want)
	}
}

func TestRegexBadPattern(t *testing.T) {
	if _, err := fnRegexMatch("", "[invalid"); nil == err {
		t.Fatal("expected invalid-pattern error")
	}
}
