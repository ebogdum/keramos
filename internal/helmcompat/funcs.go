package helmcompat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"gopkg.in/yaml.v3"
)

// addHelmFuncs installs the helm-specific template functions onto fm, bound to
// the shared template set (for include) and the collected partial sources (for
// tpl, which parses an arbitrary string that may reference shared defines).
// maxRenderDepth bounds tpl/include recursion so a self-referential chart
// value or a define that includes itself cannot stack-overflow the renderer
// (Helm enforces an equivalent limit). tpl and include share the counter, so
// the bound is on total render nesting.
const maxRenderDepth = 32

// hermeticSprigFuncMap returns the Sprig text function map with the host-facing
// env/expandenv functions removed, matching upstream Helm's engine. Rendering an
// untrusted chart must not be able to read the operator's process environment
// (cloud credentials, tokens) and copy it into a rendered manifest. Upstream
// Helm deletes exactly these two functions for the same reason.
func hermeticSprigFuncMap() template.FuncMap {
	fm := sprig.TxtFuncMap()
	delete(fm, "env")
	delete(fm, "expandenv")
	return fm
}

func addHelmFuncs(fm template.FuncMap, set *template.Template, partialSources *[]string) {
	renderDepth := 0

	fm["include"] = func(name string, data any) (string, error) {
		if renderDepth >= maxRenderDepth {
			return "", keramoserr.NewErrorf(keramoserr.ErrParse, "include: recursion depth %d exceeded (%q)", maxRenderDepth, name)
		}
		renderDepth++
		defer func() { renderDepth-- }()
		var buf bytes.Buffer
		if err := set.ExecuteTemplate(&buf, name, data); nil != err {
			return "", err
		}
		return buf.String(), nil
	}

	fm["tpl"] = func(tmplStr string, data any) (string, error) {
		if renderDepth >= maxRenderDepth {
			return "", keramoserr.NewErrorf(keramoserr.ErrParse, "tpl: recursion depth %d exceeded", maxRenderDepth)
		}
		renderDepth++
		defer func() { renderDepth-- }()
		// Build a fresh template carrying the same funcs and all shared
		// partial defines so `include`/`define` resolve inside the string.
		t := template.New("keramos-tpl").Funcs(hermeticSprigFuncMap()).Funcs(fm)
		for i, src := range *partialSources {
			if _, err := t.New(fmt.Sprintf("__partial_%d__", i)).Parse(src); nil != err {
				return "", err
			}
		}
		if _, err := t.New("__tpl__").Parse(tmplStr); nil != err {
			return "", keramoserr.WrapErrorf(keramoserr.ErrParse, err, "tpl: parse")
		}
		var buf bytes.Buffer
		if err := t.ExecuteTemplate(&buf, "__tpl__", data); nil != err {
			return "", keramoserr.WrapErrorf(keramoserr.ErrParse, err, "tpl: execute")
		}
		return buf.String(), nil
	}

	fm["required"] = func(msg string, v any) (any, error) {
		if isEmptyValue(v) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "required value missing: %s", msg)
		}
		return v, nil
	}

	fm["toYaml"] = func(v any) (string, error) {
		out, err := yaml.Marshal(v)
		if nil != err {
			return "", err
		}
		return strings.TrimRight(string(out), "\n"), nil
	}
	fm["fromYaml"] = func(s string) (map[string]any, error) {
		m := map[string]any{}
		err := yaml.Unmarshal([]byte(s), &m)
		return m, err
	}
	fm["fromYamlArray"] = func(s string) ([]any, error) {
		var a []any
		err := yaml.Unmarshal([]byte(s), &a)
		return a, err
	}
	fm["toJson"] = func(v any) (string, error) {
		out, err := json.Marshal(v)
		return string(out), err
	}
	fm["fromJson"] = func(s string) (map[string]any, error) {
		m := map[string]any{}
		err := json.Unmarshal([]byte(s), &m)
		return m, err
	}
	fm["fromJsonArray"] = func(s string) ([]any, error) {
		var a []any
		err := json.Unmarshal([]byte(s), &a)
		return a, err
	}

	// lookup: render-time has no cluster connection, so return empty, matching
	// `helm template`. Charts guard with `if (lookup ...)`.
	fm["lookup"] = func(_, _, _, _ string) (map[string]any, error) {
		return map[string]any{}, nil
	}
}

func isEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return "" == x
	case []any:
		return 0 == len(x)
	case map[string]any:
		return 0 == len(x)
	case bool:
		return !x
	}
	return false
}
