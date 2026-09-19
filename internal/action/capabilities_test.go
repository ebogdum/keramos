package action

import (
	"testing"

	"github.com/ebogdum/keramos/v3/internal/engine"
)

func TestOverrideCapabilitiesAPIVersions(t *testing.T) {
	caps := map[string]any{}
	overrideCapabilities(caps, []string{"policy/v1", "apps/v1"}, "")
	vs, ok := caps["apiVersions"].(*engine.VersionSet)
	if !ok {
		t.Fatalf("apiVersions = %T, want *engine.VersionSet", caps["apiVersions"])
	}
	if !vs.Has("policy/v1") || !vs.Has("apps/v1") {
		t.Errorf("VersionSet missing entries: %v", vs.AllVersions())
	}
}

func TestOverrideCapabilitiesKubeVersion(t *testing.T) {
	caps := map[string]any{}
	overrideCapabilities(caps, nil, "v1.28.2")
	kvMap, ok := caps["kubeVersion"].(map[string]any)
	if !ok {
		t.Fatalf("kubeVersion = %T, want map[string]any", caps["kubeVersion"])
	}
	if "1" != kvMap["Major"] {
		t.Errorf("Major = %v", kvMap["Major"])
	}
	if "28" != kvMap["Minor"] {
		t.Errorf("Minor = %v", kvMap["Minor"])
	}
	if _, ok := caps["kubeVersionStruct"].(*engine.KubeVersion); !ok {
		t.Errorf("kubeVersionStruct missing")
	}
}

func TestOverrideCapabilitiesPreservesExisting(t *testing.T) {
	caps := map[string]any{
		"apiVersions": engine.NewVersionSet("apps/v1"),
	}
	overrideCapabilities(caps, []string{"policy/v1"}, "")
	vs := caps["apiVersions"].(*engine.VersionSet)
	if !vs.Has("apps/v1") || !vs.Has("policy/v1") {
		t.Errorf("merge lost an entry: %v", vs.AllVersions())
	}
}
