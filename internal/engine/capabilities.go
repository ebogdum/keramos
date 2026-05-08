package engine

import (
	"strings"
)

// VersionSet is the API-version registry surfaced to templates as
// `.Capabilities.APIVersions`. It supports the `Has` method that templates
// use to gate api-version-specific resources:
//
//	{{ if .Capabilities.APIVersions.Has "policy/v1" }}
type VersionSet struct {
	versions map[string]bool
}

// NewVersionSet builds a VersionSet from a slice of group/version strings.
func NewVersionSet(versions ...string) *VersionSet {
	out := &VersionSet{versions: make(map[string]bool, len(versions))}
	for _, v := range versions {
		out.versions[v] = true
	}
	return out
}

// Has returns true if the given group/version (or group/version/kind) is
// available. Both "policy/v1" and "policy/v1/PodDisruptionBudget" forms work.
func (v *VersionSet) Has(version string) bool {
	if nil == v || nil == v.versions {
		return false
	}
	if v.versions[version] {
		return true
	}
	// If caller passed group/version/kind, also try the group/version prefix.
	if idx := strings.LastIndex(version, "/"); idx > 0 {
		if v.versions[version[:idx]] {
			return true
		}
	}
	return false
}

// AllVersions returns the registered version strings for serialization
// or merging purposes. Order is unspecified.
func (v *VersionSet) AllVersions() []string {
	if nil == v {
		return nil
	}
	out := make([]string, 0, len(v.versions))
	for k := range v.versions {
		out = append(out, k)
	}
	return out
}

// KubeVersion is the cluster's reported version surfaced to templates as
// .Capabilities.KubeVersion.{Major,Minor,Version}.
type KubeVersion struct {
	Version    string
	Major      string
	Minor      string
	GitVersion string
}

// String makes the struct print as the GitVersion when used in templates.
func (k *KubeVersion) String() string {
	if nil == k {
		return ""
	}
	if "" != k.GitVersion {
		return k.GitVersion
	}
	return k.Version
}
