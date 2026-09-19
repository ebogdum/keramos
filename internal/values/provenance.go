package values

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ebogdum/keramos/v3/internal/layer"
)

// Source identifies where a value came from in the resolution chain.
type Source string

const (
	SourceDefault   Source = "package-default"
	SourceLayer     Source = "layer"
	SourceProfile   Source = "profile"
	SourceValueFile Source = "values-file"
	SourceSet       Source = "set"
	SourceSetString Source = "set-string"
	SourceSetFile   Source = "set-file"
	SourceSetJSON   Source = "set-json"
	SourceReuse     Source = "reuse-values"
)

// Step is a single contribution to a value's resolution. The chain is
// recorded in resolution order: earlier steps are weaker, later steps win.
type Step struct {
	Source Source
	Origin string // file path / layer name / set expression
	Value  any    // value at this step
}

// Trace maps dotted paths to the chain of contributions.
type Trace map[string][]Step

// Resolver records every override applied so operators can later ask
// "where did Values.replicas=3 come from?".
type Resolver struct {
	current map[string]any
	trace   Trace
}

// NewResolver starts a resolution from the given defaults. Pass nil for an
// empty starting point.
func NewResolver(defaults map[string]any) *Resolver {
	current := layer.DeepMerge(nil, defaults)
	if nil == current {
		// DeepMerge(nil, nil) yields a nil map; a subsequent --set would then
		// panic on assignment to a nil map. Start from an empty map instead.
		current = make(map[string]any)
	}
	r := &Resolver{
		current: current,
		trace:   make(Trace),
	}
	r.recordTree("", defaults, Step{Source: SourceDefault, Origin: "values.yaml"})
	return r
}

// ApplyMap merges `m` on top of the current resolution and records each
// affected leaf in the trace under `source` and `origin`.
func (r *Resolver) ApplyMap(m map[string]any, source Source, origin string) {
	if 0 == len(m) {
		return
	}
	r.current = layer.DeepMerge(r.current, m)
	r.recordTree("", m, Step{Source: source, Origin: origin})
}

// ApplySet applies a single --set-style expression. The full chain of leaves
// affected by the expression is recorded.
func (r *Resolver) ApplySet(expr string, source Source) error {
	key, val, found := strings.Cut(expr, "=")
	if !found {
		return fmt.Errorf("invalid set expression %q", expr)
	}
	parts, err := splitDotPath(key)
	if nil != err {
		return err
	}
	parsed := inferType(val)
	if err := setNestedValue(r.current, parts, parsed); nil != err {
		return err
	}
	path := strings.Join(parts, ".")
	// If this key previously held a map, its sub-leaf provenance no longer
	// applies — prune it so the trace matches the now-scalar value.
	if _, isMap := parsed.(map[string]any); !isMap {
		for existing := range r.trace {
			if strings.HasPrefix(existing, path+".") {
				delete(r.trace, existing)
			}
		}
	}
	r.trace[path] = append(r.trace[path], Step{Source: source, Origin: expr, Value: parsed})
	return nil
}

// Result returns the fully merged map.
func (r *Resolver) Result() map[string]any { return r.current }

// Trace returns a copy of the per-path resolution chain.
func (r *Resolver) Trace() Trace {
	out := make(Trace, len(r.trace))
	for k, v := range r.trace {
		out[k] = append([]Step{}, v...)
	}
	return out
}

// Provenance flattens the trace into a "where did each value come from" map:
// dotted key -> "source (origin)" of the winning contribution. Suitable for
// recording in a release so the origin of every value survives in the state.
func (t Trace) Provenance() map[string]string {
	out := make(map[string]string, len(t))
	for key, steps := range t {
		if 0 == len(steps) {
			continue
		}
		s := steps[len(steps)-1]
		out[key] = fmt.Sprintf("%s (%s)", s.Source, s.Origin)
	}
	return out
}

