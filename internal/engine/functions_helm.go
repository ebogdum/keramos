package engine

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Masterminds/sprig/v3"
	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

type helmSubject int

const (
	subjectFirst helmSubject = 0
	subjectLast  helmSubject = -1
	subjectNone  helmSubject = -2
)

func subjectAt(n int) helmSubject { return helmSubject(n) }

var sprigFuncs = func() map[string]any {
	fm := map[string]any(sprig.TxtFuncMap())
	delete(fm, "env")
	delete(fm, "expandenv")
	return fm
}()

var bridged = map[string]helmSubject{
	"abbrev":                     subjectLast,
	"abbrevboth":                 subjectLast,
	"add1":                       subjectFirst,
	"add1f":                      subjectFirst,
	"all":                        subjectNone,
	"any":                        subjectNone,
	"atoi":                       subjectFirst,
	"b64dec":                     subjectFirst,
	"b64enc":                     subjectFirst,
	"buildCustomCert":            subjectFirst,
	"date_in_zone":               subjectLast,
	"date_modify":                subjectLast,
	"duration":                   subjectFirst,
	"durationRound":              subjectFirst,
	"fromJsonArray":              subjectFirst,
	"genCAWithKey":               subjectFirst,
	"genSelfSignedCertWithKey":   subjectFirst,
	"genSignedCertWithKey":       subjectFirst,
	"hello":                      subjectNone,
	"html":                       subjectFirst,
	"js":                         subjectFirst,
	"maxf":                       subjectFirst,
	"minf":                       subjectFirst,
	"mustFromJsonArray":          subjectFirst,
	"mustPush":                   subjectFirst,
	"mustRegexReplaceAllLiteral": subjectAt(1),
	"mustToPrettyJson":           subjectFirst,
	"must_date_modify":           subjectLast,
	"plural":                     subjectLast,
	"push":                       subjectFirst,
	"regexReplaceAllLiteral":     subjectAt(1),
	"shuffle":                    subjectFirst,
	"splitList":                  subjectLast,
	"toDecimal":                  subjectFirst,
	"toPrettyJson":               subjectFirst,
	"toStrings":                  subjectFirst,
	"toToml":                     subjectFirst,
	"trimAll":                    subjectLast,
	"trimall":                    subjectLast,
	"typeIsLike":                 subjectLast,
	"unixEpoch":                  subjectFirst,
}

func registerHelmFuncs(r *FuncRegistry) {
	for name, subject := range bridged {
		fn, ok := sprigFuncs[name]
		if !ok {
			continue
		}
		r.Register(name, makeSprigBridge(name, fn, subject))
	}

	r.Register("eq", fnEq)
	r.Register("ne", fnNe)
	r.Register("lt", fnLt)
	r.Register("le", fnLe)
	r.Register("gt", fnGt)
	r.Register("ge", fnGe)
	r.Register("and", fnAnd)
	r.Register("or", fnOr)
	r.Register("not", fnNot)
	r.Register("list", fnList)
	r.Register("index", fnIndex)
	r.Register("print", fnPrint)
	r.Register("println", fnPrintln)
	r.Register("toYamlPretty", fnToYamlPretty)
}

func makeSprigBridge(name string, fn any, subject helmSubject) Func {
	rv := reflect.ValueOf(fn)
	if reflect.Func != rv.Kind() {
		return func(any, ...any) (any, error) {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "%s: not a function", name)
		}
	}
	return func(value any, args ...any) (any, error) {
		call := assembleArgs(value, args, subject)
		converted, err := convertArgs(name, rv.Type(), call)
		if nil != err {
			return nil, err
		}
		out, err := callReflect(name, rv, converted)
		if nil != err {
			return nil, err
		}
		return normalizeResult(out), nil
	}
}

func normalizeResult(value any) any {
	switch v := value.(type) {
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeResult(item))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = normalizeResult(item)
		}
		return out
	case nil:
		return nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out = append(out, normalizeResult(rv.Index(i).Interface()))
		}
		return out
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		for _, key := range rv.MapKeys() {
			out[coerceString(key.Interface())] = normalizeResult(rv.MapIndex(key).Interface())
		}
		return out
	}
	return value
}

