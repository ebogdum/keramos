package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebogdum/keramos/internal/engine"
)

// --- renderHooks tests ---

func TestRenderHooks_EmptyMap(t *testing.T) {
	eng := engine.New()
	ctx := &engine.RenderContext{
		Values:       map[string]any{},
		Package:      map[string]any{"name": "test", "version": "1.0.0", "appVersion": ""},
		Release:      map[string]any{"name": "test", "namespace": "default", "revision": 1, "isUpgrade": false, "isInstall": true},
		Capabilities: map[string]any{},
	}
	result, err := renderHooks(eng, map[string]string{}, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(result) {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestRenderHooks_SimpleHook(t *testing.T) {
	eng := engine.New()
	ctx := &engine.RenderContext{
		Values:       map[string]any{},
		Package:      map[string]any{"name": "test", "version": "1.0.0", "appVersion": ""},
		Release:      map[string]any{"name": "test", "namespace": "default", "revision": 1, "isUpgrade": false, "isInstall": true},
		Capabilities: map[string]any{},
	}
	hookTemplates := map[string]string{
		"pre-install": "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: pre-install\n",
	}
	result, err := renderHooks(eng, hookTemplates, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(result) {
		t.Fatalf("expected 1 rendered hook, got %d", len(result))
	}
	if !strings.Contains(result["pre-install"], "kind: Job") {
		t.Error("expected rendered hook to contain kind: Job")
	}
}

func TestRenderHooks_MultipleHooks(t *testing.T) {
	eng := engine.New()
	ctx := &engine.RenderContext{
		Values:       map[string]any{},
		Package:      map[string]any{"name": "test", "version": "1.0.0", "appVersion": ""},
		Release:      map[string]any{"name": "test", "namespace": "default", "revision": 1, "isUpgrade": false, "isInstall": true},
		Capabilities: map[string]any{},
	}
	hookTemplates := map[string]string{
		"pre-install":  "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: pre\n",
		"post-install": "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: post\n",
	}
	result, err := renderHooks(eng, hookTemplates, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(result) {
		t.Fatalf("expected 2 rendered hooks, got %d", len(result))
	}
}

// --- joinDocs additional ---

func TestJoinDocs_ThreeDocs(t *testing.T) {
	docs := []string{"doc1\n", "doc2\n", "doc3\n"}
	result := joinDocs(docs)
	if !strings.Contains(result, "---\n") {
		t.Error("expected separator between docs")
	}
	parts := strings.Split(result, "---\n")
	if 3 != len(parts) {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}
}

func TestJoinDocs_SingleDoc(t *testing.T) {
	result := joinDocs([]string{"only-doc"})
	if "only-doc" != result {
		t.Errorf("expected 'only-doc', got %q", result)
	}
}

// --- extractNotes additional ---

func TestExtractNotes_OnlyNotesDoc(t *testing.T) {
	rendered := "message: |\n  Welcome!\n"
	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(notes, "Welcome!") {
		t.Errorf("expected notes to contain Welcome, got %q", notes)
	}
	if "" != manifest {
		t.Errorf("expected empty manifest, got %q", manifest)
	}
}

func TestExtractNotes_NonYAMLContent(t *testing.T) {
	rendered := "not valid yaml {{{{\n"
	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" != notes {
		t.Errorf("expected no notes, got %q", notes)
	}
	if "" == manifest {
		t.Error("expected non-empty manifest")
	}
}

func TestExtractNotes_WhitespaceOnlyDocs(t *testing.T) {
	rendered := "   \n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"
	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" != notes {
		t.Errorf("expected no notes, got %q", notes)
	}
	if !strings.Contains(manifest, "ConfigMap") {
		t.Error("expected ConfigMap in manifest")
	}
}

// --- parseNotesDoc additional ---

func TestParseNotesDoc_NotAMap(t *testing.T) {
	// parseNotesDoc expects map[string]any, so a plain string returns an error
	_, isNotes, err := parseNotesDoc("just a string")
	// yaml.Unmarshal into map returns error for non-map input
	if nil == err {
		// If it didn't error, isNotes should be false
		if isNotes {
			t.Error("expected isNotes=false for non-map doc")
		}
	}
}

func TestParseNotesDoc_EmptyMap(t *testing.T) {
	_, isNotes, err := parseNotesDoc("{}")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNotes {
		t.Error("expected isNotes=false for empty map")
	}
}

func TestParseNotesDoc_DifferentKey(t *testing.T) {
	_, isNotes, err := parseNotesDoc("notmessage: hello")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNotes {
		t.Error("expected isNotes=false when key is not 'message'")
	}
}

func TestParseNotesDoc_ValidNotes(t *testing.T) {
	msg, isNotes, err := parseNotesDoc("message: hello world")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNotes {
		t.Error("expected isNotes=true for valid notes doc")
	}
	if "hello world" != msg {
		t.Errorf("expected 'hello world', got %q", msg)
	}
}

func TestParseNotesDoc_InvalidYAML(t *testing.T) {
	_, _, err := parseNotesDoc("{{invalid")
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

// --- ValidateReleaseName edge cases ---

func TestValidateReleaseName_SingleHyphen(t *testing.T) {
	err := ValidateReleaseName("-")
	if nil == err {
		t.Fatal("expected error for single hyphen")
	}
}

func TestValidateReleaseName_DoubleHyphen(t *testing.T) {
	err := ValidateReleaseName("a--b")
	if nil != err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateReleaseName_NumericOnly(t *testing.T) {
	err := ValidateReleaseName("12345")
	if nil != err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateReleaseName_LeadingHyphen(t *testing.T) {
	err := ValidateReleaseName("-abc")
	if nil == err {
		t.Fatal("expected error for leading hyphen")
	}
}

func TestValidateReleaseName_TrailingHyphen(t *testing.T) {
	err := ValidateReleaseName("abc-")
	if nil == err {
		t.Fatal("expected error for trailing hyphen")
	}
}

func TestValidateReleaseName_Underscore(t *testing.T) {
	err := ValidateReleaseName("my_app")
	if nil == err {
		t.Fatal("expected error for underscore")
	}
}

func TestValidateReleaseName_Uppercase(t *testing.T) {
	err := ValidateReleaseName("MyApp")
	if nil == err {
		t.Fatal("expected error for uppercase")
	}
}

func TestValidateReleaseName_Dot(t *testing.T) {
	err := ValidateReleaseName("my.app")
	if nil == err {
		t.Fatal("expected error for dot")
	}
}

func TestValidateReleaseName_Space(t *testing.T) {
	err := ValidateReleaseName("my app")
	if nil == err {
		t.Fatal("expected error for space")
	}
}

// --- Install dry-run with profile ---

func TestInstallDryRunWithProfile(t *testing.T) {
	dir := filepath.Join(fixturesPath(), "with-profiles")

	opts := &InstallOptions{
		ReleaseName: "profile-install",
		Namespace:   "default",
		DryRun:      "client",
		Profile:     "prod",
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(nil, dir, opts)
	if nil != err {
		t.Fatalf("install with profile failed: %v", err)
	}
	if "profile-install" != rel.Name {
		t.Errorf("expected name profile-install, got %s", rel.Name)
	}
	if "pending-install" != string(rel.Status) {
		t.Errorf("expected status pending-install, got %s", rel.Status)
	}
}

func TestInstallDryRunWithDescription(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "desc-test",
		Namespace:   "default",
		DryRun:      "client",
		Description: "My release description",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "My release description" != rel.Info.Description {
		t.Errorf("expected description, got %q", rel.Info.Description)
	}
}

func TestInstallDryRunUserValuesTracking(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "user-vals",
		Namespace:   "default",
		DryRun:      "client",
		Sets:        []string{"custom=value"},
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if nil == rel.UserValues {
		t.Error("expected non-nil UserValues")
	}
	if "value" != rel.UserValues["custom"] {
		t.Errorf("expected custom=value in UserValues, got %v", rel.UserValues["custom"])
	}
}

func TestInstallDryRunTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	before := time.Now().UTC().Add(-time.Second)
	opts := &InstallOptions{
		ReleaseName: "time-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	if rel.Info.FirstDeployed.Before(before) || rel.Info.FirstDeployed.After(after) {
		t.Errorf("FirstDeployed %v not in expected range", rel.Info.FirstDeployed)
	}
	if rel.Info.LastDeployed.Before(before) || rel.Info.LastDeployed.After(after) {
		t.Errorf("LastDeployed %v not in expected range", rel.Info.LastDeployed)
	}
}

func TestInstallDryRunWithNotesAndManifest(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	// Add notes template
	notesContent := "message: |\n  Thank you for installing.\n"
	os.WriteFile(filepath.Join(tmpDir, "templates", "notes.yaml"), []byte(notesContent), 0o644)

	opts := &InstallOptions{
		ReleaseName: "notes-manifest-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "" == rel.Notes {
		t.Error("expected non-empty notes")
	}
	if strings.Contains(rel.Manifest, "message:") {
		t.Error("notes should be extracted from manifest")
	}
}

func TestInstallDryRunWithValuesFileAndSets(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	vf := filepath.Join(tmpDir, "custom.yaml")
	os.WriteFile(vf, []byte("env: staging\n"), 0o644)

	opts := &InstallOptions{
		ReleaseName: "combined-vals",
		Namespace:   "default",
		DryRun:      "client",
		ValueFiles:  []string{vf},
		Sets:        []string{"replicas=3"},
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "staging" != rel.Values["env"] {
		t.Errorf("expected env=staging, got %v", rel.Values["env"])
	}
	if 3 != rel.Values["replicas"] {
		t.Errorf("expected replicas=3, got %v", rel.Values["replicas"])
	}
}

// --- Lint additional coverage ---

func TestLint_InvalidKeramosYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "{invalid yaml {{")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected errors for invalid YAML")
	}
}

func TestLint_NoTemplatesDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if 0 == len(result.Warnings) {
		t.Error("expected warning for missing templates/")
	}
}

func TestLint_BasePackage_MissingBase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\nbase: nonexistent\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for missing base directory")
	}
}

func TestLint_BasePackage_NoKeramosYAMLInBase(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "mybase")
	mkDir(t, baseDir)
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\nbase: mybase\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/dummy.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for base without keramos.yaml")
	}
}

