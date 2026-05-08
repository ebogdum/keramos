package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); nil != err {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	must("keramos.yaml", "apiVersion: keramos/v1\nname: showme\nversion: 1.2.3\n")
	must("values.yaml", "replica: 3\n")
	must("README.md", "# show test\n")

	crdDir := filepath.Join(dir, "crds")
	if err := os.MkdirAll(crdDir, 0o755); nil != err {
		t.Fatal(err)
	}
	must("crds/foo.yaml", "kind: CustomResourceDefinition\nmetadata:\n  name: foos.example\n")
	return dir
}

func TestShowChart(t *testing.T) {
	pkg := makeTestPackage(t)
	cmd := newShowChartCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{pkg}); nil != err {
		t.Fatalf("show chart: %v", err)
	}
	if !strings.Contains(buf.String(), "name: showme") {
		t.Errorf("expected name in output:\n%s", buf.String())
	}
}

func TestShowValues(t *testing.T) {
	pkg := makeTestPackage(t)
	cmd := newShowValuesCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{pkg}); nil != err {
		t.Fatalf("show values: %v", err)
	}
	if !strings.Contains(buf.String(), "replica: 3") {
		t.Errorf("expected values in output:\n%s", buf.String())
	}
}

func TestShowReadme(t *testing.T) {
	pkg := makeTestPackage(t)
	cmd := newShowReadmeCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{pkg}); nil != err {
		t.Fatalf("show readme: %v", err)
	}
	if !strings.Contains(buf.String(), "show test") {
		t.Errorf("expected README contents:\n%s", buf.String())
	}
}

func TestShowCRDs(t *testing.T) {
	pkg := makeTestPackage(t)
	cmd := newShowCRDsCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{pkg}); nil != err {
		t.Fatalf("show crds: %v", err)
	}
	if !strings.Contains(buf.String(), "foos.example") {
		t.Errorf("expected CRD name in output:\n%s", buf.String())
	}
}

func TestShowAll(t *testing.T) {
	pkg := makeTestPackage(t)
	cmd := newShowAllCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, []string{pkg}); nil != err {
		t.Fatalf("show all: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"# Chart", "# Values", "# README"} {
		if !strings.Contains(out, want) {
			t.Errorf("show all missing section %q\n%s", want, out)
		}
	}
}
