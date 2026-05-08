package layer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_AllDirectoriesPresent(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: full-pkg\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"),
		[]byte("name: full\nreplicas: 2\n"), 0o644)

	// templates
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deployment.yaml"),
		[]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: full\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "templates", "_helpers.yaml"),
		[]byte("helper: content\n"), 0o644)

	// hooks
	os.MkdirAll(filepath.Join(tmpDir, "hooks"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "hooks", "pre-install.yaml"),
		[]byte("apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: pre-hook\n"), 0o644)

	// tests
	os.MkdirAll(filepath.Join(tmpDir, "tests"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "tests", "test-connection.yaml"),
		[]byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-conn\n"), 0o644)

	// profiles
	profileDir := filepath.Join(tmpDir, "profiles", "prod")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "values.yaml"),
		[]byte("replicas: 5\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "full-pkg" != resolved.Metadata.Name {
		t.Errorf("expected name=full-pkg, got %s", resolved.Metadata.Name)
	}
	if _, ok := resolved.Templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml in templates")
	}
	if _, ok := resolved.Partials["_helpers.yaml"]; !ok {
		t.Error("expected _helpers.yaml in partials")
	}
	if _, ok := resolved.Hooks["pre-install.yaml"]; !ok {
		t.Error("expected pre-install.yaml in hooks")
	}
	if _, ok := resolved.Tests["test-connection.yaml"]; !ok {
		t.Error("expected test-connection.yaml in tests")
	}
}

