package engine

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
)

// omitSentinel marks a value that must be DROPPED from its containing map or
// slice (the key disappears entirely) rather than rendered as `null`. It lets
// optional fields cleanly omit themselves: a field-level `$if` with no matching
// branch, an `$each` over a missing collection, or `${value | omitempty}` all
// resolve to this sentinel, and the surrounding map/slice prunes it.
type omitSentinel struct{}

var omit = omitSentinel{}

// isOmit reports whether v is the omit sentinel.
func isOmit(v any) bool {
	_, ok := v.(omitSentinel)
	return ok
}

// ProcessControlFlow walks the YAML tree and evaluates all control flow directives.
// It returns a new tree with directives resolved.
func ProcessControlFlow(node any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	switch v := node.(type) {
	case map[string]any:
		return processMap(v, ctx, funcs)
	case []any:
		return processSlice(v, ctx, funcs)
	default:
		return node, nil
	}
}

func processMap(m map[string]any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	if _, ok := m[textKey]; ok {
		return processText(m, ctx, funcs)
	}
	if _, ok := m[letKey]; ok {
		return processLet(m, ctx, funcs)
	}
	// Check for $if directive
	if ifExpr, ok := m["$if"]; ok {
		return processIf(m, ifExpr, ctx, funcs)
	}

	// Check for $each directive
	if eachExpr, ok := m["$each"]; ok {
		return processEach(m, eachExpr, ctx, funcs)
	}

	// Check for $switch directive
	if switchExpr, ok := m["$switch"]; ok {
		return processSwitch(m, switchExpr, ctx, funcs)
	}

	// Regular map: recursively process values. A value resolving to the omit
	// sentinel drops its key entirely (optional-field omission).
	result := make(map[string]any, len(m))
	var merged any
	for k, v := range m {
		processed, err := ProcessControlFlow(v, ctx, funcs)
		if nil != err {
			return nil, err
		}
		if isOmit(processed) {
			continue
		}
		if mergeKey == k {
			merged = processed
			continue
		}
		result[k] = processed
	}
	if nil != merged {
		if 0 == len(result) {
			return soleMerge(merged, ctx, funcs)
		}
		if err := applyMerge(result, merged, ctx, funcs); nil != err {
			return nil, err
		}
	}
	return result, nil
}

func soleMerge(value any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	resolved, err := resolveValue(value, ctx, funcs)
	if nil != err {
		return nil, err
	}
	resolved = parseMergeText(resolved)
	switch v := resolved.(type) {
	case nil:
		return omit, nil
	case []any:
		return mergeList(v, ctx, funcs)
	case map[string]any:
		return v, nil
	default:
		return resolved, nil
	}
}

func mergeList(items []any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	out := map[string]any{}
	list := make([]any, 0, len(items))
	for _, item := range items {
		resolved, err := resolveValue(item, ctx, funcs)
		if nil != err {
			return nil, err
		}
		switch v := resolved.(type) {
		case nil:
		case map[string]any:
			for k, val := range v {
				if isOmit(val) {
					continue
				}
				out[k] = val
			}
		case []any:
			list = append(list, v...)
		default:
			if !isOmit(resolved) {
				list = append(list, resolved)
			}
		}
	}
	if 0 < len(list) && 0 == len(out) {
		return list, nil
	}
	return out, nil
}

const mergeKey = "$merge"

func applyMerge(dst map[string]any, value any, ctx *RenderContext, funcs *FuncRegistry) error {
	resolved, err := resolveValue(value, ctx, funcs)
	if nil != err {
		return err
	}
	resolved = parseMergeText(resolved)
	switch v := resolved.(type) {
	case nil:
		return nil
	case map[string]any:
		for k, item := range v {
			if isOmit(item) {
				continue
			}
			dst[k] = item
		}
		return nil
	case []any:
		for _, item := range v {
			if isOmit(item) || nil == item {
				continue
			}
			if err := applyMerge(dst, item, ctx, funcs); nil != err {
				return err
			}
		}
		return nil
	default:
		return keramoserrors.NewErrorf(keramoserrors.ErrType,
			"$merge: expected a map or a list of maps, got %T", resolved)
	}
}