func TestLint_DuplicateTemplates(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "mybase")
	mkDir(t, baseDir)
	writeFile(t, baseDir, "keramos.yaml", "apiVersion: keramos/v1\nname: base\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(baseDir, "templates"))
	writeFile(t, filepath.Join(baseDir, "templates"), "svc.yaml", "apiVersion: v1\nkind: Service\nmetadata:\n  name: base-svc\n")

	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: overlay\nversion: 1.0.0\nbase: mybase\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, filepath.Join(dir, "templates"), "svc.yaml", "apiVersion: v1\nkind: Service\nmetadata:\n  name: overlay-svc\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	hasOverrideWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "overrides base template") {
			hasOverrideWarning = true
		}
	}
	if !hasOverrideWarning {
		t.Error("expected warning about template overriding base")
	}
}

func TestLintResult_IsValid(t *testing.T) {
	result := &LintResult{}
	if !result.IsValid() {
		t.Error("expected empty result to be valid")
	}

	result.Errors = append(result.Errors, LintMessage{Severity: "error", Message: "fail"})
	if result.IsValid() {
		t.Error("expected result with errors to be invalid")
	}
}

// --- Create additional ---

func TestCreate_ValuesContent(t *testing.T) {
	dir := t.TempDir()
	if err := Create("testpkg", dir); nil != err {
		t.Fatalf("Create failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "testpkg", "values.yaml"))
	if nil != err {
		t.Fatalf("failed to read values.yaml: %v", err)
	}
	if !strings.Contains(string(data), "name: testpkg") {
		t.Error("expected package name in values.yaml")
	}
	if !strings.Contains(string(data), "replicaCount: 1") {
		t.Error("expected replicaCount in values.yaml")
	}
}

func TestCreate_KeramosignoreContent(t *testing.T) {
	dir := t.TempDir()
	if err := Create("testpkg", dir); nil != err {
		t.Fatalf("Create failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "testpkg", ".keramosignore"))
	if nil != err {
		t.Fatalf("failed to read .keramosignore: %v", err)
	}
	if !strings.Contains(string(data), ".git") {
		t.Error("expected .git in .keramosignore")
	}
}

func TestCreate_TemplateContent(t *testing.T) {
	dir := t.TempDir()
	if err := Create("testpkg", dir); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "testpkg", "templates", "deployment.yaml"))
	if nil != err {
		t.Fatalf("failed to read deployment.yaml: %v", err)
	}
	if !strings.Contains(string(data), "kind: Deployment") {
		t.Error("expected Deployment kind in template")
	}

	data, err = os.ReadFile(filepath.Join(dir, "testpkg", "templates", "service.yaml"))
	if nil != err {
		t.Fatalf("failed to read service.yaml: %v", err)
	}
	if !strings.Contains(string(data), "kind: Service") {
		t.Error("expected Service kind in template")
	}

	data, err = os.ReadFile(filepath.Join(dir, "testpkg", "templates", "notes.yaml"))
	if nil != err {
		t.Fatalf("failed to read notes.yaml: %v", err)
	}
	if !strings.Contains(string(data), "message:") {
		t.Error("expected message key in notes template")
	}
}

// --- Install with multiple value files ---

func TestInstallDryRunWithMultipleValueFiles(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	f1 := filepath.Join(tmpDir, "vals1.yaml")
	os.WriteFile(f1, []byte("key1: val1\n"), 0o644)

	f2 := filepath.Join(tmpDir, "vals2.yaml")
	os.WriteFile(f2, []byte("key2: val2\n"), 0o644)

	opts := &InstallOptions{
		ReleaseName: "multi-vals",
		Namespace:   "default",
		DryRun:      "client",
		ValueFiles:  []string{f1, f2},
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "val1" != rel.Values["key1"] {
		t.Errorf("expected key1=val1, got %v", rel.Values["key1"])
	}
	if "val2" != rel.Values["key2"] {
		t.Errorf("expected key2=val2, got %v", rel.Values["key2"])
	}
}

// --- Install with hooks ---

func TestInstallDryRunWithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	// Add hooks directory
	hooksDir := filepath.Join(tmpDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookContent := "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: pre-install-hook\n  annotations:\n    keramos/hook: pre-install\n"
	os.WriteFile(filepath.Join(hooksDir, "pre-install.yaml"), []byte(hookContent), 0o644)

	opts := &InstallOptions{
		ReleaseName: "hooks-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	// Dry run should succeed even with hooks (they're not executed)
	if "pending-install" != string(rel.Status) {
		t.Errorf("expected pending-install, got %s", rel.Status)
	}
}

// --- templateNames helper ---

func TestTemplateNames_ValidDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "svc.yaml"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(dir, "dep.yaml"), []byte("test"), 0o644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)

	names, err := templateNames(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(names) {
		t.Errorf("expected 2 template names, got %d", len(names))
	}
	if !names["svc.yaml"] {
		t.Error("expected svc.yaml in names")
	}
}

func TestTemplateNames_NonexistentDir(t *testing.T) {
	names, err := templateNames("/nonexistent/dir")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(names) {
		t.Errorf("expected empty map, got %d entries", len(names))
	}
}

// --- Lint with values file overrides ---

func TestLint_WithValuesFileOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	writeFile(t, dir, "values.yaml", "name: default\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	overrideFile := filepath.Join(dir, "override.yaml")
	os.WriteFile(overrideFile, []byte("name: overridden\n"), 0o644)

	result, err := Lint(dir, []string{overrideFile}, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("error: %s: %s", msg.File, msg.Message)
		}
	}
}

func TestLint_WithSetOverrides(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	writeFile(t, dir, "values.yaml", "name: default\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, []string{"name=override"}, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("error: %s: %s", msg.File, msg.Message)
		}
	}
}

func TestLint_ValidSchemaJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	writeFile(t, dir, "values.yaml", "name: test\n")
	writeFile(t, dir, "values.schema.json", `{"type": "object", "properties": {"name": {"type": "string"}}}`)
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("error: %s: %s", msg.File, msg.Message)
		}
	}
}

