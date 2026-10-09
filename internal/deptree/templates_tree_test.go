package deptree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplatesWalksSubdirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "templates", "worker")
	if err := os.MkdirAll(nested, 0o755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "service.yaml"), []byte("kind: Service\n"), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deployment.yaml"), []byte("kind: Deployment\n"), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "_helpers.yaml"), []byte("worker.labels:\n  app: worker\n"), 0o644); nil != err {
		t.Fatal(err)
	}

	templates, partials, err := loadTemplates(root)
	if nil != err {
		t.Fatalf("loadTemplates: %v", err)
	}
	if _, ok := templates["worker/deployment.yaml"]; !ok {
		t.Errorf("nested template not loaded: %v", templates)
	}
	if _, ok := templates["service.yaml"]; !ok {
		t.Errorf("top-level template not loaded: %v", templates)
	}
	if _, ok := partials["worker.labels"]; !ok {
		t.Errorf("nested partial not loaded: %v", partials)
	}
	if _, ok := templates["worker/_helpers.yaml"]; ok {
		t.Error("partial file was loaded as a template")
	}
}
