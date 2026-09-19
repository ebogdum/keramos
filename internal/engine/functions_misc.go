package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	keramoserrors "github.com/ebogdum/keramos/v2/internal/errors"
	"gopkg.in/yaml.v3"
)

func fnToJSON(value any, args ...any) (any, error) {
	out, err := json.Marshal(value)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toJson: marshal failed", err)
	}
	return string(out), nil
}

func fnToYAML(value any, args ...any) (any, error) {
	out, err := yaml.Marshal(value)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toYaml: marshal failed", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func registerMiscFuncs(r *FuncRegistry) {
	r.Register("printf", fnPrintf)
	r.Register("sprintf", fnPrintf)
	r.Register("dict", fnDict)
	r.Register("set", fnSet)
	r.Register("unset", fnUnset)
	r.Register("get", fnGet)
	r.Register("hasKey", fnHasKey)
	r.Register("merge", fnMerge)
	r.Register("mergeOverwrite", fnMergeOverwrite)
	r.Register("pick", fnPick)
	r.Register("omit", fnOmit)
	r.Register("fail", fnFail)
	r.Register("kindOf", fnKindOf)
	r.Register("typeOf", fnKindOf)
	r.Register("kindIs", fnKindIs)
	r.Register("typeIs", fnKindIs)
	r.Register("semver", fnSemver)
	r.Register("semverCompare", fnSemverCompare)
	r.Register("coalesce", fnCoalesce)
	r.Register("toJson", fnToJSON)
	r.Register("toYAML", fnToYAML)
	r.Register("len", fnLen)
	r.Register("repeat", fnRepeat)
	r.Register("contains", fnContains)
	r.Register("hasPrefix", fnHasPrefix)
	r.Register("hasSuffix", fnHasSuffix)
	r.Register("split", fnSplit)
	r.Register("title", fnTitle)
	r.Register("untitle", fnUntitle)
	r.Register("substr", fnSubstr)
	r.Register("cat", fnCat)
}

// fnPrintf treats `value` as the format string and `args` as positional
// arguments (best-effort numeric coercion for %d/%f).
func fnPrintf(value any, args ...any) (any, error) {
	format := fmt.Sprintf("%v", value)
	return fmt.Sprintf(format, args...), nil
}

// fnDict builds a map from alternating key/value args. The pipeline value is
// prepended as the first key, then the args provide the remaining
// key/value pairs. Values can be any type — maps, slices, scalars — and
// are passed through without lossy stringification.
func fnDict(value any, args ...any) (any, error) {
	full := args
	if nil != value {
		full = append([]any{value}, args...)
	}
	if 1 == len(full)%2 {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "dict requires an even number of key/value arguments")
	}
	out := make(map[string]any, len(full)/2)
	for i := 0; i < len(full); i += 2 {
		out[coerceString(full[i])] = full[i+1]
	}
	return out, nil
}

func fnSet(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "set: expected map, got %T", value)
	}
	if 2 != len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "set requires key and value arguments")
	}
	// Copy-on-write: never mutate the shared render context map in place,
	// which would corrupt other keys/renders and race under parallel render.
	out := deepCopyMap(m)
	out[coerceString(args[0])] = args[1]
	return out, nil
}

func fnUnset(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "unset: expected map, got %T", value)
	}
	out := deepCopyMap(m)
	for _, k := range args {
		delete(out, coerceString(k))
	}
	return out, nil
}

func fnGet(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "get: expected map, got %T", value)
	}
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "get requires a key argument")
	}
	if v, exists := m[coerceString(args[0])]; exists {
		return v, nil
	}
	return nil, nil
}

func fnHasKey(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return false, nil
	}
	if 0 == len(args) {
		return false, nil
	}
	_, exists := m[coerceString(args[0])]
	return exists, nil
}

