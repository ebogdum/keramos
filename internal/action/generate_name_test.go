package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallDryRunWithGeneratedName(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	// Simulate what the CLI does: provide a release name with a suffix
	opts := &InstallOptions{
		ReleaseName: "test-app-ab12c",
		Namespace:   "default",
		DryRun:      "client",
		Timeout:     5 * time.Minute,
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("dry-run install failed: %v", err)
	}

	if "test-app-ab12c" != rel.Name {
		t.Errorf("expected release name test-app-ab12c, got %s", rel.Name)
	}
}

func TestInstallServerDryRunRequiresClient(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	opts := &InstallOptions{
		ReleaseName: "server-dry",
		Namespace:   "default",
		DryRun:      "server",
		Timeout:     5 * time.Minute,
	}

	_, err := Install(nil, tmpDir, opts)
	if nil == err {
		t.Fatal("expected error for server dry-run without client")
	}
	if !strings.Contains(err.Error(), "server-side dry-run requires a cluster connection") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInstallDryRunModes(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	tests := []struct {
		name   string
		dryRun string
	}{
		{"client", "client"},
		{"empty-runs-normal", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &InstallOptions{
				ReleaseName: "mode-test",
				Namespace:   "default",
				DryRun:      tt.dryRun,
				Timeout:     5 * time.Minute,
			}

			if "" == tt.dryRun {
				// Normal mode requires a client; skip actual execution
				return
			}

			rel, err := Install(nil, tmpDir, opts)
			if nil != err {
				t.Fatalf("install failed: %v", err)
			}

			if "pending-install" != string(rel.Status) {
				t.Errorf("expected status pending-install, got %s", rel.Status)
			}
		})
	}
}

func TestInstallNotesRendered(t *testing.T) {
	tmpDir := t.TempDir()
	createPackageWithTemplate(t, tmpDir)

	// Add notes.yaml
	notesDir := filepath.Join(tmpDir, "templates")
	notesContent := "message: |\n  Thank you for installing test-app.\n"
	if err := os.WriteFile(filepath.Join(notesDir, "notes.yaml"), []byte(notesContent), 0o644); nil != err {
		t.Fatalf("failed to write notes.yaml: %v", err)
	}

	opts := &InstallOptions{
		ReleaseName: "notes-test",
		Namespace:   "default",
		DryRun:      "client",
	}

	rel, err := Install(nil, tmpDir, opts)
	if nil != err {
		t.Fatalf("install failed: %v", err)
	}

	if "" == rel.Notes {
		t.Error("expected non-empty notes")
	}
}
