package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	"github.com/ebogdum/keramos/v2/internal/diff"
	"github.com/ebogdum/keramos/v2/internal/engine"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/layer"
	"github.com/ebogdum/keramos/v2/internal/pkg"
	"github.com/ebogdum/keramos/v2/internal/release"
	"github.com/ebogdum/keramos/v2/internal/values"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validatePlanPath rejects plan-supplied paths that are absolute or contain
// traversal sequences. Without this, a hostile plan file could trick `keramos
// apply` into reading any directory readable by the operator and rendering
// it as a keramos package — including /tmp, /etc, or user home directories.
func validatePlanPath(p string) error {
	if "" == p {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "plan packagePath is empty")
	}
	if filepath.IsAbs(p) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"plan packagePath %q must be relative; refusing to apply plan referencing absolute paths", p)
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"plan packagePath %q contains a traversal sequence", p)
	}
	return nil
}

// keramosPlan is the on-disk representation of an `keramos plan` invocation. Apply
// reads this back, verifies the package digest still matches, and runs the
// install/upgrade with the same rendered manifest.
type keramosPlan struct {
	APIVersion  string            `json:"apiVersion"`
	Kind        string            `json:"kind"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Action      string            `json:"action"`
	ReleaseName string            `json:"releaseName"`
	Namespace   string            `json:"namespace"`
	PackagePath string            `json:"packagePath"`
	Profile     string            `json:"profile,omitempty"`
	ValueFiles  []string          `json:"valueFiles,omitempty"`
	Sets        []string          `json:"sets,omitempty"`
	SetStrings  []string          `json:"setStrings,omitempty"`
	Manifest    string            `json:"manifest"`
	ManifestSHA string            `json:"manifestSha256"`
	Notes       string            `json:"notes,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func newPlanCommand() *cobra.Command {
	var (
		valueFiles  []string
		sets        []string
		setStrings  []string
		profile     string
		out         string
		actionKind  string
		labels      []string
		format      string
		noColor     bool
		releaseName string
	)
	cmd := &cobra.Command{
		Use:   "plan [package-path]",
		Short: "Preview what a package would change against the current state",
		Long: `Render a package and compare it against the current stored state — the
Terraform 'plan' for keramos. Point it at a directory (default '.') and it shows
what applying that directory would add, change, or destroy.

No release name is required: the release is derived from the package's
keramos.yaml 'name', and the comparison runs against the LATEST recorded state
for it. Use -r/--release only to target a state stored under a different name.

    keramos plan                 # compare ./ against its latest state
    keramos plan ./mychart       # compare ./mychart against its latest state
    keramos plan -r prod-web .    # compare ./ against the 'prod-web' state

The terminal view is a per-resource change preview (every field shown, so an
edited label is never hidden) ending in 'N to add, M to change, K to destroy'.
State lookup is best-effort — with no reachable cluster or no prior state,
every resource is reported as a create.

The machine-readable, apply-able JSON artifact (rendered manifest plus its
sha256 integrity digest) is written when --out names a file, or emitted to
stdout with --format json. 'keramos apply --plan <file>' consumes that JSON;
the human diff is a review view only.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if 1 == len(args) {
				dir = args[0]
			}
			labelMap, lerr := parseLabelFlags(labels)
			if nil != lerr {
				return lerr
			}
			// Validate the same way `keramos apply` will validate later, so a
			// plan we cannot apply is rejected up front rather than written
			// to disk and discovered hours later.
			if pErr := validatePlanPath(dir); nil != pErr {
				return pErr
			}
			for _, vf := range valueFiles {
				if pErr := validatePlanPath(vf); nil != pErr {
					return pErr
				}
			}
			if "install" != actionKind && "upgrade" != actionKind {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"unsupported --action %q; use 'install' or 'upgrade'", actionKind)
			}
			// Derive the release identity from the package unless the operator
			// pinned one with -r. The stored state is keyed by this name; the
			// diff runs against its latest revision.
			if "" == releaseName {
				meta, metaErr := pkg.LoadPackageMetadata(dir)
				if nil != metaErr {
					return metaErr
				}
				if "" == meta.Name {
					return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
						"package at %q has no 'name' in keramos.yaml; pass -r/--release to name the state", dir)
				}
				releaseName = meta.Name
			}
			rel, err := action.Install(nil, dir, &action.InstallOptions{
				ReleaseName: releaseName,
				Namespace:   namespace,
				ValueFiles:  valueFiles,
				Sets:        sets,
				SetStrings:  setStrings,
				Profile:     profile,
				DryRun:      "client",
				Labels:      labelMap,
			})
			if nil != err {
				return err
			}
			sum := sha256.Sum256([]byte(rel.Manifest))
			plan := keramosPlan{
				APIVersion:  "keramos/v1",
				Kind:        "Plan",
				GeneratedAt: time.Now().UTC(),
				Action:      actionKind,
				ReleaseName: releaseName,
				Namespace:   namespace,
				PackagePath: dir,
				Profile:     profile,
				ValueFiles:  valueFiles,
				Sets:        sets,
				SetStrings:  setStrings,
				Manifest:    rel.Manifest,
				ManifestSHA: hex.EncodeToString(sum[:]),
				Notes:       rel.Notes,
				Labels:      labelMap,
			}
			data, mErr := json.MarshalIndent(plan, "", "  ")
			if nil != mErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "marshal plan", mErr)
			}
			// A named --out always receives the machine-readable JSON
			// artifact that `keramos apply` verifies and applies, regardless of
			// --format: the artifact must stay parseable.
			if "" != out && "-" != out {
				if writeErr := os.WriteFile(out, data, 0o600); nil != writeErr {
					return keramoserr.WrapError(keramoserr.ErrInternal, "write plan", writeErr)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "plan written to %s\n", out)
				return nil
			}
			// Stdout: JSON when explicitly requested (pipelines redirect this
			// into an apply-able file); otherwise the human review diff.
			switch format {
			case "json":
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			case "text", "":
				// Provenance is part of the basic plan, not an opt-in: build
				// it best-effort so a changed field can show where its new
				// value came from. A build failure never sinks the plan.
				prov, _ := buildPlanProvenance(dir, profile, valueFiles, sets, setStrings)
				return renderPlanDiff(cmd, &plan, actionKind, !noColor, prov)
			default:
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"invalid --format %q; use 'text' or 'json'", format)
			}
		},
	}
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "key=value forced as string (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().StringVarP(&releaseName, "release", "r", "", "state to compare against (default: derived from keramos.yaml name, latest revision)")
	cmd.Flags().StringVar(&actionKind, "action", "install", "action the plan represents: install or upgrade")
	cmd.Flags().StringVarP(&out, "out", "o", "-", "write the JSON plan artifact to this file (- for stdout)")
	cmd.Flags().StringVar(&format, "format", "text", "stdout format when not writing a file: text (change preview) or json (apply-able artifact)")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored diff output")
	cmd.Flags().StringArrayVar(&labels, "labels", nil, "label key=value (repeatable)")
	return cmd
}

// renderPlanDiff prints a Terraform-style change preview for a freshly
// rendered plan: the plan's manifest diffed against the last stored release,
// followed by an add/change/destroy summary. State lookup is best-effort so
// the preview still works offline — an unreachable cluster or a first-time
// release both degrade to "every resource is a create".
func renderPlanDiff(cmd *cobra.Command, plan *keramosPlan, actionKind string, color bool, prov *planProvenance) error {
	base, verb, note := planBaseManifest(plan)
	return writePlanDiff(cmd.OutOrStdout(), plan, actionKind, base, verb, note, color, prov)
}

// writePlanDiff renders the change preview for a plan given the manifest to
// diff against. Split from state fetching so the diff rendering — which must
// never hide an operator's edit — is exercised directly in tests.
func writePlanDiff(w io.Writer, plan *keramosPlan, actionKind, base, verb, note string, color bool, prov *planProvenance) error {
	fmt.Fprintf(w, "keramos plan: %s  %s", verb, plan.ReleaseName)
	if "" != plan.Namespace {
		fmt.Fprintf(w, " / %s", plan.Namespace)
	}
	fmt.Fprintf(w, "  (package %s)\n", plan.PackagePath)
	if "" != note {
		fmt.Fprintf(w, "  note: %s\n", note)
	}
	fmt.Fprintln(w)

	// Show every field: both sides are keramos-rendered manifests, so the smart
	// diff's noise filters (built for live-cluster churn) could only erase the
	// operator's own edits — a changed label must never vanish from a plan.
	changes, dErr := diff.Compute(base, plan.Manifest, allShownFilters())
	if nil != dErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "compute plan diff", dErr)
	}
	if 0 == len(changes) {
		fmt.Fprintln(w, "No changes. Rendered package matches the stored release.")
		return nil
	}

	fmt.Fprint(w, formatPlanChanges(changes, color, prov, "state"))

	var add, change, destroy int
	for _, c := range changes {
		switch c.Kind {
		case diff.ChangeAdd:
			add++
		case diff.ChangeModify:
			change++
		case diff.ChangeRemove:
			destroy++
		}
	}
	fmt.Fprintf(w, "\nPlan: %d to add, %d to change, %d to destroy.\n", add, change, destroy)
	fmt.Fprintf(w, "\nApply with 'keramos plan %s %s --action %s --out plan.json && keramos apply --plan plan.json',\n",
		plan.ReleaseName, plan.PackagePath, actionKind)
	fmt.Fprintln(w, "or re-run with --format json to capture the apply-able artifact.")
	return nil
}

// planBaseManifest returns the manifest to diff the plan against, a short verb
// describing the transition, and an optional operator-facing note. It reads
// the last stored release for the plan's release name; any failure (no
// cluster, no prior release) yields an empty base so the plan reads as a fresh
// create rather than erroring — plan must remain usable without a cluster.
func planBaseManifest(plan *keramosPlan) (base, verb, note string) {
	client, err := kube.NewClient(kubeconfig, kubeContext, plan.Namespace)
	if nil != err {
		return "", "create", "no reachable cluster; showing every resource as a create"
	}
	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
	current, err := storage.Last(plan.ReleaseName)
	if nil != err {
		var he *keramoserr.KeramosError
		if errors.As(err, &he) && keramoserr.ErrReleaseNotFound == he.Type {
			return "", "create", ""
		}
		return "", "create", fmt.Sprintf("could not read stored release (%v); showing every resource as a create", err)
	}
	return current.Manifest, "update", ""
}

// ANSI colors for the plan diff, matched to Terraform's create/update/destroy
// conventions: green add, yellow change, red destroy.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
)

// valueRefRe matches a values.<dotted.path> reference inside a ${...}
// expression in a template, so we can tie a resource back to the value keys it
// consumes.
var valueRefRe = regexp.MustCompile(`values\.([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)

// planProvenance answers "where in the package does this come from" for a plan:
// which template file emitted each resource, the exact template document that
// produced it (so a changed field can be traced to its ${values.x}), and where
// each value was resolved from.
type planProvenance struct {
	resourceFile map[string]string // resource key -> source template file
	resourceDoc  map[string]string // resource key -> raw (pre-render) template doc
	trace        values.Trace      // value key -> resolution chain
}

// buildPlanProvenance renders the package one template document at a time (so
// each resource can be attributed to its file AND its pre-render source doc)
// and captures the value resolution trace. Offline — no cluster access.
func buildPlanProvenance(dir, profile string, valueFiles, sets, setStrings []string) (*planProvenance, error) {
	resolved, err := layer.Resolve(dir, profile)
	if nil != err {
		return nil, err
	}
	merged, trace, err := values.ResolveAllWithTrace(map[string]any(resolved.Values), valueFiles, sets, setStrings, nil, nil)
	if nil != err {
		return nil, err
	}
	name, ver, appVer := "", "", ""
	if nil != resolved.Metadata {
		name, ver, appVer = resolved.Metadata.Name, resolved.Metadata.Version, resolved.Metadata.AppVersion
	}
	ctx := &engine.RenderContext{
		Values:       merged,
		Package:      map[string]any{"name": name, "version": ver, "appVersion": appVer},
		Release:      map[string]any{"name": name, "namespace": namespace, "revision": 1, "isInstall": true, "isUpgrade": false},
		Capabilities: map[string]any{},
		Files:        resolved.Files,
	}
	eng := engine.New()
	prov := &planProvenance{resourceFile: map[string]string{}, resourceDoc: map[string]string{}, trace: trace}
	for tname, content := range resolved.Templates {
		base := tname
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		if strings.HasPrefix(base, "_") {
			continue // partial
		}
		// Render each source document on its own so we keep the raw doc that
		// produced each resource — that raw doc still holds the ${values.x}
		// references a rendered manifest has already substituted away.
		for _, raw := range splitYAMLDocsRaw(content) {
			if "" == strings.TrimSpace(raw) {
				continue
			}
			docs, derr := eng.RenderFile(tname, raw, resolved.Partials, ctx)
			if nil != derr {
				continue // best-effort
			}
			for _, d := range docs {
				parsed, perr := diff.Parse(d)
				if nil != perr {
					continue
				}
				for k := range parsed {
					prov.resourceFile[k] = tname
					prov.resourceDoc[k] = raw
				}
			}
		}
	}
	return prov, nil
}

// splitYAMLDocsRaw splits template content into raw documents on `---` markers,
// preserving each doc's source text (unlike a YAML decode, which discards it).
func splitYAMLDocsRaw(content string) []string {
	var docs []string
	var cur []string
	for _, ln := range strings.Split(content, "\n") {
		if "---" == strings.TrimSpace(ln) {
			docs = append(docs, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, ln)
	}
	docs = append(docs, strings.Join(cur, "\n"))
	return docs
}

// fieldOrigin traces a changed manifest field back to the ${values.x} in its
// source template doc and reports where that value was resolved from. Empty
// when the field is not a direct value substitution (computed, control-flow,
// or inside an array the diff engine reports whole).
func fieldOrigin(prov *planProvenance, resourceKey, path string) string {
	if nil == prov {
		return ""
	}
	raw, ok := prov.resourceDoc[resourceKey]
	if !ok {
		return ""
	}
	var doc map[string]any
	if yaml.Unmarshal([]byte(raw), &doc) != nil {
		return ""
	}
	node := walkYAMLPath(doc, strings.Split(path, "."))
	s, ok := node.(string)
	if !ok {
		return ""
	}
	m := valueRefRe.FindStringSubmatch(s)
	if nil == m {
		return ""
	}
	return valueOrigin(prov.trace, m[1])
}

// walkYAMLPath follows a dotted path (numeric segments index into arrays)
// through a decoded YAML tree, returning nil when the path does not resolve.
func walkYAMLPath(cur any, segs []string) any {
	for _, seg := range segs {
		switch c := cur.(type) {
		case map[string]any:
			cur = c[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if nil != err || idx < 0 || idx >= len(c) {
				return nil
			}
			cur = c[idx]
		default:
			return nil
		}
		if nil == cur {
			return nil
		}
	}
	return cur
}

// valueOrigin describes where a value key (or its leaves) was resolved from,
// as "source (origin)" pairs — e.g. "set (replicas=9)" or "values-file (prod.yaml)".
func valueOrigin(trace values.Trace, ref string) string {
	seen := map[string]bool{}
	var origins []string
	for key, steps := range trace {
		if (key == ref || strings.HasPrefix(key, ref+".")) && 0 < len(steps) {
			s := steps[len(steps)-1]
			tag := fmt.Sprintf("%s (%s)", s.Source, s.Origin)
			if !seen[tag] {
				seen[tag] = true
				origins = append(origins, tag)
			}
		}
	}
	sort.Strings(origins)
	return strings.Join(origins, ", ")
}

// templateLine names the source template file for a resource, empty when
// provenance is unavailable or the resource is not from the package.
func templateLine(prov *planProvenance, key string) string {
	if nil == prov {
		return ""
	}
	if file, ok := prov.resourceFile[key]; ok {
		return "      from: " + file + "\n"
	}
	return ""
}

// allShownFilters returns diff filters with every field shown. Used wherever
// both sides of a comparison are keramos-rendered manifests (plan, file diff) —
// there is no live-cluster noise to suppress, so hiding anything would only
// mask a real change the operator made.
func allShownFilters() diff.Filters {
	return diff.Filters{
		ShowStatus:          true,
		ShowManagedFields:   true,
		ShowGeneration:      true,
		ShowDefaultedFields: true,
		ShowAnnotations:     true,
		ShowLabels:          true,
		ShowImagePullPolicy: true,
		ShowFinalizers:      true,
		ShowOwnerRefs:       true,
		ShowSecretRotation:  true,
	}
}

// changeSummary returns a trailing count line for a set of changes.
func changeSummary(changes []diff.Change) string {
	var add, mod, rem int
	for _, c := range changes {
		switch c.Kind {
		case diff.ChangeAdd:
			add++
		case diff.ChangeModify:
			mod++
		case diff.ChangeRemove:
			rem++
		}
	}
	return fmt.Sprintf("\nSummary: %d added, %d changed, %d removed.\n", add, mod, rem)
}

// formatPlanChanges renders resource changes in a plan-specific, human-first
// layout — 'update Kind/name (namespace ns)' headers and inline
// 'field: old → new' lines — rather than the diff package's pipe-keyed,
// live-cluster-oriented format. Kept local to the plan command so `keramos diff`
// output is unaffected.
func formatPlanChanges(changes []diff.Change, color bool, prov *planProvenance, oldLabel string) string {
	var b strings.Builder
	for _, c := range changes {
		switch c.Kind {
		case diff.ChangeAdd:
			b.WriteString(planHeader("+", "create", c.Resource, ansiGreen, color))
			b.WriteString(templateLine(prov, c.Key))
			b.WriteString(indentLines(c.NewYAML, "      "))
		case diff.ChangeRemove:
			b.WriteString(planHeader("-", "destroy", c.Resource, ansiRed, color))
			if "" != oldLabel {
				b.WriteString("      (present in " + oldLabel + ", absent from package)\n")
			}
		case diff.ChangeModify:
			b.WriteString(planHeader("~", "update", c.Resource, ansiYellow, color))
			b.WriteString(templateLine(prov, c.Key))
			for _, f := range c.FieldDiff {
				b.WriteString(formatFieldChange(f, fieldOrigin(prov, c.Key, f.Path), oldLabel))
			}
		}
	}
	return b.String()
}

// planHeader renders one resource header line, e.g. "~ update  Service/zephyra
// (namespace apps)". Namespace is omitted when empty (cluster-scoped objects).
func planHeader(symbol, verb string, r diff.Resource, colorCode string, color bool) string {
	ident := r.Kind + "/" + r.Name
	if "" != r.Namespace {
		ident += "  (namespace " + r.Namespace + ")"
	}
	line := fmt.Sprintf("%s %-7s %s\n", symbol, verb, ident)
	if color {
		return colorCode + line + ansiReset
	}
	return line
}

// formatFieldChange renders one field edit line-by-line: the path, the current
// value (from state), and the new value with its origin appended when known.
//
//	~ spec.replicas
//	    - 1              (state)
//	    + 7              ← set (replicas=7)
func formatFieldChange(f diff.FieldChange, origin, oldLabel string) string {
	oldV := planValue(f.Old)
	newV := planValue(f.New)
	oldSuffix := ""
	if "" != oldLabel {
		oldSuffix = "   (" + oldLabel + ")"
	}
	newSuffix := ""
	if "" != origin {
		newSuffix = "   ← " + origin
	}
	var b strings.Builder
	b.WriteString("      ~ " + f.Path + "\n")
	if strings.Contains(oldV, "\n") || strings.Contains(newV, "\n") {
		b.WriteString(indentLines(oldV, "          - "))
		if "" != origin {
			b.WriteString("          (new value ← " + origin + ")\n")
		}
		b.WriteString(indentLines(newV, "          + "))
		return b.String()
	}
	b.WriteString("          - " + oldV + oldSuffix + "\n")
	b.WriteString("          + " + newV + newSuffix + "\n")
	return b.String()
}

// planValue stringifies a field value for display: nil as "null", strings
// quoted so empty/whitespace edits are visible, everything else via %v.
func planValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("%q", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// indentLines prefixes every line of s; returns "" for empty input.
func indentLines(s, prefix string) string {
	if "" == s {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}

func newApplyCommand() *cobra.Command {
	var (
		planFile string
		dryRun   string
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a plan produced by 'keramos plan'",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if "" == planFile {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "--plan is required")
			}
			data, err := os.ReadFile(planFile)
			if nil != err {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "read plan", err)
			}
			var p keramosPlan
			if jErr := json.Unmarshal(data, &p); nil != jErr {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse plan", jErr)
			}
			if "keramos/v1" != p.APIVersion || "Plan" != p.Kind {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"unsupported plan kind %s/%s", p.APIVersion, p.Kind)
			}
			if "install" != p.Action && "upgrade" != p.Action {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"plan declares unsupported action %q; expected install or upgrade", p.Action)
			}
			if pErr := validatePlanPath(p.PackagePath); nil != pErr {
				return pErr
			}
			for _, vf := range p.ValueFiles {
				if pErr := validatePlanPath(vf); nil != pErr {
					return pErr
				}
			}
			client, kErr := kube.NewClient(kubeconfig, kubeContext, p.Namespace)
			if nil != kErr {
				return kErr
			}
			// Plan integrity: re-render with a client-side dry-run and verify
			// the manifest hash still matches the plan. Detects drift between
			// plan and apply caused by package or value-file edits.
			preview, prevErr := action.Install(nil, p.PackagePath, &action.InstallOptions{
				ReleaseName: p.ReleaseName,
				Namespace:   p.Namespace,
				ValueFiles:  p.ValueFiles,
				Sets:        p.Sets,
				SetStrings:  p.SetStrings,
				Profile:     p.Profile,
				DryRun:      "client",
				Labels:      p.Labels,
			})
			if nil != prevErr {
				return prevErr
			}
			if "" == p.ManifestSHA {
				return keramoserr.NewError(keramoserr.ErrCLIValidation,
					"plan is missing manifestSha256 integrity field; refusing to apply")
			}
			gotSum := sha256.Sum256([]byte(preview.Manifest))
			if p.ManifestSHA != hex.EncodeToString(gotSum[:]) {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"plan integrity check failed: package or values changed since plan was generated (expected sha %s, got %s)",
					p.ManifestSHA, hex.EncodeToString(gotSum[:]))
			}
			switch p.Action {
			case "upgrade":
				rel, uErr := action.Upgrade(client, p.PackagePath, &action.UpgradeOptions{
					ReleaseName: p.ReleaseName,
					Namespace:   p.Namespace,
					ValueFiles:  p.ValueFiles,
					Sets:        p.Sets,
					SetStrings:  p.SetStrings,
					Profile:     p.Profile,
					Atomic:      true,
					Wait:        true,
					Timeout:     5 * time.Minute,
					DryRun:      dryRun,
					Labels:      p.Labels,
				})
				if nil != uErr {
					return uErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied upgrade for %s revision %d\n", rel.Name, rel.Revision)
			default:
				rel, iErr := action.Install(client, p.PackagePath, &action.InstallOptions{
					ReleaseName: p.ReleaseName,
					Namespace:   p.Namespace,
					ValueFiles:  p.ValueFiles,
					Sets:        p.Sets,
					SetStrings:  p.SetStrings,
					Profile:     p.Profile,
					Atomic:      true,
					Wait:        true,
					Timeout:     5 * time.Minute,
					DryRun:      dryRun,
					Labels:      p.Labels,
				})
				if nil != iErr {
					return iErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied install for %s revision %d\n", rel.Name, rel.Revision)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&planFile, "plan", "", "plan file produced by 'keramos plan'")
	cmd.Flags().StringVar(&dryRun, "dry-run", "", "dry-run mode: client or server")
	return cmd
}