func TestResolve_TemplateOverrideInBaseChain(t *testing.T) {
	tmpDir := t.TempDir()

	// Base package
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(filepath.Join(baseDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(baseDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: base\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "values.yaml"),
		[]byte("name: base-val\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "templates", "shared.yaml"),
		[]byte("kind: BaseConfigMap\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "templates", "base-only.yaml"),
		[]byte("kind: BaseOnly\n"), 0o644)

	// Overlay package
	overlayDir := filepath.Join(tmpDir, "overlay")
	os.MkdirAll(filepath.Join(overlayDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(overlayDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: overlay\nversion: 2.0.0\nbase: ../base\n"), 0o644)
	os.WriteFile(filepath.Join(overlayDir, "values.yaml"),
		[]byte("name: overlay-val\n"), 0o644)
	os.WriteFile(filepath.Join(overlayDir, "templates", "shared.yaml"),
		[]byte("kind: OverlayConfigMap\n"), 0o644)

	resolved, err := Resolve(overlayDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overlay template should override base
	if !strings.Contains(resolved.Templates["shared.yaml"], "OverlayConfigMap") {
		t.Error("expected overlay to override base template shared.yaml")
	}
	// Base-only template should still be present
	if _, ok := resolved.Templates["base-only.yaml"]; !ok {
		t.Error("expected base-only.yaml from base")
	}
	// Overlay metadata should win
	if "overlay" != resolved.Metadata.Name {
		t.Errorf("expected name=overlay, got %s", resolved.Metadata.Name)
	}
}

func TestResolve_ProfileWithTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: proftempl\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"),
		[]byte("name: test\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deploy.yaml"),
		[]byte("kind: Deployment\n"), 0o644)

	// Profile with its own templates
	profileDir := filepath.Join(tmpDir, "profiles", "custom")
	os.MkdirAll(filepath.Join(profileDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "values.yaml"),
		[]byte("env: custom\n"), 0o644)
	os.WriteFile(filepath.Join(profileDir, "templates", "extra.yaml"),
		[]byte("kind: CustomExtra\n"), 0o644)
	// Profile can override a template
	os.WriteFile(filepath.Join(profileDir, "templates", "deploy.yaml"),
		[]byte("kind: CustomDeployment\n"), 0o644)

	resolved, err := Resolve(tmpDir, "custom")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "custom" != resolved.Values["env"] {
		t.Errorf("expected env=custom, got %v", resolved.Values["env"])
	}
	if _, ok := resolved.Templates["extra.yaml"]; !ok {
		t.Error("expected extra.yaml from profile")
	}
	if !strings.Contains(resolved.Templates["deploy.yaml"], "CustomDeployment") {
		t.Error("expected profile to override deploy.yaml")
	}
}

func TestResolve_InvalidKeramosYaml(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("{{{invalid yaml content!!!"), 0o644)

	_, err := Resolve(tmpDir, "")
	if nil == err {
		t.Fatal("expected error for invalid keramos.yaml")
	}
}

func TestResolve_MissingValuesInProfile(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: novals-prof\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"),
		[]byte("name: original\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "svc.yaml"),
		[]byte("kind: Service\n"), 0o644)

	// Profile without values.yaml
	profileDir := filepath.Join(tmpDir, "profiles", "minimal")
	os.MkdirAll(profileDir, 0o755)
	// No values.yaml in profile - should work fine

	resolved, err := Resolve(tmpDir, "minimal")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original values should be preserved
	if "original" != resolved.Values["name"] {
		t.Errorf("expected name=original, got %v", resolved.Values["name"])
	}
}

func TestResolve_BaseWithHooksAndTests(t *testing.T) {
	tmpDir := t.TempDir()

	// Base with hooks and tests
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(filepath.Join(baseDir, "templates"), 0o755)
	os.MkdirAll(filepath.Join(baseDir, "hooks"), 0o755)
	os.MkdirAll(filepath.Join(baseDir, "tests"), 0o755)
	os.WriteFile(filepath.Join(baseDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: base\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "templates", "svc.yaml"),
		[]byte("kind: Service\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "hooks", "pre-install.yaml"),
		[]byte("kind: Job\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "tests", "test.yaml"),
		[]byte("kind: Pod\n"), 0o644)

	// Overlay
	overlayDir := filepath.Join(tmpDir, "overlay")
	os.MkdirAll(filepath.Join(overlayDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(overlayDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: overlay\nversion: 2.0.0\nbase: ../base\n"), 0o644)

	resolved, err := Resolve(overlayDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hooks and tests from base should be inherited
	if _, ok := resolved.Hooks["pre-install.yaml"]; !ok {
		t.Error("expected pre-install.yaml hook from base")
	}
	if _, ok := resolved.Tests["test.yaml"]; !ok {
		t.Error("expected test.yaml from base")
	}
}

func TestResolve_ProfileNotADirectory(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: proffile\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "profiles"), 0o755)
	// Create a file instead of directory for the profile
	os.WriteFile(filepath.Join(tmpDir, "profiles", "badprof"), []byte("not a dir"), 0o644)

	_, err := Resolve(tmpDir, "badprof")
	if nil == err {
		t.Fatal("expected error for profile that is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %s", err.Error())
	}
}

func TestResolve_NonYAMLFilesSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: nonyaml\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deploy.yaml"),
		[]byte("kind: Deployment\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "templates", "readme.txt"),
		[]byte("this is a readme"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "templates", "notes.md"),
		[]byte("# Notes"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := resolved.Templates["deploy.yaml"]; !ok {
		t.Error("expected deploy.yaml")
	}
	if _, ok := resolved.Templates["readme.txt"]; ok {
		t.Error("readme.txt should not be in templates")
	}
	if _, ok := resolved.Templates["notes.md"]; ok {
		t.Error("notes.md should not be in templates")
	}
}

func TestResolve_YMLExtension(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: ymlext\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "svc.yml"),
		[]byte("kind: Service\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := resolved.Templates["svc.yml"]; !ok {
		t.Error("expected svc.yml in templates")
	}
}

func TestResolve_DeepMergeNestedValues(t *testing.T) {
	tmpDir := t.TempDir()

	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(baseDir, 0o755)
	os.WriteFile(filepath.Join(baseDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: base\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "values.yaml"),
		[]byte("image:\n  repository: nginx\n  tag: \"1.19\"\nresources:\n  cpu: 100m\n"), 0o644)

	overlayDir := filepath.Join(tmpDir, "overlay")
	os.MkdirAll(overlayDir, 0o755)
	os.WriteFile(filepath.Join(overlayDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: overlay\nversion: 1.0.0\nbase: ../base\n"), 0o644)
	os.WriteFile(filepath.Join(overlayDir, "values.yaml"),
		[]byte("image:\n  tag: \"1.20\"\n"), 0o644)

	resolved, err := Resolve(overlayDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	image, ok := resolved.Values["image"].(map[string]any)
	if !ok {
		t.Fatal("expected image to be a map")
	}
	if "nginx" != image["repository"] {
		t.Errorf("expected repository=nginx from base, got %v", image["repository"])
	}
	if "1.20" != image["tag"] {
		t.Errorf("expected tag=1.20 from overlay, got %v", image["tag"])
	}
	if _, ok := resolved.Values["resources"]; !ok {
		t.Error("expected resources from base to be preserved")
	}
}

func TestResolve_ValuesWithSlicesAndNils(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: slicepkg\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"),
		[]byte("items:\n  - name: first\n  - name: second\nnullable: null\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "svc.yaml"),
		[]byte("kind: Service\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	items, ok := resolved.Values["items"].([]any)
	if !ok {
		t.Fatal("expected items to be a slice")
	}
	if 2 != len(items) {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestResolve_HooksInSubdir(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: hookpkg\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "hooks"), 0o755)
	// Subdirectories inside hooks should be skipped
	os.MkdirAll(filepath.Join(tmpDir, "hooks", "subdir"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "hooks", "pre-install.yaml"),
		[]byte("kind: Job\n"), 0o644)
	// Non-yaml files should be skipped
	os.WriteFile(filepath.Join(tmpDir, "hooks", "readme.txt"),
		[]byte("not a hook"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := resolved.Hooks["pre-install.yaml"]; !ok {
		t.Error("expected pre-install.yaml in hooks")
	}
	if 1 != len(resolved.Hooks) {
		t.Errorf("expected 1 hook, got %d", len(resolved.Hooks))
	}
}

func TestResolve_TemplatesDirNotDir(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: templfile\nversion: 1.0.0\n"), 0o644)
	// Create templates as a file, not dir
	os.WriteFile(filepath.Join(tmpDir, "templates"), []byte("not a dir"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should treat it as no templates
	if 0 != len(resolved.Templates) {
		t.Errorf("expected 0 templates, got %d", len(resolved.Templates))
	}
}

func TestResolve_HooksDirNotDir(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: hookfile\nversion: 1.0.0\n"), 0o644)
	// Create hooks as a file, not dir
	os.WriteFile(filepath.Join(tmpDir, "hooks"), []byte("not a dir"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(resolved.Hooks) {
		t.Errorf("expected 0 hooks, got %d", len(resolved.Hooks))
	}
}

func TestResolve_ProfileWithPartials(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"),
		[]byte("apiVersion: keramos/v1\nname: partials-prof\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deploy.yaml"),
		[]byte("kind: Deployment\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "templates", "_helpers.yaml"),
		[]byte("original: helper\n"), 0o644)

	profileDir := filepath.Join(tmpDir, "profiles", "custom")
	os.MkdirAll(filepath.Join(profileDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "templates", "_helpers.yaml"),
		[]byte("custom: helper\n"), 0o644)

	resolved, err := Resolve(tmpDir, "custom")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	partial, ok := resolved.Partials["_helpers.yaml"]
	if !ok {
		t.Fatal("expected _helpers.yaml in partials")
	}
	if !strings.Contains(partial.(string), "custom") {
		t.Error("expected profile partial to override base partial")
	}
}
