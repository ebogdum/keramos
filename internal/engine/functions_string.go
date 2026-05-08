package engine

import (
	"fmt"
	"strconv"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

func registerStringFuncs(r *FuncRegistry) {
	r.Register("upper", fnUpper)
	r.Register("lower", fnLower)
	r.Register("trim", fnTrim)
	r.Register("trimPrefix", fnTrimPrefix)
	r.Register("trimSuffix", fnTrimSuffix)
	r.Register("replace", fnReplace)
	r.Register("quote", fnQuote)
	r.Register("squote", fnSquote)
	r.Register("indent", fnIndent)
	r.Register("nindent", fnNindent)
	r.Register("trunc", fnTrunc)
}

func fnUpper(value any, args ...any) (any, error) {
	return strings.ToUpper(fmt.Sprintf("%v", value)), nil
}

func fnLower(value any, args ...any) (any, error) {
	return strings.ToLower(fmt.Sprintf("%v", value)), nil
}

func fnTrim(value any, args ...any) (any, error) {
	return strings.TrimSpace(fmt.Sprintf("%v", value)), nil
}

func fnTrimPrefix(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "trimPrefix requires a prefix argument")
	}
	return strings.TrimPrefix(fmt.Sprintf("%v", value), coerceString(args[0])), nil
}

func fnTrimSuffix(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "trimSuffix requires a suffix argument")
	}
	return strings.TrimSuffix(fmt.Sprintf("%v", value), coerceString(args[0])), nil
}

func fnReplace(value any, args ...any) (any, error) {
	if len(args) < 2 {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "replace requires old and new arguments")
	}
	return strings.ReplaceAll(fmt.Sprintf("%v", value), coerceString(args[0]), coerceString(args[1])), nil
}

func fnQuote(value any, args ...any) (any, error) {
	return fmt.Sprintf("%q", fmt.Sprintf("%v", value)), nil
}

func fnSquote(value any, args ...any) (any, error) {
	return "'" + fmt.Sprintf("%v", value) + "'", nil
}

// maxRepeatWidth bounds how many spaces / repetitions a template may
// request from indent/nindent/repeat/wrap-style functions. A user-supplied
// expression like ${"x" | repeat 1000000000} would otherwise OOM the
// renderer with a single line of YAML. 1<<16 (65 536) is far past anything
// a real package needs and well below the cliff where a single allocation
// becomes a problem.
const maxRepeatWidth = 1 << 16

func fnIndent(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "indent requires a width argument")
	}
	n, err := strconv.Atoi(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "indent: invalid width %q", args[0])
	}
	if n < 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "indent: width must be non-negative, got %d", n)
	}
	if n > maxRepeatWidth {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "indent: width %d exceeds %d", n, maxRepeatWidth)
	}
	pad := strings.Repeat(" ", n)
	s := fmt.Sprintf("%v", value)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n"), nil
}

func fnNindent(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "nindent requires a width argument")
	}
	n, err := strconv.Atoi(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "nindent: invalid width %q", args[0])
	}
	if n < 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "nindent: width must be non-negative, got %d", n)
	}
	if n > maxRepeatWidth {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "nindent: width %d exceeds %d", n, maxRepeatWidth)
	}
	pad := strings.Repeat(" ", n)
	s := fmt.Sprintf("%v", value)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return "\n" + strings.Join(lines, "\n"), nil
}

func fnTrunc(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "trunc requires a length argument")
	}
	n, err := strconv.Atoi(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "trunc: invalid length %q", args[0])
	}
	s := fmt.Sprintf("%v", value)
	if n < 0 {
		return "", nil
	}
	runes := []rune(s)
	if n >= len(runes) {
		return s, nil
	}
	return string(runes[:n]), nil
}
