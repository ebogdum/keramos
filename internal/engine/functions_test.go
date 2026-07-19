package engine

import (
	"strings"
	"testing"
)

func TestStringFunctions(t *testing.T) {
	r := NewFuncRegistry()

	tests := []struct {
		name     string
		fn       string
		value    any
		args     []string
		expected any
	}{
		{"upper", "upper", "hello", nil, "HELLO"},
		{"upper empty", "upper", "", nil, ""},
		{"lower", "lower", "HELLO", nil, "hello"},
		{"trim", "trim", "  hello  ", nil, "hello"},
		{"trimPrefix", "trimPrefix", "hello-world", []string{"hello-"}, "world"},
		{"trimSuffix", "trimSuffix", "hello-world", []string{"-world"}, "hello"},
		{"replace", "replace", "hello world", []string{"world", "go"}, "hello go"},
		{"quote", "quote", "hello", nil, `"hello"`},
		{"squote", "squote", "hello", nil, "'hello'"},
		{"trunc 3", "trunc", "hello", []string{"3"}, "hel"},
		{"trunc longer than string", "trunc", "hi", []string{"10"}, "hi"},
		{"trunc negative", "trunc", "hello", []string{"-1"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := r.Get(tt.fn)
			if !ok {
				t.Fatalf("function %q not found", tt.fn)
			}
			anyArgs := make([]any, len(tt.args))
			for i, a := range tt.args {
				anyArgs[i] = a
			}
			result, err := fn(tt.value, anyArgs...)
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIndentFunction(t *testing.T) {
	r := NewFuncRegistry()
	fn, _ := r.Get("indent")

	result, err := fn("line1\nline2", "4")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "    line1\n    line2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNindentFunction(t *testing.T) {
	r := NewFuncRegistry()
	fn, _ := r.Get("nindent")

	result, err := fn("line1\nline2", "2")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	s := result.(string)
	if !strings.HasPrefix(s, "\n") {
		t.Error("nindent should start with newline")
	}
	if !strings.Contains(s, "  line1") {
		t.Errorf("expected indented content, got %q", s)
	}
}

func TestTypeFunctions(t *testing.T) {
	r := NewFuncRegistry()

	t.Run("toString", func(t *testing.T) {
		fn, _ := r.Get("toString")
		result, err := fn(42)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "42" != result {
			t.Errorf("expected '42', got %v", result)
		}
	})

	t.Run("toInt from string", func(t *testing.T) {
		fn, _ := r.Get("toInt")
		result, err := fn("42")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if 42 != result {
			t.Errorf("expected 42, got %v", result)
		}
	})

	t.Run("toInt from float", func(t *testing.T) {
		fn, _ := r.Get("toInt")
		result, err := fn(3.14)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if 3 != result {
			t.Errorf("expected 3, got %v", result)
		}
	})

	t.Run("toInt from bool", func(t *testing.T) {
		fn, _ := r.Get("toInt")
		result, err := fn(true)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if 1 != result {
			t.Errorf("expected 1, got %v", result)
		}
	})

	t.Run("toBool from string", func(t *testing.T) {
		fn, _ := r.Get("toBool")
		result, err := fn("true")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if true != result {
			t.Errorf("expected true, got %v", result)
		}
	})

	t.Run("toBool from nil", func(t *testing.T) {
		fn, _ := r.Get("toBool")
		result, err := fn(nil)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if false != result {
			t.Errorf("expected false, got %v", result)
		}
	})

	t.Run("toYaml", func(t *testing.T) {
		fn, _ := r.Get("toYaml")
		result, err := fn(map[string]any{"key": "val"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		s := result.(string)
		if !strings.Contains(s, "key: val") {
			t.Errorf("expected YAML output, got %q", s)
		}
	})

	t.Run("toJson", func(t *testing.T) {
		fn, _ := r.Get("toJson")
		result, err := fn(map[string]any{"key": "val"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		s := result.(string)
		if !strings.Contains(s, `"key":"val"`) {
			t.Errorf("expected JSON output, got %q", s)
		}
	})
}

func TestEncodingFunctions(t *testing.T) {
	r := NewFuncRegistry()

	t.Run("b64encode and decode", func(t *testing.T) {
		enc, _ := r.Get("b64encode")
		dec, _ := r.Get("b64decode")

		encoded, err := enc("hello world")
		if nil != err {
			t.Fatalf("encode error: %v", err)
		}
		decoded, err := dec(encoded)
		if nil != err {
			t.Fatalf("decode error: %v", err)
		}
		if "hello world" != decoded {
			t.Errorf("round-trip failed: got %v", decoded)
		}
	})

	t.Run("sha256", func(t *testing.T) {
		fn, _ := r.Get("sha256")
		result, err := fn("hello")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		s := result.(string)
		if 64 != len(s) {
			t.Errorf("expected 64-char hex string, got len %d", len(s))
		}
	})

	t.Run("b64decode invalid", func(t *testing.T) {
		fn, _ := r.Get("b64decode")
		_, err := fn("not-valid-base64!!!")
		if nil == err {
			t.Error("expected error for invalid base64")
		}
	})
}

func TestLogicFunctions(t *testing.T) {
	r := NewFuncRegistry()

	t.Run("default with nil value", func(t *testing.T) {
		fn, _ := r.Get("default")
		result, err := fn(nil, "fallback")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "fallback" != result {
			t.Errorf("expected 'fallback', got %v", result)
		}
	})

	t.Run("default with existing value", func(t *testing.T) {
		fn, _ := r.Get("default")
		result, err := fn("existing", "fallback")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "existing" != result {
			t.Errorf("expected 'existing', got %v", result)
		}
	})

	t.Run("default with empty string", func(t *testing.T) {
		fn, _ := r.Get("default")
		result, err := fn("", "fallback")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "fallback" != result {
			t.Errorf("expected 'fallback' for empty string, got %v", result)
		}
	})

	t.Run("required with value", func(t *testing.T) {
		fn, _ := r.Get("required")
		result, err := fn("exists", "must have value")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "exists" != result {
			t.Errorf("expected 'exists', got %v", result)
		}
	})

	t.Run("required with nil", func(t *testing.T) {
		fn, _ := r.Get("required")
		_, err := fn(nil, "must have value")
		if nil == err {
			t.Error("expected error for nil required value")
		}
	})

	t.Run("empty with nil", func(t *testing.T) {
		fn, _ := r.Get("empty")
		result, err := fn(nil)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if true != result {
			t.Errorf("expected true for nil, got %v", result)
		}
	})

	t.Run("empty with value", func(t *testing.T) {
		fn, _ := r.Get("empty")
		result, err := fn("something")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if false != result {
			t.Errorf("expected false for non-empty, got %v", result)
		}
	})

	t.Run("ternary true", func(t *testing.T) {
		fn, _ := r.Get("ternary")
		result, err := fn(true, "yes", "no")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "yes" != result {
			t.Errorf("expected 'yes', got %v", result)
		}
	})

	t.Run("ternary false", func(t *testing.T) {
		fn, _ := r.Get("ternary")
		result, err := fn(false, "yes", "no")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "no" != result {
			t.Errorf("expected 'no', got %v", result)
		}
	})
}

func TestCollectionFunctions(t *testing.T) {
	r := NewFuncRegistry()

	t.Run("keys", func(t *testing.T) {
		fn, _ := r.Get("keys")
		result, err := fn(map[string]any{"b": 1, "a": 2})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if 2 != len(list) {
			t.Fatalf("expected 2 keys, got %d", len(list))
		}
		if "a" != list[0] || "b" != list[1] {
			t.Errorf("expected sorted keys [a, b], got %v", list)
		}
	})

	t.Run("values", func(t *testing.T) {
		fn, _ := r.Get("values")
		result, err := fn(map[string]any{"b": 2, "a": 1})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if 2 != len(list) {
			t.Fatalf("expected 2 values, got %d", len(list))
		}
		// Sorted by key: a=1, b=2
		if 1 != list[0] || 2 != list[1] {
			t.Errorf("expected [1, 2], got %v", list)
		}
	})

	t.Run("first", func(t *testing.T) {
		fn, _ := r.Get("first")
		result, err := fn([]any{"a", "b", "c"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "a" != result {
			t.Errorf("expected 'a', got %v", result)
		}
	})

	t.Run("first empty", func(t *testing.T) {
		fn, _ := r.Get("first")
		result, err := fn([]any{})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if nil != result {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("last", func(t *testing.T) {
		fn, _ := r.Get("last")
		result, err := fn([]any{"a", "b", "c"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "c" != result {
			t.Errorf("expected 'c', got %v", result)
		}
	})

	t.Run("join", func(t *testing.T) {
		fn, _ := r.Get("join")
		result, err := fn([]any{"a", "b", "c"}, ", ")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "a, b, c" != result {
			t.Errorf("expected 'a, b, c', got %v", result)
		}
	})

	t.Run("join default sep", func(t *testing.T) {
		fn, _ := r.Get("join")
		result, err := fn([]any{"a", "b"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if "a,b" != result {
			t.Errorf("expected 'a,b', got %v", result)
		}
	})

	t.Run("sortAlpha", func(t *testing.T) {
		fn, _ := r.Get("sortAlpha")
		result, err := fn([]any{"c", "a", "b"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if "a" != list[0] || "b" != list[1] || "c" != list[2] {
			t.Errorf("expected [a, b, c], got %v", list)
		}
	})

	t.Run("sortNumeric", func(t *testing.T) {
		fn, _ := r.Get("sortNumeric")
		// numeric sort, not lexical: 10 comes after 2, and types are preserved.
		result, err := fn([]any{10, 2, 1})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if 1 != list[0] || 2 != list[1] || 10 != list[2] {
			t.Errorf("expected [1, 2, 10], got %v", list)
		}
		// numeric strings coerce and sort by value.
		result, err = fn([]any{"10", "2", "1"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list = result.([]any)
		if "1" != list[0] || "2" != list[1] || "10" != list[2] {
			t.Errorf("expected [1, 2, 10] (strings), got %v", list)
		}
		// a non-numeric element is an error.
		if _, err := fn([]any{1, "abc"}); nil == err {
			t.Errorf("expected error on non-numeric element, got nil")
		}
	})

	t.Run("uniq", func(t *testing.T) {
		fn, _ := r.Get("uniq")
		result, err := fn([]any{"a", "b", "a", "c"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if 3 != len(list) {
			t.Errorf("expected 3 unique items, got %d", len(list))
		}
	})

	t.Run("compact", func(t *testing.T) {
		fn, _ := r.Get("compact")
		result, err := fn([]any{"a", "", nil, "b"})
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		list := result.([]any)
		if 2 != len(list) {
			t.Errorf("expected 2 items after compact, got %d: %v", len(list), list)
		}
	})

	t.Run("has in map", func(t *testing.T) {
		fn, _ := r.Get("has")
		result, err := fn(map[string]any{"foo": 1}, "foo")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if true != result {
			t.Error("expected true for existing key")
		}
	})

	t.Run("has missing in map", func(t *testing.T) {
		fn, _ := r.Get("has")
		result, err := fn(map[string]any{"foo": 1}, "bar")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if false != result {
			t.Error("expected false for missing key")
		}
	})

	t.Run("has in list", func(t *testing.T) {
		fn, _ := r.Get("has")
		result, err := fn([]any{"a", "b", "c"}, "b")
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		if true != result {
			t.Error("expected true for existing item")
		}
	})
}

func TestFunctionErrors(t *testing.T) {
	r := NewFuncRegistry()

	t.Run("trimPrefix no args", func(t *testing.T) {
		fn, _ := r.Get("trimPrefix")
		_, err := fn("hello")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("replace insufficient args", func(t *testing.T) {
		fn, _ := r.Get("replace")
		_, err := fn("hello", "h")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("indent no args", func(t *testing.T) {
		fn, _ := r.Get("indent")
		_, err := fn("hello")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("toInt invalid string", func(t *testing.T) {
		fn, _ := r.Get("toInt")
		_, err := fn("not-a-number")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("keys non-map", func(t *testing.T) {
		fn, _ := r.Get("keys")
		_, err := fn("not-a-map")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("first non-list", func(t *testing.T) {
		fn, _ := r.Get("first")
		_, err := fn("not-a-list")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("has no args", func(t *testing.T) {
		fn, _ := r.Get("has")
		_, err := fn(map[string]any{})
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("ternary insufficient args", func(t *testing.T) {
		fn, _ := r.Get("ternary")
		_, err := fn(true, "yes")
		if nil == err {
			t.Error("expected error")
		}
	})

	t.Run("default no args", func(t *testing.T) {
		fn, _ := r.Get("default")
		_, err := fn(nil)
		if nil == err {
			t.Error("expected error")
		}
	})
}
