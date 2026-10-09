package helmcompat

import (
	"bytes"
	"fmt"
	"text/template"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
)

type SnippetContext struct {
	Values       map[string]any
	Release      ReleaseMeta
	Chart        map[string]any
	Capabilities CapabilitiesMeta
	Files        map[string][]byte
	Partials     []string
	Include      func(name string) (string, error)
	TemplateName string
	BasePath     string
}

func RenderSnippet(snippet string, ctx SnippetContext) (string, error) {
	set := template.New("__keramos_snippet__")
	fm := hermeticSprigFuncMap()
	sources := append([]string{}, ctx.Partials...)
	addHelmFuncs(fm, set, &sources)
	set = set.Funcs(fm)

	for i, src := range ctx.Partials {
		if _, err := set.New(fmt.Sprintf("__snippet_partial_%d__", i)).Parse(src); nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrParse, "helm: cannot parse partials", err)
		}
	}
	if nil != ctx.Include {
		fm["include"] = snippetInclude(set, ctx.Include)
		fm["template"] = fm["include"]
		set = set.Funcs(fm)
	}

	body, err := set.New("__keramos_snippet__").Parse(snippet)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrParse, "helm: cannot parse snippet", err)
	}

	var buf bytes.Buffer
	if err := body.Execute(&buf, snippetData(ctx)); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrParse, "helm: cannot render snippet", err)
	}
	return buf.String(), nil
}

func snippetInclude(set *template.Template, fallback func(string) (string, error)) func(string, any) (string, error) {
	return func(name string, data any) (string, error) {
		if nil != set.Lookup(name) {
			var buf bytes.Buffer
			if err := set.ExecuteTemplate(&buf, name, data); nil != err {
				return "", err
			}
			return buf.String(), nil
		}
		return fallback(name)
	}
}

func snippetData(ctx SnippetContext) map[string]any {
	values := ctx.Values
	if nil == values {
		values = map[string]any{}
	}
	chart := ctx.Chart
	if nil == chart {
		chart = map[string]any{}
	}
	return map[string]any{
		"Values":       helmValues(values),
		"Release":      releaseMap(ctx.Release),
		"Chart":        chart,
		"Capabilities": newCapabilities(ctx.Capabilities),
		"Files":        newFiles(ctx.Files),
		"Template":     map[string]any{"Name": ctx.TemplateName, "BasePath": ctx.BasePath},
	}
}
