package deptree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureDir() string {
	// Navigate from internal/deptree to project root
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "test", "fixtures")
}

func TestBuildSimpleTree(t *testing.T) {
	root, err := Build(filepath.Join(fixtureDir(), "simple"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if "simple-app" != root.Name {
		t.Errorf("expected root name 'simple-app', got %q", root.Name)
	}
	if KindRoot != root.Kind {
		t.Errorf("expected KindRoot, got %d", root.Kind)
	}
	if 0 != root.Depth {
		t.Errorf("expected depth 0, got %d", root.Depth)
	}
	if 0 != len(root.Children) {
		t.Errorf("expected 0 children, got %d", len(root.Children))
	}
}

func TestBuildWithLayers(t *testing.T) {
	root, err := Build(filepath.Join(fixtureDir(), "with-base", "overlay"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if "overlay-app" != root.Name {
		t.Errorf("expected root name 'overlay-app', got %q", root.Name)
	}
	if 1 != len(root.Children) {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}

	child := root.Children[0]
	if "base" != child.Name {
		t.Errorf("expected child name 'base', got %q", child.Name)
	}
	if KindLayer != child.Kind {
		t.Errorf("expected KindLayer, got %d", child.Kind)
	}
	if 1 != child.Depth {
		t.Errorf("expected depth 1, got %d", child.Depth)
	}
	if root != child.Parent {
		t.Error("expected child.Parent to be root")
	}
}

func TestBuildNestedLayers(t *testing.T) {
	// app -> base, service; service -> base
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if "app" != root.Name {
		t.Errorf("expected root name 'app', got %q", root.Name)
	}
	if 2 != len(root.Children) {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}

	base := root.Children[0]
	service := root.Children[1]

	if "base" != base.Name {
		t.Errorf("expected first child 'base', got %q", base.Name)
	}
	if "service" != service.Name {
		t.Errorf("expected second child 'service', got %q", service.Name)
	}

	// service should have base as its child
	if 1 != len(service.Children) {
		t.Fatalf("expected service to have 1 child, got %d", len(service.Children))
	}
	if "base" != service.Children[0].Name {
		t.Errorf("expected service's child to be 'base', got %q", service.Children[0].Name)
	}
	if 2 != service.Children[0].Depth {
		t.Errorf("expected depth 2, got %d", service.Children[0].Depth)
	}
}

func TestBuildWithRequires(t *testing.T) {
	// Create a temp fixture with requires
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	reqDir := filepath.Join(tmpDir, "prometheus")

	_ = os.MkdirAll(mainDir, 0755)
	_ = os.MkdirAll(reqDir, 0755)

	_ = os.WriteFile(filepath.Join(mainDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: main-app
version: 1.0.0
requires:
  - name: prometheus
    source: ../prometheus
`), 0644)

	_ = os.WriteFile(filepath.Join(reqDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: prometheus
version: 1.0.0
`), 0644)

	root, err := Build(mainDir)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if 1 != len(root.Children) {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}

	req := root.Children[0]
	if "prometheus" != req.Name {
		t.Errorf("expected child name 'prometheus', got %q", req.Name)
	}
	if KindRequire != req.Kind {
		t.Errorf("expected KindRequire, got %d", req.Kind)
	}
}

func TestBuildMixedLayersAndRequires(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	baseDir := filepath.Join(tmpDir, "base")
	reqDir := filepath.Join(tmpDir, "redis")

	_ = os.MkdirAll(mainDir, 0755)
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(reqDir, 0755)

	_ = os.WriteFile(filepath.Join(mainDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: main-app
version: 1.0.0
layers:
  - name: base
    source: ../base
requires:
  - name: redis
    source: ../redis
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(reqDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: redis
version: 1.0.0
`), 0644)

	root, err := Build(mainDir)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if 2 != len(root.Children) {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}
	if KindLayer != root.Children[0].Kind {
		t.Errorf("expected first child to be KindLayer")
	}
	if KindRequire != root.Children[1].Kind {
		t.Errorf("expected second child to be KindRequire")
	}
}

func TestBuildCycleDetection(t *testing.T) {
	tmpDir := t.TempDir()
	aDir := filepath.Join(tmpDir, "a")
	bDir := filepath.Join(tmpDir, "b")

	_ = os.MkdirAll(aDir, 0755)
	_ = os.MkdirAll(bDir, 0755)

	_ = os.WriteFile(filepath.Join(aDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: a
version: 1.0.0
layers:
  - name: b
    source: ../b
`), 0644)
	_ = os.WriteFile(filepath.Join(bDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: b
version: 1.0.0
layers:
  - name: a
    source: ../a
`), 0644)

	_, err := Build(aDir)
	if nil == err {
		t.Fatal("expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected error to mention 'cycle', got %q", err.Error())
	}
}

func TestWalkLayersOrder(t *testing.T) {
	// app -> base, service; service -> base
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	nodes := WalkLayers(root)

	// Expected order: base (under app), base (under service), service, app
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}

	// Deepest first: base (from app's child), base (from service's child), service, app
	if 4 != len(nodes) {
		t.Fatalf("expected 4 nodes, got %d: %v", len(nodes), names)
	}

	// The first node should be a base (deepest)
	if "base" != names[0] {
		t.Errorf("expected first node to be 'base', got %q", names[0])
	}
	// Last node should be app (root)
	if "app" != names[len(names)-1] {
		t.Errorf("expected last node to be 'app', got %q", names[len(names)-1])
	}
}

func TestWalkRequires(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main")
	baseDir := filepath.Join(tmpDir, "base")
	reqDir := filepath.Join(tmpDir, "redis")

	_ = os.MkdirAll(mainDir, 0755)
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.MkdirAll(reqDir, 0755)

	_ = os.WriteFile(filepath.Join(mainDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: main-app
version: 1.0.0
layers:
  - name: base
    source: ../base
requires:
  - name: redis
    source: ../redis
`), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: base
version: 1.0.0
`), 0644)
	_ = os.WriteFile(filepath.Join(reqDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: redis
version: 1.0.0
`), 0644)

	root, err := Build(mainDir)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	requires := WalkRequires(root)
	if 1 != len(requires) {
		t.Fatalf("expected 1 require, got %d", len(requires))
	}
	if "redis" != requires[0].Name {
		t.Errorf("expected require name 'redis', got %q", requires[0].Name)
	}
}

func TestPrintTree(t *testing.T) {
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	output := PrintTree(root)
	if !strings.Contains(output, "app@2.0.0") {
		t.Errorf("expected output to contain 'app@2.0.0', got:\n%s", output)
	}
	if !strings.Contains(output, "[layer]") {
		t.Errorf("expected output to contain '[layer]', got:\n%s", output)
	}
	if !strings.Contains(output, "base") {
		t.Errorf("expected output to contain 'base', got:\n%s", output)
	}
	if !strings.Contains(output, "service") {
		t.Errorf("expected output to contain 'service', got:\n%s", output)
	}
}

func TestPopulate(t *testing.T) {
	root, err := Build(filepath.Join(fixtureDir(), "layered", "app"))
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if popErr := Populate(root); nil != popErr {
		t.Fatalf("expected no error from Populate, got %v", popErr)
	}

	// Root (app) should have values
	if nil == root.Values {
		t.Fatal("expected root values to be populated")
	}
	if _, ok := root.Values["replicas"]; !ok {
		t.Error("expected root values to contain 'replicas'")
	}

	// Check base child has values
	baseNode := root.Children[0]
	if nil == baseNode.Values {
		t.Fatal("expected base values to be populated")
	}
	if _, ok := baseNode.Values["image"]; !ok {
		t.Error("expected base values to contain 'image'")
	}

	// Check templates are loaded
	if 0 == len(root.Templates) {
		t.Error("expected root to have templates")
	}
}
