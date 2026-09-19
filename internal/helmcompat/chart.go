package helmcompat

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"gopkg.in/yaml.v3"
)

// chart is a loaded Helm chart node (root or subchart).
type chart struct {
	dir       string
	name      string         // Chart.yaml name
	fqName    string         // fully-qualified name path, e.g. "parent/charts/child" basename chain
	metadata  map[string]any // Chart.yaml
	values    map[string]any // values.yaml defaults
	templates map[string]string
	files     map[string][]byte
	subcharts []*chart
	alias     string
	condition string
	tags      []string
	enabled   bool

	scoped map[string]any // values after scoping/coalescing; set by assignValues
}

// loadChart reads a chart directory tree (Chart.yaml, values.yaml, templates/,
// charts/<sub>, and any other files for the .Files API).
func loadChart(dir string) (*chart, error) {
	return loadChartNamed(dir, "")
}

func loadChartNamed(dir, parentFQ string) (*chart, error) {
	st, err := os.Stat(dir)
	if nil != err || !st.IsDir() {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "not a chart directory: %s", dir)
	}
	c := &chart{
		dir:       dir,
		metadata:  map[string]any{},
		values:    map[string]any{},
		templates: map[string]string{},
		files:     map[string][]byte{},
	}

	// Chart.yaml (required).
	chartYAML := filepath.Join(dir, "Chart.yaml")
	data, err := readFile(chartYAML)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "read Chart.yaml in %s", dir)
	}
	if uErr := yaml.Unmarshal(data, &c.metadata); nil != uErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrParse, uErr, "parse Chart.yaml in %s", dir)
	}
	c.name, _ = c.metadata["name"].(string)
	if "" == c.name {
		c.name = filepath.Base(dir)
	}
	if "" == parentFQ {
		c.fqName = c.name
	} else {
		c.fqName = parentFQ + "/charts/" + c.name
	}

	// values.yaml (optional).
	if vData, vErr := readFile(filepath.Join(dir, "values.yaml")); nil == vErr {
		if uErr := yaml.Unmarshal(vData, &c.values); nil != uErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrParse, uErr, "parse values.yaml in %s", dir)
		}
		if nil == c.values {
			c.values = map[string]any{}
		}
	}

	// templates/ tree.
	tmplDir := filepath.Join(dir, "templates")
	if _, sErr := os.Stat(tmplDir); nil == sErr {
		if wErr := loadTemplates(tmplDir, c); nil != wErr {
			return nil, wErr
		}
	}

	// Non-template files for the .Files API (everything except Chart.yaml,
	// values.yaml, the templates/ and charts/ trees).
	if fErr := loadFiles(dir, c); nil != fErr {
		return nil, fErr
	}

	// Subcharts under charts/.
	chartsDir := filepath.Join(dir, "charts")
	if entries, rErr := os.ReadDir(chartsDir); nil == rErr {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sub, sErr := loadChartNamed(filepath.Join(chartsDir, e.Name()), c.fqName)
			if nil != sErr {
				return nil, sErr
			}
			sub.enabled = true
			applyDependencyMeta(c, sub)
			c.subcharts = append(c.subcharts, sub)
		}
	}

	return c, nil
}

func loadTemplates(tmplDir string, c *chart) error {
	return filepath.Walk(tmplDir, func(path string, info os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		if info.IsDir() {
			return nil
		}
		// Refuse symlinks (a symlink could read host files into the render).
		if 0 != info.Mode()&os.ModeSymlink {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "refusing symlink template %s", path)
		}
		rel, rErr := filepath.Rel(tmplDir, path)
		if nil != rErr {
			return rErr
		}
		ext := strings.ToLower(filepath.Ext(path))
		base := filepath.Base(path)
		if ".yaml" != ext && ".yml" != ext && ".tpl" != ext && ".txt" != ext && "NOTES.txt" != base {
			return nil
		}
		body, readErr := readFile(path)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, readErr, "read template %s", path)
		}
		c.templates[filepath.ToSlash(rel)] = string(body)
		return nil
	})
}

func loadFiles(dir string, c *chart) error {
	ignored := loadHelmIgnore(dir)

	return filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		if info.IsDir() {
			// Skip the templates/ and charts/ subtrees entirely.
			base := filepath.Base(path)
			if path != dir && ("templates" == base || "charts" == base) {
				return filepath.SkipDir
			}
			if path != dir {
				rel, relErr := filepath.Rel(dir, path)
				if nil == relErr && ignored.matches(filepath.ToSlash(rel)+"/") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if 0 != info.Mode()&os.ModeSymlink {
			return nil // ignore symlinked files
		}
		rel, rErr := filepath.Rel(dir, path)
		if nil != rErr {
			return rErr
		}
		switch rel {
		case "Chart.yaml", "values.yaml", "Chart.lock":
			return nil
		}
		if ignored.matches(filepath.ToSlash(rel)) {
			return nil
		}
		// Bound file size so a hostile chart can't exhaust memory via .Files.
		if info.Size() > 10*1024*1024 {
			return nil
		}
		body, readErr := readFile(path)
		if nil != readErr {
			return nil // best-effort: a non-readable aux file is not fatal
		}
		c.files[filepath.ToSlash(rel)] = body
		return nil
	})
}

