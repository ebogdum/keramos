package pkg

import "fmt"

// LayerSource describes a composition layer or a required co-deployed package.
type LayerSource struct {
	Name      string   `yaml:"name"`
	Source    string   `yaml:"source"`              // local path, https URL, or git:: URL
	Version   string   `yaml:"version,omitempty"`   // semver constraint for registry sources
	Ref       string   `yaml:"ref,omitempty"`       // git ref (branch, tag, commit)
	Condition string   `yaml:"condition,omitempty"` // dotted values path; layer enabled iff truthy
	Tags      []string `yaml:"tags,omitempty"`      // tag names; layer enabled iff any tag is enabled in values
	Alias     string   `yaml:"alias,omitempty"`     // alternate name for the layer in this package
	Enabled   *bool    `yaml:"enabled,omitempty"`   // explicit override; nil = follow condition/tags
}

// PackageMetadata represents the keramos.yaml package manifest.
type PackageMetadata struct {
	APIVersion  string            `yaml:"apiVersion"`
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	AppVersion  string            `yaml:"appVersion,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Type        string            `yaml:"type,omitempty"`
	KubeVersion string            `yaml:"kubeVersion,omitempty"`
	Base        string            `yaml:"base,omitempty"`         // deprecated: use layers
	Layers      []LayerSource     `yaml:"layers,omitempty"`
	Requires    []LayerSource     `yaml:"requires,omitempty"`
	Dependencies []Dependency     `yaml:"dependencies,omitempty"` // deprecated: use layers
	Immutable   []string          `yaml:"immutable,omitempty"`
	Maintainers []Maintainer      `yaml:"maintainers,omitempty"`
	Keywords    []string          `yaml:"keywords,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`

	// Environments is a first-class keramos addition: declared deployment
	// environments (dev, staging, prod, …) with per-env value overrides
	// and inheritance. Replaces the values-{env}.yaml proliferation.
	Environments map[string]Environment `yaml:"environments,omitempty"`
}

// Environment is a named deployment target with its own value overrides and
// optional parent for inheritance.
type Environment struct {
	Inherits   string         `yaml:"inherits,omitempty"`  // name of another env this env extends
	ValueFiles []string       `yaml:"valueFiles,omitempty"` // additional -f files
	Values     map[string]any `yaml:"values,omitempty"`     // inline overrides
	Profile    string         `yaml:"profile,omitempty"`
	Namespace  string         `yaml:"namespace,omitempty"`
	Cluster    string         `yaml:"cluster,omitempty"`    // kubeconfig context
}

// ResolveEnvironment walks the inheritance chain for an environment and
// returns the merged Environment. Cycles are reported as an error.
func (m *PackageMetadata) ResolveEnvironment(name string) (Environment, error) {
	visited := make(map[string]bool)
	chain := make([]Environment, 0)
	cur := name
	for "" != cur {
		if visited[cur] {
			return Environment{}, fmt.Errorf("environment inheritance cycle at %q", cur)
		}
		visited[cur] = true
		env, ok := m.Environments[cur]
		if !ok {
			return Environment{}, fmt.Errorf("environment %q not declared in keramos.yaml", cur)
		}
		chain = append(chain, env)
		cur = env.Inherits
	}
	// Merge oldest-first: ancestor → child overrides.
	merged := Environment{Values: map[string]any{}}
	for i := len(chain) - 1; i >= 0; i-- {
		envMergeInto(&merged, chain[i])
	}
	return merged, nil
}

func envMergeInto(dst *Environment, src Environment) {
	if "" != src.Profile {
		dst.Profile = src.Profile
	}
	if "" != src.Namespace {
		dst.Namespace = src.Namespace
	}
	if "" != src.Cluster {
		dst.Cluster = src.Cluster
	}
	dst.ValueFiles = append(dst.ValueFiles, src.ValueFiles...)
	if 0 < len(src.Values) {
		dst.Values = mergeAny(dst.Values, src.Values)
	}
}

func mergeAny(dst, src map[string]any) map[string]any {
	if nil == dst {
		dst = map[string]any{}
	}
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		dm, dok := dv.(map[string]any)
		sm, sok := sv.(map[string]any)
		if dok && sok {
			dst[k] = mergeAny(dm, sm)
			continue
		}
		dst[k] = sv
	}
	return dst
}

// Dependency represents a legacy package dependency in keramos.yaml.
// Deprecated: use LayerSource with layers field instead.
type Dependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
	Condition  string `yaml:"condition,omitempty"`
	Alias      string `yaml:"alias,omitempty"`
}

// Maintainer represents a package maintainer.
type Maintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// EffectiveLayers returns the resolved list of layers, applying backward
// compatibility for the old base and dependencies fields.
// Old base: converted to a single local-path layer named "base".
// Old dependencies: each converted to a layer with registry source.
// Explicit layers always take priority if present.
func (m *PackageMetadata) EffectiveLayers() []LayerSource {
	if 0 < len(m.Layers) {
		return m.Layers
	}

	var result []LayerSource

	if "" != m.Base {
		result = append(result, LayerSource{
			Name:   "base",
			Source: m.Base,
		})
	}

	for _, dep := range m.Dependencies {
		result = append(result, LayerSource{
			Name:      dep.Name,
			Source:    dep.Repository,
			Version:   dep.Version,
			Condition: dep.Condition,
			Alias:     dep.Alias,
		})
	}

	return result
}

// IsEnabled evaluates whether a layer should be activated given the merged
// values and global tag selection. Resolution order:
//
//   - Enabled (explicit) overrides everything when non-nil.
//   - Condition is a dotted path into values; the layer is enabled when
//     evaluating to a truthy value.
//   - Tags: the layer is enabled when *any* tag is enabled in
//     values.tags.<name>.
//   - With no condition / tags / explicit override the layer is enabled.
func (l LayerSource) IsEnabled(values map[string]any) bool {
	if nil != l.Enabled {
		return *l.Enabled
	}
	if "" != l.Condition {
		if v, found := lookupDotted(values, l.Condition); found {
			return isTruthyValue(v)
		}
		return false
	}
	if 0 < len(l.Tags) {
		tags, _ := values["tags"].(map[string]any)
		for _, t := range l.Tags {
			if v, ok := tags[t]; ok && isTruthyValue(v) {
				return true
			}
		}
		return false
	}
	return true
}

func lookupDotted(values map[string]any, path string) (any, bool) {
	parts := splitDot(path)
	var cur any = values
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, exists := m[p]
		if !exists {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func splitDot(s string) []string {
	out := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if '.' == s[i] {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func isTruthyValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return "" != x && "false" != x
	case int:
		return 0 != x
	case int64:
		return 0 != x
	case float64:
		return 0 != x
	case nil:
		return false
	}
	return true
}