func parseMergeText(value any) any {
	text, isText := value.(string)
	if !isText {
		return value
	}
	if "" == strings.TrimSpace(text) {
		return nil
	}
	var parsed any
	if err := yaml.Unmarshal([]byte(text), &parsed); nil != err {
		return value
	}
	return normalizeYAML(parsed)
}

func normalizeYAML(value any) any {
	switch v := value.(type) {
	case map[any]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[fmt.Sprintf("%v", k)] = normalizeYAML(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = normalizeYAML(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeYAML(item))
		}
		return out
	}
	return value
}

func spliceMerge(dst []any, value any, ctx *RenderContext, funcs *FuncRegistry) ([]any, error) {
	resolved, err := resolveValue(value, ctx, funcs)
	if nil != err {
		return nil, err
	}
	switch v := resolved.(type) {
	case nil:
		return dst, nil
	case []any:
		for _, item := range v {
			if isOmit(item) || nil == item {
				continue
			}
			dst = append(dst, item)
		}
		return dst, nil
	default:
		return append(dst, resolved), nil
	}
}

func processSlice(s []any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	result := make([]any, 0, len(s))
	for _, item := range s {
		if spliced, ok := listMergeValue(item); ok {
			processed, err := ProcessControlFlow(spliced, ctx, funcs)
			if nil != err {
				return nil, err
			}
			if isOmit(processed) {
				continue
			}
			result, err = spliceMerge(result, processed, ctx, funcs)
			if nil != err {
				return nil, err
			}
			continue
		}
		processed, err := ProcessControlFlow(item, ctx, funcs)
		if nil != err {
			return nil, err
		}
		if isOmit(processed) {
			continue
		}
		result = append(result, processed)
	}
	return result, nil
}

func listMergeValue(item any) (any, bool) {
	m, isMap := item.(map[string]any)
	if !isMap || 1 != len(m) {
		return nil, false
	}
	v, ok := m[mergeKey]
	return v, ok
}

func processIf(m map[string]any, ifExpr any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	condition, err := resolveCondition(ifExpr, ctx, funcs)
	if nil != err {
		return nil, err
	}

	logger.Debug("$if condition evaluated to %v", condition)

	_, hasThen := m["$then"]
	_, hasElse := m["$else"]

	// Structured $if/$then/$else
	if hasThen {
		if condition {
			return processControlFlowResult(m["$then"], ctx, funcs)
		}
		if hasElse {
			return processControlFlowResult(m["$else"], ctx, funcs)
		}
		// No matching branch: omit this node so a field-level $if drops only
		// its own key rather than emitting `null`.
		return omit, nil
	}

	// Document/block-level $if: if falsy, omit the node. At a map field this
	// drops the key; at the document root the document is removed (see
	// renderDocumentWithRegistry, which treats omit like nil).
	if !condition {
		return omit, nil
	}

	// If truthy, return the map without $if
	result := make(map[string]any, len(m)-1)
	for k, v := range m {
		if "$if" == k {
			continue
		}
		processed, err := ProcessControlFlow(v, ctx, funcs)
		if nil != err {
			return nil, err
		}
		result[k] = processed
	}
	return result, nil
}

func processEach(m map[string]any, eachExpr any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	iterable, err := resolveValue(eachExpr, ctx, funcs)
	if nil != err {
		return nil, err
	}

	asName := "$item"
	if as, ok := m["$as"]; ok {
		asName = fmt.Sprintf("%v", as)
	}

	yieldTemplate, ok := m["$yield"]
	if !ok {
		return nil, keramoserrors.NewError(keramoserrors.ErrExpression, "$each requires a $yield block")
	}

	logger.Debug("$each iterating with var %s", asName)

	switch collection := iterable.(type) {
	case []any:
		return eachOverList(collection, asName, yieldTemplate, ctx, funcs)
	case map[string]any:
		return eachOverMap(collection, asName, yieldTemplate, ctx, funcs)
	case nil:
		// Iterating a MISSING collection omits the field entirely; iterating an
		// explicitly empty list (`[]`) still yields an empty list (see the
		// []any branch above), preserving the missing-vs-empty distinction.
		return omit, nil
	default:
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrType, "$each: expected list or map, got %T", iterable)
	}
}

