package engine

import (
	"strconv"
	"fmt"
	"regexp"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
)

var exprPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// EvaluateExpression evaluates a single ${...} expression against the context.
// Returns the resolved value (can be any type: string, int, bool, map, slice).
func EvaluateExpression(expr string, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	expr = strings.TrimSpace(expr)
	logger.Debug("evaluating expression: %s", expr)

	segments, err := splitPipeline(expr)
	if nil != err {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrExpression, "failed to parse expression %q", expr).WithExpression(expr)
	}
	if 0 == len(segments) {
		return nil, keramoserrors.NewError(keramoserrors.ErrExpression, "empty expression").WithExpression(expr)
	}

	// First segment is either a path expression OR a bare function call
	// of the form `name arg1 "arg2"`. When the leading identifier
	// matches a registered function and is followed by whitespace-separated
	// args (which a path expression cannot have), evaluate it as a function
	// applied to a nil value. This makes `${http "url"}`, `${sops "x.enc"}`,
	// `${vault "secret/path" "key"}` work without requiring an empty pipe
	// (`${"" | http "url"}`).
	firstSeg := strings.TrimSpace(segments[0])
	var value any
	// String literal as the first segment: `${"hello" | upper}`. Strip the
	// quotes and use the literal as the pipeline input.
	if isStringLiteral(firstSeg) {
		value = stripQuotes(firstSeg)
	} else if isNumericLiteral(firstSeg) {
		value = inferArgLiteral(firstSeg)
	} else if isBoolLiteral(firstSeg) {
		value = "true" == firstSeg
	} else if isNullLiteral(firstSeg) {
		value = nil
	} else if name, args, ok := parseSpaceCall(firstSeg); ok {
		if fn, exists := funcs.Get(name); exists {
			typed := make([]any, len(args))
			for i, a := range args {
				typed[i] = inferArgLiteral(a)
			}
			// Bare-call convention: the first arg is the pipeline input
			// (value), remaining args are extras. This makes
			// `${upper "hello"}` work. For constructors like `dict` and
			// `list` that don't use `value`, the first-arg slot is
			// harmlessly absorbed as their first argument.
			var v any
			var fnErr error
			if 0 < len(typed) {
				v, fnErr = fn(typed[0], typed[1:]...)
			} else {
				v, fnErr = fn(nil)
			}
			if nil != fnErr {
				return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, fnErr,
					"function %q failed", name).WithExpression(expr)
			}
			value = v
		} else {
			// Not a known function — fall back to path resolution.
			v, pErr := resolvePath(firstSeg, ctx)
			if nil != pErr {
				return nil, pErr
			}
			value = v
		}
	} else {
		v, pErr := resolvePath(firstSeg, ctx)
		if nil != pErr {
			return nil, pErr
		}
		value = v
	}

	// Remaining segments are function calls. Each segment can use either the
	// paren form (`name(arg1, arg2)`) or the bare space form
	// (`name arg1 "arg2"`). parseSpaceCall handles the latter.
	for i := 1; i < len(segments); i++ {
		seg := strings.TrimSpace(segments[i])
		var fnName string
		var fnArgs []string
		if name, args, ok := parseSpaceCall(seg); ok {
			fnName, fnArgs = name, args
		} else {
			n, a, parseErr := parseFuncCall(seg)
			if nil != parseErr {
				return nil, parseErr
			}
			fnName, fnArgs = n, a
		}

		fn, ok := funcs.Get(fnName)
		if !ok {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "unknown function %q", fnName).WithExpression(expr)
		}

		typed := make([]any, len(fnArgs))
		for i, a := range fnArgs {
			typed[i] = inferArgLiteral(a)
		}
		var fnErr error
		value, fnErr = fn(value, typed...)
		if nil != fnErr {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, fnErr, "function %q failed", fnName).WithExpression(expr)
		}
	}

	return value, nil
}

