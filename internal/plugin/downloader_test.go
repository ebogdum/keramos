package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func writePluginYAML(t *testing.T, root string, plug Plugin) string {
	t.Helper()
	dir := filepath.Join(root, plug.Name)
	if err := os.MkdirAll(dir, 0o755); nil != err {
		t.Fatalf("mkdir: %v", err)
	}
	body, err := yamlMarshal(plug)
	if nil != err {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), body, 0o644); nil != err {
		t.Fatalf("write: %v", err)
	}
	// Make a stub command file referenced by the plugin.
	if "" != plug.Command {
		cmdPath := filepath.Join(dir, plug.Command)
		if err := os.WriteFile(cmdPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); nil != err {
			t.Fatalf("write cmd: %v", err)
		}
	}
	return dir
}

// yamlMarshal is a tiny helper that imports yaml only inside the test.
func yamlMarshal(p Plugin) ([]byte, error) {
	return yaml.Marshal(p)
}

func TestFindDownloader_FindsByScheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir) // so PluginDir() resolves under our temp tree
	pluginRoot := filepath.Join(dir, ".config", "keramos", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); nil != err {
		t.Fatal(err)
	}
	writePluginYAML(t, pluginRoot, Plugin{
		Name:    "s3-getter",
		Version: "0.1.0",
		Command: "downloader.sh",
		Downloaders: []Downloader{
			{Command: "downloader.sh", Protocols: []string{"s3"}},
		},
	})

	p, cmd, found := FindDownloader("s3://bucket/key")
	if !found {
		t.Fatal("expected downloader to be found")
	}
	if "s3-getter" != p.Name {
		t.Errorf("plugin = %s", p.Name)
	}
	if "downloader.sh" != cmd {
		t.Errorf("cmd = %s", cmd)
	}
}

func TestFindDownloader_CompoundScheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pluginRoot := filepath.Join(dir, ".config", "keramos", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); nil != err {
		t.Fatal(err)
	}
	writePluginYAML(t, pluginRoot, Plugin{
		Name:    "s3-mtls",
		Version: "0.1.0",
		Command: "go.sh",
		Downloaders: []Downloader{
			{Command: "go.sh", Protocols: []string{"s3"}},
		},
	})

	// Compound `s3+https://...` should match the `s3` declared protocol.
	if _, _, found := FindDownloader("s3+https://bucket/key"); !found {
		t.Error("compound scheme not matched")
	}
}

func TestFindDownloader_Deterministic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pluginRoot := filepath.Join(dir, ".config", "keramos", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); nil != err {
		t.Fatal(err)
	}
	writePluginYAML(t, pluginRoot, Plugin{
		Name: "z-getter", Version: "0.1.0", Command: "z.sh",
		Downloaders: []Downloader{{Command: "z.sh", Protocols: []string{"foo"}}},
	})
	writePluginYAML(t, pluginRoot, Plugin{
		Name: "a-getter", Version: "0.1.0", Command: "a.sh",
		Downloaders: []Downloader{{Command: "a.sh", Protocols: []string{"foo"}}},
	})

	// Plugins are scanned in name order — `a-getter` should win.
	p, _, _ := FindDownloader("foo://x")
	if "a-getter" != p.Name {
		t.Errorf("expected a-getter to win deterministic ordering, got %q", p.Name)
	}
}

func TestFindDownloader_NoMatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pluginRoot := filepath.Join(dir, ".config", "keramos", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); nil != err {
		t.Fatal(err)
	}
	if _, _, found := FindDownloader("https://example.com"); found {
		t.Error("https should not match (no plugin claims it)")
	}
	if _, _, found := FindDownloader(""); found {
		t.Error("empty URL should not match")
	}
}
