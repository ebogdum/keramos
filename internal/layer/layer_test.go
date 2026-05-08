package layer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturesDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if nil != err {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "test", "fixtures")
}

func TestResolve_SimplePackage(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")
	resolved, err := Resolve(dir, "")
	if nil != err {
		t.Fatal(err)
	}

	if "simple-app" != resolved.Metadata.Name {
		t.Errorf("expected name=simple-app, got %s", resolved.Metadata.Name)
	}
	if "1.0.0" != resolved.Metadata.Version {
		t.Errorf("expected version=1.0.0, got %s", resolved.Metadata.Version)
	}
	if "myapp" != resolved.Values["name"] {
		t.Errorf("expected values.name=myapp, got %v", resolved.Values["name"])
	}
	if _, ok := resolved.Templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml template")
	}
	if _, ok := resolved.Templates["service.yaml"]; !ok {
		t.Error("expected service.yaml template")
	}
}

func TestResolve_WithBase(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-base", "overlay")
	resolved, err := Resolve(dir, "")
	if nil != err {
		t.Fatal(err)
	}

	if "overlay-app" != resolved.Metadata.Name {
		t.Errorf("expected name=overlay-app, got %s", resolved.Metadata.Name)
	}

	// Overlay values override base
	if 5 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=5, got %v", resolved.Values["replicas"])
	}

	// Base value preserved when not overridden
	if "base" != resolved.Values["name"] {
		t.Errorf("expected name=base (from base), got %v", resolved.Values["name"])
	}

	// Deep merge: image.repository from base, image.tag from overlay
	image, ok := resolved.Values["image"].(map[string]any)
	if !ok {
		t.Fatal("expected image to be map")
	}
	if "nginx" != image["repository"] {
		t.Errorf("expected image.repository=nginx, got %v", image["repository"])
	}
	if "latest" != image["tag"] {
		t.Errorf("expected image.tag=latest, got %v", image["tag"])
	}

	// Base templates inherited
	if _, ok := resolved.Templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml from base")
	}
	if _, ok := resolved.Templates["service.yaml"]; !ok {
		t.Error("expected service.yaml from base")
	}
	// Overlay adds new template
	if _, ok := resolved.Templates["ingress.yaml"]; !ok {
		t.Error("expected ingress.yaml from overlay")
	}
}

func TestResolve_WithProfile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")

	resolved, err := Resolve(dir, "prod")
	if nil != err {
		t.Fatal(err)
	}

	if 5 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=5, got %v", resolved.Values["replicas"])
	}
	if "production" != resolved.Values["env"] {
		t.Errorf("expected env=production, got %v", resolved.Values["env"])
	}
	// Non-overridden value preserved
	if "myapp" != resolved.Values["name"] {
		t.Errorf("expected name=myapp, got %v", resolved.Values["name"])
	}
}

func TestResolve_ProfileNotFound(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")
	_, err := Resolve(dir, "nonexistent")
	if nil == err {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("expected 'profile not found' error, got: %s", err.Error())
	}
}

func TestResolve_CycleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two packages that reference each other
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

func TestResolve_MissingPackage(t *testing.T) {
	_, err := Resolve("/nonexistent/path", "")
	if nil == err {
		t.Fatal("expected error for missing package")
	}
}

// --- Multi-layer tests ---

func TestResolve_MultiLayerComposition(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "layered", "app")
	resolved, err := Resolve(dir, "")
	if nil != err {
		t.Fatal(err)
	}

	// App metadata wins
	if "app" != resolved.Metadata.Name {
		t.Errorf("expected name=app, got %s", resolved.Metadata.Name)
	}
	if "2.0.0" != resolved.Metadata.Version {
		t.Errorf("expected version=2.0.0, got %s", resolved.Metadata.Version)
	}

	// App values override layers: replicas=3 (app wins over service=2 and base=1)
	if 3 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=3 (from app), got %v", resolved.Values["replicas"])
	}

	// App overrides name
	if "my-app" != resolved.Values["name"] {
		t.Errorf("expected name=my-app (from app), got %v", resolved.Values["name"])
	}

	// Service layer value preserved (port from service layer)
	if 8080 != resolved.Values["port"] {
		t.Errorf("expected port=8080 (from service layer), got %v", resolved.Values["port"])
	}

	// Deep merge: image.repository from base, tag overridden by app
	image, ok := resolved.Values["image"].(map[string]any)
	if !ok {
		t.Fatal("expected image to be map")
	}
	if "nginx" != image["repository"] {
		t.Errorf("expected image.repository=nginx (from base), got %v", image["repository"])
	}
	if "2.0" != image["tag"] {
		t.Errorf("expected image.tag=2.0 (from app), got %v", image["tag"])
	}

	// Templates: base deployment, service service, app ingress
	if _, ok := resolved.Templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml from base layer")
	}
	if _, ok := resolved.Templates["service.yaml"]; !ok {
		t.Error("expected service.yaml from service layer")
	}
	if _, ok := resolved.Templates["ingress.yaml"]; !ok {
		t.Error("expected ingress.yaml from app")
	}
}