func TestLint_OnlyPartials(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))
	// Only underscore file (partial), no .yaml
	writeFile(t, dir, "templates/_helpers.yaml", "labels:\n  app: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	// Should warn about no .yaml files
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "no .yaml files") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected warning about no .yaml template files")
	}
}

func TestLint_EmptyApiVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "name: test\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for missing apiVersion")
	}
}

func TestLint_EmptyName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nversion: 1.0.0\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for missing name")
	}
}

func TestLint_EmptyVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\n")
	mkDir(t, filepath.Join(dir, "templates"))

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected error for missing version")
	}
}

func TestLint_ValidSemverPrerelease(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keramos.yaml", "apiVersion: keramos/v1\nname: test\nversion: 1.0.0-alpha.1\n")
	mkDir(t, filepath.Join(dir, "templates"))
	writeFile(t, dir, "templates/cm.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := Lint(dir, nil, nil, "", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("error: %s: %s", msg.File, msg.Message)
		}
	}
}

func TestLint_WithProfile(t *testing.T) {
	dir := filepath.Join(fixturesPath(), "with-profiles")
	result, err := Lint(dir, nil, nil, "prod", false)
	if nil != err {
		t.Fatalf("Lint returned error: %v", err)
	}
	if !result.IsValid() {
		for _, msg := range result.Errors {
			t.Errorf("error: %s: %s", msg.File, msg.Message)
		}
	}
}

