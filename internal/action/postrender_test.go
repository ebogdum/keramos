package action

import (
	"strings"
	"testing"
	"time"
)

func TestPostRenderer_Empty(t *testing.T) {
	out, err := runPostRenderer("", "manifest", time.Second)
	if nil != err {
		t.Fatalf("err: %v", err)
	}
	if "manifest" != out {
		t.Errorf("expected pass-through")
	}
}

func TestPostRenderer_RejectsShellMeta(t *testing.T) {
	for _, bad := range []string{"foo;bar", "foo|bar", "foo`bar`", "foo$VAR"} {
		_, err := runPostRenderer(bad, "x", time.Second)
		if nil == err {
			t.Errorf("expected rejection of %q", bad)
		}
	}
}

func TestPostRenderer_Cat(t *testing.T) {
	// `cat` is part of every POSIX environment used in CI.
	out, err := runPostRenderer("cat", "hello\n", 5*time.Second)
	if nil != err {
		t.Fatalf("cat: %v", err)
	}
	if "hello\n" != out {
		t.Errorf("cat output = %q", out)
	}
}

func TestPostRenderer_NotFound(t *testing.T) {
	_, err := runPostRenderer("definitely-not-a-binary-7421", "x", time.Second)
	if nil == err {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found on PATH") {
		t.Errorf("err = %v", err)
	}
}

func TestPostRendererChain(t *testing.T) {
	out, err := runPostRenderers([]string{"cat", "cat"}, "chained\n", 5*time.Second)
	if nil != err {
		t.Fatalf("chain: %v", err)
	}
	if "chained\n" != out {
		t.Errorf("chain output = %q", out)
	}
}
