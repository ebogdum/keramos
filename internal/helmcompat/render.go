// Package helmcompat renders unmodified upstream Helm v3 charts using a Go
// text/template engine with the Sprig function set and the Helm rendering
// context (.Values/.Chart/.Release/.Capabilities/.Files/.Template) plus the
// helm-specific helpers (include, tpl, required, toYaml/fromYaml, lookup).
// Rendered manifests are returned so keramos can apply and track them under its
// own release record — i.e. run Helm charts without converting them first.
//
// Supported: values.yaml + user overrides (deep-merged), named templates
// (_helpers.tpl / define), include, tpl, required, toYaml/fromYaml/toJson/
// fromJson, the .Files API, .Capabilities, and recursive subcharts with Helm
// value scoping and global propagation. lookup returns empty (render-time, no
// cluster), matching `helm template`.
package helmcompat

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
)

// ReleaseMeta supplies the .Release context.
type ReleaseMeta struct {
	Name      string
	Namespace string
	Revision  int
	IsInstall bool
	IsUpgrade bool
}

// CapabilitiesMeta supplies the .Capabilities context.
type CapabilitiesMeta struct {
	KubeVersion string   // e.g. "v1.29.0"
	APIVersions []string // e.g. ["apps/v1", "v1", ...]
}

// Options configures a render.
type Options struct {
	Release      ReleaseMeta
	Capabilities CapabilitiesMeta
	UserValues   map[string]any
}

// Render loads the chart at chartPath and returns rendered manifests keyed by
// their template path (e.g. "mychart/templates/deployment.yaml"). Partial
// templates (names beginning with "_") and NOTES.txt are not emitted.
func Render(chartPath string, opts Options) (map[string]string, error) {
	root, err := loadChart(chartPath)
	if nil != err {
		return nil, err
	}

	caps := newCapabilities(opts.Capabilities)
	rel := opts.Release

	// Compute scoped values for the whole tree (subchart scoping + globals).
	if nil == opts.UserValues {
		opts.UserValues = map[string]any{}
	}
	rootValues := coalesce(deepCopy(root.values), opts.UserValues)
	assignValues(root, rootValues)

	// Build the shared template set: every template across the tree is parsed
	// into one set so `define`/`include` are global, exactly as Helm does.
	// Functions must be registered BEFORE parsing, so the helm helpers are
	// bound up front; tpl reads partialSources (filled during collection) via
	// the pointer, which is safe because tpl only runs at execute time.
	tmplSet := template.New("keramos-helmcompat").Funcs(hermeticSprigFuncMap())
	fm := template.FuncMap{}
	var partialSources []string
	addHelmFuncs(fm, tmplSet, &partialSources)
	tmplSet.Funcs(fm)

	var renderUnits []renderUnit
	if walkErr := collectTemplates(root, tmplSet, &partialSources, &renderUnits); nil != walkErr {
		return nil, walkErr
	}

	out := make(map[string]string, len(renderUnits))
	for _, u := range renderUnits {
		ctx := map[string]any{
			"Values":       u.chart.scoped,
			"Chart":        u.chart.metadata,
			"Release":      releaseMap(rel),
			"Capabilities": caps,
			"Files":        newFiles(u.chart.files),
			"Template":     map[string]any{"Name": u.name, "BasePath": u.chart.fqName + "/templates"},
		}
		var buf bytes.Buffer
		if execErr := tmplSet.ExecuteTemplate(&buf, u.name, ctx); nil != execErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrParse, execErr, "render %s", u.name)
		}
		rendered := buf.String()
		if "" == strings.TrimSpace(stripComments(rendered)) {
			continue // template produced only whitespace/comments
		}
		out[u.name] = rendered
	}
	return out, nil
}

type renderUnit struct {
	name  string
	chart *chart
}

// collectTemplates parses every template in the chart tree into tmplSet, records
// the source of partial (_*) templates for tpl, and lists the units to render.
func collectTemplates(c *chart, set *template.Template, partials *[]string, units *[]renderUnit) error {
	names := make([]string, 0, len(c.templates))
	for n := range c.templates {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, rel := range names {
		body := c.templates[rel]
		fullName := c.fqName + "/templates/" + rel
		if _, parseErr := set.New(fullName).Parse(body); nil != parseErr {
			return keramoserr.WrapErrorf(keramoserr.ErrParse, parseErr, "parse template %s", fullName)
		}
		base := filepath.Base(rel)
		if strings.HasPrefix(base, "_") {
			*partials = append(*partials, body)
			continue
		}
		if "NOTES.txt" == base {
			continue
		}
		*units = append(*units, renderUnit{name: fullName, chart: c})
	}
	for _, sub := range c.subcharts {
		if err := collectTemplates(sub, set, partials, units); nil != err {
			return err
		}
	}
	return nil
}

func releaseMap(r ReleaseMeta) map[string]any {
	return map[string]any{
		"Name":      r.Name,
		"Namespace": r.Namespace,
		"Revision":  r.Revision,
		"IsInstall": r.IsInstall,
		"IsUpgrade": r.IsUpgrade,
		"Service":   "Helm",
	}
}

// stripComments removes YAML comment lines so an all-comment render counts as
// empty (Helm omits such documents).
func stripComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// readFile is indirected for tests; not exported.
var readFile = os.ReadFile