// --- Install edge cases for coverage ---

func TestInstallDryRunWithExplicitNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "ns-explicit",
		Namespace:   "custom-ns",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "custom-ns" != rel.Namespace {
		t.Errorf("expected namespace custom-ns, got %s", rel.Namespace)
	}
}

func TestInstallDryRunPackageRef(t *testing.T) {
	tmpDir := t.TempDir()
	keramosYaml := "apiVersion: keramos/v1\nname: my-pkg\nversion: 2.3.4\nappVersion: \"5.6.7\"\n"
	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte(keramosYaml), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("k: v\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0o644)

	opts := &InstallOptions{
		ReleaseName: "pkg-ref-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}
	if "my-pkg" != rel.Package.Name {
		t.Errorf("expected package name my-pkg, got %s", rel.Package.Name)
	}
	if "2.3.4" != rel.Package.Version {
		t.Errorf("expected version 2.3.4, got %s", rel.Package.Version)
	}
	if "5.6.7" != rel.Package.AppVersion {
		t.Errorf("expected appVersion 5.6.7, got %s", rel.Package.AppVersion)
	}
}

func TestInstallDryRunInvalidReleaseName(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"My-App"},
		{"my_app"},
		{"-leading"},
		{"trailing-"},
		{strings.Repeat("x", 54)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &InstallOptions{
				ReleaseName: tt.name,
				DryRun:      "client",
			}
			_, err := Install(nil, "/tmp", opts)
			if nil == err {
				t.Errorf("expected error for invalid release name %q", tt.name)
			}
		})
	}
}

// --- Helpers ---

func fixturesPath() string {
	return filepath.Join("..", "..", "test", "fixtures")
}
