package engine

// Func is a template function. Args are passed as `any` so callers can pass
// numbers, booleans, maps, and slices directly without lossy stringification.
// Helpers `coerceString`/`coerceFloat`/`coerceBool` convert when a function
// truly needs a primitive.
type Func func(value any, args ...any) (any, error)

// FuncRegistry holds all available template functions. A registry can be
// composed: an `override` map shadows entries in `base`, allowing a per-render
// registry to layer context-bound closures (`tpl`, `lookup`, `Files.*`,
// `include`) on top of the engine-wide stateless built-ins without mutating
// the shared registry.
type FuncRegistry struct {
	funcs    map[string]Func
	base     *FuncRegistry
	override map[string]Func
}

// NewFuncRegistry creates a registry with all built-in stateless functions.
func NewFuncRegistry() *FuncRegistry {
	r := &FuncRegistry{
		funcs: make(map[string]Func, 256),
	}
	registerStringFuncs(r)
	registerTypeFuncs(r)
	registerEncodingFuncs(r)
	registerLogicFuncs(r)
	registerCollectionFuncs(r)
	registerMathFuncs(r)
	registerRegexFuncs(r)
	registerDateFuncs(r)
	registerMiscFuncs(r)
	registerCryptoFuncs(r)
	registerPathFuncs(r)
	registerCaseConvFuncs(r)
	registerSprigExtras(r)
	registerSprigRemainder(r)
	registerSprigFinal(r)
	registerSecretFuncs(r)
	registerExternalFuncs(r)
	return r
}

// NewLayeredRegistry produces a derived registry that consults `override`
// first then falls back to `base`. The override map is owned by the caller
// and not copied; callers should not mutate it after creation.
func NewLayeredRegistry(base *FuncRegistry, override map[string]Func) *FuncRegistry {
	if nil == override {
		override = make(map[string]Func)
	}
	return &FuncRegistry{base: base, override: override}
}

// Get retrieves a function by name, checking the per-render override map
// first then the base registry.
func (r *FuncRegistry) Get(name string) (Func, bool) {
	if nil != r.override {
		if fn, ok := r.override[name]; ok {
			return fn, true
		}
	}
	if nil != r.funcs {
		if fn, ok := r.funcs[name]; ok {
			return fn, true
		}
	}
	if nil != r.base {
		return r.base.Get(name)
	}
	return nil, false
}

// Register adds a function. On a layered registry the function lands in the
// override map so it does not leak into the shared base.
func (r *FuncRegistry) Register(name string, fn Func) {
	if nil != r.override {
		r.override[name] = fn
		return
	}
	if nil == r.funcs {
		r.funcs = make(map[string]Func)
	}
	r.funcs[name] = fn
}
