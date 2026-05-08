package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCRDs_NoDir(t *testing.T) {
	dir := t.TempDir()
	out, err := loadCRDs(dir)
	if nil != err {
		t.Fatalf("missing crds/ should not error: %v", err)
	}
	if "" != out {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestLoadCRDs_Concatenates(t *testing.T) {
	dir := t.TempDir()
	crdDir := filepath.Join(dir, "crds")
	if err := os.MkdirAll(crdDir, 0o755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crdDir, "a.yaml"), []byte("kind: A"), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crdDir, "b.yaml"), []byte("kind: B"), 0o644); nil != err {
		t.Fatal(err)
	}
	out, err := loadCRDs(dir)
	if nil != err {
		t.Fatalf("loadCRDs: %v", err)
	}
	if !strings.Contains(out, "kind: A") || !strings.Contains(out, "kind: B") {
		t.Errorf("output missing CRDs: %s", out)
	}
}

func TestLoadCRDs_Recursive(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "crds", "sub")
	if err := os.MkdirAll(nested, 0o755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.yaml"), []byte("kind: Deep"), 0o644); nil != err {
		t.Fatal(err)
	}
	out, _ := loadCRDs(dir)
	if !strings.Contains(out, "kind: Deep") {
		t.Errorf("recursive walk missed nested CRD: %s", out)
	}
}
