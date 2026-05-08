package engine

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

// registerSprigExtras adds the remaining Sprig-parity functions keramos's engine
// did not previously expose. None of these need access to the engine or
// render context, so they live in the engine-wide registry.
func registerSprigExtras(r *FuncRegistry) {
	// Math / iteration
	r.Register("until", fnUntil)
	r.Register("untilStep", fnUntilStep)
	r.Register("randInt", fnRandInt)

	// String builders
	r.Register("wrap", fnWrap)
	r.Register("wrapWith", fnWrapWith)
	r.Register("nospace", fnNospace)

	// Collections
	r.Register("concat", fnConcat)
	r.Register("slice", fnSlice)
	r.Register("prepend", fnPrepend)
	r.Register("append", fnAppend)
	r.Register("reverse", fnReverse)
	r.Register("pluck", fnPluck)
	r.Register("pickv", fnPick) // alias of pick
	r.Register("dig", fnDig)
	r.Register("mustAppend", fnAppend)
	r.Register("mustPrepend", fnPrepend)
	r.Register("mustSlice", fnSlice)
	r.Register("mustReverse", fnReverse)

	// Encoding
	r.Register("b32enc", fnB32Enc)
	r.Register("b32dec", fnB32Dec)
	r.Register("fromJson", fnFromJSON)
	r.Register("fromYaml", fnFromYAML)
	r.Register("toRawJson", fnToRawJSON)
	r.Register("mustToJson", fnToJSON)
	r.Register("mustToRawJson", fnToRawJSON)
	r.Register("mustFromJson", fnFromJSON)

	// OS / env
	r.Register("env", fnEnv)
	r.Register("expandenv", fnExpandEnv)
	r.Register("getHostByName", fnGetHostByName)

	// Must-variants: same behavior as their non-must counterparts, but Sprig
	// users expect them to error rather than return zero values on failure.
	// Keramos's existing fns already error, so the must* variants are aliases.
	r.Register("mustRegexMatch", fnRegexMatch)
	r.Register("mustRegexFind", fnRegexFind)
	r.Register("mustRegexFindAll", fnRegexFindAll)
	r.Register("mustRegexReplaceAll", fnRegexReplaceAll)
	r.Register("mustRegexSplit", fnRegexSplit)
	r.Register("mustDate", fnDate)
	r.Register("mustToDate", fnToDate)

	// Reflection extras (kindIs/typeIs already cover the basics)
	r.Register("biggest", fnMax)
	r.Register("empty", fnEmpty)
}

// maxRangeLen bounds the number of items keramos is willing to materialise
// in until/untilStep/seq/randBytes/etc. — caps single-expression DoS.
const maxRangeLen = 1 << 16

func fnUntil(value any, args ...any) (any, error) {
	n, ok := coerceInt(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "until: expected integer, got %T", value)
	}
	if n > maxRangeLen || n < -maxRangeLen {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"until: range size %d exceeds %d", n, maxRangeLen)
	}
	if n < 0 {
		out := make([]any, 0, -n)
		for i := 0; i > n; i-- {
			out = append(out, i)
		}
		return out, nil
	}
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out, nil
}

func fnUntilStep(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "untilStep requires stop and step arguments")
	}
	start, ok1 := coerceInt(value)
	stop, ok2 := coerceInt(args[0])
	step, ok3 := coerceInt(args[1])
	if !ok1 || !ok2 || !ok3 {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "untilStep: all three arguments must be integers")
	}
	if 0 == step {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "untilStep: step must be non-zero")
	}
	// Pre-compute the range size and reject before allocating.
	var size int64
	if step > 0 {
		if start >= stop {
			return []any{}, nil
		}
		size = int64((stop - start + step - 1) / step)
	} else {
		if start <= stop {
			return []any{}, nil
		}
		size = int64((start - stop - step - 1) / -step)
	}
	if size > maxRangeLen {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"untilStep: range size %d exceeds %d", size, maxRangeLen)
	}
	out := make([]any, 0, size)
	if step > 0 {
		for i := start; i < stop; i += step {
			out = append(out, i)
		}
	} else {
		for i := start; i > stop; i += step {
			out = append(out, i)
		}
	}
	return out, nil
}

func fnRandInt(value any, args ...any) (any, error) {
	min, ok := coerceInt(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "randInt: expected min integer, got %T", value)
	}
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "randInt requires a max argument")
	}
	max, ok := coerceInt(args[0])
	if !ok {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "randInt: max must be integer")
	}
	if min >= max {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "randInt: min %d >= max %d", min, max)
	}
	span := big.NewInt(int64(max - min))
	idx, err := rand.Int(rand.Reader, span)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "randInt: read failed", err)
	}
	return min + int(idx.Int64()), nil
}

func fnWrap(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "wrap requires a width argument")
	}
	width, ok := coerceInt(args[0])
	if !ok || width <= 0 {
		return value, nil
	}
	return wrapAt(coerceString(value), width, " "), nil
}

func fnWrapWith(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "wrapWith requires width and break-string arguments")
	}
	width, ok := coerceInt(args[0])
	if !ok || width <= 0 {
		return value, nil
	}
	return wrapAt(coerceString(value), width, coerceString(args[1])), nil
}

