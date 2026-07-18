package policy

import "testing"

func TestImageMatchesRegistryBoundary(t *testing.T) {
	cases := []struct {
		img, allowed string
		want         bool
	}{
		{"registry.internal/app:1", "registry.internal", true},
		{"registry.internal:5000/app", "registry.internal", true},
		{"registry.internal", "registry.internal", true},
		{"registry.internal.attacker.com/evil:1", "registry.internal", false}, // bypass blocked
		{"ghcr.io/myorg/app:1", "ghcr.io/myorg", true},
		{"ghcr.io/myorg-evil/x", "ghcr.io/myorg", false}, // sibling blocked
	}
	for _, c := range cases {
		if got := imageMatchesRegistry(c.img, c.allowed); got != c.want {
			t.Errorf("imageMatchesRegistry(%q,%q)=%v want %v", c.img, c.allowed, got, c.want)
		}
	}
}

func TestImageIsUnpinned(t *testing.T) {
	cases := []struct {
		img  string
		want bool
	}{
		{"nginx", true},
		{"nginx:latest", true},
		{"nginx:1.25", false},
		{"localhost:5000/img", true},      // port ':' is not a tag
		{"localhost:5000/img:1.0", false}, // real tag
		{"nginx@sha256:abc", false},       // digest-pinned
	}
	for _, c := range cases {
		if got := imageIsUnpinned(c.img); got != c.want {
			t.Errorf("imageIsUnpinned(%q)=%v want %v", c.img, got, c.want)
		}
	}
}

func TestLoadRulesRejectsUnknownSeverity(t *testing.T) {
	dir := t.TempDir()
	pdir := dir + "/policies"
	if err := mkdirWrite(pdir, "r.yaml", "name: r\nseverity: deney\nmatch: {kinds: [Deployment]}\n"); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRules(dir)
	if err == nil {
		t.Fatal("expected error for unknown severity 'deney'")
	}
}
