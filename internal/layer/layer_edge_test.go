package layer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_NoTemplatesDir(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: notemplates\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("name: test\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(resolved.Templates) {
		t.Errorf("expected 0 templates, got %d", len(resolved.Templates))
	}
	if "notemplates" != resolved.Metadata.Name {
		t.Errorf("expected name=notemplates, got %s", resolved.Metadata.Name)
	}
}

func TestResolve_NoValuesYaml(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: novalues\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "svc.yaml"), []byte("kind: Service\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil == resolved.Values {
		t.Fatal("expected non-nil values even without values.yaml")
	}
	if 0 != len(resolved.Values) {
		t.Errorf("expected empty values, got %d entries", len(resolved.Values))
	}
	if _, ok := resolved.Templates["svc.yaml"]; !ok {
		t.Error("expected svc.yaml template")
	}
}

func TestResolve_InvalidBasePath(t *testing.T) {
	_, err := Resolve("/absolutely/nonexistent/path", "")
	if nil == err {
		t.Fatal("expected error for invalid base path")
	}
}

func TestResolve_NonexistentProfile(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: myapp\nversion: 1.0.0\n"), 0o644)

	_, err := Resolve(tmpDir, "nonexistent")
	if nil == err {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("expected 'profile not found' error, got: %s", err.Error())
	}
}

func TestResolve_ThreeLevelBaseChain(t *testing.T) {
	tmpDir := t.TempDir()

	// Create A -> B -> C chain
	pkgC := filepath.Join(tmpDir, "c")
	pkgB := filepath.Join(tmpDir, "b")
	pkgA := filepath.Join(tmpDir, "a")

	os.MkdirAll(filepath.Join(pkgC, "templates"), 0o755)
	os.MkdirAll(filepath.Join(pkgB, "templates"), 0o755)
	os.MkdirAll(filepath.Join(pkgA, "templates"), 0o755)

	// C: base package
	os.WriteFile(filepath.Join(pkgC, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: c\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(pkgC, "values.yaml"), []byte("base: c\nlevel: 1\n"), 0o644)
	os.WriteFile(filepath.Join(pkgC, "templates", "base.yaml"), []byte("kind: BaseResource\n"), 0o644)

	// B: inherits from C
	os.WriteFile(filepath.Join(pkgB, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: b\nversion: 1.0.0\nbase: ../c\n"), 0o644)
	os.WriteFile(filepath.Join(pkgB, "values.yaml"), []byte("level: 2\nmid: true\n"), 0o644)
	os.WriteFile(filepath.Join(pkgB, "templates", "mid.yaml"), []byte("kind: MidResource\n"), 0o644)

	// A: inherits from B
	os.WriteFile(filepath.Join(pkgA, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: a\nversion: 1.0.0\nbase: ../b\n"), 0o644)
	os.WriteFile(filepath.Join(pkgA, "values.yaml"), []byte("level: 3\n"), 0o644)
	os.WriteFile(filepath.Join(pkgA, "templates", "top.yaml"), []byte("kind: TopResource\n"), 0o644)

	resolved, err := Resolve(pkgA, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// A's metadata wins
	if "a" != resolved.Metadata.Name {
		t.Errorf("expected name=a, got %s", resolved.Metadata.Name)
	}

	// A's value wins for level
	if 3 != resolved.Values["level"] {
		t.Errorf("expected level=3, got %v", resolved.Values["level"])
	}

	// B's value for mid should be inherited
	if true != resolved.Values["mid"] {
		t.Errorf("expected mid=true from B, got %v", resolved.Values["mid"])
	}

	// C's value for base should be inherited
	if "c" != resolved.Values["base"] {
		t.Errorf("expected base=c from C, got %v", resolved.Values["base"])
	}

	// All templates should be present
	if _, ok := resolved.Templates["base.yaml"]; !ok {
		t.Error("expected base.yaml from C")
	}
	if _, ok := resolved.Templates["mid.yaml"]; !ok {
		t.Error("expected mid.yaml from B")
	}
	if _, ok := resolved.Templates["top.yaml"]; !ok {
		t.Error("expected top.yaml from A")
	}
}

func TestResolve_CircularBase(t *testing.T) {
	tmpDir := t.TempDir()

	pkgA := filepath.Join(tmpDir, "a")
	pkgB := filepath.Join(tmpDir, "b")
	os.MkdirAll(pkgA, 0o755)
	os.MkdirAll(pkgB, 0o755)

	os.WriteFile(filepath.Join(pkgA, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: a\nversion: 1.0.0\nbase: ../b\n"), 0o644)
	os.WriteFile(filepath.Join(pkgB, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: b\nversion: 1.0.0\nbase: ../a\n"), 0o644)

	_, err := Resolve(pkgA, "")
	if nil == err {
		t.Fatal("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %s", err.Error())
	}
}

func TestResolve_ProfileOverridesValues(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: proftest\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("replicas: 1\nenv: dev\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deploy.yaml"), []byte("kind: Deployment\n"), 0o644)

	profileDir := filepath.Join(tmpDir, "profiles", "staging")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "values.yaml"), []byte("replicas: 3\nenv: staging\n"), 0o644)

	resolved, err := Resolve(tmpDir, "staging")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 3 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=3, got %v", resolved.Values["replicas"])
	}
	if "staging" != resolved.Values["env"] {
		t.Errorf("expected env=staging, got %v", resolved.Values["env"])
	}
}

func TestResolve_PartialFilesSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: partialtest\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "_helpers.yaml"), []byte("helper: content\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deploy.yaml"), []byte("kind: Deployment\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := resolved.Templates["deploy.yaml"]; !ok {
		t.Error("expected deploy.yaml in templates")
	}
	// Partial files (starting with _) should be in partials, not templates
	if _, ok := resolved.Templates["_helpers.yaml"]; ok {
		t.Error("_helpers.yaml should not be in templates")
	}
	if _, ok := resolved.Partials["_helpers.yaml"]; !ok {
		t.Error("expected _helpers.yaml in partials")
	}
}
