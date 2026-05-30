package engine

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

// registerSprigFinal closes the last set of Sprig parity gaps: type casts
// (int/int64/float64), URL escaping (urlquery/urlqueryescape), multi-document
// YAML (fromYamlArray and must variants), collection extras (without,
// initial, rest, tuple, splitn, seq), and regexQuoteMeta.
func registerSprigFinal(r *FuncRegistry) {
	r.Register("int", fnInt)
	r.Register("int64", fnInt64)
	r.Register("float64", fnFloat64)
	r.Register("urlquery", fnURLQueryEscape)
	r.Register("urlqueryescape", fnURLQueryEscape)
	r.Register("fromYamlArray", fnFromYAMLArray)
	r.Register("mustFromYamlArray", fnFromYAMLArray)
	r.Register("mustFromYaml", fnFromYAML)
	r.Register("seq", fnSeq)
	r.Register("splitn", fnSplitN)
	r.Register("without", fnWithout)
	r.Register("initial", fnInitial)
	r.Register("rest", fnRest)
	r.Register("tuple", fnTuple)
	r.Register("regexQuoteMeta", fnRegexQuoteMeta)
	// must-variants of common collection ops — same behavior, error semantics
	// preserved (keramos's existing fns already error on type mismatch).
	r.Register("mustHas", fnHas)
	r.Register("mustFirst", fnFirst)
	r.Register("mustLast", fnLast)
	r.Register("mustInitial", fnInitial)
	r.Register("mustRest", fnRest)
	r.Register("mustWithout", fnWithout)
	r.Register("mustUniq", fnUniq)
	r.Register("mustCompact", fnCompact)
	r.Register("mustConcat", fnConcat)
	r.Register("mustPick", fnPick)
	r.Register("mustOmit", fnOmit)
	r.Register("mustMerge", fnMerge)
	r.Register("mustMergeOverwrite", fnMergeOverwrite)
}

func fnInt(value any, args ...any) (any, error) {
	if i, ok := coerceInt(value); ok {
		return i, nil
	}
	return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "int: cannot coerce %T", value)
}

func fnInt64(value any, args ...any) (any, error) {
	f, ok := coerceFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "int64: cannot coerce %T", value)
	}
	return int64(f), nil
}

func fnFloat64(value any, args ...any) (any, error) {
	f, ok := coerceFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "float64: cannot coerce %T", value)
	}
	return f, nil
}

func fnURLQueryEscape(value any, args ...any) (any, error) {
	return url.QueryEscape(coerceString(value)), nil
}

// fnFromYAMLArray decodes a multi-document YAML stream into a slice. Each
// document becomes one element; mismatched types pass through as raw any.
func fnFromYAMLArray(value any, args ...any) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(coerceString(value)))
	out := make([]any, 0, 4)
	for {
		var doc any
		err := dec.Decode(&doc)
		if nil != err {
			if "EOF" == err.Error() {
				break
			}
			return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "fromYamlArray: invalid YAML", err)
		}
		out = append(out, doc)
	}
	return out, nil
}

// fnSeq reproduces the unix `seq` semantics: 1-arg `seq end`, 2-arg
// `seq start end`, 3-arg `seq start step end`. Pipeline form pushes the
// first arg through value.
func fnSeq(value any, args ...any) (any, error) {
	all := make([]any, 0, len(args)+1)
	all = append(all, value)
	all = append(all, args...)
	nums := make([]int, 0, len(all))
	for _, a := range all {
		i, ok := coerceInt(a)
		if !ok {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "seq: arg %v is not an integer", a)
		}
		nums = append(nums, i)
	}
	var start, end, step int
	switch len(nums) {
	case 1:
		start, end, step = 1, nums[0], 1
	case 2:
		start, end, step = nums[0], nums[1], 1
	case 3:
		start, end, step = nums[0], nums[2], nums[1]
	default:
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "seq accepts 1, 2, or 3 integer arguments")
	}
	if 0 == step {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "seq: step must be non-zero")
	}
	// Bound the materialised slice to avoid OOM from a single expression.
	// Computed in float64 so an enormous start/end can't overflow the count.
	approx := (float64(end) - float64(start)) / float64(step)
	if approx < 0 {
		approx = -approx
	}
	if approx > float64(maxRangeLen) {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "seq: range size %d exceeds %d", int64(approx)+1, maxRangeLen)
	}
	out := make([]any, 0)
	if step > 0 {
		for i := start; i <= end; i += step {
			out = append(out, i)
		}
	} else {
		for i := start; i >= end; i += step {
			out = append(out, i)
		}
	}
	return out, nil
}

// fnSplitN splits with a maximum count.
//
// Pipeline form: ${value | splitn sep n} → list of at most n parts.
func fnSplitN(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "splitn requires sep and count arguments")
	}
	sep := coerceString(args[0])
	n, ok := coerceInt(args[1])
	if !ok {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "splitn: count must be integer")
	}
	parts := strings.SplitN(coerceString(value), sep, n)
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

// fnWithout returns the list with `args` removed (set difference).
func fnWithout(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "without: expected list, got %T", value)
	}
	out := make([]any, 0, len(list))
	for _, item := range list {
		drop := false
		for _, a := range args {
			if reflect.DeepEqual(item, a) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, item)
		}
	}
	return out, nil
}

func fnInitial(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "initial: expected list, got %T", value)
	}
	if 0 == len(list) {
		return []any{}, nil
	}
	return list[:len(list)-1], nil
}

func fnRest(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "rest: expected list, got %T", value)
	}
	if 0 == len(list) {
		return []any{}, nil
	}
	return list[1:], nil
}

// fnTuple builds a list from positional args. Pipeline form prepends the
// pipeline value as the first element when non-nil — matches Sprig's intent
// of producing an ad-hoc list.
func fnTuple(value any, args ...any) (any, error) {
	out := make([]any, 0, len(args)+1)
	if nil != value {
		out = append(out, value)
	}
	out = append(out, args...)
	return out, nil
}

func fnRegexQuoteMeta(value any, args ...any) (any, error) {
	return regexp.QuoteMeta(coerceString(value)), nil
}

// touch fmt/strconv to keep imports stable across compile modes.
var _ = fmt.Sprintf
var _ = strconv.Atoi
