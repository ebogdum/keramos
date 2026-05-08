package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

func registerTypeFuncs(r *FuncRegistry) {
	r.Register("toYaml", fnToYaml)
	r.Register("toJson", fnToJson)
	r.Register("toString", fnToString)
	r.Register("toInt", fnToInt)
	r.Register("toBool", fnToBool)
}

func fnToYaml(value any, args ...any) (any, error) {
	b, err := yaml.Marshal(value)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toYaml: marshal failed", err)
	}
	return strings.TrimRight(string(b), "\n"), nil
}

func fnToJson(value any, args ...any) (any, error) {
	b, err := json.Marshal(value)
	if nil != err {
		return nil, keramoserrors.WrapError(keramoserrors.ErrFunction, "toJson: marshal failed", err)
	}
	return string(b), nil
}

func fnToString(value any, args ...any) (any, error) {
	return fmt.Sprintf("%v", value), nil
}

func fnToInt(value any, args ...any) (any, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		n, err := strconv.Atoi(v)
		if nil != err {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "toInt: cannot convert %q", v)
		}
		return n, nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "toInt: unsupported type %T", value)
	}
}

func fnToBool(value any, args ...any) (any, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if nil != err {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "toBool: cannot convert %q", v)
		}
		return b, nil
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case nil:
		return false, nil
	default:
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "toBool: unsupported type %T", value)
	}
}