// SubstituteAll walks a YAML tree and replaces all ${...} expressions in string values.
// If a string value is ENTIRELY a single ${expr}, the result preserves the expression's native type.
// If the string contains mixed content like "prefix-${expr}-suffix", result is always string.
func SubstituteAll(node any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	switch v := node.(type) {
	case string:
		return substituteString(v, ctx, funcs)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			resolved, err := SubstituteAll(val, ctx, funcs)
			if nil != err {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			resolved, err := SubstituteAll(val, ctx, funcs)
			if nil != err {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return node, nil
	}
}

// substituteString handles expression substitution in a single string value.
func substituteString(s string, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	// Check if the entire string is a single expression
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "${") && strings.HasSuffix(trimmed, "}") {
		inner := trimmed[2 : len(trimmed)-1]
		// Verify no other ${ exists inside (which would mean it's not a single expression)
		if !strings.Contains(inner, "${") {
			return EvaluateExpression(inner, ctx, funcs)
		}
	}

	// Mixed content: replace all ${...} and produce a string
	var evalErr error
	result := exprPattern.ReplaceAllStringFunc(s, func(match string) string {
		if nil != evalErr {
			return match
		}
		inner := match[2 : len(match)-1]
		val, err := EvaluateExpression(inner, ctx, funcs)
		if nil != err {
			evalErr = err
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if nil != evalErr {
		return nil, evalErr
	}
	return result, nil
}

// resolvePath resolves a dotted path against the render context.
// Paths like "values.x.y" resolve to ctx.Values["x"]["y"].
func resolvePath(path string, ctx *RenderContext) (any, error) {
	parts := strings.SplitN(path, ".", 2)
	namespace := parts[0]
	var root map[string]any

	switch namespace {
	case "values":
		root = ctx.Values
	case "package":
		root = ctx.Package
	case "release":
		root = ctx.Release
	case "capabilities":
		root = ctx.Capabilities
	default:
		// Try values as default namespace
		root = ctx.Values
		return lookupPath(root, path)
	}

	if len(parts) < 2 {
		return root, nil
	}
	return lookupPath(root, parts[1])
}

// lookupPath traverses a nested map by dotted path. A numeric path
// component (e.g. `values.ports.0` or `values.images.0.repo`) indexes into
// a slice — both forms `arr.0` and `arr[0]` are supported.
func lookupPath(data map[string]any, path string) (any, error) {
	if nil == data {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrUndefinedVar, "cannot resolve path %q: nil data", path)
	}
	// Preserve "empty path → nil" semantics that downstream code (default,
	// existence-check) relies on.
	if "" == strings.TrimSpace(path) {
		return nil, nil
	}
	parts := splitPathSegments(path)
	if 0 == len(parts) {
		return nil, nil
	}
	var current any = data

	for _, part := range parts {
		switch container := current.(type) {
		case map[string]any:
			val, exists := container[part]
			if !exists {
				return nil, nil // undefined yields nil; default() can run
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(part)
			if nil != err {
				return nil, keramoserrors.NewErrorf(keramoserrors.ErrUndefinedVar,
					"cannot index slice with non-integer key %q", part)
			}
			if idx < 0 || idx >= len(container) {
				return nil, nil
			}
			current = container[idx]
		default:
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrUndefinedVar,
				"cannot index into %T at %q", current, part)
		}
	}
	return current, nil
}

// splitPathSegments splits a dotted path with bracket-style array indexing
// folded into dotted form: `images[0].repo` → ["images","0","repo"].
func splitPathSegments(path string) []string {
	expanded := strings.NewReplacer("[", ".", "]", "").Replace(path)
	parts := strings.Split(expanded, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if "" != p {
			out = append(out, p)
		}
	}
	return out
}

// splitPipeline splits an expression on | characters, but not inside parentheses or quotes.
func splitPipeline(expr string) ([]string, error) {
	segments := make([]string, 0, 4)
	depth := 0
	inSingle := false
	inDouble := false
	start := 0
	runes := []rune(expr)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case '\\' == ch && i+1 < len(runes):
			i++ // skip escaped character
		case '\'' == ch && !inDouble:
			inSingle = !inSingle
		case '"' == ch && !inSingle:
			inDouble = !inDouble
		case '(' == ch && !inSingle && !inDouble:
			depth++
		case ')' == ch && !inSingle && !inDouble:
			depth--
		case '|' == ch && !inSingle && !inDouble && 0 == depth:
			segments = append(segments, string(runes[start:i]))
			start = i + 1
		}
	}
	segments = append(segments, string(runes[start:]))
	return segments, nil
}

// isStringLiteral returns true when s is `"..."` or `'...'` and contains a
// matching closing quote. Used as the first-segment check so string-literal
// pipelines `${"hello" | upper}` parse correctly.
func isStringLiteral(s string) bool {
	if 2 > len(s) {
		return false
	}
	if '"' == s[0] && '"' == s[len(s)-1] {
		return true
	}
	if '\'' == s[0] && '\'' == s[len(s)-1] {
		return true
	}
	return false
}

// isNumericLiteral returns true when s is a plain int/float literal.
func isNumericLiteral(s string) bool {
	if "" == s {
		return false
	}
	if _, err := strconv.Atoi(s); nil == err {
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); nil == err {
		return true
	}
	return false
}

// isBoolLiteral returns true for the YAML/JSON-canonical boolean tokens.
func isBoolLiteral(s string) bool {
	return "true" == s || "false" == s
}

// isNullLiteral returns true for the YAML/JSON-canonical null tokens.
func isNullLiteral(s string) bool {
	return "null" == s || "nil" == s
}

// parseSpaceCall recognises bare calls of the form
// `name arg1 "arg2 with spaces" 'arg3'`. Returns ok=false if the string is a
// bare identifier (which the caller should resolve as a path) or contains
// `(` (which goes through parseFuncCall instead).
func parseSpaceCall(s string) (string, []string, bool) {
	s = strings.TrimSpace(s)
	if "" == s {
		return "", nil, false
	}
	if strings.ContainsAny(s, "(") {
		return "", nil, false
	}
	// Find first whitespace not inside quotes.
	runes := []rune(s)
	inSingle, inDouble := false, false
	idx := -1
	for i, ch := range runes {
		switch {
		case '\\' == ch && i+1 < len(runes):
			// no-op, the following char is consumed by the loop normally
		case '\'' == ch && !inDouble:
			inSingle = !inSingle
		case '"' == ch && !inSingle:
			inDouble = !inDouble
		case (' ' == ch || '\t' == ch) && !inSingle && !inDouble:
			idx = i
		}
		if idx >= 0 {
			break
		}
	}
	if idx < 0 {
		return "", nil, false
	}
	name := strings.TrimSpace(string(runes[:idx]))
	if "" == name || !isIdentifier(name) {
		return "", nil, false
	}
	rest := strings.TrimSpace(string(runes[idx+1:]))
	args := splitSpaceArgs(rest)
	return name, args, true
}

func isIdentifier(s string) bool {
	for i, ch := range s {
		if !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || '_' == ch) {
			if 0 == i {
				return false
			}
			// Dots are allowed AFTER position 0 to support dotted function
			// names like `Files.Get` and `Capabilities.APIVersions.Has`.
			if !('0' <= ch && ch <= '9') && '.' != ch {
				return false
			}
		}
	}
	return true
}

