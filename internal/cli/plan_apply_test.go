package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebogdum/keramos/v3/internal/diff"
)

// makePlanTestPackage writes a minimal renderable package into a temp dir,
// chdirs into it (plan requires a relative package path), and returns "." as
// the path to plan. The original working directory is restored on cleanup.
func makePlanTestPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must := func(name, body string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); nil != err {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); nil != err {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	must("keramos.yaml", "apiVersion: keramos/v1\nname: planme\nversion: 1.0.0\n")
	must("values.yaml", "image: nginx:1.25\nreplicas: 1\n")
	must("templates/deployment.yaml", ""+
		"apiVersion: apps/v1\n"+
		"kind: Deployment\n"+
		"metadata:\n"+
		"  name: planme\n"+
		"spec:\n"+
		"  replicas: ${values.replicas}\n"+
		"  template:\n"+
		"    spec:\n"+
		"      containers:\n"+
		"        - name: app\n"+
		"          image: \"${values.image}\"\n")

	origWD, err := os.Getwd()
	if nil != err {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); nil != err {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	return "."
}

func runPlan(t *testing.T, pkg string, flags map[string]string, args ...string) (string, error) {
	t.Helper()
	cmd := newPlanCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); nil != err {
			t.Fatalf("set flag %s=%s: %v", k, v, err)
		}
	}
	err := cmd.RunE(cmd, append([]string{pkg}, args...))
	return buf.String(), err
}

// TestPlanJSONArtifactStaysApplyable is the apply-safety contract: whatever the
// human-facing default becomes, --format json must still emit a Plan document
// whose manifest is intact and whose manifestSha256 matches it, because
// `keramos apply` re-verifies that digest before touching the cluster.
func TestPlanJSONArtifactStaysApplyable(t *testing.T) {
	pkg := makePlanTestPackage(t)
	out, err := runPlan(t, pkg, map[string]string{"format": "json"})
	if nil != err {
		t.Fatalf("plan --format json: %v\n%s", err, out)
	}
	var p keramosPlan
	if jErr := json.Unmarshal([]byte(out), &p); nil != jErr {
		t.Fatalf("output is not valid JSON: %v\n%s", jErr, out)
	}
	if "keramos/v1" != p.APIVersion || "Plan" != p.Kind {
		t.Fatalf("unexpected kind %s/%s", p.APIVersion, p.Kind)
	}
	if !strings.Contains(p.Manifest, "kind: Deployment") {
		t.Fatalf("manifest missing rendered content:\n%s", p.Manifest)
	}
	sum := sha256.Sum256([]byte(p.Manifest))
	if want := hex.EncodeToString(sum[:]); want != p.ManifestSHA {
		t.Fatalf("integrity digest mismatch: manifest hashes to %s, plan records %s", want, p.ManifestSHA)
	}
}

