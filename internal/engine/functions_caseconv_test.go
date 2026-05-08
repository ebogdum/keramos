package engine

import "testing"

func TestCaseConv(t *testing.T) {
	cases := []struct {
		fn   string
		val  string
		want string
	}{
		{"camelcase", "hello world foo", "helloWorldFoo"},
		{"kebabcase", "HelloWorld foo", "hello-world-foo"},
		{"snakecase", "HelloWorld foo", "hello_world_foo"},
		{"swapcase", "Hello World", "hELLO wORLD"},
		{"initials", "Foo Bar Baz", "FBB"},
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
			t.Errorf("%s(%q) = %q, want %q", c.fn, c.val, got, c.want)
		}
	}
}
