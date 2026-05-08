package repo

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// createMaliciousTarGz creates a tar.gz with a malicious entry for testing.
func createMaliciousTarGz(t *testing.T, destPath string, entries []tar.Header) {
	t.Helper()

	f, err := os.Create(destPath)
	if nil != err {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, h := range entries {
		header := h
		if err := tw.WriteHeader(&header); nil != err {
			t.Fatal(err)
		}
		if tar.TypeReg == header.Typeflag && 0 < header.Size {
			data := make([]byte, header.Size)
			if _, err := tw.Write(data); nil != err {
				t.Fatal(err)
			}
		}
	}
}

func TestExtractArchive_RejectsPathTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "malicious.tgz")
	extractDir := t.TempDir()

	createMaliciousTarGz(t, archivePath, []tar.Header{
		{
			Name:     "../../../etc/passwd",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     5,
		},
	})

	err := ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestExtractArchive_RejectsAbsolutePath(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "malicious.tgz")
	extractDir := t.TempDir()

	createMaliciousTarGz(t, archivePath, []tar.Header{
		{
			Name:     "/etc/passwd",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     5,
		},
	})

	err := ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestExtractArchive_RejectsSymlinkEscape(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "malicious.tgz")
	extractDir := t.TempDir()

	createMaliciousTarGz(t, archivePath, []tar.Header{
		{
			Name:     "evil-link",
			Typeflag: tar.TypeSymlink,
			Linkname: "../../../etc/passwd",
		},
	})

	err := ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for symlink escape, got nil")
	}
}

func TestExtractArchive_AllowsSafeEntries(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "safe.tgz")
	extractDir := t.TempDir()

	createMaliciousTarGz(t, archivePath, []tar.Header{
		{
			Name:     "mypackage/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		},
		{
			Name:     "mypackage/values.yaml",
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     10,
		},
	})

	err := ExtractArchive(archivePath, extractDir)
	if nil != err {
		t.Fatalf("expected safe archive to extract, got: %v", err)
	}

	valuesPath := filepath.Join(extractDir, "mypackage", "values.yaml")
	if _, statErr := os.Stat(valuesPath); nil != statErr {
		t.Fatalf("expected values.yaml to exist: %v", statErr)
	}
}

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		name     string
		destDir  string
		target   string
		expected bool
	}{
		{"safe nested", "/tmp/dest", "/tmp/dest/sub/file.txt", true},
		{"safe root", "/tmp/dest", "/tmp/dest/file.txt", true},
		{"traversal", "/tmp/dest", "/tmp/other/file.txt", false},
		{"parent", "/tmp/dest", "/tmp/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSafePath(tt.destDir, tt.target)
			if tt.expected != result {
				t.Errorf("isSafePath(%q, %q) = %v, want %v", tt.destDir, tt.target, result, tt.expected)
			}
		})
	}
}
