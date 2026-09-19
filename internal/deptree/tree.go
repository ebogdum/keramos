package deptree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/pkg"
	"github.com/ebogdum/keramos/v2/internal/repo"
	"gopkg.in/yaml.v3"
)

// NodeKind distinguishes between different types of nodes in the dependency tree.
type NodeKind int

const (
	// KindRoot is the top-level package node.
	KindRoot NodeKind = iota
	// KindLayer is a composition layer (merged into parent).
	KindLayer
	// KindRequire is a co-deployed package (installed separately).
	KindRequire
)

// Node represents a package in the dependency tree.
type Node struct {
	Name      string
	Source    string // resolved local path
	Kind      NodeKind
	Children  []*Node // ordered children (layers first, then requires)
	Parent    *Node
	Depth     int
	Metadata  *pkg.PackageMetadata
	Values    map[string]any
	Templates map[string]string
	Partials  map[string]any
	Hooks     map[string]string
	Tests     map[string]string
}

// Build constructs the full dependency tree from a root package path.
// Phase 1: reads keramos.yaml at each level, resolves sources, builds tree structure.
// Detects cycles by tracking visited paths.
func Build(packagePath string) (*Node, error) {
	return BuildWithOverrides(packagePath, nil)
}

// BuildWithOverrides is like Build but also threads user-supplied value
// overrides into condition/tags evaluation when filtering enabled layers,
// so `--set widgets.enabled=true` activates a layer whose condition is
// `widgets.enabled`.
func BuildWithOverrides(packagePath string, overrides map[string]any) (*Node, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve package path", err)
	}

	visited := make(map[string]bool)
	count := 0
	return buildNode(absPath, KindRoot, "", nil, 0, visited, &count, overrides)
}

// maxTreeNodes bounds the total nodes built for one dependency tree. Cycles are
// caught separately; this bounds a DAG whose packages are reachable through
// many aliased paths, which would otherwise rebuild subtrees exponentially
// (a DoS from a small malicious package set).
const maxTreeNodes = 10000

func buildNode(absPath string, kind NodeKind, name string, parent *Node, depth int, visited map[string]bool, count *int, rootOverrides map[string]any) (*Node, error) {
	if visited[absPath] {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCycle, "cycle detected in dependency tree: %s", absPath)
	}
	*count++
	if *count > maxTreeNodes {
		return nil, keramoserr.NewErrorf(keramoserr.ErrPackageInvalid,
			"dependency tree exceeds the %d-node limit (aliased/fan-out graph too large)", maxTreeNodes)
	}
	visited[absPath] = true

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return nil, err
	}

	nodeName := name
	if "" == nodeName {
		nodeName = meta.Name
	}

	node := &Node{
		Name:     nodeName,
		Source:   absPath,
		Kind:     kind,
		Parent:   parent,
		Depth:    depth,
		Metadata: &meta,
	}

	cacheDir := defaultCacheDir()
	layers := meta.EffectiveLayers()

	// Pre-load this package's own values, then merge user overrides on top
	// so we can evaluate condition/tags on child layers and requires:
	// `--set widgets.enabled=true` must activate a conditioned layer.
	// Only the root node receives user overrides; deeper layers see their
	// own values plus the propagated `tags.*` slice from the root.
	filterValues := loadEffectiveValuesForFilter(absPath, rootOverrides)
	if 0 < depth {
		filterValues = propagateTags(filterValues, rootOverrides)
	}

	for _, ls := range layers {
		if !ls.IsEnabled(filterValues) {
			continue
		}
		childPath, fetchErr := resolveSource(ls, cacheDir, absPath)
		if nil != fetchErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, fetchErr,
				"failed to resolve layer %s from source %s", ls.Name, ls.Source)
		}

		childName := ls.Name
		if "" != ls.Alias {
			childName = ls.Alias
		}
		child, childErr := buildNode(childPath, KindLayer, childName, node, depth+1, visited, count, rootOverrides)
		if nil != childErr {
			return nil, childErr
		}
		node.Children = append(node.Children, child)
	}

	for _, req := range meta.Requires {
		if !req.IsEnabled(filterValues) {
			continue
		}
		childPath, fetchErr := resolveSource(req, cacheDir, absPath)
		if nil != fetchErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, fetchErr,
				"failed to resolve require %s from source %s", req.Name, req.Source)
		}

		childName := req.Name
		if "" != req.Alias {
			childName = req.Alias
		}
		child, childErr := buildNode(childPath, KindRequire, childName, node, depth+1, visited, count, rootOverrides)
		if nil != childErr {
			return nil, childErr
		}
		node.Children = append(node.Children, child)
	}

	// Unmark so the same package can appear in sibling chains.
	visited[absPath] = false

	return node, nil
}

