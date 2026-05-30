package engine

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
	"gopkg.in/yaml.v3"
)

// Engine renders keramos templates into Kubernetes manifests.
type Engine struct {
	funcRegistry *FuncRegistry
}

// New creates a new Engine.
func New() *Engine {
	return &Engine{
		funcRegistry: NewFuncRegistry(),
	}
}

// LookupFn performs a live cluster read for `lookup`. apiVersion, kind,
// namespace, and name identify the target object; an empty name returns a
// list (the items live under the "items" key). Implementations should
// return nil, nil when the resource is missing rather than an error, so
// templates can guard with conditionals.
type LookupFn func(apiVersion, kind, namespace, name string) (map[string]any, error)

// RenderContext holds all data available to templates during rendering.
type RenderContext struct {
	Values       map[string]any    // merged values
	Package      map[string]any    // package metadata (name, version, appVersion)
	Release      map[string]any    // release info (name, namespace, revision, isUpgrade, isInstall)
	Capabilities map[string]any    // cluster info
	Files        map[string][]byte // non-template files
	Lookup       LookupFn          // optional cluster lookup for the `lookup` function
	tplDepth     atomic.Int32      // recursion guard for `tpl` (atomic for parallel renders)
	lookupCache  map[string]any    // per-render cache for the `lookup` function
	lookupMu     sync.Mutex        // guards lookupCache under parallel renders
}

// Render takes template file contents (map of filename->content), a partials map (from _helpers.yaml),
// and a RenderContext, and returns rendered YAML documents as a single multi-document string.
func (e *Engine) Render(templates map[string]string, partials map[string]any, ctx *RenderContext) (string, error) {
	var allDocs []string

	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		content := templates[name]
		// Skip partials (files starting with _)
		baseName := name
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			baseName = name[idx+1:]
		}
		if strings.HasPrefix(baseName, "_") {
			logger.Debug("skipping partial file: %s", name)
			continue
		}

		docs, err := e.RenderFile(name, content, partials, ctx)
		if nil != err {
			return "", err
		}
		allDocs = append(allDocs, docs...)
	}

	return strings.Join(allDocs, "---\n"), nil
}

// RenderFile renders a single template file and returns the rendered YAML documents.
func (e *Engine) RenderFile(name, content string, partials map[string]any, ctx *RenderContext) ([]string, error) {
	logger.Debug("rendering file: %s", name)

	// Build a per-render registry that layers context-bound functions
	// (tpl, lookup, include, Files.*) on top of the engine-wide built-ins.
	// This avoids races on the shared registry under concurrent renders.
	reg := buildRenderRegistry(e, ctx, partials)

	docs, err := splitYAMLDocuments(content)
	if nil != err {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrParse, err, "failed to parse YAML in %s", name)
	}

	result := make([]string, 0, len(docs))

	for _, doc := range docs {
		rendered, err := e.renderDocumentWithRegistry(doc, partials, ctx, reg)
		if nil != err {
			return nil, err
		}
		if nil == rendered {
			continue // document was conditionally removed
		}
		out, err := marshalYAML(rendered)
		if nil != err {
			return nil, keramoserrors.WrapErrorf(keramoserrors.ErrParse, err, "failed to marshal rendered YAML in %s", name)
		}
		if 0 == len(strings.TrimSpace(out)) {
			continue
		}
		result = append(result, out)
	}

	return result, nil
}

// buildRenderRegistry produces a per-render layered registry. It is safe to
// call concurrently because each call builds an isolated override map.
func buildRenderRegistry(e *Engine, ctx *RenderContext, partials map[string]any) *FuncRegistry {
	override := make(map[string]Func, 12)
	override["tpl"] = makeTplFunc(e, ctx, partials)
	override["include"] = makeIncludeFunc(e, ctx, partials)
	override["lookup"] = makeLookupFunc(ctx)
	override["Files.Get"] = makeFilesGet(ctx)
	override["Files.Glob"] = makeFilesGlob(ctx)
	override["Files.AsConfig"] = makeFilesAsConfig(ctx)
	override["Files.AsSecrets"] = makeFilesAsSecrets(ctx)
	override["Files.Lines"] = makeFilesLines(ctx)
	return NewLayeredRegistry(e.funcRegistry, override)
}

// renderDocument is the convenience entry point used in tests; production
// code paths build the per-call registry once and use
// renderDocumentWithRegistry directly.
func (e *Engine) renderDocument(doc any, partials map[string]any, ctx *RenderContext) (any, error) {
	reg := buildRenderRegistry(e, ctx, partials)
	return e.renderDocumentWithRegistry(doc, partials, ctx, reg)
}

func (e *Engine) renderDocumentWithRegistry(doc any, partials map[string]any, ctx *RenderContext, reg *FuncRegistry) (any, error) {
	if nil == doc {
		return nil, nil
	}

	// Phase 1: Resolve includes
	resolved, err := ResolveIncludes(doc, partials, make(map[string]bool))
	if nil != err {
		return nil, err
	}

	// Phase 2: Process control flow
	processed, err := ProcessControlFlow(resolved, ctx, reg)
	if nil != err {
		return nil, err
	}
	if nil == processed || isOmit(processed) {
		return nil, nil // whole document omitted (e.g. a root-level $if was false)
	}

	// Phase 3: Substitute variables
	substituted, err := SubstituteAll(processed, ctx, reg)
	if nil != err {
		return nil, err
	}
	if isOmit(substituted) {
		return nil, nil
	}

	// Phase 4: Clean $-prefixed keys
	cleaned := cleanDollarKeys(substituted)

	return cleaned, nil
}