// fnMerge deep-merges JSON-encoded maps from args into the pipeline map.
// Existing keys in the destination win when the source value is the zero
// value for its kind.
func fnMerge(value any, args ...any) (any, error) {
	dst, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "merge: expected map, got %T", value)
	}
	out := deepCopyMap(dst)
	for _, raw := range args {
		var src map[string]any
		if jsonErr := json.Unmarshal([]byte(coerceString(raw)), &src); nil != jsonErr {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, jsonErr, "merge: arg is not valid JSON: %q", raw)
		}
		out = mergeMaps(out, src, false)
	}
	return out, nil
}

// fnMergeOverwrite is like merge but the source value always wins, even
// when it is the zero value for its kind.
func fnMergeOverwrite(value any, args ...any) (any, error) {
	dst, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "mergeOverwrite: expected map, got %T", value)
	}
	out := deepCopyMap(dst)
	for _, raw := range args {
		var src map[string]any
		if jsonErr := json.Unmarshal([]byte(coerceString(raw)), &src); nil != jsonErr {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, jsonErr, "mergeOverwrite: arg is not valid JSON: %q", raw)
		}
		out = mergeMaps(out, src, true)
	}
	return out, nil
}

func mergeMaps(dst, src map[string]any, overwrite bool) map[string]any {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		if dm, ok := dv.(map[string]any); ok {
			if sm, ok := sv.(map[string]any); ok {
				dst[k] = mergeMaps(dm, sm, overwrite)
				continue
			}
		}
		if overwrite {
			dst[k] = sv
			continue
		}
		// merge (non-overwrite): keep destination unless source is non-zero
		if !isZeroValue(dv) {
			continue
		}
		dst[k] = sv
	}
	return dst
}

func isZeroValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return "" == x
	case bool:
		return !x
	case int:
		return 0 == x
	case int64:
		return 0 == x
	case float64:
		return 0 == x
	case []any:
		return 0 == len(x)
	case map[string]any:
		return 0 == len(x)
	}
	return false
}

func deepCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

// deepCopyValue recursively copies maps AND slices so a copy-on-write of a
// render-context map cannot alias nested containers back to the shared
// original (which would reintroduce cross-render mutation / data races).
func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyMap(t)
	case []any:
		cp := make([]any, len(t))
		for i, e := range t {
			cp[i] = deepCopyValue(e)
		}
		return cp
	default:
		return v
	}
}

func fnPick(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "pick: expected map, got %T", value)
	}
	out := make(map[string]any, len(args))
	for _, ka := range args {
		k := coerceString(ka)
		if v, exists := m[k]; exists {
			out[k] = v
		}
	}
	return out, nil
}

func fnOmit(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "omit: expected map, got %T", value)
	}
	skip := make(map[string]bool, len(args))
	for _, ka := range args {
		skip[coerceString(ka)] = true
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if !skip[k] {
			out[k] = v
		}
	}
	return out, nil
}

func fnFail(value any, args ...any) (any, error) {
	msg := fmt.Sprintf("%v", value)
	if 0 < len(args) {
		msg = msg + " " + strings.Join(stringArgs(args), " ")
	}
	return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "fail: %s", msg)
}

func fnKindOf(value any, args ...any) (any, error) {
	if nil == value {
		return "invalid", nil
	}
	switch value.(type) {
	case bool:
		return "bool", nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int", nil
	case float32, float64:
		return "float64", nil
	case string:
		return "string", nil
	case map[string]any:
		return "map", nil
	case []any:
		return "slice", nil
	}
	return reflect.TypeOf(value).Kind().String(), nil
}

func fnKindIs(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return false, nil
	}
	kind, _ := fnKindOf(value)
	return kind == args[0], nil
}

func fnSemver(value any, args ...any) (any, error) {
	s, ok := value.(string)
	if !ok {
		s = fmt.Sprintf("%v", value)
	}
	v, err := semver.NewVersion(s)
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "semver: invalid version %q", s)
	}
	return v.String(), nil
}

