package engine

import (
	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

func registerLogicFuncs(r *FuncRegistry) {
	r.Register("default", fnDefault)
	r.Register("required", fnRequired)
	r.Register("empty", fnEmpty)
	r.Register("ternary", fnTernary)
}

func fnDefault(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "default requires a fallback argument")
	}
	if isEmpty(value) {
		return args[0], nil
	}
	return value, nil
}

func fnRequired(value any, args ...any) (any, error) {
	if isEmpty(value) {
		msg := "value is required"
		if len(args) > 0 {
			msg = coerceString(args[0])
		}
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, msg)
	}
	return value, nil
}

func fnEmpty(value any, args ...any) (any, error) {
	return isEmpty(value), nil
}

func fnTernary(value any, args ...any) (any, error) {
	if len(args) < 2 {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "ternary requires trueVal and falseVal arguments")
	}
	if isTruthy(value) {
		return args[0], nil
	}
	return args[1], nil
}

func isEmpty(v any) bool {
	if nil == v {
		return true
	}
	switch val := v.(type) {
	case string:
		return 0 == len(val)
	case bool:
		return !val
	case int:
		return 0 == val
	case int64:
		return 0 == val
	case float64:
		return 0 == val
	case []any:
		return 0 == len(val)
	case map[string]any:
		return 0 == len(val)
	default:
		return false
	}
}

func isTruthy(v any) bool {
	if nil == v {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		// Documented behaviour (docs/templates/expressions.md):
		// "false" / "FALSE" / "0" / "" are treated as falsy. Without
		// this, a value round-tripped through a string-only source
		// (env var, `--set foo=false`) would be truthy because the
		// string is non-empty.
		switch val {
		case "", "false", "False", "FALSE", "0", "no", "No", "NO":
			return false
		}
		return true
	case int:
		return 0 != val
	case int64:
		return 0 != val
	case float64:
		return 0 != val
	case []any:
		return len(val) > 0
	case map[string]any:
		return len(val) > 0
	default:
		return true
	}
}