func wrapAt(s string, width int, brk string) string {
	if 0 >= width {
		return s
	}
	var b strings.Builder
	col := 0
	for _, w := range strings.Fields(s) {
		if col > 0 && col+1+len(w) > width {
			b.WriteString(brk)
			col = 0
		}
		if col > 0 {
			b.WriteByte(' ')
			col++
		}
		b.WriteString(w)
		col += len(w)
	}
	return b.String()
}

func fnNospace(value any, args ...any) (any, error) {
	s := coerceString(value)
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if ' ' == c || '\t' == c || '\n' == c || '\r' == c {
			continue
		}
		out = append(out, c)
	}
	return string(out), nil
}

func fnConcat(value any, args ...any) (any, error) {
	out := make([]any, 0)
	if v, ok := value.([]any); ok {
		out = append(out, v...)
	} else if nil != value {
		out = append(out, value)
	}
	for _, a := range args {
		if v, ok := a.([]any); ok {
			out = append(out, v...)
		} else {
			out = append(out, a)
		}
	}
	return out, nil
}

func fnSlice(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "slice: expected list, got %T", value)
	}
	n := len(list)
	start, end := 0, n
	if 0 < len(args) {
		if s, ok := coerceInt(args[0]); ok {
			start = s
		}
	}
	if 1 < len(args) {
		if e, ok := coerceInt(args[1]); ok {
			end = e
		}
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start > end {
		return []any{}, nil
	}
	return list[start:end], nil
}

func fnPrepend(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "prepend: expected list, got %T", value)
	}
	out := make([]any, 0, len(list)+len(args))
	out = append(out, args...)
	out = append(out, list...)
	return out, nil
}

func fnAppend(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "append: expected list, got %T", value)
	}
	out := make([]any, 0, len(list)+len(args))
	out = append(out, list...)
	out = append(out, args...)
	return out, nil
}

func fnReverse(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "reverse: expected list, got %T", value)
	}
	out := make([]any, len(list))
	for i, v := range list {
		out[len(list)-1-i] = v
	}
	return out, nil
}

// fnPluck: pluck("name", $listOfDicts) → list of "name" values from each dict.
// In keramos pipeline form: ${listOfDicts | pluck "name"}.
func fnPluck(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "pluck: expected list, got %T", value)
	}
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "pluck requires a key argument")
	}
	key := coerceString(args[0])
	out := make([]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			if v, exists := m[key]; exists {
				out = append(out, v)
			}
		}
	}
	return out, nil
}

// fnDig walks a nested map by a key path: dig "a" "b" "c" defaultVal $map.
// In keramos pipeline form: ${map | dig "a" "b" "c" "default"} — the last arg is
// the default returned when the path is missing.
func fnDig(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "dig: expected map, got %T", value)
	}
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "dig requires at least a default argument")
	}
	def := args[len(args)-1]
	path := args[:len(args)-1]
	cur := any(m)
	for _, p := range path {
		curMap, ok := cur.(map[string]any)
		if !ok {
			return def, nil
		}
		next, exists := curMap[coerceString(p)]
		if !exists {
			return def, nil
		}
		cur = next
	}
	return cur, nil
}

func fnB32Enc(value any, args ...any) (any, error) {
	return base32.StdEncoding.EncodeToString([]byte(coerceString(value))), nil
}

func fnB32Dec(value any, args ...any) (any, error) {
	out, err := base32.StdEncoding.DecodeString(coerceString(value))
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "b32dec: invalid base32 input", err)
	}
	return string(out), nil
}

func fnFromJSON(value any, args ...any) (any, error) {
	var out any
	if err := json.Unmarshal([]byte(coerceString(value)), &out); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "fromJson: invalid JSON", err)
	}
	return out, nil
}

func fnFromYAML(value any, args ...any) (any, error) {
	var out any
	if err := yaml.Unmarshal([]byte(coerceString(value)), &out); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "fromYaml: invalid YAML", err)
	}
	return out, nil
}

// fnToRawJSON serialises without HTML-escaping (Sprig's toRawJson).
func fnToRawJSON(value any, args ...any) (any, error) {
	var b strings.Builder
	enc := json.NewEncoder(stringWriter{&b})
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toRawJson: marshal failed", err)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

type stringWriter struct{ b *strings.Builder }

func (w stringWriter) Write(p []byte) (int, error) { return w.b.Write(p) }

func fnEnv(value any, args ...any) (any, error) {
	return os.Getenv(coerceString(value)), nil
}

func fnExpandEnv(value any, args ...any) (any, error) {
	return os.ExpandEnv(coerceString(value)), nil
}

func fnGetHostByName(value any, args ...any) (any, error) {
	addrs, err := net.LookupHost(coerceString(value))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "getHostByName: lookup failed for %v", value)
	}
	if 0 == len(addrs) {
		return "", nil
	}
	return addrs[0], nil
}

// fnEmpty mirrors Sprig's `empty` function (existing fnEmpty in functions_logic.go
// already does this; this is registered via fnEmpty there). The reference here
// is unused but kept to demonstrate intent — alias only.
var _ = fnEmpty

// Helpers used by other extras. Kept here so this file is self-contained for
// the remaining wires.

func deepEqualValue(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// touch unused symbols to keep them in the binary even when no template uses them.
var (
	_ = deepEqualValue
	_ = sortedKeys
	_ = time.Now
	_ = fmt.Sprintf
	_ = regexp.Compile
)
