package engine

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pat   string
		name  string
		match bool
	}{
		{"*", "foo.yaml", true},
		{"*.yaml", "foo.yaml", true},
		{"*.yaml", "foo.yml", false},
		{"**/*.yaml", "a/b/c.yaml", true},
		{"templates/**/*.yaml", "templates/sub/x.yaml", true},
		{"templates/*.yaml", "templates/x.yaml", true},
		{"templates/*.yaml", "other/x.yaml", false},
	}
	for _, c := range cases {
		got, err := matchGlob(c.pat, c.name)
		if nil != err {
			t.Errorf("matchGlob(%q, %q): %v", c.pat, c.name, err)
			continue
		}
		if got != c.match {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pat, c.name, got, c.match)
		}
	}
}
