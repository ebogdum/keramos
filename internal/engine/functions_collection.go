package engine

import (
	"fmt"
	"sort"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

func registerCollectionFuncs(r *FuncRegistry) {
	r.Register("keys", fnKeys)
	r.Register("values", fnValues)
	r.Register("first", fnFirst)
	r.Register("last", fnLast)
	r.Register("join", fnJoin)
	r.Register("sortAlpha", fnSortAlpha)
	r.Register("sortNumeric", fnSortNumeric)
	r.Register("uniq", fnUniq)
	r.Register("compact", fnCompact)
	r.Register("has", fnHas)
}

func fnKeys(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "keys: expected map, got %T", value)
	}
	result := make([]any, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].(string) < result[j].(string)
	})
	return result, nil
}

func fnValues(value any, args ...any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "values: expected map, got %T", value)
	}
	// Sort by key for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]any, 0, len(m))
	for _, k := range keys {
		result = append(result, m[k])
	}
	return result, nil
}

func fnFirst(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "first: expected list, got %T", value)
	}
	if 0 == len(list) {
		return nil, nil
	}
	return list[0], nil
}

func fnLast(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "last: expected list, got %T", value)
	}
	if 0 == len(list) {
		return nil, nil
	}
	return list[len(list)-1], nil
}

func fnJoin(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "join: expected list, got %T", value)
	}
	sep := ","
	if len(args) > 0 {
		sep = coerceString(args[0])
	}
	strs := make([]string, len(list))
	for i, v := range list {
		strs[i] = fmt.Sprintf("%v", v)
	}
	return strings.Join(strs, sep), nil
}

func fnSortAlpha(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "sortAlpha: expected list, got %T", value)
	}
	strs := make([]string, len(list))
	for i, v := range list {
		strs[i] = fmt.Sprintf("%v", v)
	}
	sort.Strings(strs)
	result := make([]any, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result, nil
}

// fnSortNumeric sorts a list by the numeric value of each element (ascending),
// preserving the original element and its type. Unlike sortAlpha it compares
// numbers, so [10, 2, 1] sorts to [1, 2, 10] rather than ["1", "10", "2"].
// Every element must be numeric or a numeric string; otherwise it errors.
func fnSortNumeric(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "sortNumeric: expected list, got %T", value)
	}
	type numbered struct {
		orig any
		num  float64
	}
	items := make([]numbered, len(list))
	for i, v := range list {
		f, okNum := coerceFloat(v)
		if !okNum {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "sortNumeric: element %d (%v) is not numeric", i, v)
		}
		items[i] = numbered{orig: v, num: f}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].num < items[j].num
	})
	result := make([]any, len(items))
	for i, it := range items {
		result[i] = it.orig
	}
	return result, nil
}

func fnUniq(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "uniq: expected list, got %T", value)
	}
	seen := make(map[string]bool, len(list))
	result := make([]any, 0, len(list))
	for _, v := range list {
		key := fmt.Sprintf("%v", v)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, v)
	}
	return result, nil
}

func fnCompact(value any, args ...any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "compact: expected list, got %T", value)
	}
	result := make([]any, 0, len(list))
	for _, v := range list {
		if !isEmpty(v) {
			result = append(result, v)
		}
	}
	return result, nil
}

func fnHas(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "has requires an item argument")
	}
	target := coerceString(args[0])

	switch v := value.(type) {
	case map[string]any:
		_, ok := v[target]
		return ok, nil
	case []any:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == target {
				return true, nil
			}
		}
		return false, nil
	default:
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "has: expected map or list, got %T", value)
	}
}