func resolveSource(ls pkg.LayerSource, cacheDir, parentPath string) (string, error) {
	layerPath, err := repo.FetchSource(ls.Source, ls.Ref, ls.Version, ls.Name, cacheDir, parentPath)
	if nil != err {
		return "", err
	}

	absPath, absErr := filepath.Abs(layerPath)
	if nil != absErr {
		return "", keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve path", absErr)
	}

	logger.Debug("resolved source %s -> %s", ls.Name, absPath)
	return absPath, nil
}

// Populate loads values, templates, hooks, and tests for every node in the tree.
// Phase 2: walks breadth-first and loads content.
func Populate(root *Node) error {
	queue := []*Node{root}

	for 0 < len(queue) {
		node := queue[0]
		queue = queue[1:]

		if err := loadNodeContent(node); nil != err {
			return err
		}

		queue = append(queue, node.Children...)
	}

	return nil
}

func loadNodeContent(node *Node) error {
	vals, err := loadValuesOptional(node.Source)
	if nil != err {
		return err
	}
	node.Values = vals

	templates, partials, err := loadTemplates(node.Source)
	if nil != err {
		return err
	}
	node.Templates = templates
	node.Partials = partials

	hooksMap, err := loadDir(filepath.Join(node.Source, "hooks"))
	if nil != err {
		return err
	}
	node.Hooks = hooksMap

	testsMap, err := loadDir(filepath.Join(node.Source, "tests"))
	if nil != err {
		return err
	}
	node.Tests = testsMap

	return nil
}

// WalkLayers walks the tree depth-first and returns layer nodes in merge order.
// Deeper layers come first (lowest precedence), root package last (highest).
func WalkLayers(root *Node) []*Node {
	var result []*Node
	walkLayersRecursive(root, &result)
	return result
}

func walkLayersRecursive(node *Node, result *[]*Node) {
	// Process layer children first (depth-first: deepest layers have lowest precedence)
	for _, child := range node.Children {
		if KindLayer == child.Kind {
			walkLayersRecursive(child, result)
		}
	}
	// Then add the current node
	*result = append(*result, node)
}

// WalkRequires returns all Require nodes for separate deployment.
func WalkRequires(root *Node) []*Node {
	var result []*Node
	walkRequiresRecursive(root, &result)
	return result
}

func walkRequiresRecursive(node *Node, result *[]*Node) {
	for _, child := range node.Children {
		if KindRequire == child.Kind {
			*result = append(*result, child)
			// Also collect nested requires from the require node
			walkRequiresRecursive(child, result)
		}
	}
}

// PrintTree formats the tree for display.
func PrintTree(root *Node) string {
	var b strings.Builder
	version := ""
	if nil != root.Metadata {
		version = root.Metadata.Version
	}
	fmt.Fprintf(&b, "%s@%s\n", root.Name, version)
	printChildren(&b, root, "")
	return b.String()
}

func printChildren(b *strings.Builder, node *Node, prefix string) {
	childCount := len(node.Children)
	for i, child := range node.Children {
		isLast := (i == childCount-1)
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		kindLabel := "layer"
		if KindRequire == child.Kind {
			kindLabel = "requires"
		}

		version := ""
		if nil != child.Metadata {
			version = child.Metadata.Version
		}

		fmt.Fprintf(b, "%s%s[%s] %s@%s (%s)\n", prefix, connector, kindLabel, child.Name, version, child.Source)

		childPrefix := prefix + "│   "
		if isLast {
			childPrefix = prefix + "    "
		}
		printChildren(b, child, childPrefix)
	}
}

func defaultCacheDir() string {
	home, err := os.UserCacheDir()
	if nil != err {
		return filepath.Join(os.TempDir(), "keramos-cache")
	}
	return filepath.Join(home, "keramos")
}

// --- file loading helpers (reuse layer package helpers via delegation) ---

