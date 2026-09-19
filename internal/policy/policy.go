// Package policy is keramos's embedded policy engine. It loads rules from a
// `policies/` directory in the package and evaluates them against rendered
// manifests at install/upgrade/template time.
//
// The DSL is intentionally simpler than OPA/Rego: each rule is a YAML
// document declaring a `match` selector (kind, namespace, label, etc.) and
// one or more `require:` predicates. Failed rules surface as errors that
// abort the operation by default; `severity: warn` rules log instead.
//
// Built-in predicates handle the 80% of policies operators want without a
// scripting language: required-fields, forbidden-fields, image registry
// allowlist, resource-limit minimums, label requirements.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"gopkg.in/yaml.v3"
)

// Severity controls whether a policy violation aborts (deny) or logs (warn).
type Severity string

const (
	SeverityDeny Severity = "deny"
	SeverityWarn Severity = "warn"
)

// Rule is a single policy.
type Rule struct {
	Name     string   `yaml:"name"`
	Severity Severity `yaml:"severity,omitempty"`
	Match    Match    `yaml:"match,omitempty"`
	Require  Require  `yaml:"require,omitempty"`
	Forbid   Require  `yaml:"forbid,omitempty"`
	Message  string   `yaml:"message,omitempty"`
}

// Match selects which manifest documents a rule applies to. Empty fields
// are wildcards.
type Match struct {
	Kinds      []string `yaml:"kinds,omitempty"`
	Namespaces []string `yaml:"namespaces,omitempty"`
	Names      []string `yaml:"names,omitempty"`
	APIVersion string   `yaml:"apiVersion,omitempty"`
}

// Require declares predicates that must be true for matching documents.
// All predicates use a dotted path into the manifest body.
type Require struct {
	// Fields lists JSON-pointer-like paths that must exist and be non-empty.
	Fields []string `yaml:"fields,omitempty"`
	// LabelKeys: every named label key must be present and non-empty.
	LabelKeys []string `yaml:"labelKeys,omitempty"`
	// AnnotationKeys: same for annotations.
	AnnotationKeys []string `yaml:"annotationKeys,omitempty"`
	// ImageRegistries: container images must match one of these registry
	// hosts (e.g. ["registry.internal", "ghcr.io/myorg"]).
	ImageRegistries []string `yaml:"imageRegistries,omitempty"`
	// ImageNotTagged: when true, images must not use ":latest" or no-tag form.
	ImageNotTagged bool `yaml:"imageNotTagged,omitempty"`
	// ResourceRequests / ResourceLimits: when true, every container must
	// declare both requests and limits (cpu+memory).
	ResourceRequests bool `yaml:"resourceRequests,omitempty"`
	ResourceLimits   bool `yaml:"resourceLimits,omitempty"`
	// MinReplicas: when >0, Deployments/StatefulSets must have spec.replicas >= this.
	MinReplicas int `yaml:"minReplicas,omitempty"`
}

// Violation is a single failed rule against a single document.
type Violation struct {
	Rule       string
	Severity   Severity
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Detail     string
}

