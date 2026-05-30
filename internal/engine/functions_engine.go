package engine

import (
	"fmt"
	"strings"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

// Engine-aware function constructors live below. They are bound per-render via
// buildRenderRegistry in engine.go, which produces an isolated layered registry
// rather than mutating the engine-wide one. This avoids data races under
// concurrent renders.

const maxTplDepth = 16

func makeTplFunc(e *Engine, ctx *RenderContext, partials map[string]any) Func {
	return func(value any, args ...any) (any, error) {
		if int32(maxTplDepth) <= ctx.tplDepth.Load() {
			return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "tpl: recursion depth exceeded")
		}
		s := fmt.Sprintf("%v", value)
		ctx.tplDepth.Add(1)
		defer ctx.tplDepth.Add(-1)
		// Render the template body as a single anonymous file.
		out, err := e.RenderFile("__tpl__", s, partials, ctx)
		if nil != err {
			return nil, err
		}
		return strings.Join(out, "---\n"), nil
	}
}

func makeIncludeFunc(e *Engine, ctx *RenderContext, partials map[string]any) Func {
	return func(value any, args ...any) (any, error) {
		name := fmt.Sprintf("%v", value)
		raw, ok := partials[name]
		if !ok {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "include: partial %q not found", name)
		}
		if int32(maxTplDepth) <= ctx.tplDepth.Load() {
			return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "include: recursion depth exceeded")
		}
		var body string
		switch v := raw.(type) {
		case string:
			body = v
		default:
			// Structured partial (map/slice from _helpers.yaml): marshal to
			// YAML so the result is valid YAML rather than Go's %v map syntax.
			marshaled, mErr := marshalYAML(v)
			if nil != mErr {
				return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, mErr, "include: cannot marshal partial %q", name)
			}
			body = marshaled
		}
		ctx.tplDepth.Add(1)
		defer ctx.tplDepth.Add(-1)
		out, err := e.RenderFile("__include__:"+name, body, partials, ctx)
		if nil != err {
			return nil, err
		}
		return strings.Join(out, "\n"), nil
	}
}

func makeLookupFunc(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		if nil == ctx.Lookup {
			// At template-only time (no cluster connection) `lookup` returns
			// nil so packages can guard with `if (lookup ...)` without
			// erroring during dry-run/render.
			return nil, nil
		}
		apiVersion := fmt.Sprintf("%v", value)
		kind, ns, name := "", "", ""
		if 0 < len(args) {
			kind = coerceString(args[0])
		}
		if 1 < len(args) {
			ns = coerceString(args[1])
		}
		if 2 < len(args) {
			name = coerceString(args[2])
		}
		// Per-render cache: identical lookup() calls hit the API once.
		// Cache access is mutex-guarded because parallel renders can share
		// a context (the per-render registry isolates fns, not data).
		cacheKey := apiVersion + "|" + kind + "|" + ns + "|" + name
		ctx.lookupMu.Lock()
		if nil == ctx.lookupCache {
			ctx.lookupCache = make(map[string]any, 4)
		}
		if cached, hit := ctx.lookupCache[cacheKey]; hit {
			ctx.lookupMu.Unlock()
			return cached, nil
		}
		ctx.lookupMu.Unlock()

		obj, err := ctx.Lookup(apiVersion, kind, ns, name)
		if nil != err {
			return nil, err
		}
		var result any = obj
		if nil == obj {
			result = map[string]any{}
		}
		ctx.lookupMu.Lock()
		ctx.lookupCache[cacheKey] = result
		ctx.lookupMu.Unlock()
		return result, nil
	}
}

func makeFilesGet(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		name := fmt.Sprintf("%v", value)
		if data, ok := ctx.Files[name]; ok {
			return string(data), nil
		}
		return "", nil
	}
}

func makeFilesGlob(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		pattern := fmt.Sprintf("%v", value)
		out := make(map[string]any)
		for name, data := range ctx.Files {
			matched, err := matchGlob(pattern, name)
			if nil != err {
				return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, err, "Files.Glob: bad pattern %q", pattern)
			}
			if matched {
				out[name] = string(data)
			}
		}
		return out, nil
	}
}

func makeFilesAsConfig(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		pattern := fmt.Sprintf("%v", value)
		out := make(map[string]any)
		for name, data := range ctx.Files {
			matched, _ := matchGlob(pattern, name)
			if matched {
				out[name] = string(data)
			}
		}
		return out, nil
	}
}

func makeFilesAsSecrets(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		pattern := fmt.Sprintf("%v", value)
		out := make(map[string]any)
		for name, data := range ctx.Files {
			matched, _ := matchGlob(pattern, name)
			if matched {
				out[name] = base64Encode(data)
			}
		}
		return out, nil
	}
}

func makeFilesLines(ctx *RenderContext) Func {
	return func(value any, args ...any) (any, error) {
		name := fmt.Sprintf("%v", value)
		data, ok := ctx.Files[name]
		if !ok {
			return []any{}, nil
		}
		lines := strings.Split(string(data), "\n")
		out := make([]any, len(lines))
		for i, l := range lines {
			out[i] = l
		}
		return out, nil
	}
}