// loadEffectiveValuesForFilter combines this package's own values.yaml with
// user-supplied overrides (--set / -f). The result is used when evaluating
// LayerSource.IsEnabled so that user-supplied flags can enable or disable
// dependent layers at runtime.
func loadEffectiveValuesForFilter(dirPath string, overrides map[string]any) map[string]any {
	own, _ := loadValuesOptional(dirPath)
	if 0 == len(overrides) {
		return own
	}
	if nil == own {
		own = make(map[string]any)
	}
	return mergeForFilter(own, overrides)
}

// propagateTags carries the user's `tags.*` overrides into a child's
// filter-values map so a deep descendant can be enabled or disabled via the
// root's --set tags.foo=true/false (global tag selection).
func propagateTags(childFilter, rootOverrides map[string]any) map[string]any {
	tags, _ := rootOverrides["tags"].(map[string]any)
	if 0 == len(tags) {
		return childFilter
	}
	if nil == childFilter {
		childFilter = make(map[string]any)
	}
	existingTags, _ := childFilter["tags"].(map[string]any)
	merged := make(map[string]any, len(existingTags)+len(tags))
	for k, v := range existingTags {
		merged[k] = v
	}
	for k, v := range tags {
		merged[k] = v
	}
	childFilter["tags"] = merged
	return childFilter
}

func mergeForFilter(dst, src map[string]any) map[string]any {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		dm, dok := dv.(map[string]any)
		sm, sok := sv.(map[string]any)
		if dok && sok {
			dst[k] = mergeForFilter(dm, sm)
			continue
		}
		dst[k] = sv
	}
	return dst
}

func loadValuesOptional(dirPath string) (map[string]any, error) {
	vals, err := pkg.LoadValues(dirPath)
	if nil != err {
		if os.IsNotExist(extractCause(err)) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	if nil == vals {
		return make(map[string]any), nil
	}
	return normalizeMap(map[string]any(vals)), nil
}

func loadTemplates(dirPath string) (map[string]string, map[string]any, error) {
	templatesDir := filepath.Join(dirPath, "templates")
	templates := make(map[string]string)
	partials := make(map[string]any)

	info, err := os.Stat(templatesDir)
	if nil != err {
		if os.IsNotExist(err) {
			return templates, partials, nil
		}
		return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to stat templates directory", err)
	}
	if !info.IsDir() {
		return templates, partials, nil
	}

	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read templates directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isYAMLFile(name) {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(templatesDir, name))
		if nil != readErr {
			return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read template file", readErr)
		}

		if strings.HasPrefix(name, "_") {
			// Partial files are YAML maps whose top-level keys become
			// individually-addressable partials. Both the filename-keyed
			// raw string AND each parsed top-level key are stored so
			// callers can address either form.
			var parsed map[string]any
			if pErr := yaml.Unmarshal(data, &parsed); nil != pErr {
				return nil, nil, keramoserr.WrapErrorf(keramoserr.ErrPackageInvalid, pErr,
					"failed to parse partials file %s", name)
			}
			partials[name] = string(data)
			for k, v := range parsed {
				partials[k] = v
			}
		} else {
			templates[name] = string(data)
		}
	}

	return templates, partials, nil
}

func loadDir(dirPath string) (map[string]string, error) {
	result := make(map[string]string)

	info, err := os.Stat(dirPath)
	if nil != err {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to stat directory", err)
	}
	if !info.IsDir() {
		return result, nil
	}

	entries, err := os.ReadDir(dirPath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ".yaml" != ext && ".yml" != ext {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dirPath, name))
		if nil != readErr {
			return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read file", readErr)
		}
		result[name] = string(data)
	}

	return result, nil
}

func isYAMLFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

func normalizeMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = normalizeValue(v)
	}
	return result
}

func normalizeValue(v any) any {
	if nil == v {
		return nil
	}
	if m, ok := v.(pkg.Values); ok {
		return normalizeMap(map[string]any(m))
	}
	if m, ok := v.(map[string]any); ok {
		return normalizeMap(m)
	}
	if s, ok := v.([]any); ok {
		result := make([]any, len(s))
		for i, item := range s {
			result[i] = normalizeValue(item)
		}
		return result
	}
	return v
}

func extractCause(err error) error {
	he, ok := err.(*keramoserr.KeramosError)
	if ok && nil != he.Cause {
		return he.Cause
	}
	return err
}
