package helmcompat

import (
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
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
		subVals := deepCopy(sub.values)
		if override, ok := vals[sub.name].(map[string]any); ok {
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