// fnSemverCompare is invoked as `${version | semverCompare "<constraint>"}`:
// the pipeline value is the version under test and args[0] is the constraint.
func fnSemverCompare(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "semverCompare requires a constraint argument")
	}
	v, parseErr := semver.NewVersion(fmt.Sprintf("%v", value))
	if nil != parseErr {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, parseErr, "semverCompare: invalid version %q", value)
	}
	constraint, err := semver.NewConstraint(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "semverCompare: invalid constraint %q", args[0])
	}
	return constraint.Check(v), nil
}

func fnCoalesce(value any, args ...any) (any, error) {
	if !isEmpty(value) {
		return value, nil
	}
	for _, a := range args {
		if !isEmpty(a) {
			return a, nil
		}
	}
	return nil, nil
}

func fnLen(value any, args ...any) (any, error) {
	switch v := value.(type) {
	case string:
		return len(v), nil
	case []any:
		return len(v), nil
	case map[string]any:
		return len(v), nil
	}
	return 0, nil
}

func fnRepeat(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "repeat requires a count argument")
	}
	n, ok := toFloat(args[0])
	if !ok || n < 0 {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "repeat: invalid count %q", args[0])
	}
	// Defensive cap: a user-supplied count of 1e9 against even a small
	// value would OOM the renderer in a single expression.
	const maxRepeat = 1 << 16
	if int(n) > maxRepeat {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"repeat: count %d exceeds %d", int(n), maxRepeat)
	}
	s := fmt.Sprintf("%v", value)
	// Cap the RESULT size too. Capping only the count lets chained repeats
	// (`x | repeat 65536 | repeat 65536`) amplify to gigabytes and OOM.
	if err := checkOutputSize(len(s), int(n)); nil != err {
		return nil, err
	}
	return strings.Repeat(s, int(n)), nil
}

// maxTemplateOutputBytes bounds the byte size a single expansion helper may
// produce, so chained/large expansions cannot OOM the renderer.
const maxTemplateOutputBytes = 32 * 1024 * 1024 // 32 MB

// checkOutputSize rejects an expansion whose result (unit bytes × count) would
// exceed the per-expression output cap.
func checkOutputSize(unit, count int) error {
	if unit > 0 && count > maxTemplateOutputBytes/unit {
		return keramoserrors.NewErrorf(keramoserrors.ErrFunction,
			"expansion result would exceed the %d-byte output cap", maxTemplateOutputBytes)
	}
	return nil
}

func fnContains(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return false, nil
	}
	return strings.Contains(fmt.Sprintf("%v", value), coerceString(args[0])), nil
}

func fnHasPrefix(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return false, nil
	}
	return strings.HasPrefix(fmt.Sprintf("%v", value), coerceString(args[0])), nil
}

func fnHasSuffix(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return false, nil
	}
	return strings.HasSuffix(fmt.Sprintf("%v", value), coerceString(args[0])), nil
}

func fnSplit(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "split requires a separator argument")
	}
	parts := strings.Split(fmt.Sprintf("%v", value), coerceString(args[0]))
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out, nil
}

func fnTitle(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	words := strings.Fields(s)
	for i, w := range words {
		if 0 == len(w) {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " "), nil
}

func fnUntitle(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	words := strings.Fields(s)
	for i, w := range words {
		if 0 == len(w) {
			continue
		}
		words[i] = strings.ToLower(w[:1]) + w[1:]
	}
	return strings.Join(words, " "), nil
}

func fnSubstr(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "substr requires start and end arguments")
	}
	startF, sok := toFloat(args[0])
	endF, eok := toFloat(args[1])
	if !sok || !eok {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "substr: start and end must be numeric")
	}
	start, end := int(startF), int(endF)
	runes := []rune(fmt.Sprintf("%v", value))
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return "", nil
	}
	return string(runes[start:end]), nil
}

func fnCat(value any, args ...any) (any, error) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, fmt.Sprintf("%v", value))
	parts = append(parts, stringArgs(args)...)
	return strings.Join(parts, " "), nil
}
