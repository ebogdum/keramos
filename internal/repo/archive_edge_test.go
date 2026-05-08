package repo

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractArchiveAbsolutePathRejected(t *testing.T) {
	// Create an archive with an absolute path entry
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "malicious.keramos.tgz")

	f, err := os.Create(archivePath)
	if nil != err {
		t.Fatal(err)
	}

	gzWriter := gzip.NewWriter(f)
	tw := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name:     "/etc/passwd",
		Size:     5,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
	}
	tw.WriteHeader(header)
	tw.Write([]byte("evil\n"))
	tw.Close()
	gzWriter.Close()
	f.Close()

	extractDir := t.TempDir()
	err = ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for archive with absolute path")
	}
}

func TestExtractArchivePathTraversalRejected(t *testing.T) {
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "traversal.keramos.tgz")

	f, err := os.Create(archivePath)
	if nil != err {
		t.Fatal(err)
	}

	gzWriter := gzip.NewWriter(f)
	tw := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name:     "../../../etc/shadow",
		Size:     5,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
	}
	tw.WriteHeader(header)
	tw.Write([]byte("evil\n"))
	tw.Close()
	gzWriter.Close()
	f.Close()

	extractDir := t.TempDir()
	err = ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for archive with path traversal")
	}
}

func TestExtractArchiveSymlinkEscapeRejected(t *testing.T) {
	archiveDir := t.TempDir()
	archivePath := filepath.Join(archiveDir, "symlink.keramos.tgz")

	f, err := os.Create(archivePath)
	if nil != err {
		t.Fatal(err)
	}

	gzWriter := gzip.NewWriter(f)
	tw := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name:     "pkg/link",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../../../etc/passwd",
		Mode:     0o777,
	}
	tw.WriteHeader(header)
	tw.Close()
	gzWriter.Close()
	f.Close()

	extractDir := t.TempDir()
	err = ExtractArchive(archivePath, extractDir)
	if nil == err {
		t.Fatal("expected error for symlink escaping destination")
	}

	// The symlink should not have been created
	linkPath := filepath.Join(extractDir, "pkg", "link")
	if _, statErr := os.Lstat(linkPath); !os.IsNotExist(statErr) {
		t.Error("symlink escaping destination should not be created")
	}
}

func TestPackageArchiveMissingKeramosYaml(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// No keramos.yaml present
	_, err := PackageArchive(srcDir, destDir, "")
	if nil == err {
		t.Fatal("expected error for missing keramos.yaml")
	}
}

func TestPackageArchiveAndExtractRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	extractDir := t.TempDir()

	createTestPackage(t, srcDir)

	archivePath, err := PackageArchive(srcDir, destDir, "")
	if nil != err {
		t.Fatalf("PackageArchive failed: %v", err)
	}

	if err := ExtractArchive(archivePath, extractDir); nil != err {
		t.Fatalf("ExtractArchive failed: %v", err)
	}

	// Verify key files present
	keramosPath := filepath.Join(extractDir, "testpkg", "keramos.yaml")
	if _, err := os.Stat(keramosPath); nil != err {
		t.Errorf("keramos.yaml missing after round-trip: %v", err)
	}
	tmplPath := filepath.Join(extractDir, "testpkg", "templates", "deployment.yaml")
	if _, err := os.Stat(tmplPath); nil != err {
		t.Errorf("deployment.yaml missing after round-trip: %v", err)
	}
}

func TestShouldIgnoreEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		isDir    bool
		patterns []string
		expected bool
	}{
		{
			name:     "empty patterns",
			path:     "anything.yaml",
			isDir:    false,
			patterns: []string{},
			expected: false,
		},
		{
			name:     "glob star in subdir",
			path:     "deep/file.tmp",
			isDir:    false,
			patterns: []string{"*.tmp"},
			expected: true,
		},
		{
			name:     "dir pattern does not match file",
			path:     ".git",
			isDir:    false,
			patterns: []string{".git/"},
			expected: false,
		},
		{
			name:     "dir pattern matches directory",
			path:     ".git",
			isDir:    true,
			patterns: []string{".git/"},
			expected: true,
		},
		{
			name:     "negation overrides ignore",
			path:     "important.tmp",
			isDir:    false,
			patterns: []string{"*.tmp", "!important.tmp"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldIgnore(tt.path, tt.isDir, tt.patterns)
			if result != tt.expected {
				t.Errorf("shouldIgnore(%q, %v, %v) = %v, want %v",
					tt.path, tt.isDir, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestIsSafePathEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		destDir  string
		target   string
		expected bool
	}{
		{"inside", "/dest", "/dest/pkg/file.yaml", true},
		{"outside", "/dest", "/other/file.yaml", false},
		{"traversal", "/dest", "/dest/../etc/passwd", false},
		{"same dir", "/dest", "/dest/file", true},
		{"deeply nested", "/dest", "/dest/a/b/c/d/file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSafePath(tt.destDir, tt.target)
			if result != tt.expected {
				t.Errorf("isSafePath(%q, %q) = %v, want %v", tt.destDir, tt.target, result, tt.expected)
			}
		})
	}
}

func TestExtractArchiveNonexistentFile(t *testing.T) {
	err := ExtractArchive("/nonexistent/archive.keramos.tgz", t.TempDir())
	if nil == err {
		t.Fatal("expected error for nonexistent archive")
	}
}

func TestExtractArchiveInvalidGzip(t *testing.T) {
	archiveDir := t.TempDir()
	badPath := filepath.Join(archiveDir, "bad.keramos.tgz")
	os.WriteFile(badPath, []byte("not gzip data"), 0o644)

	err := ExtractArchive(badPath, t.TempDir())
	if nil == err {
		t.Fatal("expected error for invalid gzip data")
	}
}