// TestPlanOutFileMatchesJSONFormat proves --out writes the identical apply-able
// artifact that --format json prints, so the file path and the pipeline path
// stay interchangeable for `keramos apply`.
func TestPlanOutFileMatchesJSONFormat(t *testing.T) {
	pkg := makePlanTestPackage(t)
	stdout, err := runPlan(t, pkg, map[string]string{"format": "json"})
	if nil != err {
		t.Fatalf("plan --format json: %v", err)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if _, err := runPlan(t, pkg, map[string]string{"out": planFile}); nil != err {
		t.Fatalf("plan --out: %v", err)
	}
	fileBytes, rErr := os.ReadFile(planFile)
	if nil != rErr {
		t.Fatalf("read plan file: %v", rErr)
	}
	var a, b keramosPlan
	if err := json.Unmarshal([]byte(stdout), &a); nil != err {
		t.Fatalf("stdout json: %v", err)
	}
	if err := json.Unmarshal(fileBytes, &b); nil != err {
		t.Fatalf("file json: %v", err)
	}
	if a.ManifestSHA != b.ManifestSHA || a.Manifest != b.Manifest {
		t.Fatalf("--out and --format json disagree on the manifest/digest")
	}
}

// TestPlanDefaultTextIsReadableDiff verifies the default terminal view is the
// human change preview, not the escaped JSON blob. With no reachable cluster it
// degrades to an all-create plan, which must still read as a diff with a summary.
func TestPlanDefaultTextIsReadableDiff(t *testing.T) {
	t.Setenv("KUBECONFIG", "/dev/null")
	pkg := makePlanTestPackage(t)
	out, err := runPlan(t, pkg, map[string]string{"no-color": "true"})
	if nil != err {
		t.Fatalf("plan (default): %v\n%s", err, out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("default view is raw JSON, expected a diff:\n%s", out)
	}
	if strings.Contains(out, `\n`) {
		t.Fatalf("default view contains escaped newlines (unreadable):\n%s", out)
	}
	for _, want := range []string{"kind: Deployment", "Plan:", "to add"} {
		if !strings.Contains(out, want) {
			t.Fatalf("default view missing %q:\n%s", want, out)
		}
	}
}

// TestPlanShowsLabelChange is the regression for the smart-diff noise filters
// silently dropping a label edit. Both sides are keramos-rendered manifests, so a
// changed label MUST appear in the plan — filtering it out makes plan lie.
func TestPlanShowsLabelChange(t *testing.T) {
	const stored = "apiVersion: apps/v1\n" +
		"kind: Deployment\n" +
		"metadata:\n" +
		"  name: planme\n" +
		"  labels:\n" +
		"    tier: old\n" +
		"spec:\n" +
		"  replicas: 1\n"
	const desired = "apiVersion: apps/v1\n" +
		"kind: Deployment\n" +
		"metadata:\n" +
		"  name: planme\n" +
		"  labels:\n" +
		"    tier: new\n" +
		"spec:\n" +
		"  replicas: 1\n"

	plan := &keramosPlan{ReleaseName: "demo", Namespace: "apps", PackagePath: ".", Manifest: desired}
	var buf bytes.Buffer
	if err := writePlanDiff(&buf, plan, "upgrade", stored, "update", "", false, nil); nil != err {
		t.Fatalf("writePlanDiff: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "No changes") {
		t.Fatalf("label change reported as no-change:\n%s", out)
	}
	// Readable, plan-specific rendering: an "update" header naming the
	// resource, and an inline "path: old → new" field line — not the diff
	// package's pipe-keyed "v1|Service|apps|zephyra" form.
	if !strings.Contains(out, "update  Deployment/planme") {
		t.Fatalf("missing human resource header:\n%s", out)
	}
	if strings.Contains(out, "|") {
		t.Fatalf("plan leaked the pipe-joined resource key:\n%s", out)
	}
	if !strings.Contains(out, "~ metadata.labels.tier") ||
		!strings.Contains(out, `"old"`) || !strings.Contains(out, `"new"`) {
		t.Fatalf("plan did not surface the label edit:\n%s", out)
	}
	if !strings.Contains(out, "1 to change") {
		t.Fatalf("summary should report 1 change:\n%s", out)
	}
}

// TestPlanDerivesReleaseFromKeramosYaml proves `keramos plan` needs no release
// positional: the state identity is taken from the package's keramos.yaml name.
func TestPlanDerivesReleaseFromKeramosYaml(t *testing.T) {
	pkg := makePlanTestPackage(t) // keramos.yaml name: planme
	out, err := runPlan(t, pkg, map[string]string{"format": "json"})
	if nil != err {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	var p keramosPlan
	if jErr := json.Unmarshal([]byte(out), &p); nil != jErr {
		t.Fatalf("bad json: %v", jErr)
	}
	if "planme" != p.ReleaseName {
		t.Fatalf("expected release derived from keramos.yaml name 'planme', got %q", p.ReleaseName)
	}
}

// TestPlanReleaseOverride proves -r pins the state name instead of deriving it.
func TestPlanReleaseOverride(t *testing.T) {
	pkg := makePlanTestPackage(t)
	out, err := runPlan(t, pkg, map[string]string{"format": "json", "release": "prod-web"})
	if nil != err {
		t.Fatalf("plan -r: %v\n%s", err, out)
	}
	var p keramosPlan
	if jErr := json.Unmarshal([]byte(out), &p); nil != jErr {
		t.Fatalf("bad json: %v", jErr)
	}
	if "prod-web" != p.ReleaseName {
		t.Fatalf("expected -r override 'prod-web', got %q", p.ReleaseName)
	}
}

// TestPlanProvenance proves the plan attributes each resource to its source
// template file and traces a changed scalar field back to the value that drove
// it — this is default behaviour, not opt-in.
func TestPlanProvenance(t *testing.T) {
	makePlanTestPackage(t)
	prov, err := buildPlanProvenance(".", "", nil, []string{"replicas=7"}, nil)
	if nil != err {
		t.Fatalf("buildPlanProvenance: %v", err)
	}
	// Locate the Deployment's resource key.
	var key, file string
	for k, f := range prov.resourceFile {
		if strings.Contains(k, "Deployment") {
			key, file = k, f
		}
	}
	if "" == key {
		t.Fatalf("no template attributed to the Deployment: %+v", prov.resourceFile)
	}
	if !strings.Contains(file, "deployment.yaml") {
		t.Fatalf("expected deployment.yaml, got %q", file)
	}
	// spec.replicas is a direct ${values.replicas} substitution overridden by a
	// --set, so its origin must trace to that set expression.
	o := fieldOrigin(prov, key, "spec.replicas")
	if !strings.Contains(o, "set") || !strings.Contains(o, "replicas=7") {
		t.Fatalf("expected spec.replicas traced to 'set (replicas=7)', got %q", o)
	}
}

// TestPlanFieldChangeRendersMultiLine locks the readable per-field layout:
// path, current (state) value, and new value with its origin, each on its own line.
func TestPlanFieldChangeRendersMultiLine(t *testing.T) {
	out := formatFieldChange(diff.FieldChange{Path: "spec.replicas", Old: 1, New: 7}, "set (replicas=7)", "state")
	for _, want := range []string{"~ spec.replicas", "- 1", "(state)", "+ 7", "← set (replicas=7)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in field render:\n%s", want, out)
		}
	}
	if strings.Count(out, "\n") < 3 {
		t.Fatalf("expected a multi-line block, got:\n%s", out)
	}
}

func TestPlanInvalidFormatRejected(t *testing.T) {
	pkg := makePlanTestPackage(t)
	_, err := runPlan(t, pkg, map[string]string{"format": "xml"})
	if nil == err {
		t.Fatal("expected error for --format xml, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
