package engine

import (
	"strings"

	keramoserrors "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
)

// maxIncludeDepth caps the chain length of nested $include calls. Cycle
// detection rejects same-name re-entry, but a long chain of distinct
// partials A→B→C→…→Z would recurse N frames without bound, growing the
// goroutine stack to Go's max (~1 GB) before crashing without a useful
// message. 64 is far past anything legitimate templates need.
const maxIncludeDepth = 64

// ResolveIncludes walks the tree and replaces $include directives with partial content.
// Partials come from _helpers.yaml files parsed as YAML maps of named blocks.
func ResolveIncludes(node any, partials map[string]any, visited map[string]bool) (any, error) {
	return resolveIncludesAtDepth(node, partials, visited, 0)
}

func resolveIncludesAtDepth(node any, partials map[string]any, visited map[string]bool, depth int) (any, error) {
	if depth > maxIncludeDepth {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrIncludeCycle,
			"$include chain exceeds depth limit (%d)", maxIncludeDepth)
	}
	switch v := node.(type) {
	case map[string]any:
		return resolveIncludeMapAtDepth(v, partials, visited, depth)
	case []any:
		return resolveIncludeSliceAtDepth(v, partials, visited, depth)
	default:
		return node, nil
	}
}

func resolveIncludeMapAtDepth(m map[string]any, partials map[string]any, visited map[string]bool, depth int) (any, error) {
	includeName, hasInclude := m["$include"]
	if !hasInclude {
		result := make(map[string]any, len(m))
		for k, v := range m {
			resolved, err := resolveIncludesAtDepth(v, partials, visited, depth)
			if nil != err {
				return nil, err
			}
			result[k] = resolved
		}
		return result, nil
	}

	name, ok := includeName.(string)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrExpression, "$include value must be a string, got %T", includeName)
	}
	name = strings.TrimSpace(name)

	logger.Debug("resolving include: %s", name)

	if visited[name] {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrIncludeCycle, "include cycle detected: %s", name)
	}

	partial, exists := partials[name]
	if !exists {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrIncludeNotFound, "partial %q not found", name)
	}

	// Track visited for cycle detection
	visited[name] = true
	defer func() { visited[name] = false }()

	// Recursively resolve includes in the partial — depth+1 enforces
	// the chain-depth bound even when each link is a distinct name
	// (cycle detection only rejects same-name re-entry).
	resolved, err := resolveIncludesAtDepth(partial, partials, visited, depth+1)
	if nil != err {
		return nil, err
	}

	// If the partial is a map, merge overrides from the current map
	resolvedMap, isMap := resolved.(map[string]any)
	if !isMap {
		return resolved, nil
	}

	// Collect overrides (non-$include keys)
	result := make(map[string]any, len(resolvedMap)+len(m))
	for k, v := range resolvedMap {
		result[k] = v
	}
	for k, v := range m {
		if "$include" == k {
			continue
		}
		result[k] = v
	}
	return result, nil
}

func resolveIncludeSliceAtDepth(s []any, partials map[string]any, visited map[string]bool, depth int) (any, error) {
	result := make([]any, len(s))
	for i, item := range s {
		resolved, err := resolveIncludesAtDepth(item, partials, visited, depth)
		if nil != err {
			return nil, err
		}
		result[i] = resolved
	}
	return result, nil
}
