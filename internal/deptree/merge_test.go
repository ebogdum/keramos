package deptree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeValuesSimple(t *testing.T) {
	// base values + root values, root wins
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(rootDir, 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "values.yaml"), []byte(`replicas: 1
image:
  repository: nginx
  tag: "1.0"
`), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "values.yaml"), []byte(`replicas: 3
`), 0644)

	root, err := Build(rootDir)
	if nil != err {
		t.Fatalf("build error: %v", err)
	}
	if popErr := Populate(root); nil != popErr {
		t.Fatalf("populate error: %v", popErr)
	}

	merged, mergeErr := MergeValues(root)
	if nil != mergeErr {
		t.Fatalf("merge error: %v", mergeErr)
	}

	// Root's replicas (3) should win over base's (1)
	if 3 != merged["replicas"] {
		t.Errorf("expected replicas=3, got %v", merged["replicas"])
	}

	// Base's image should be present
	imageMap, ok := merged["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image to be a map, got %T", merged["image"])
	}
	if "nginx" != imageMap["repository"] {
		t.Errorf("expected image.repository='nginx', got %v", imageMap["repository"])
	}
}

func TestMergeValuesOverridePrecedence(t *testing.T) {
	// root wins over layer
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(rootDir, 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "values.yaml"), []byte(`image:
  repository: nginx
  tag: "1.0"
`), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "values.yaml"), []byte(`image:
  tag: "2.0"
`), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	merged, _ := MergeValues(root)

	imageMap := merged["image"].(map[string]any)
	if "2.0" != imageMap["tag"] {
		t.Errorf("expected image.tag='2.0', got %v", imageMap["tag"])
	}
	if "nginx" != imageMap["repository"] {
		t.Errorf("expected image.repository='nginx' from base, got %v", imageMap["repository"])
	}
}

func TestMergeValuesMultiLevel(t *testing.T) {
	// app -> service -> base
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("build error: %v", err)
	}
	if popErr := Populate(root); nil != popErr {
		t.Fatalf("populate error: %v", popErr)
	}

	merged, mergeErr := MergeValues(root)
	if nil != mergeErr {
		t.Fatalf("merge error: %v", mergeErr)
	}

	// app's replicas (3) should win
	if 3 != merged["replicas"] {
		t.Errorf("expected replicas=3 from app, got %v", merged["replicas"])
	}

	// app's image.tag should be "2.0"
	imageMap, ok := merged["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image to be a map, got %T", merged["image"])
	}
	if "2.0" != imageMap["tag"] {
		t.Errorf("expected image.tag='2.0' from app, got %v", imageMap["tag"])
	}
}

func TestMergeValuesScopedLayerOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(rootDir, 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "values.yaml"), []byte(`namespace: default
replicas: 1
`), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "values.yaml"), []byte(`layers:
  base:
    namespace: monitoring
`), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	merged, _ := MergeValues(root)

	// Scoped override should set namespace to "monitoring"
	if "monitoring" != merged["namespace"] {
		t.Errorf("expected namespace='monitoring' from scoped override, got %v", merged["namespace"])
	}
}

func TestMergeValuesForcedPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(rootDir, 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "values.yaml"), []byte(`namespace: default
`), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "values.yaml"), []byte(`layers:
  base:
    "!namespace": forced-ns
`), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	merged, _ := MergeValues(root)

	if "forced-ns" != merged["namespace"] {
		t.Errorf("expected namespace='forced-ns' from forced precedence, got %v", merged["namespace"])
	}
}

func TestMergeValuesScopedRequires(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	reqDir := filepath.Join(tmpDir, "prometheus")

	_ = os.MkdirAll(rootDir, 0755)
	_ = os.MkdirAll(reqDir, 0755)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
requires:
  - name: prometheus
    source: ../prometheus
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "values.yaml"), []byte(`requires:
  prometheus:
    retention: 30d
`), 0644)

	_ = os.WriteFile(filepath.Join(reqDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: prometheus
version: 1.0.0
`), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	// Test scoped require values extraction
	scoped := ScopedRequireValues(root.Values, "prometheus")
	if nil == scoped {
		t.Fatal("expected scoped require values, got nil")
	}
	if "30d" != scoped["retention"] {
		t.Errorf("expected retention='30d', got %v", scoped["retention"])
	}
}

func TestMergeTemplatesOverrideAndAdd(t *testing.T) {
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("build error: %v", err)
	}
	if popErr := Populate(root); nil != popErr {
		t.Fatalf("populate error: %v", popErr)
	}

	templates, _, tmplErr := MergeTemplates(root)
	if nil != tmplErr {
		t.Fatalf("merge error: %v", tmplErr)
	}

	// app has ingress.yaml, base has deployment.yaml, service has service.yaml
	if _, ok := templates["ingress.yaml"]; !ok {
		t.Error("expected ingress.yaml from app")
	}
	if _, ok := templates["deployment.yaml"]; !ok {
		t.Error("expected deployment.yaml from base")
	}
	if _, ok := templates["service.yaml"]; !ok {
		t.Error("expected service.yaml from service layer")
	}
}

func TestMergeHooksAdditive(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(filepath.Join(baseDir, "hooks"), 0755)
	_ = os.MkdirAll(filepath.Join(rootDir, "hooks"), 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "hooks", "pre-install.yaml"), []byte("base pre-install"), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "hooks", "post-install.yaml"), []byte("root post-install"), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	hooks := MergeHooks(root)
	if 2 != len(hooks) {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}
	if _, ok := hooks["pre-install.yaml"]; !ok {
		t.Error("expected pre-install.yaml from base")
	}
	if _, ok := hooks["post-install.yaml"]; !ok {
		t.Error("expected post-install.yaml from root")
	}
}

func TestMergeTestsAdditive(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(filepath.Join(baseDir, "tests"), 0755)
	_ = os.MkdirAll(filepath.Join(rootDir, "tests"), 0755)

	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "tests", "base-test.yaml"), []byte("base test"), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: base
    source: ../base
`), 0644)
	_ = os.WriteFile(filepath.Join(rootDir, "tests", "root-test.yaml"), []byte("root test"), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	tests := MergeTests(root)
	if 2 != len(tests) {
		t.Errorf("expected 2 tests, got %d", len(tests))
	}
}

func TestMergeValuesDeclarationOrder(t *testing.T) {
	tmpDir := t.TempDir()
	layer1Dir := filepath.Join(tmpDir, "layer1")
	layer2Dir := filepath.Join(tmpDir, "layer2")
	rootDir := filepath.Join(tmpDir, "root")

	_ = os.MkdirAll(layer1Dir, 0755)
	_ = os.MkdirAll(layer2Dir, 0755)
	_ = os.MkdirAll(rootDir, 0755)

	_ = os.WriteFile(filepath.Join(layer1Dir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: layer1
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(layer1Dir, "values.yaml"), []byte(`color: red
`), 0644)

	_ = os.WriteFile(filepath.Join(layer2Dir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: layer2
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(layer2Dir, "values.yaml"), []byte(`color: blue
`), 0644)

	_ = os.WriteFile(filepath.Join(rootDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: root
version: 1.0.0
layers:
  - name: layer1
    source: ../layer1
  - name: layer2
    source: ../layer2
`), 0644)

	root, _ := Build(rootDir)
	_ = Populate(root)

	merged, _ := MergeValues(root)

	// layer2 declared after layer1, so layer2 wins for same-level
	if "blue" != merged["color"] {
		t.Errorf("expected color='blue' (layer2 declared last at same level), got %v", merged["color"])
	}
}