func assembleArgs(value any, args []any, subject helmSubject) []any {
	switch {
	case subjectNone == subject:
		return args
	case subjectLast == subject:
		out := make([]any, 0, len(args)+1)
		out = append(out, args...)
		return append(out, value)
	case 0 >= int(subject):
		out := make([]any, 0, len(args)+1)
		out = append(out, value)
		return append(out, args...)
	default:
		idx := int(subject)
		if idx > len(args) {
			idx = len(args)
		}
		out := make([]any, 0, len(args)+1)
		out = append(out, args[:idx]...)
		out = append(out, value)
		return append(out, args[idx:]...)
	}
}

func convertArgs(name string, ft reflect.Type, args []any) ([]reflect.Value, error) {
	numIn := ft.NumIn()
	out := make([]reflect.Value, 0, len(args))
	for i, a := range args {
		var want reflect.Type
		switch {
		case ft.IsVariadic() && i >= numIn-1:
			want = ft.In(numIn - 1).Elem()
		case i < numIn:
			want = ft.In(i)
		default:
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction,
				"%s: takes %d arguments, got %d", name, numIn, len(args))
		}
		v, err := coerceToType(a, want)
		if nil != err {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "%s: argument %d", name, i+1)
		}
		out = append(out, v)
	}
	if !ft.IsVariadic() && len(out) < numIn {
		for i := len(out); i < numIn; i++ {
			out = append(out, reflect.Zero(ft.In(i)))
		}
	}
	return out, nil
}

func coerceToType(v any, want reflect.Type) (reflect.Value, error) {
	if reflect.Interface == want.Kind() && 0 == want.NumMethod() {
		if nil == v {
			return reflect.Zero(want), nil
		}
		return reflect.ValueOf(v), nil
	}
	if nil == v {
		return reflect.Zero(want), nil
	}
	rv := reflect.ValueOf(v)
	if rv.Type() == want {
		return rv, nil
	}
	switch want.Kind() {
	case reflect.String:
		return reflect.ValueOf(coerceString(v)), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := coerceInt(v)
		if !ok {
			return reflect.Value{}, keramoserrors.NewErrorf(keramoserrors.ErrType, "cannot use %v as integer", v)
		}
		return reflect.ValueOf(int64(n)).Convert(want), nil
	case reflect.Float32, reflect.Float64:
		f, ok := coerceFloat(v)
		if !ok {
			return reflect.Value{}, keramoserrors.NewErrorf(keramoserrors.ErrType, "cannot use %v as number", v)
		}
		return reflect.ValueOf(f).Convert(want), nil
	case reflect.Bool:
		return reflect.ValueOf(isTruthy(v)), nil
	}
	if rv.Type().ConvertibleTo(want) {
		return rv.Convert(want), nil
	}
	return reflect.Value{}, keramoserrors.NewErrorf(keramoserrors.ErrType,
		"cannot use %T as %s", v, want)
}

func callReflect(name string, fn reflect.Value, args []reflect.Value) (result any, err error) {
	defer func() {
		if r := recover(); nil != r {
			result = nil
			err = keramoserrors.NewErrorf(keramoserrors.ErrFunction, "%s failed: %v", name, r)
		}
	}()
	results := fn.Call(args)
	if 0 == len(results) {
		return nil, nil
	}
	last := results[len(results)-1]
	if last.Type() == reflect.TypeOf((*error)(nil)).Elem() {
		if !last.IsNil() {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction,
				last.Interface().(error), "%s failed", name)
		}
		if 1 == len(results) {
			return nil, nil
		}
		return results[0].Interface(), nil
	}
	return results[0].Interface(), nil
}