// assignValues sets c.scoped to vals and recurses into subcharts applying
// Helm's value scoping: a subchart receives its own defaults coalesced under
// the parent's values keyed by the subchart name, with the parent's `global`
// block propagated down.
func assignValues(c *chart, vals map[string]any) {
	c.scoped = vals
	global, _ := vals["global"].(map[string]any)
	for _, sub := range c.subcharts {
		sub.enabled = subchartEnabled(sub, vals)
		subVals := deepCopy(sub.values)
		if override, ok := vals[subchartKey(sub)].(map[string]any); ok {
			subVals = coalesce(subVals, override)
		}
		if nil != global {
			existing, _ := subVals["global"].(map[string]any)
			subVals["global"] = coalesce(deepCopy(global), existing)
		}
		assignValues(sub, subVals)
	}
}

// coalesce deep-merges override onto base (override wins) and returns base.
func coalesce(base, override map[string]any) map[string]any {
	if nil == base {
		base = map[string]any{}
	}
	for k, ov := range override {
		if bv, ok := base[k]; ok {
			if bm, ok1 := bv.(map[string]any); ok1 {
				if om, ok2 := ov.(map[string]any); ok2 {
					base[k] = coalesce(bm, om)
					continue
				}
			}
		}
		base[k] = ov
	}
	return base
}

func deepCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]any); ok {
			out[k] = deepCopy(nested)
			continue
		}
		out[k] = v
	}
	return out
}

type helmIgnore struct {
	patterns []string
}

func loadHelmIgnore(dir string) helmIgnore {
	body, err := readFile(filepath.Join(dir, ".helmignore"))
	if nil != err {
		return helmIgnore{}
	}

	out := helmIgnore{}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if "" == trimmed || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out.patterns = append(out.patterns, trimmed)
	}
	return out
}

func (h helmIgnore) matches(rel string) bool {
	if 0 == len(h.patterns) {
		return false
	}

	clean := strings.TrimSuffix(rel, "/")
	base := path.Base(clean)

	for _, pattern := range h.patterns {
		p := strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
		if "" == p {
			continue
		}
		if p == clean || p == base {
			return true
		}
		if strings.HasPrefix(clean, p+"/") {
			return true
		}
		if ok, mErr := path.Match(p, base); nil == mErr && ok {
			return true
		}
		if ok, mErr := path.Match(p, clean); nil == mErr && ok {
			return true
		}
	}
	return false
}

func EffectiveValues(chartPath string, userValues map[string]any) (map[string]any, error) {
	root, err := loadChart(chartPath)
	if nil != err {
		return nil, err
	}
	if nil == userValues {
		userValues = map[string]any{}
	}
	return coalesce(deepCopy(root.values), userValues), nil
}

func applyDependencyMeta(parent, sub *chart) {
	deps, _ := parent.metadata["dependencies"].([]any)
	for _, raw := range deps {
		dep, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := dep["name"].(string)
		if name != sub.name {
			continue
		}
		if alias, ok := dep["alias"].(string); ok && "" != alias {
			sub.alias = alias
		}
		if cond, ok := dep["condition"].(string); ok {
			sub.condition = cond
		}
		for _, tag := range toStringSlice(dep["tags"]) {
			sub.tags = append(sub.tags, tag)
		}
		return
	}
}

func toStringSlice(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func subchartKey(sub *chart) string {
	if "" != sub.alias {
		return sub.alias
	}
	return sub.name
}

func subchartEnabled(sub *chart, parentValues map[string]any) bool {
	for _, path := range strings.Split(sub.condition, ",") {
		path = strings.TrimSpace(path)
		if "" == path {
			continue
		}
		if val, found := lookupBoolPath(parentValues, path); found {
			return val
		}
	}

	tagsBlock, _ := parentValues["tags"].(map[string]any)
	decided := false
	enabled := false
	for _, tag := range sub.tags {
		if val, ok := tagsBlock[tag].(bool); ok {
			decided = true
			enabled = enabled || val
		}
	}
	if decided {
		return enabled
	}

	return true
}

func lookupBoolPath(values map[string]any, path string) (bool, bool) {
	parts := strings.Split(path, ".")
	var cursor any = values
	for _, part := range parts {
		m, ok := cursor.(map[string]any)
		if !ok {
			return false, false
		}
		cursor, ok = m[part]
		if !ok {
			return false, false
		}
	}
	b, ok := cursor.(bool)
	return b, ok
}
