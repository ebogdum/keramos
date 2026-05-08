package engine

import "testing"

func TestMathFns(t *testing.T) {
	cases := []struct {
		name string
		fn   string
		val  any
		args []any
		want any
	}{
		{"add ints", "add", 1, []any{2, 3}, int64(6)},
		{"add float", "add", 1.5, []any{2.5}, int64(4)},
		{"sub", "sub", 10, []any{3, 2}, int64(5)},
		{"mul", "mul", 2, []any{3, 4}, int64(24)},
		{"div", "div", 12, []any{3, 2}, int64(2)},
		{"mod", "mod", 7, []any{3}, int64(1)},
		{"max", "max", 1, []any{5, 2, 4}, int64(5)},
		{"min", "min", 5, []any{2, 4}, int64(2)},
		{"floor", "floor", 1.7, nil, int64(1)},
		{"ceil", "ceil", 1.2, nil, int64(2)},
		{"round whole", "round", 1.5, nil, int64(2)},
		{"abs", "abs", -3.5, nil, 3.5},
	}
	r := NewFuncRegistry()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn, ok := r.Get(c.fn)
			if !ok {
				t.Fatalf("function %q not registered", c.fn)
			}
			got, err := fn(c.val, c.args...)
			if nil != err {
				t.Fatalf("%s: %v", c.fn, err)
			}
			if got != c.want {
				t.Errorf("%s(%v, %v) = %v (%T), want %v (%T)", c.fn, c.val, c.args, got, got, c.want, c.want)
			}
		})
	}
}

func TestDivByZero(t *testing.T) {
	if _, err := fnDiv(10, 0); nil == err {
		t.Fatal("expected division-by-zero error")
	}
}
