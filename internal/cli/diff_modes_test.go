package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// makeDiffPkg writes a minimal renderable package with a parameterised image
// and returns its directory.
func makeDiffPkg(t *testing.T, image string) string {
	t.Helper()
	dir := t.TempDir()
	w := func(name, body string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); nil != err {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); nil != err {
			t.Fatal(err)
		}
	}
	w("keramos.yaml", "apiVersion: keramos/v1\nname: dpkg\nversion: 1.0.0\n")
	w("values.yaml", "image: "+image+"\n")
	w("templates/deploy.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: dpkg\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: \"${values.image}\"\n")
	return dir
}

func runDiffCmd(t *testing.T, args []string, flags map[string]string) (string, error) {
	t.Helper()
	cmd := newDiffCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); nil != err {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// Mode 1: two package directories.
func TestDiffTwoPackageDirs(t *testing.T) {
	a := makeDiffPkg(t, "nginx:1.24")
	b := makeDiffPkg(t, "nginx:1.25")
	out, err := runDiffCmd(t, []string{a, b}, map[string]string{"no-color": "true"})
	if nil != err {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if !contains(out, "1.24") || !contains(out, "1.25") {
		t.Fatalf("expected both image tags in diff:\n%s", out)
	}
	if !contains(out, "1 changed") {
		t.Fatalf("expected summary '1 changed':\n%s", out)
	}
}

// Mode 2: two rendered manifest files.
func TestDiffTwoManifestFiles(t *testing.T) {
	dir := t.TempDir()
	fa := filepath.Join(dir, "a.yaml")
	fb := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(fa, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  k: old\n"), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(fb, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  k: new\n"), 0o644); nil != err {
		t.Fatal(err)
	}
	out, err := runDiffCmd(t, []string{fa, fb}, map[string]string{"no-color": "true"})
	if nil != err {
		t.Fatalf("diff files: %v\n%s", err, out)
	}
	if !contains(out, "old") || !contains(out, "new") {
		t.Fatalf("expected old→new in diff:\n%s", out)
	}
}

// Mode 3: one package, two value sets.
func TestDiffSameDirTwoValueSets(t *testing.T) {
	pkg := makeDiffPkg(t, "nginx:1.24")
	out, err := runDiffCmd(t, []string{pkg}, map[string]string{
		"to-set":   "image=nginx:9.9",
		"no-color": "true",
	})
	if nil != err {
		t.Fatalf("diff value-sets: %v\n%s", err, out)
	}
	if !contains(out, "9.9") {
		t.Fatalf("expected the to-set override in diff:\n%s", out)
	}
	if !contains(out, "1 changed") {
		t.Fatalf("expected '1 changed':\n%s", out)
	}
}

// Identical inputs report no differences.
func TestDiffNoDifferences(t *testing.T) {
	pkg := makeDiffPkg(t, "nginx:1.24")
	out, err := runDiffCmd(t, []string{pkg, pkg}, map[string]string{"no-color": "true"})
	if nil != err {
		t.Fatalf("diff: %v", err)
	}
	if !contains(out, "No differences") {
		t.Fatalf("expected no differences:\n%s", out)
	}
}

// Mixed dir + file is rejected clearly.
func TestDiffMixedInputsRejected(t *testing.T) {
	pkg := makeDiffPkg(t, "nginx:1.24")
	f := filepath.Join(t.TempDir(), "m.yaml")
	_ = os.WriteFile(f, []byte("kind: ConfigMap\n"), 0o644)
	_, err := runDiffCmd(t, []string{pkg, f}, nil)
	if nil == err {
		t.Fatal("expected error for dir+file mix")
	}
}