// splitYAMLDocuments splits a multi-document YAML string into parsed documents.
//
// Keramos's `${...}` expressions can contain characters that confuse the YAML
// parser when they appear inside flow-style maps/lists, e.g.
//   selector: {app: ${release.name}}
// The trailing `}` of the expression is read as the end of the flow map.
// We sidestep this by scanning the raw content for `${...}` occurrences
// and replacing each with a placeholder token that is YAML-safe, parsing
// the document, then walking the parsed tree and restoring the original
// expression strings. Substitution proceeds normally afterwards.
func splitYAMLDocuments(content string) ([]any, error) {
	stashed, exprs := stashExpressions(content)
	decoder := yaml.NewDecoder(strings.NewReader(stashed))
	var docs []any

	for {
		var doc any
		err := decoder.Decode(&doc)
		if nil != err {
			if "EOF" == err.Error() {
				break
			}
			return nil, err
		}
		docs = append(docs, restoreExpressions(doc, exprs))
	}

	return docs, nil
}

// keramosExprToken format: __KERAMOS_EXPR_<index>__. The token is a valid YAML
// scalar in every context (block, flow, mapping key, sequence value).
const keramosExprPrefix = "__KERAMOS_EXPR_"
const keramosExprSuffix = "__"

// stashExpressions replaces every top-level `${...}` occurrence in `content`
// with a placeholder, returning the rewritten content and the original list
// of expressions in order. Nesting inside double-quoted strings is preserved
// because the YAML parser will hand us those strings verbatim — but they
// still get a placeholder so substitution finds them.
func stashExpressions(content string) (string, []string) {
	exprs := make([]string, 0, 8)
	var b strings.Builder
	b.Grow(len(content))
	runes := []rune(content)
	i := 0
	for i < len(runes) {
		if '$' == runes[i] && i+1 < len(runes) && '{' == runes[i+1] {
			// scan to matching `}`, respecting quotes inside the expression
			depth := 1
			j := i + 2
			inSingle, inDouble := false, false
			for j < len(runes) && 0 < depth {
				ch := runes[j]
				switch {
				case '\\' == ch && j+1 < len(runes):
					j++
				case '\'' == ch && !inDouble:
					inSingle = !inSingle
				case '"' == ch && !inSingle:
					inDouble = !inDouble
				case '{' == ch && !inSingle && !inDouble:
					depth++
				case '}' == ch && !inSingle && !inDouble:
					depth--
				}
				j++
			}
			if 0 == depth {
				expr := string(runes[i:j])
				token := fmt.Sprintf("%s%d%s", keramosExprPrefix, len(exprs), keramosExprSuffix)
				exprs = append(exprs, expr)
				b.WriteString(token)
				i = j
				continue
			}
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String(), exprs
}

// restoreExpressions walks the parsed YAML tree replacing every placeholder
// occurrence with the original `${...}` expression so the substitute pass
// can evaluate it.
func restoreExpressions(node any, exprs []string) any {
	switch v := node.(type) {
	case string:
		return restoreInString(v, exprs)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			restoredKey := restoreInString(k, exprs)
			out[restoredKey] = restoreExpressions(val, exprs)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = restoreExpressions(val, exprs)
		}
		return out
	default:
		return node
	}
}

func restoreInString(s string, exprs []string) string {
	if !strings.Contains(s, keramosExprPrefix) {
		return s
	}
	for idx, expr := range exprs {
		token := fmt.Sprintf("%s%d%s", keramosExprPrefix, idx, keramosExprSuffix)
		s = strings.ReplaceAll(s, token, expr)
	}
	return s
}

// marshalYAML serializes a value to a YAML string.
func marshalYAML(v any) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	err := encoder.Encode(v)
	if nil != err {
		return "", err
	}
	err = encoder.Close()
	if nil != err {
		return "", err
	}
	return buf.String(), nil
}

// keramosDirectiveKeys lists the $-prefixed keys keramos's control-flow phase
// owns. Any key in this set is keramos-internal scaffolding that must be
// stripped from the rendered manifest. Other $-prefixed keys (e.g.
// JSON Schema's `$ref`/`$schema`/`$defs`, OpenAPI extensions like
// `$id`, CRDs that legitimately use `$`-keys) are preserved verbatim.
var keramosDirectiveKeys = map[string]struct{}{
	"$if":      {},
	"$then":    {},
	"$else":    {},
	"$each":    {},
	"$as":      {},
	"$yield":   {},
	"$switch":  {},
	"$cases":   {},
	"$default": {},
	"$include": {},
	"$item":    {},
	"$key":     {},
	"$value":   {},
	"$hook":    {},
}

// cleanDollarKeys removes keramos's own control-flow keys from the rendered
// tree. Other $-prefixed keys (JSON Schema, OpenAPI, CRD extensions) are
// preserved — stripping by prefix corrupted any document that legitimately
// contained `$ref`, `$schema`, etc.
func cleanDollarKeys(node any) any {
	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			if _, isDirective := keramosDirectiveKeys[k]; isDirective {
				continue
			}
			// Defence in depth: an omit sentinel embedded in a value built by
			// a function (e.g. `dict "k" (... | omitempty)`) is dropped here so
			// it can never reach the YAML marshaller and serialise as `{}`.
			if isOmit(val) {
				continue
			}
			result[k] = cleanDollarKeys(val)
		}
		return result
	case []any:
		result := make([]any, 0, len(v))
		for _, val := range v {
			if isOmit(val) {
				continue
			}
			result = append(result, cleanDollarKeys(val))
		}
		return result
	default:
		return v
	}
}
