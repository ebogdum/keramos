package engine

import (
	"sort"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

const letKey = "$let"
const inKey = "$in"

func processLet(m map[string]any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	bindings, ok := m[letKey].(map[string]any)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrType, "$let: expected a mapping of names, got %T", m[letKey])
	}
	body, hasBody := m[inKey]
	if !hasBody {
		return nil, keramoserrors.NewError(keramoserrors.ErrExpression, "$let requires an $in block")
	}

	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)

	scoped := ctx
	for _, name := range names {
		value, err := ProcessControlFlow(bindings[name], scoped, funcs)
		if nil != err {
			return nil, err
		}
		value, err = SubstituteAll(value, scoped, funcs)
		if nil != err {
			return nil, err
		}
		if isOmit(value) {
			value = nil
		}
		scoped = scopeWithVar(scoped, name, value)
		if !strings.HasPrefix(name, "$") {
			scoped = scopeWithVar(scoped, "$"+name, value)
		}
	}

	processed, err := ProcessControlFlow(body, scoped, funcs)
	if nil != err {
		return nil, err
	}
	return SubstituteAll(processed, scoped, funcs)
}