func eachOverList(list []any, asName string, yieldTemplate any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	results := make([]any, 0, len(list))

	for _, item := range list {
		// Create a scoped context with loop variable
		scopedCtx := scopeWithVar(ctx, asName, item)

		// Deep clone template, then process
		cloned := deepClone(yieldTemplate)
		processed, err := ProcessControlFlow(cloned, scopedCtx, funcs)
		if nil != err {
			return nil, err
		}
		processed, err = SubstituteAll(processed, scopedCtx, funcs)
		if nil != err {
			return nil, err
		}

		if isOmit(processed) {
			continue // a yield that omits itself contributes no element
		}
		// Flatten if yield produces a list
		if resultList, ok := processed.([]any); ok {
			results = append(results, resultList...)
		} else {
			results = append(results, processed)
		}
	}
	return results, nil
}

func eachOverMap(m map[string]any, asName string, yieldTemplate any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	results := make([]any, 0, len(m))

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m[k]
		// Set $key, $value, and the named variable as map{key, value}
		entry := map[string]any{"key": k, "value": v}
		scopedCtx := scopeWithVar(ctx, asName, entry)
		scopedCtx = scopeWithVar(scopedCtx, "$key", k)
		scopedCtx = scopeWithVar(scopedCtx, "$value", v)

		cloned := deepClone(yieldTemplate)
		processed, err := ProcessControlFlow(cloned, scopedCtx, funcs)
		if nil != err {
			return nil, err
		}
		processed, err = SubstituteAll(processed, scopedCtx, funcs)
		if nil != err {
			return nil, err
		}

		if isOmit(processed) {
			continue
		}
		if resultList, ok := processed.([]any); ok {
			results = append(results, resultList...)
		} else {
			results = append(results, processed)
		}
	}
	return results, nil
}

func processSwitch(m map[string]any, switchExpr any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	switchVal, err := resolveValue(switchExpr, ctx, funcs)
	if nil != err {
		return nil, err
	}

	switchStr := fmt.Sprintf("%v", switchVal)
	logger.Debug("$switch on value: %s", switchStr)

	cases, ok := m["$cases"].(map[string]any)
	if !ok {
		return nil, keramoserrors.NewError(keramoserrors.ErrExpression, "$switch requires a $cases map")
	}

	if caseVal, found := cases[switchStr]; found {
		return processControlFlowResult(caseVal, ctx, funcs)
	}

	if defaultVal, hasDefault := m["$default"]; hasDefault {
		return processControlFlowResult(defaultVal, ctx, funcs)
	}

	// No matching case and no $default: omit the field entirely (drop the key),
	// matching $if. Returning nil would write an unintended `key: null`.
	return omit, nil
}

// processControlFlowResult recursively processes a control flow result.
func processControlFlowResult(node any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	return ProcessControlFlow(node, ctx, funcs)
}

// resolveCondition evaluates a value to a boolean condition.
func resolveCondition(expr any, ctx *RenderContext, funcs *FuncRegistry) (bool, error) {
	val, err := resolveValue(expr, ctx, funcs)
	if nil != err {
		return false, err
	}
	return isTruthy(val), nil
}

// resolveValue resolves an expression value — if it's a string containing ${...}, evaluate it.
func resolveValue(expr any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	s, ok := expr.(string)
	if !ok {
		return expr, nil
	}
	result, err := substituteString(s, ctx, funcs)
	if nil != err {
		return nil, err
	}
	return result, nil
}

// scopeWithVar creates a new RenderContext with an additional variable in Values.
func scopeWithVar(ctx *RenderContext, name string, value any) *RenderContext {
	newValues := make(map[string]any, len(ctx.Values)+1)
	for k, v := range ctx.Values {
		newValues[k] = v
	}
	newValues[name] = value
	return &RenderContext{
		Values:       newValues,
		Package:      ctx.Package,
		Release:      ctx.Release,
		Capabilities: ctx.Capabilities,
		Files:        ctx.Files,
	}
}

// deepClone creates a deep copy of a YAML tree.
func deepClone(node any) any {
	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = deepClone(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = deepClone(val)
		}
		return result
	default:
		return v
	}
}
