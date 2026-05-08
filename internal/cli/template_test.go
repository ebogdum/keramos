package cli

import (
	"bytes"
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

func TestTemplateCommand_SimplePackage(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir})

	err := cmd.Execute()
	if nil != err {
		t.Fatalf("template command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "myapp") {
		t.Errorf("expected output to contain 'myapp', got:\n%s", output)
	}
	if !strings.Contains(output, "nginx:latest") {
		t.Errorf("expected output to contain 'nginx:latest', got:\n%s", output)
	}
}

func TestTemplateCommand_WithSet(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir, "--set", "name=overridden"})

	err := cmd.Execute()
	if nil != err {
		t.Fatalf("template command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "overridden") {
		t.Errorf("expected output to contain 'overridden', got:\n%s", output)
	}
}

func TestTemplateCommand_WithProfile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir, "--profile", "prod"})

	err := cmd.Execute()
	if nil != err {
		t.Fatalf("template command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "production") {
		t.Errorf("expected output to contain 'production', got:\n%s", output)
	}
}

func TestTemplateCommand_ShowOnly(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir, "--show-only", "service.yaml"})

	err := cmd.Execute()
	if nil != err {
		t.Fatalf("template command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Service") {
		t.Errorf("expected output to contain 'Service', got:\n%s", output)
	}
	// Should NOT contain deployment
	if strings.Contains(output, "Deployment") {
		t.Errorf("expected output to NOT contain 'Deployment' with --show-only service.yaml, got:\n%s", output)
	}
}

func TestTemplateCommand_ShowOnlyNotFound(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir, "--show-only", "nonexistent.yaml"})

	err := cmd.Execute()
	if nil == err {
		t.Fatal("expected error for nonexistent template")
	}
}

func TestTemplateCommand_MissingPackage(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", "/nonexistent/path"})

	err := cmd.Execute()
	if nil == err {
		t.Fatal("expected error for missing package")
	}
}

func TestTemplateCommand_WithValuesFile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "override.yaml")
	os.WriteFile(overridePath, []byte("name: custom-name\n"), 0o644)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", dir, "-f", overridePath})

	err := cmd.Execute()
	if nil != err {
		t.Fatalf("template command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "custom-name") {
		t.Errorf("expected output to contain 'custom-name', got:\n%s", output)
	}
}

func TestTemplateCommand_NoArgs(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template"})

	err := cmd.Execute()
	if nil == err {
		t.Fatal("expected error for missing args")
	}
}
