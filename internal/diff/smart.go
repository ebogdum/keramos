// Package diff provides Kubernetes-aware structured diffing that separates
// signal from noise. Plain line-level diffs are dominated by rotating SHAs,
// server-managed fields, and defaulted values — none of which an operator
// cares about. This package parses manifests as resources and offers
// per-noise-type filters.
package diff

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Filters control which classes of changes are surfaced. Each is independent
// — set Show* flags to opt back in when the operator wants noise.
type Filters struct {
	ShowStatus          bool // changes under .status
	ShowManagedFields   bool // metadata.managedFields
	ShowGeneration      bool // metadata.generation, resourceVersion, uid, creationTimestamp
	ShowDefaultedFields bool // server-side defaults (e.g. Service.spec.clusterIP, ports[].protocol)
	ShowAnnotations     bool // metadata.annotations (often drift from controllers)
	ShowLabels          bool // metadata.labels added by mutating webhooks
	ShowImagePullPolicy bool // container imagePullPolicy server-defaults to "Always" or "IfNotPresent"
	ShowFinalizers      bool // metadata.finalizers added by controllers
	ShowOwnerRefs       bool // metadata.ownerReferences
	ShowSecretRotation  bool // string Secret data values that change every render due to genX functions
}

// Resource is a parsed Kubernetes object.
type Resource struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Body       map[string]any // full parsed YAML
}

// Key produces a stable identifier for matching resources across renders.
func (r Resource) Key() string {
	return r.APIVersion + "|" + r.Kind + "|" + r.Namespace + "|" + r.Name
}

// Change is a single resource diff in the structured output.
type Change struct {
	Key       string
	Kind      ChangeKind
	Resource  Resource
	OldYAML   string // empty when Kind == ChangeAdd
	NewYAML   string // empty when Kind == ChangeRemove
	FieldDiff []FieldChange
}

type ChangeKind int

const (
	ChangeAdd ChangeKind = iota
	ChangeRemove
	ChangeModify
)

func (c ChangeKind) String() string {
	switch c {
	case ChangeAdd:
		return "+"
	case ChangeRemove:
		return "-"
	case ChangeModify:
		return "~"
	}
	return "?"
}

// FieldChange describes a single filtered, kept field-level edit.
type FieldChange struct {
	Path string
	Old  any
	New  any
}

// Parse reads a multi-document YAML manifest into a map keyed by Resource.Key.
func Parse(manifest string) (map[string]Resource, error) {
	out := make(map[string]Resource)
	if "" == strings.TrimSpace(manifest) {
		return out, nil
	}
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if nil != err {
			if "EOF" == err.Error() {
				break
			}
			return nil, fmt.Errorf("parse manifest: %w", err)
		}
		if nil == doc {
			continue
		}
		r := resourceOf(doc)
		if "" == r.Key() || "|||" == r.Key() {
			continue
		}
		out[r.Key()] = r
	}
	return out, nil
}

func resourceOf(doc map[string]any) Resource {
	apiVersion, _ := doc["apiVersion"].(string)
	kind, _ := doc["kind"].(string)
	meta, _ := doc["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	return Resource{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		Body:       doc,
	}
}

// Compute walks two manifest sets and returns a list of Changes after
// applying the noise filters. Resources are matched by GVK+namespace+name.
func Compute(oldManifest, newManifest string, filters Filters) ([]Change, error) {
	oldRes, err := Parse(oldManifest)
	if nil != err {
		return nil, err
	}
	newRes, err := Parse(newManifest)
	if nil != err {
		return nil, err
	}

	keys := unionKeys(oldRes, newRes)
	sort.Strings(keys)

	out := make([]Change, 0)
	for _, k := range keys {
		oldR, hasOld := oldRes[k]
		newR, hasNew := newRes[k]

		switch {
		case !hasOld:
			out = append(out, Change{
				Key: k, Kind: ChangeAdd, Resource: newR,
				NewYAML: marshalSorted(filterNoise(newR.Body, filters)),
			})
		case !hasNew:
			out = append(out, Change{
				Key: k, Kind: ChangeRemove, Resource: oldR,
				OldYAML: marshalSorted(filterNoise(oldR.Body, filters)),
			})
		default:
			oldFiltered := filterNoise(oldR.Body, filters)
			newFiltered := filterNoise(newR.Body, filters)
			fields := computeFieldChanges("", oldFiltered, newFiltered)
			if 0 < len(fields) {
				out = append(out, Change{
					Key: k, Kind: ChangeModify, Resource: newR,
					OldYAML:   marshalSorted(oldFiltered),
					NewYAML:   marshalSorted(newFiltered),
					FieldDiff: fields,
				})
			}
		}
	}
	return out, nil
}