func TestResolve_LayerPrecedence(t *testing.T) {
	// Service layer has base as its own layer, so:
	// base -> service (which includes base) -> app
	// The service layer itself inherits base values and overrides replicas to 2
	dir := filepath.Join(fixturesDir(t), "layered", "service")
	resolved, err := Resolve(dir, "")
	if nil != err {
		t.Fatal(err)
	}

	// Service overrides replicas
	if 2 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=2 (from service), got %v", resolved.Values["replicas"])
	}

	// Base value inherited
	if "base-app" != resolved.Values["name"] {
		t.Errorf("expected name=base-app (from base), got %v", resolved.Values["name"])
	}

	// Service adds port
	if 8080 != resolved.Values["port"] {
		t.Errorf("expected port=8080 (from service), got %v", resolved.Values["port"])
	}

	// Both templates present
	if _, ok := resolved.Templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml from base layer")
	}
	if _, ok := resolved.Templates["service.yaml"]; !ok {
		t.Error("expected service.yaml from service itself")
	}
}

func TestResolve_CycleDetectionThroughLayers(t *testing.T) {
	tmpDir := t.TempDir()

	pkgA := filepath.Join(tmpDir, "a")
	pkgB := filepath.Join(tmpDir, "b")
	os.MkdirAll(pkgA, 0o755)
	os.MkdirAll(pkgB, 0o755)

	os.WriteFile(filepath.Join(pkgA, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: a
version: 1.0.0
layers:
  - name: b
    source: ../b
`), 0o644)

	os.WriteFile(filepath.Join(pkgB, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: b
version: 1.0.0
layers:
  - name: a
    source: ../a
`), 0o644)

	_, err := Resolve(pkgA, "")
	if nil == err {
		t.Fatal("expected cycle detection error through layers")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %s", err.Error())
	}
}

func TestResolve_SingleLayerLocal(t *testing.T) {
	tmpDir := t.TempDir()

	baseDir := filepath.Join(tmpDir, "base")
	appDir := filepath.Join(tmpDir, "app")
	os.MkdirAll(filepath.Join(baseDir, "templates"), 0o755)
	os.MkdirAll(filepath.Join(appDir, "templates"), 0o755)

	os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: base\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "values.yaml"), []byte("name: base-val\nreplicas: 1\n"), 0o644)
	os.WriteFile(filepath.Join(baseDir, "templates", "deploy.yaml"), []byte("kind: Deployment\n"), 0o644)

	os.WriteFile(filepath.Join(appDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: app
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0o644)
	os.WriteFile(filepath.Join(appDir, "values.yaml"), []byte("replicas: 5\n"), 0o644)
	os.WriteFile(filepath.Join(appDir, "templates", "svc.yaml"), []byte("kind: Service\n"), 0o644)

	resolved, err := Resolve(appDir, "")
	if nil != err {
		t.Fatal(err)
	}

	if "app" != resolved.Metadata.Name {
		t.Errorf("expected name=app, got %s", resolved.Metadata.Name)
	}

	// App wins on replicas
	if 5 != resolved.Values["replicas"] {
		t.Errorf("expected replicas=5, got %v", resolved.Values["replicas"])
	}

	// Base value inherited
	if "base-val" != resolved.Values["name"] {
		t.Errorf("expected name=base-val, got %v", resolved.Values["name"])
	}

	// Both templates present
	if _, ok := resolved.Templates["deploy.yaml"]; !ok {
		t.Error("expected deploy.yaml from base")
	}
	if _, ok := resolved.Templates["svc.yaml"]; !ok {
		t.Error("expected svc.yaml from app")
	}
}

func TestResolve_NoLayers(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: standalone\nversion: 1.0.0\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("key: value\n"), 0o644)

	resolved, err := Resolve(tmpDir, "")
	if nil != err {
		t.Fatal(err)
	}

	if "standalone" != resolved.Metadata.Name {
		t.Errorf("expected name=standalone, got %s", resolved.Metadata.Name)
	}
	if "value" != resolved.Values["key"] {
		t.Errorf("expected key=value, got %v", resolved.Values["key"])
	}
}