// recordTree walks `m` adding a Step for every leaf under `prefix`. When a
// path changes shape between contributions (scalar↔map), stale trace entries
// are pruned so the recorded provenance matches the final value structure.
func (r *Resolver) recordTree(prefix string, m map[string]any, base Step) {
	for k, v := range m {
		path := k
		if "" != prefix {
			path = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok {
			// Now a subtree: drop any prior scalar-leaf record at this exact path.
			delete(r.trace, path)
			r.recordTree(path, nested, base)
			continue
		}
		// Now a leaf: drop any prior sub-leaf records beneath this path, which
		// no longer exist in the merged result.
		for existing := range r.trace {
			if strings.HasPrefix(existing, path+".") {
				delete(r.trace, existing)
			}
		}
		step := base
		step.Value = v
		r.trace[path] = append(r.trace[path], step)
	}
}

// FormatTrace renders a single key's resolution chain as text. Pass an empty
// path to render every traced key.
func FormatTrace(trace Trace, path string) string {
	if "" != path {
		steps, ok := trace[path]
		if !ok {
			return fmt.Sprintf("no resolution recorded for %q", path)
		}
		return formatPath(path, steps)
	}
	keys := make([]string, 0, len(trace))
	for k := range trace {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(formatPath(k, trace[k]))
		b.WriteByte('\n')
	}
	return b.String()
}

func formatPath(path string, steps []Step) string {
	var b strings.Builder
	b.WriteString(path)
	b.WriteString(":\n")
	for i, s := range steps {
		marker := "    "
		if i == len(steps)-1 {
			marker = "  → "
		}
		b.WriteString(fmt.Sprintf("%s%s (%s) = %v\n", marker, s.Source, s.Origin, s.Value))
	}
	return b.String()
}

// ResolveAllWithTrace mirrors ResolveAll but also returns the resolution
// trace so callers (`keramos values --trace`) can show provenance.
func ResolveAllWithTrace(defaults map[string]any, valueFiles, sets, setStrings, setFiles, setJSON []string) (map[string]any, Trace, error) {
	r := NewResolver(defaults)

	for _, filePath := range valueFiles {
		fileVals, err := loadValuesFile(filePath)
		if nil != err {
			return nil, nil, err
		}
		r.ApplyMap(fileVals, SourceValueFile, filePath)
	}
	for _, s := range sets {
		if err := r.ApplySet(s, SourceSet); nil != err {
			return nil, nil, err
		}
	}
	for _, s := range setStrings {
		key, val, found := strings.Cut(s, "=")
		if !found {
			return nil, nil, fmt.Errorf("invalid --set-string %q", s)
		}
		parts, err := splitDotPath(key)
		if nil != err {
			return nil, nil, err
		}
		if err := setNestedValue(r.current, parts, val); nil != err {
			return nil, nil, err
		}
		r.trace[strings.Join(parts, ".")] = append(r.trace[strings.Join(parts, ".")],
			Step{Source: SourceSetString, Origin: s, Value: val})
	}
	for _, s := range setFiles {
		if err := ParseSetFile(r.current, s); nil != err {
			return nil, nil, err
		}
		// Record using the raw expression so trace shows the source path.
		key, _, _ := strings.Cut(s, "=")
		parts, _ := splitDotPath(key)
		r.trace[strings.Join(parts, ".")] = append(r.trace[strings.Join(parts, ".")],
			Step{Source: SourceSetFile, Origin: s, Value: "<file contents>"})
	}
	for _, s := range setJSON {
		if err := ParseSetJSON(r.current, s); nil != err {
			return nil, nil, err
		}
		key, val, _ := strings.Cut(s, "=")
		parts, _ := splitDotPath(key)
		r.trace[strings.Join(parts, ".")] = append(r.trace[strings.Join(parts, ".")],
			Step{Source: SourceSetJSON, Origin: s, Value: val})
	}
	return r.current, r.trace, nil
}
