package engine

import (
	"fmt"
	"regexp"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

func registerRegexFuncs(r *FuncRegistry) {
	r.Register("regexMatch", fnRegexMatch)
	r.Register("regexFind", fnRegexFind)
	r.Register("regexFindAll", fnRegexFindAll)
	r.Register("regexReplaceAll", fnRegexReplaceAll)
	r.Register("regexSplit", fnRegexSplit)
}

func fnRegexMatch(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "regexMatch requires a pattern argument")
	}
	re, err := regexp.Compile(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "regexMatch: invalid pattern %q", args[0])
	}
	return re.MatchString(fmt.Sprintf("%v", value)), nil
}

func fnRegexFind(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "regexFind requires a pattern argument")
	}
	re, err := regexp.Compile(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "regexFind: invalid pattern %q", args[0])
	}
	return re.FindString(fmt.Sprintf("%v", value)), nil
}

// maxRegexMatches caps the number of matches returned by regexFindAll /
// regexSplit. Without it, a pattern like `\b` against a 100KB string
// produces millions of empty-string matches, allocating millions of
// any-boxed strings — a single-expression DoS. Callers can pass an
// explicit n via the second argument; -1 / unspecified gets clamped.
const maxRegexMatches = 1024

func fnRegexFindAll(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "regexFindAll requires a pattern argument")
	}
	re, err := regexp.Compile(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "regexFindAll: invalid pattern %q", args[0])
	}
	limit := maxRegexMatches
	if 2 <= len(args) {
		if n, ok := toFloat(args[1]); ok && n > 0 && int(n) < maxRegexMatches {
			limit = int(n)
		}
	}
	matches := re.FindAllString(fmt.Sprintf("%v", value), limit)
	out := make([]any, len(matches))
	for i, m := range matches {
		out[i] = m
	}
	return out, nil
}

func fnRegexReplaceAll(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "regexReplaceAll requires pattern and replacement arguments")
	}
	re, err := regexp.Compile(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "regexReplaceAll: invalid pattern %q", args[0])
	}
	return re.ReplaceAllString(fmt.Sprintf("%v", value), coerceString(args[1])), nil
}

func fnRegexSplit(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "regexSplit requires a pattern argument")
	}
	re, err := regexp.Compile(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "regexSplit: invalid pattern %q", args[0])
	}
	limit := maxRegexMatches
	if 2 <= len(args) {
		if n, ok := toFloat(args[1]); ok && n > 0 && int(n) < maxRegexMatches {
			limit = int(n)
		}
	}
	parts := re.Split(fmt.Sprintf("%v", value), limit)
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}
