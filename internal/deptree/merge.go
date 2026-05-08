package deptree

import (
	"sort"
	"strings"

	"github.com/ebogdum/keramos/internal/maputil"
)

// MergeValues walks the tree in merge order and produces the final merged values.
// Merge order: deepest layers first, then shallower layers, then root (highest precedence).
// Values with "!" prefix keys get forced precedence at their level.
func MergeValues(root *Node) (map[string]any, error) {
	nodes := WalkLayers(root)
	merged := make(map[string]any)

	// Group nodes by parent for "!" forced precedence handling.
	// We process in walk order (deepest first), collecting forced values per parent group.
	parentForced := make(map[*Node]map[string]any)

	for _, node := range nodes {
		// Get this node's own values (strip layers/requires sections)
		nodeValues := ownValues(node.Values)

		// Apply scoped overrides from parent
		if nil != node.Parent {
			scopedOverrides := scopedLayerValues(node.Parent.Values, node.Name)
			if 0 < len(scopedOverrides) {
				clean, forced := extractForced(scopedOverrides)
				nodeValues = maputil.DeepMerge(nodeValues, clean)

				// Collect forced values for later application
				if 0 < len(forced) {
					if nil == parentForced[node.Parent] {
						parentForced[node.Parent] = make(map[string]any)
					}
					parentForced[node.Parent] = maputil.DeepMerge(parentForced[node.Parent], forced)
				}
			}
		}

		// Extract forced values from the node's own values
		cleanNodeValues, forcedNodeValues := extractForced(nodeValues)

		merged = maputil.DeepMerge(merged, cleanNodeValues)

		// Apply forced values from this node (overrides siblings at same level)
		if 0 < len(forcedNodeValues) {
			merged = maputil.DeepMerge(merged, forcedNodeValues)
		}
	}

	// Apply any remaining forced values from parent scoped overrides in deterministic order
	parentNodes := make([]*Node, 0, len(parentForced))
	for node := range parentForced {
		parentNodes = append(parentNodes, node)
	}
	sort.Slice(parentNodes, func(i, j int) bool {
		return parentNodes[i].Name < parentNodes[j].Name
	})
	for _, node := range parentNodes {
		merged = maputil.DeepMerge(merged, parentForced[node])
	}

	return merged, nil
}

// ownValues returns values excluding the "layers" and "requires" sections.
func ownValues(vals map[string]any) map[string]any {
	if nil == vals {
		return make(map[string]any)
	}

	result := make(map[string]any, len(vals))
	for k, v := range vals {
		if "layers" == k || "requires" == k {
			continue
		}
		result[k] = v
	}
	return result
}

// scopedLayerValues extracts values from the "layers.<name>" section of a parent's values.
func scopedLayerValues(parentValues map[string]any, layerName string) map[string]any {
	if nil == parentValues {
		return nil
	}

	layersSection, ok := parentValues["layers"]
	if !ok {
		return nil
	}

	layersMap, ok := layersSection.(map[string]any)
	if !ok {
		return nil
	}

	scopedRaw, ok := layersMap[layerName]
	if !ok {
		return nil
	}

	scoped, ok := scopedRaw.(map[string]any)
	if !ok {
		return nil
	}

	return scoped
}

// extractForced separates "!" prefixed keys from regular keys.
// Returns (cleanValues, forcedValues).
func extractForced(vals map[string]any) (map[string]any, map[string]any) {
	if nil == vals {
		return make(map[string]any), nil
	}

	clean := make(map[string]any, len(vals))
	var forced map[string]any

	for k, v := range vals {
		if strings.HasPrefix(k, "!") {
			realKey := k[1:]
			if nil == forced {
				forced = make(map[string]any)
			}
			forced[realKey] = v
			continue
		}

		// Recurse into nested maps
		if m, ok := v.(map[string]any); ok {
			childClean, childForced := extractForced(m)
			clean[k] = childClean
			if 0 < len(childForced) {
				if nil == forced {
					forced = make(map[string]any)
				}
				forced[k] = childForced
			}
			continue
		}

		clean[k] = v
	}

	return clean, forced
}

// MergeTemplates produces the final template set following the same merge rules.
// Returns (templates, partials, error).
func MergeTemplates(root *Node) (map[string]string, map[string]any, error) {
	nodes := WalkLayers(root)

	mergedTemplates := make(map[string]string)
	mergedPartials := make(map[string]any)

	for _, node := range nodes {
		for name, content := range node.Templates {
			mergedTemplates[name] = content
		}
		for name, content := range node.Partials {
			mergedPartials[name] = content
		}
	}

	return mergedTemplates, mergedPartials, nil
}

// MergeHooks produces merged hooks (additive across all layers).
func MergeHooks(root *Node) map[string]string {
	nodes := WalkLayers(root)
	merged := make(map[string]string)

	for _, node := range nodes {
		for name, content := range node.Hooks {
			merged[name] = content
		}
	}

	return merged
}

// MergeTests produces merged tests (additive across all layers).
func MergeTests(root *Node) map[string]string {
	nodes := WalkLayers(root)
	merged := make(map[string]string)

	for _, node := range nodes {
		for name, content := range node.Tests {
			merged[name] = content
		}
	}

	return merged
}

// ScopedRequireValues extracts scoped values for a require from its parent.
func ScopedRequireValues(parentValues map[string]any, requireName string) map[string]any {
	if nil == parentValues {
		return nil
	}

	requiresSection, ok := parentValues["requires"]
	if !ok {
		return nil
	}

	requiresMap, ok := requiresSection.(map[string]any)
	if !ok {
		return nil
	}

	scopedRaw, ok := requiresMap[requireName]
	if !ok {
		return nil
	}

	scoped, ok := scopedRaw.(map[string]any)
	if !ok {
		return nil
	}

	return scoped
}
