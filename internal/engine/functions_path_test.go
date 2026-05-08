package engine

import "testing"

func TestPathFns(t *testing.T) {
	cases := []struct {
		fn   string
		val  any
		want any
	}{
		{"base", "/a/b/c.txt", "c.txt"},
		{"dir", "/a/b/c.txt", "/a/b"},
		{"clean", "/a/./b/../c", "/a/c"},
		{"ext", "/a/b/c.txt", ".txt"},
		{"isAbs", "/a", true},
		{"isAbs", "a", false},
	}
	r := NewFuncRegistry()
	for _, c := range cases {
		fn, _ := r.Get(c.fn)
		got, err := fn(c.val)
		if nil != err {
			t.Errorf("%s: %v", c.fn, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s(%v) = %v, want %v", c.fn, c.val, got, c.want)
		}
	}
}

func TestURLParseJoin(t *testing.T) {
	parsed, err := fnURLParse("https://user:pass@host:8080/path?q=1#frag")
	if nil != err {
		t.Fatalf("urlParse: %v", err)
	}
	m := parsed.(map[string]any)
	if "https" != m["scheme"] || "host:8080" != m["host"] || "/path" != m["path"] {
		t.Errorf("urlParse = %v", m)
	}
}
