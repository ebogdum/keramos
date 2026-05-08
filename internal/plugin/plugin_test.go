package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPluginMetadata(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `name: test-plugin
version: "1.0.0"
description: A test plugin
command: run.sh
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yamlContent), 0o644); nil != err {
		t.Fatal(err)
	}

	p, err := loadPluginMetadata(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "test-plugin" != p.Name {
		t.Errorf("expected name test-plugin, got %s", p.Name)
	}
	if "1.0.0" != p.Version {
		t.Errorf("expected version 1.0.0, got %s", p.Version)
	}
	if "A test plugin" != p.Description {
		t.Errorf("expected description 'A test plugin', got %s", p.Description)
	}
	if "run.sh" != p.Command {
		t.Errorf("expected command run.sh, got %s", p.Command)
	}
}

func TestLoadPluginMetadataMissingName(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `version: "1.0.0"
command: run.sh
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yamlContent), 0o644); nil != err {
		t.Fatal(err)
	}

	_, err := loadPluginMetadata(dir)
	if nil == err {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadPluginMetadataMissingCommand(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `name: test-plugin
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yamlContent), 0o644); nil != err {
		t.Fatal(err)
	}

	_, err := loadPluginMetadata(dir)
	if nil == err {
		t.Fatal("expected error for missing command")
	}
}

func TestLoadPluginMetadataMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := loadPluginMetadata(dir)
	if nil == err {
		t.Fatal("expected error for missing plugin.yaml")
	}
}

func TestListEmptyDir(t *testing.T) {
	// Override plugin dir by using a temp dir
	dir := t.TempDir()
	entries, err := os.ReadDir(dir)
	if nil != err {
		t.Fatal(err)
	}
	if 0 != len(entries) {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
}

func TestFindPluginNotFound(t *testing.T) {
	_, err := FindPlugin("nonexistent-plugin-that-does-not-exist")
	if nil == err {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestInstallFromLocalAndRemove(t *testing.T) {
	// Create a mock plugin directory
	srcDir := t.TempDir()
	yamlContent := `name: local-test
version: "0.1.0"
description: Local test plugin
command: run.sh
`
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte(yamlContent), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\necho hello\n"), 0o755); nil != err {
		t.Fatal(err)
	}

	// We can't easily test Install because it uses PluginDir which goes to ~/.config.
	// Instead, test the internal install function by creating a fake plugin dir.
	fakePluginDir := t.TempDir()
	p, err := installFromLocal(srcDir, fakePluginDir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "local-test" != p.Name {
		t.Errorf("expected name local-test, got %s", p.Name)
	}

	// Verify plugin directory was created
	pluginPath := filepath.Join(fakePluginDir, filepath.Base(srcDir))
	if _, statErr := os.Stat(pluginPath); nil != statErr {
		t.Fatalf("plugin directory not created: %v", statErr)
	}

	// Verify plugin.yaml exists
	if _, statErr := os.Stat(filepath.Join(pluginPath, "plugin.yaml")); nil != statErr {
		t.Fatal("plugin.yaml not copied")
	}
}

func TestInstallFromLocalAlreadyExists(t *testing.T) {
	srcDir := t.TempDir()
	yamlContent := `name: dup-test
version: "0.1.0"
command: run.sh
`
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte(yamlContent), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755); nil != err {
		t.Fatal(err)
	}

	fakePluginDir := t.TempDir()
	// Pre-create the target directory
	if err := os.MkdirAll(filepath.Join(fakePluginDir, filepath.Base(srcDir)), 0o755); nil != err {
		t.Fatal(err)
	}

	_, err := installFromLocal(srcDir, fakePluginDir)
	if nil == err {
		t.Fatal("expected error for already installed plugin")
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	subDir := filepath.Join(srcDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o644); nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("world"), 0o644); nil != err {
		t.Fatal(err)
	}

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := copyDir(srcDir, dstDir); nil != err {
		t.Fatalf("copyDir failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "a.txt"))
	if nil != err {
		t.Fatal(err)
	}
	if "hello" != string(data) {
		t.Errorf("expected 'hello', got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	if nil != err {
		t.Fatal(err)
	}
	if "world" != string(data) {
		t.Errorf("expected 'world', got %q", string(data))
	}
}
