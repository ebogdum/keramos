package engine

import (
	"fmt"
	"strconv"
)

// coerceString returns a string view of any value. Maps/slices fall through to
// fmt.Sprintf for diagnostic compatibility; format-aware functions should
// prefer json.Marshal explicitly.
func coerceString(v any) string {
	if nil == v {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// coerceFloat extracts a float64 from any numeric type or numeric string.
func coerceFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	case string:
		if f, err := strconv.ParseFloat(n, 64); nil == err {
			return f, true
		}
	}
	return 0, false
}

// coerceInt extracts an int from a numeric value, truncating floats.
func coerceInt(v any) (int, bool) {
	f, ok := coerceFloat(v)
	if !ok {
		return 0, false
	}
	return int(f), true
}

// stringArgs converts ...any args to ...string using coerceString. Convenience
// wrapper for fns that historically only accepted strings.
func stringArgs(args []any) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = coerceString(a)
	}
	return out
}

// inferArgLiteral converts a parsed argument string to its native type when
// the literal looks numeric/boolean. Used by EvaluateExpression so functions
// like printf and dict can receive correctly-typed args without each fn
// having to re-parse from a string.
func inferArgLiteral(s string) any {
	if "true" == s {
		return true
	}
	if "false" == s {
		return false
	}
	if "null" == s || "nil" == s {
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); nil == err {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); nil == err {
		return f
	}
	return s
}