func compareValues(a, b any) int {
	if isNumericValue(a) && isNumericValue(b) {
		an, aOK := coerceFloat(a)
		bn, bOK := coerceFloat(b)
		if aOK && bOK {
			switch {
			case an < bn:
				return -1
			case an > bn:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(coerceString(a), coerceString(b))
}

func isNumericValue(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	return false
}

func fnEq(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "eq: needs at least one argument")
	}
	for _, a := range args {
		if !valuesEqual(value, a) {
			return false, nil
		}
	}
	return true, nil
}

func fnNe(value any, args ...any) (any, error) {
	eq, err := fnEq(value, args...)
	if nil != err {
		return nil, err
	}
	return !eq.(bool), nil
}

func valuesEqual(a, b any) bool {
	if nil == a || nil == b {
		return nil == a && nil == b
	}
	if isNumericValue(a) && isNumericValue(b) {
		af, aOK := coerceFloat(a)
		bf, bOK := coerceFloat(b)
		return aOK && bOK && af == bf
	}
	if reflect.DeepEqual(a, b) {
		return true
	}
	_, aIsBool := a.(bool)
	_, bIsBool := b.(bool)
	if aIsBool != bIsBool {
		return false
	}
	return coerceString(a) == coerceString(b)
}

func fnLt(value any, args ...any) (any, error) { return cmpFunc("lt", value, args, -1, -1) }
func fnLe(value any, args ...any) (any, error) { return cmpFunc("le", value, args, -1, 0) }
func fnGt(value any, args ...any) (any, error) { return cmpFunc("gt", value, args, 1, 1) }
func fnGe(value any, args ...any) (any, error) { return cmpFunc("ge", value, args, 0, 1) }

func cmpFunc(name string, value any, args []any, wantA, wantB int) (any, error) {
	if 1 != len(args) {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "%s: needs exactly one argument", name)
	}
	c := compareValues(value, args[0])
	return c == wantA || c == wantB, nil
}

func fnAnd(value any, args ...any) (any, error) {
	if !isTruthy(value) {
		return value, nil
	}
	result := value
	for _, a := range args {
		if !isTruthy(a) {
			return a, nil
		}
		result = a
	}
	return result, nil
}

func fnOr(value any, args ...any) (any, error) {
	if isTruthy(value) {
		return value, nil
	}
	var last any = value
	for _, a := range args {
		if isTruthy(a) {
			return a, nil
		}
		last = a
	}
	return last, nil
}

func fnNot(value any, args ...any) (any, error) {
	return !isTruthy(value), nil
}

func fnList(value any, args ...any) (any, error) {
	out := make([]any, 0, len(args)+1)
	if nil != value {
		out = append(out, value)
	}
	out = append(out, args...)
	return out, nil
}

func fnIndex(value any, args ...any) (any, error) {
	current := value
	for _, key := range args {
		switch container := current.(type) {
		case map[string]any:
			current = container[coerceString(key)]
		case map[any]any:
			current = container[key]
		case []any:
			n, ok := coerceInt(key)
			if !ok {
				return nil, keramoserrors.NewErrorf(keramoserrors.ErrType, "index: %v is not a list key", key)
			}
			if n < 0 || n >= len(container) {
				return nil, nil
			}
			current = container[n]
		case nil:
			return nil, nil
		default:
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrType, "index: cannot index %T", current)
		}
	}
	return current, nil
}

func fnPrint(value any, args ...any) (any, error) {
	return fmt.Sprint(assembleArgs(value, args, subjectFirst)...), nil
}

func fnPrintln(value any, args ...any) (any, error) {
	return fmt.Sprintln(assembleArgs(value, args, subjectFirst)...), nil
}

func fnToYamlPretty(value any, args ...any) (any, error) {
	out, err := marshalYAML(value)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toYamlPretty failed", err)
	}
	return strings.TrimRight(out, "\n"), nil
}

func HelmSubjectPosition(name string) (int, bool) {
	if pos, ok := helmSubjectOverrides[name]; ok {
		return int(pos), true
	}
	if pos, ok := bridged[name]; ok {
		return int(pos), true
	}
	if KnownFunc(name) {
		return int(subjectFirst), true
	}
	return 0, false
}
