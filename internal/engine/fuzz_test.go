package engine

import (
	"strings"
	"testing"
)

// FuzzSubstituteString feeds arbitrary strings through the expression
// substitution path. The contract under fuzz: never panic. Returning an error
// is fine; crashing the renderer on hostile/odd template text is not.
func FuzzSubstituteString(f *testing.F) {
	seeds := []string{
		"${values.x}",
		"plain text",
		"$${escaped}",
		`mixed ${"a}b" | upper} tail`,
		"${",
		"${}",
		"${ | | }",
		"${values.x | default 'y' | upper | nindent 2}",
		"${seq 1 3}",
		"$${${values.x}}",
		"${dict \"a\" 1 \"b\" 2}",
		"${1 | add 2 | mul 3}",
		strings.Repeat("${a}", 50),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	ctx := &RenderContext{
		Values:       map[string]any{"x": "hello", "n": 3, "list": []any{1, 2, 3}},
		Release:      map[string]any{"name": "r"},
		Package:      map[string]any{"name": "p"},
		Capabilities: map[string]any{},
	}
	funcs := NewFuncRegistry()

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic for any input. Errors are acceptable.
		defer func() {
			if r := recover(); nil != r {
				t.Fatalf("panic on input %q: %v", input, r)
			}
		}()
		_, _ = SubstituteAll(input, ctx, funcs)
	})
}