func unionKeys(a, b map[string]Resource) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}

// filterNoise removes fields the operator considers noise per `f`. It returns
// a deep-cloned tree; the input is not mutated.
func filterNoise(in map[string]any, f Filters) map[string]any {
	out := deepCloneMap(in)

	if !f.ShowStatus {
		delete(out, "status")
	}

	if meta, ok := out["metadata"].(map[string]any); ok {
		if !f.ShowManagedFields {
			delete(meta, "managedFields")
		}
		if !f.ShowGeneration {
			delete(meta, "generation")
			delete(meta, "resourceVersion")
			delete(meta, "uid")
			delete(meta, "creationTimestamp")
			delete(meta, "selfLink")
		}
		if !f.ShowAnnotations {
			delete(meta, "annotations")
		}
		if !f.ShowLabels {
			// Preserve labels that are part of the spec selector by keeping
			// 'app', 'component', 'name' but stripping controller-injected
			// ones. Conservative: clear all when flag is false.
			delete(meta, "labels")
		}
		if !f.ShowFinalizers {
			delete(meta, "finalizers")
		}
		if !f.ShowOwnerRefs {
			delete(meta, "ownerReferences")
		}
		if 0 == len(meta) {
			delete(out, "metadata")
		}
	}

	if !f.ShowDefaultedFields {
		stripDefaults(out)
	}
	if !f.ShowImagePullPolicy {
		stripImagePullPolicy(out)
	}
	if !f.ShowSecretRotation {
		stripSecretData(out)
	}
	return out
}

// stripDefaults removes fields that the API server commonly populates with
// defaults that vary across renders (Service.spec.clusterIP/clusterIPs,
// ports[].protocol when "TCP", revisionHistoryLimit, etc.).
func stripDefaults(o map[string]any) {
	kind, _ := o["kind"].(string)
	spec, _ := o["spec"].(map[string]any)
	if nil == spec {
		return
	}
	switch kind {
	case "Service":
		delete(spec, "clusterIP")
		delete(spec, "clusterIPs")
		delete(spec, "ipFamilies")
		delete(spec, "ipFamilyPolicy")
		delete(spec, "internalTrafficPolicy")
		delete(spec, "sessionAffinity")
		if ports, ok := spec["ports"].([]any); ok {
			for _, p := range ports {
				if pm, ok := p.(map[string]any); ok {
					if "TCP" == pm["protocol"] {
						delete(pm, "protocol")
					}
					delete(pm, "nodePort") // server-allocated
				}
			}
		}
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		delete(spec, "revisionHistoryLimit")
		delete(spec, "progressDeadlineSeconds")
		if strategy, ok := spec["strategy"].(map[string]any); ok {
			delete(strategy, "rollingUpdate")
		}
	}
}

func stripImagePullPolicy(o map[string]any) {
	walkContainers(o, func(c map[string]any) {
		delete(c, "imagePullPolicy")
		delete(c, "terminationMessagePath")
		delete(c, "terminationMessagePolicy")
	})
}

// stripSecretData removes string data values from Secrets. Charts that use
// genCA / genSelfSignedCert / randAlphaNum produce different bytes per
// render even when the resource intent has not changed.
func stripSecretData(o map[string]any) {
	if "Secret" != o["kind"] {
		return
	}
	if data, ok := o["data"].(map[string]any); ok {
		for k := range data {
			data[k] = "<redacted-by-smart-diff>"
		}
	}
	if data, ok := o["stringData"].(map[string]any); ok {
		for k := range data {
			data[k] = "<redacted-by-smart-diff>"
		}
	}
}

