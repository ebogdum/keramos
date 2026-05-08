package engine

import "testing"

func TestVersionSetHas(t *testing.T) {
	vs := NewVersionSet("policy/v1", "apps/v1")
	cases := map[string]bool{
		"policy/v1":                        true,
		"policy/v1/PodDisruptionBudget":    true, // group/version/kind also matches
		"apps/v1":                          true,
		"missing/v1":                       false,
		"":                                 false,
	}
	for in, want := range cases {
		got := vs.Has(in)
		if got != want {
			t.Errorf("Has(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestVersionSetNil(t *testing.T) {
	var vs *VersionSet
	if vs.Has("anything") {
		t.Error("nil VersionSet.Has should be false")
	}
}

func TestKubeVersionString(t *testing.T) {
	kv := &KubeVersion{Version: "1.28.0", GitVersion: "v1.28.0+gke.1"}
	if "v1.28.0+gke.1" != kv.String() {
		t.Errorf("KubeVersion.String() = %q", kv.String())
	}
}

func TestKubeVersionStringFallback(t *testing.T) {
	kv := &KubeVersion{Version: "1.28.0"}
	if "1.28.0" != kv.String() {
		t.Errorf("KubeVersion.String() fallback = %q", kv.String())
	}
}
