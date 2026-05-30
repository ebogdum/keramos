package engine

import (
	"fmt"
	"sort"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
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
	for k, v := range m {
		processed, err := ProcessControlFlow(v, ctx, funcs)
		if nil != err {
			return nil, err
		}
		if isOmit(processed) {
			continue
		}
		result[k] = processed
	}
	return result, nil
}

func processSlice(s []any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	result := make([]any, 0, len(s))
	for _, item := range s {
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

	return nil, nil
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
