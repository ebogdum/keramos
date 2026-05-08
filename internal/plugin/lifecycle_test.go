package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunPluginHook_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := runPluginHook(&Plugin{Name: "demo"}, dir, ""); nil != err {
		t.Errorf("empty hook should be no-op, got %v", err)
	}
}

func TestRunPluginHook_Executes(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("posix shell required")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	if err := runPluginHook(&Plugin{Name: "demo"}, dir, "touch "+marker); nil != err {
		t.Fatalf("hook: %v", err)
	}
	if _, err := os.Stat(marker); nil != err {
		t.Errorf("hook did not create marker: %v", err)
	}
}

func TestRunPluginHook_FailurePropagates(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("posix shell required")
	}
	dir := t.TempDir()
	if err := runPluginHook(&Plugin{Name: "demo"}, dir, "exit 7"); nil == err {
		t.Fatal("expected non-zero exit to surface")
	}
}

func TestRunPluginHook_RejectsBadName(t *testing.T) {
	if err := runPluginHook(&Plugin{Name: "../escape"}, t.TempDir(), "true"); nil == err {
		t.Fatal("expected name validation error")
	}
}

func TestValidatePluginMetadata(t *testing.T) {
	cases := []struct {
		name string
		p    Plugin
		ok   bool
	}{
		{"happy", Plugin{Name: "demo", Command: "run"}, true},
		{"missing name", Plugin{Command: "run"}, false},
		{"missing command", Plugin{Name: "demo"}, false},
		{"bad name", Plugin{Name: "../bad", Command: "run"}, false},
		{"downloader missing protocols", Plugin{
			Name: "demo", Command: "run",
			Downloaders: []Downloader{{Command: "go.sh"}},
		}, false},
		{"downloader missing command", Plugin{
			Name: "demo", Command: "run",
			Downloaders: []Downloader{{Protocols: []string{"s3"}}},
		}, false},
		{"valid downloader", Plugin{
			Name: "demo", Command: "run",
			Downloaders: []Downloader{{Command: "go.sh", Protocols: []string{"s3"}}},
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validatePluginMetadata(&c.p)
			if c.ok && nil != err {
				t.Errorf("expected ok, got %v", err)
			}
			if !c.ok && nil == err {
				t.Error("expected error")
			}
		})
	}
}