// LoadRules reads every YAML file under <packagePath>/policies/. Multi-doc
// YAML files are supported. Returns an empty slice (not nil error) when the
// directory does not exist.
func LoadRules(packagePath string) ([]Rule, error) {
	dir := filepath.Join(packagePath, "policies")
	info, err := os.Stat(dir)
	if nil != err {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "stat policies dir", err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	out := make([]Rule, 0)
	walkErr := filepath.Walk(dir, func(path string, fi os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		if fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ".yaml" != ext && ".yml" != ext {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, readErr, "read policy %s", path)
		}
		if maxPolicyBytes < len(data) {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"policy file %s is %d bytes, exceeding the %d-byte limit", path, len(data), maxPolicyBytes)
		}
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		dec.KnownFields(true)
		for {
			var r Rule
			err := dec.Decode(&r)
			if nil != err {
				if "EOF" == err.Error() {
					break
				}
				return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "parse policy %s", path)
			}
			if "" == r.Name {
				continue
			}
			if "" == string(r.Severity) {
				r.Severity = SeverityDeny
			}
			// Fail closed on an unrecognized/typo'd severity ("deney", "block").
			// Otherwise HasDeny would treat it as non-blocking and let a
			// violation the author intended as a hard deny pass silently.
			if SeverityDeny != r.Severity && SeverityWarn != r.Severity {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"policy %q has unknown severity %q (use 'deny' or 'warn')", r.Name, r.Severity)
			}
			out = append(out, r)
		}
		return nil
	})
	if nil != walkErr {
		return nil, walkErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Size caps bound the memory cost of parsing adversarial YAML (alias bombs,
// deeply nested structures). Two orders of magnitude above any realistic
// keramos policy file or rendered manifest.
const (
	maxPolicyBytes   = 1 * 1024 * 1024
	maxManifestBytes = 16 * 1024 * 1024
)

// Evaluate runs every rule against every document parsed from `manifest`,
// returning the list of violations in deterministic order.
func Evaluate(rules []Rule, manifest string) ([]Violation, error) {
	if maxManifestBytes < len(manifest) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"manifest is %d bytes, exceeding policy evaluator limit of %d",
			len(manifest), maxManifestBytes)
	}
	docs := make([]map[string]any, 0)
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	for {
		var d map[string]any
		err := dec.Decode(&d)
		if nil != err {
			if "EOF" == err.Error() {
				break
			}
			return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse manifest", err)
		}
		if 0 < len(d) {
			docs = append(docs, d)
		}
	}
	violations := make([]Violation, 0)
	for _, r := range rules {
		for _, doc := range docs {
			if !matches(r.Match, doc) {
				continue
			}
			vs := evalRule(r, doc)
			violations = append(violations, vs...)
		}
	}
	return violations, nil
}

// HasDeny returns true when any violation is deny-severity (and would abort).
func HasDeny(vs []Violation) bool {
	for _, v := range vs {
		if SeverityDeny == v.Severity {
			return true
		}
	}
	return false
}

// FormatHuman renders violations grouped by severity for the operator.
func FormatHuman(vs []Violation) string {
	if 0 == len(vs) {
		return ""
	}
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "[%s] %s — %s/%s/%s in ns %s: %s\n",
			strings.ToUpper(string(v.Severity)), v.Rule, v.APIVersion, v.Kind, v.Name, v.Namespace, v.Detail)
	}
	return b.String()
}

