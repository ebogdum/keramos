package engine

import (
	"fmt"
	"sort"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

const textKey = "$text"

func registerFormatEach(r *FuncRegistry) {
	r.Register("formatEach", fnFormatEach)
}

func fnFormatEach(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "formatEach: needs a format string")
	}
	format := coerceString(args[0])
	fields := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		fields = append(fields, coerceString(arg))
	}

	items, err := formatEachItems(value)
	if nil != err {
		return nil, err
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		values := make([]any, 0, len(fields))
		for _, field := range fields {
			values = append(values, formatEachField(item, field))
		}
		out = append(out, fmt.Sprintf(format, values...))
	}
	return out, nil
}

func formatEachItems(value any) ([]any, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []any:
		return v, nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(v))
		for _, k := range keys {
			out = append(out, map[string]any{"key": k, "value": v[k]})
		}
		return out, nil
	}
	return nil, keramoserrors.NewErrorf(keramoserrors.ErrType, "formatEach: expected a list or map, got %T", value)
}

func formatEachField(item any, field string) any {
	if "." == field || "" == field {
		return item
	}
	current := item
	for _, part := range strings.Split(field, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

const joinKey = "$join"
const indentKey = "$indent"

func processText(m map[string]any, ctx *RenderContext, funcs *FuncRegistry) (any, error) {
	separator := "\n"
	if raw, ok := m[joinKey]; ok {
		resolved, err := resolveValue(raw, ctx, funcs)
		if nil != err {
			return nil, err
		}
		separator = coerceString(resolved)
	}

	processed, err := ProcessControlFlow(m[textKey], ctx, funcs)
	if nil != err {
		return nil, err
	}
	substituted, err := SubstituteAll(processed, ctx, funcs)
	if nil != err {
		return nil, err
	}

	parts, err := flattenText(substituted)
	if nil != err {
		return nil, err
	}
	out := strings.Join(parts, separator)

	width, err := textIndent(m, ctx, funcs)
	if nil != err {
		return nil, err
	}
	if 0 < width {
		out = indentBlock(out, width)
	}
	return out, nil
}

func textIndent(m map[string]any, ctx *RenderContext, funcs *FuncRegistry) (int, error) {
	raw, ok := m[indentKey]
	if !ok {
		return 0, nil
	}
	resolved, err := resolveValue(raw, ctx, funcs)
	if nil != err {
		return 0, err
	}
	width, converted := coerceInt(resolved)
	if !converted {
		return 0, keramoserrors.NewErrorf(keramoserrors.ErrType, "$indent: expected a number, got %v", resolved)
	}
	return width, nil
}

func flattenText(node any) ([]string, error) {
	switch v := node.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if isOmit(item) {
				continue
			}
			part, err := flattenText(item)
			if nil != err {
				return nil, err
			}
			out = append(out, part...)
		}
		return out, nil
	case map[string]any:
		return nil, keramoserrors.NewError(keramoserrors.ErrType,
			"$text: an item rendered a mapping; $text builds lines of text, so items must resolve to strings or lists")
	}
	if isOmit(node) {
		return nil, nil
	}
	return []string{fmt.Sprintf("%v", node)}, nil
}

func indentBlock(text string, width int) string {
	pad := strings.Repeat(" ", width)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if "" == strings.TrimSpace(line) {
			continue
		}
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