func walkContainers(o map[string]any, fn func(map[string]any)) {
	spec, _ := o["spec"].(map[string]any)
	if nil == spec {
		return
	}
	template, _ := spec["template"].(map[string]any)
	podSpec, _ := template["spec"].(map[string]any)
	if nil == podSpec {
		// CronJob: spec.jobTemplate.spec.template.spec
		jt, _ := spec["jobTemplate"].(map[string]any)
		if nil != jt {
			if jSpec, ok := jt["spec"].(map[string]any); ok {
				if jTemplate, ok := jSpec["template"].(map[string]any); ok {
					podSpec, _ = jTemplate["spec"].(map[string]any)
				}
			}
		}
	}
	if nil == podSpec {
		// Pod: spec is the pod spec directly.
		podSpec = spec
	}
	for _, k := range []string{"containers", "initContainers", "ephemeralContainers"} {
		if list, ok := podSpec[k].([]any); ok {
			for _, c := range list {
				if cm, ok := c.(map[string]any); ok {
					fn(cm)
				}
			}
		}
	}
}

func deepCloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCloneAny(v)
	}
	return out
}

func deepCloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCloneMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = deepCloneAny(e)
		}
		return out
	}
	return v
}

func computeFieldChanges(prefix string, a, b any) []FieldChange {
	if reflect.DeepEqual(a, b) {
		return nil
	}
	switch av := a.(type) {
	case map[string]any:
		bv, _ := b.(map[string]any)
		return mapDiff(prefix, av, bv)
	case []any:
		bv, _ := b.([]any)
		// Lists are compared as a whole if any difference exists.
		if reflect.DeepEqual(av, bv) {
			return nil
		}
		return []FieldChange{{Path: prefix, Old: av, New: bv}}
	default:
		return []FieldChange{{Path: prefix, Old: a, New: b}}
	}
}

func mapDiff(prefix string, a, b map[string]any) []FieldChange {
	out := make([]FieldChange, 0)
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		path := k
		if "" != prefix {
			path = prefix + "." + k
		}
		av, hasA := a[k]
		bv, hasB := b[k]
		switch {
		case !hasA:
			out = append(out, FieldChange{Path: path, Old: nil, New: bv})
		case !hasB:
			out = append(out, FieldChange{Path: path, Old: av, New: nil})
		default:
			out = append(out, computeFieldChanges(path, av, bv)...)
		}
	}
	return out
}

// marshalSorted serialises with deterministic key ordering — yaml.v3 does
// honour map iteration when keys are sorted upstream. We use a node-based
// approach for stable output.
func marshalSorted(m any) string {
	out, err := yaml.Marshal(sortKeys(m))
	if nil != err {
		return ""
	}
	return string(out)
}

func sortKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		// Build a sorted yaml.MapSlice-equivalent via *yaml.Node so output ordering is deterministic.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			valNode := toYAMLNode(sortKeys(x[k]))
			node.Content = append(node.Content, keyNode, valNode)
		}
		return node
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = sortKeys(e)
		}
		return out
	}
	return v
}

func toYAMLNode(v any) *yaml.Node {
	if n, ok := v.(*yaml.Node); ok {
		return n
	}
	out := &yaml.Node{}
	if err := out.Encode(v); nil != err {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
	}
	return out
}

// FormatHuman renders changes as a unified-style summary suitable for terminal output.
func FormatHuman(changes []Change, color bool) string {
	if 0 == len(changes) {
		return ""
	}
	var b strings.Builder
	for _, c := range changes {
		header := fmt.Sprintf("%s %s\n", c.Kind.String(), c.Key)
		if color {
			header = colorize(c.Kind, header)
		}
		b.WriteString(header)
		switch c.Kind {
		case ChangeAdd:
			b.WriteString(indent(c.NewYAML, "  + "))
		case ChangeRemove:
			b.WriteString(indent(c.OldYAML, "  - "))
		case ChangeModify:
			for _, f := range c.FieldDiff {
				b.WriteString(fmt.Sprintf("    %s\n      - %v\n      + %v\n", f.Path, f.Old, f.New))
			}
		}
	}
	return b.String()
}

func colorize(k ChangeKind, line string) string {
	const reset = "\033[0m"
	switch k {
	case ChangeAdd:
		return "\033[32m" + line + reset
	case ChangeRemove:
		return "\033[31m" + line + reset
	case ChangeModify:
		return "\033[33m" + line + reset
	}
	return line
}

func indent(s, prefix string) string {
	if "" == s {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}
