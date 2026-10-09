package engine

import (
	"fmt"
	"sort"

	"github.com/ebogdum/keramos/v3/internal/helmcompat"
)

func helmReleaseMeta(ctx *RenderContext) helmcompat.ReleaseMeta {
	meta := helmcompat.ReleaseMeta{IsInstall: true}
	if nil == ctx || nil == ctx.Release {
		return meta
	}
	meta.Name = coerceString(ctx.Release["name"])
	meta.Namespace = coerceString(ctx.Release["namespace"])
	if revision, ok := coerceInt(ctx.Release["revision"]); ok {
		meta.Revision = revision
	}
	if upgrade, ok := ctx.Release["isUpgrade"].(bool); ok {
		meta.IsUpgrade = upgrade
		meta.IsInstall = !upgrade
	}
	return meta
}

func helmChartMeta(ctx *RenderContext) map[string]any {
	out := map[string]any{}
	if nil == ctx || nil == ctx.Package {
		return out
	}
	for key, target := range map[string]string{
		"name": "Name", "version": "Version", "appVersion": "AppVersion",
		"description": "Description", "type": "Type",
	} {
		if v, ok := ctx.Package[key]; ok {
			out[target] = v
		}
	}
	return out
}

func helmCapabilities(ctx *RenderContext) helmcompat.CapabilitiesMeta {
	meta := helmcompat.CapabilitiesMeta{}
	if nil == ctx || nil == ctx.Capabilities {
		return meta
	}
	if kube, ok := ctx.Capabilities["kubeVersion"].(map[string]any); ok {
		meta.KubeVersion = coerceString(kube["Version"])
	}
	for version := range apiVersionSet(ctx) {
		meta.APIVersions = append(meta.APIVersions, version)
	}
	sort.Strings(meta.APIVersions)
	return meta
}

func keramosInclude(include Func) func(string) (string, error) {
	if nil == include {
		return nil
	}
	return func(name string) (string, error) {
		out, err := include(name)
		if nil != err {
			return "", err
		}
		return fmt.Sprintf("%v", out), nil
	}
}

func helmChartName(ctx *RenderContext) string {
	if nil == ctx || nil == ctx.Package {
		return ""
	}
	return coerceString(ctx.Package["name"])
}
