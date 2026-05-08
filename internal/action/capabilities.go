package action

import (
	"github.com/ebogdum/keramos/internal/engine"
)

// overrideCapabilities applies user-provided --api-versions and --kube-version
// flags onto the discovered cluster capabilities map. The result populates
// typed engine.VersionSet and engine.KubeVersion so templates can call
// `.Capabilities.APIVersions.Has "policy/v1"` and read `.Capabilities.KubeVersion.Major`.
func overrideCapabilities(caps map[string]any, apiVersions []string, kubeVersion string) {
	if nil == caps {
		return
	}

	// API versions
	merged := make([]string, 0, len(apiVersions))
	if existing, ok := caps["apiVersions"].(map[string]bool); ok {
		for k := range existing {
			merged = append(merged, k)
		}
	}
	if existingSet, ok := caps["apiVersions"].(*engine.VersionSet); ok && nil != existingSet {
		merged = append(merged, existingSet.AllVersions()...)
	}
	for _, v := range apiVersions {
		if "" != v {
			merged = append(merged, v)
		}
	}
	if 0 < len(merged) {
		caps["apiVersions"] = engine.NewVersionSet(merged...)
	}

	// Kube version. Expose both as the typed struct (for templates that
	// call `.Capabilities.KubeVersion` and check `.GitVersion`) AND as
	// a map so keramos's path resolver can navigate dotted paths like
	// `${capabilities.kubeVersion.GitVersion}` without struct reflection.
	if "" != kubeVersion {
		major, minor := splitKubeVersion(kubeVersion)
		caps["kubeVersion"] = map[string]any{
			"Version":    kubeVersion,
			"Major":      major,
			"Minor":      minor,
			"GitVersion": kubeVersion,
		}
		caps["kubeVersionStruct"] = &engine.KubeVersion{
			Version: kubeVersion, Major: major, Minor: minor, GitVersion: kubeVersion,
		}
	}
}

func splitKubeVersion(v string) (major, minor string) {
	// Accept "v1.30", "1.30.2", "v1.30.2+gke" — extract the first two dotted segments.
	s := v
	if 0 < len(s) && 'v' == s[0] {
		s = s[1:]
	}
	parts := splitOnce(s, '.')
	if 1 == len(parts) {
		return parts[0], ""
	}
	major = parts[0]
	rest := parts[1]
	rp := splitOnce(rest, '.')
	minor = rp[0]
	return major, minor
}

func splitOnce(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if sep == s[i] {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