func matches(m Match, doc map[string]any) bool {
	if 0 < len(m.Kinds) && !contains(m.Kinds, fmt.Sprintf("%v", doc["kind"])) {
		return false
	}
	if "" != m.APIVersion && m.APIVersion != fmt.Sprintf("%v", doc["apiVersion"]) {
		return false
	}
	meta, _ := doc["metadata"].(map[string]any)
	if 0 < len(m.Namespaces) && !contains(m.Namespaces, fmt.Sprintf("%v", meta["namespace"])) {
		return false
	}
	if 0 < len(m.Names) && !contains(m.Names, fmt.Sprintf("%v", meta["name"])) {
		return false
	}
	return true
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func evalRule(r Rule, doc map[string]any) []Violation {
	out := make([]Violation, 0)
	apiVer, _ := doc["apiVersion"].(string)
	kind, _ := doc["kind"].(string)
	meta, _ := doc["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	ns, _ := meta["namespace"].(string)

	emit := func(detail string) {
		msg := detail
		if "" != r.Message {
			msg = r.Message + " (" + detail + ")"
		}
		out = append(out, Violation{
			Rule: r.Name, Severity: r.Severity,
			APIVersion: apiVer, Kind: kind, Namespace: ns, Name: name,
			Detail: msg,
		})
	}

	for _, path := range r.Require.Fields {
		// Require = present and non-empty at EVERY position the path reaches,
		// including every element when it crosses an array. Any-semantics was a
		// fail-open: a rule requiring securityContext.runAsNonRoot passed when
		// only one of several containers set it.
		if !requireFieldSatisfied(doc, path) {
			emit(fmt.Sprintf("required field %q is missing or empty", path))
		}
	}
	for _, path := range r.Forbid.Fields {
		if anyNonZero(collectDotted(doc, path)) {
			emit(fmt.Sprintf("forbidden field %q is present", path))
		}
	}
	for _, k := range r.Require.LabelKeys {
		labels, _ := meta["labels"].(map[string]any)
		if v, ok := labels[k]; !ok || isZero(v) {
			emit(fmt.Sprintf("required label %q is missing", k))
		}
	}
	for _, k := range r.Require.AnnotationKeys {
		annotations, _ := meta["annotations"].(map[string]any)
		if v, ok := annotations[k]; !ok || isZero(v) {
			emit(fmt.Sprintf("required annotation %q is missing", k))
		}
	}

	containers := extractContainers(doc)
	if 0 < len(r.Require.ImageRegistries) {
		for _, c := range containers {
			img, _ := c["image"].(string)
			ok := false
			for _, allowed := range r.Require.ImageRegistries {
				if imageMatchesRegistry(img, allowed) {
					ok = true
					break
				}
			}
			if !ok {
				emit(fmt.Sprintf("container image %q not from allowed registries %v", img, r.Require.ImageRegistries))
			}
		}
	}
	if r.Require.ImageNotTagged {
		for _, c := range containers {
			img, _ := c["image"].(string)
			if imageIsUnpinned(img) {
				emit(fmt.Sprintf("container image %q uses :latest or no tag", img))
			}
		}
	}
	if r.Require.ResourceRequests {
		for _, c := range containers {
			res, _ := c["resources"].(map[string]any)
			req, _ := res["requests"].(map[string]any)
			if 0 == len(req) {
				name, _ := c["name"].(string)
				emit(fmt.Sprintf("container %q has no resources.requests", name))
			}
		}
	}
	if r.Require.ResourceLimits {
		for _, c := range containers {
			res, _ := c["resources"].(map[string]any)
			lim, _ := res["limits"].(map[string]any)
			if 0 == len(lim) {
				name, _ := c["name"].(string)
				emit(fmt.Sprintf("container %q has no resources.limits", name))
			}
		}
	}
	if 0 < r.Require.MinReplicas {
		if "Deployment" == kind || "StatefulSet" == kind || "ReplicaSet" == kind {
			spec, _ := doc["spec"].(map[string]any)
			f, ok := numericValue(spec["replicas"])
			if !ok || int(f) < r.Require.MinReplicas {
				emit(fmt.Sprintf("spec.replicas (%v) is below minReplicas %d", spec["replicas"], r.Require.MinReplicas))
			}
		}
	}
	return out
}

// extractContainers gathers every container from common workload kinds.
func extractContainers(doc map[string]any) []map[string]any {
	spec, _ := doc["spec"].(map[string]any)
	var podSpec map[string]any
	switch doc["kind"] {
	case "Pod":
		podSpec = spec
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "ReplicaSet":
		if t, ok := spec["template"].(map[string]any); ok {
			podSpec, _ = t["spec"].(map[string]any)
		}
	case "CronJob":
		if jt, ok := spec["jobTemplate"].(map[string]any); ok {
			if js, ok := jt["spec"].(map[string]any); ok {
				if t, ok := js["template"].(map[string]any); ok {
					podSpec, _ = t["spec"].(map[string]any)
				}
			}
		}
	}
	if nil == podSpec {
		return nil
	}
	out := make([]map[string]any, 0)
	for _, k := range []string{"containers", "initContainers", "ephemeralContainers"} {
		if list, ok := podSpec[k].([]any); ok {
			for _, c := range list {
				if cm, ok := c.(map[string]any); ok {
					out = append(out, cm)
				}
			}
		}
	}
	return out
}

// collectDotted resolves a dotted path, descending through BOTH maps and
// slices. When a path element lands on a slice (e.g. `spec.template.spec.
// containers` in a workload), the remaining path is applied to every element,
// so `spec.template.spec.containers.securityContext.privileged` collects that
// field from every container. It returns each present leaf value.
//
// The previous map-only walk returned "not found" the moment a path crossed a
// slice, which silently turned every array-crossing forbid rule into a no-op —
// an operator's `forbid: privileged` was never enforced. Traversing slices
// closes that fail-open.
func collectDotted(node any, path string) []any {
	if "" == path {
		return []any{node}
	}
	head, rest, _ := strings.Cut(path, ".")
	switch n := node.(type) {
	case map[string]any:
		v, ok := n[head]
		if !ok {
			return nil
		}
		return collectDotted(v, rest)
	case []any:
		// The slice sits between path segments: re-apply the *current* path to
		// each element rather than consuming a segment.
		var out []any
		for _, elem := range n {
			out = append(out, collectDotted(elem, path)...)
		}
		return out
	}
	return nil
}

// requireFieldSatisfied reports whether a dotted path is present and non-empty
// at EVERY position it reaches. When the path crosses a slice, all elements
// must satisfy it (and an empty slice fails). Unlike anyNonZero, a present
// scalar counts even when it is the zero value for its type (e.g. bool false or
// int 0) — require checks presence, not truthiness — so a required boolean
// explicitly set to false is not misreported as missing.
func requireFieldSatisfied(node any, path string) bool {
	if "" == path {
		return requireLeafPresent(node)
	}
	head, rest, _ := strings.Cut(path, ".")
	switch n := node.(type) {
	case map[string]any:
		v, ok := n[head]
		if !ok {
			return false
		}
		return requireFieldSatisfied(v, rest)
	case []any:
		if 0 == len(n) {
			return false
		}
		for _, elem := range n {
			if !requireFieldSatisfied(elem, path) {
				return false
			}
		}
		return true
	}
	return false
}

// requireLeafPresent reports whether a leaf value counts as present-and-non-empty
// for a require check. Empty string / nil / empty collection fail; any other
// scalar (including bool false and 0) passes.
func requireLeafPresent(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return "" != x
	case []any:
		return 0 < len(x)
	case map[string]any:
		return 0 < len(x)
	default:
		return true
	}
}

// anyNonZero reports whether any collected value is present and non-zero. Used
// so forbid field checks work for scalar paths and for paths that fan out
// across slices.
func anyNonZero(vals []any) bool {
	for _, v := range vals {
		if !isZero(v) {
			return true
		}
	}
	return false
}

func isZero(v any) bool {
	if nil == v {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return "" == rv.String()
	case reflect.Slice, reflect.Map:
		return 0 == rv.Len()
	case reflect.Bool:
		return !rv.Bool()
	}
	return false
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// imageMatchesRegistry reports whether an image reference belongs to an allowed
// registry/repo prefix, with a component boundary so "registry.internal" does
// NOT match "registry.internal.attacker.com". The prefix must be the whole
// reference or be followed by '/' (path) or ':' (port/tag).
func imageMatchesRegistry(img, allowed string) bool {
	return img == allowed ||
		strings.HasPrefix(img, allowed+"/") ||
		strings.HasPrefix(img, allowed+":")
}

// imageIsUnpinned reports whether an image reference lacks an explicit version
// (no tag, or the :latest tag). A digest-pinned reference (name@sha256:...) is
// considered pinned. A ':' from a registry port (localhost:5000/img) is not
// mistaken for a tag — only a ':' in the final path component counts.
func imageIsUnpinned(img string) bool {
	if strings.Contains(img, "@") {
		return false // digest-pinned
	}
	name := img
	if i := strings.LastIndex(img, "/"); i >= 0 {
		name = img[i+1:]
	}
	tag := ""
	if i := strings.LastIndex(name, ":"); i >= 0 {
		tag = name[i+1:]
	}
	return "" == tag || "latest" == tag
}
