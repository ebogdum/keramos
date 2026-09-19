package engine

import (
	"math"
	"strconv"

	keramoserrors "github.com/ebogdum/keramos/v3/internal/errors"
)

func registerMathFuncs(r *FuncRegistry) {
	r.Register("add", fnAdd)
	r.Register("sub", fnSub)
	r.Register("mul", fnMul)
	r.Register("div", fnDiv)
	r.Register("mod", fnMod)
	r.Register("max", fnMax)
	r.Register("min", fnMin)
	r.Register("floor", fnFloor)
	r.Register("ceil", fnCeil)
	r.Register("round", fnRound)
	r.Register("abs", fnAbs)
}

func parseAllNumbers(value any, args []any) ([]float64, error) {
	out := make([]float64, 0, len(args)+1)
	if v, ok := toFloat(value); ok {
		out = append(out, v)
	} else {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "expected numeric, got %T", value)
	}
	for _, a := range args {
		f, ok := coerceFloat(a)
		if !ok {
			return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "invalid number %v", a)
		}
		out = append(out, f)
	}
	return out, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if nil == err {
			return f, true
		}
	}
	return 0, false
}

func numericResult(f float64) any {
	if f == math.Trunc(f) && !math.IsInf(f, 0) {
		return int64(f)
	}
	return f
}

func fnAdd(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return numericResult(sum), nil
}

func fnSub(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	if 1 == len(nums) {
		return numericResult(-nums[0]), nil
	}
	r := nums[0]
	for _, n := range nums[1:] {
		r -= n
	}
	return numericResult(r), nil
}

func fnMul(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	r := 1.0
	for _, n := range nums {
		r *= n
	}
	return numericResult(r), nil
}

func fnDiv(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	if 1 > len(nums)-1 {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "div requires at least one divisor")
	}
	r := nums[0]
	for _, n := range nums[1:] {
		if 0 == n {
			return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "div: division by zero")
		}
		r /= n
	}
	return numericResult(r), nil
}

func fnMod(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	if 2 != len(nums) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "mod requires exactly two operands")
	}
	if 0 == nums[1] {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "mod: division by zero")
	}
	return numericResult(math.Mod(nums[0], nums[1])), nil
}

func fnMax(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	r := nums[0]
	for _, n := range nums[1:] {
		if n > r {
			r = n
		}
	}
	return numericResult(r), nil
}

func fnMin(value any, args ...any) (any, error) {
	nums, err := parseAllNumbers(value, args)
	if nil != err {
		return nil, err
	}
	r := nums[0]
	for _, n := range nums[1:] {
		if n < r {
			r = n
		}
	}
	return numericResult(r), nil
}

func fnFloor(value any, args ...any) (any, error) {
	f, ok := toFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "floor: expected numeric, got %T", value)
	}
	return int64(math.Floor(f)), nil
}

func fnCeil(value any, args ...any) (any, error) {
	f, ok := toFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "ceil: expected numeric, got %T", value)
	}
	return int64(math.Ceil(f)), nil
}

func fnRound(value any, args ...any) (any, error) {
	f, ok := toFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "round: expected numeric, got %T", value)
	}
	if 0 == len(args) {
		return int64(math.Round(f)), nil
	}
	digits, err := strconv.Atoi(coerceString(args[0]))
	if nil != err {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "round: invalid digit count %q", args[0])
	}
	mult := math.Pow(10, float64(digits))
	return numericResult(math.Round(f*mult) / mult), nil
}

func fnAbs(value any, args ...any) (any, error) {
	f, ok := toFloat(value)
	if !ok {
		return nil, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "abs: expected numeric, got %T", value)
	}
	return numericResult(math.Abs(f)), nil
}