// splitSpaceArgs splits whitespace-separated args, respecting single/double quotes.
func splitSpaceArgs(s string) []string {
	out := make([]string, 0, 4)
	runes := []rune(s)
	inSingle, inDouble := false, false
	start := 0
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case '\\' == ch && i+1 < len(runes):
			i++
			continue
		case '\'' == ch && !inDouble:
			inSingle = !inSingle
		case '"' == ch && !inSingle:
			inDouble = !inDouble
		case (' ' == ch || '\t' == ch) && !inSingle && !inDouble:
			if start < i {
				out = append(out, stripQuotes(strings.TrimSpace(string(runes[start:i]))))
			}
			start = i + 1
		}
	}
	if start < len(runes) {
		out = append(out, stripQuotes(strings.TrimSpace(string(runes[start:]))))
	}
	return out
}

// parseFuncCall parses a function call like "funcName" or "funcName('arg1', arg2)".
func parseFuncCall(s string) (string, []string, error) {
	parenIdx := strings.Index(s, "(")
	if -1 == parenIdx {
		return s, nil, nil
	}

	name := strings.TrimSpace(s[:parenIdx])
	if !strings.HasSuffix(s, ")") {
		return "", nil, keramoserrors.NewErrorf(keramoserrors.ErrExpression, "malformed function call %q: missing closing paren", s)
	}

	argsStr := s[parenIdx+1 : len(s)-1]
	args, err := parseArgs(argsStr)
	if nil != err {
		return "", nil, err
	}
	return name, args, nil
}

// parseArgs parses comma-separated arguments, respecting quotes.
func parseArgs(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if 0 == len(s) {
		return nil, nil
	}

	args := make([]string, 0, 4)
	inSingle := false
	inDouble := false
	depth := 0
	start := 0
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case '\\' == ch && i+1 < len(runes):
			i++
		case '\'' == ch && !inDouble:
			inSingle = !inSingle
		case '"' == ch && !inSingle:
			inDouble = !inDouble
		case '(' == ch && !inSingle && !inDouble:
			depth++
		case ')' == ch && !inSingle && !inDouble:
			depth--
		case ',' == ch && !inSingle && !inDouble && 0 == depth:
			args = append(args, stripQuotes(strings.TrimSpace(string(runes[start:i]))))
			start = i + 1
		}
	}
	args = append(args, stripQuotes(strings.TrimSpace(string(runes[start:]))))
	return args, nil
}

// stripQuotes removes surrounding single or double quotes from a string.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if ('\'' == s[0] && '\'' == s[len(s)-1]) || ('"' == s[0] && '"' == s[len(s)-1]) {
			return s[1 : len(s)-1]
		}
	}
	return s
}
