package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scanFixtureDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "test", "fixtures", "scan-input")
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if nil != err {
		return err
	}

	if err := os.MkdirAll(dst, 0755); nil != err {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); nil != err {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if nil != err {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); nil != err {
			return err
		}
	}

	return nil
}

func TestScanDryRun(t *testing.T) {
	result, err := Scan(scanFixtureDir(), "", true)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if nil == result {
		t.Fatal("expected non-nil result")
	}

	if !strings.Contains(result.Report, "DRY RUN") {
		t.Error("expected report to contain 'DRY RUN'")
	}

	if !strings.Contains(result.Report, "Packages scanned: 4") {
		t.Errorf("expected 4 packages scanned, got:\n%s", result.Report)
	}
}

func TestScanCommonValueExtraction(t *testing.T) {
	result, err := Scan(scanFixtureDir(), "", true)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	// "namespace: production" appears in all 4 packages
	if nil == result.CommonValues {
		t.Fatal("expected common values")
	}

	ns, ok := result.CommonValues["namespace"]
	if !ok {
		t.Error("expected 'namespace' in common values")
	} else if "production" != ns {
		t.Errorf("expected namespace='production', got %v", ns)
	}
}

func TestScanBaseLayerGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Copy fixtures to temp dir
	if err := copyDir(scanFixtureDir(), tmpDir); nil != err {
		t.Fatalf("failed to copy fixtures: %v", err)
	}

	result, err := Scan(tmpDir, tmpDir, false)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	basePath := filepath.Join(tmpDir, "base")
	if result.BasePackage != basePath {
		t.Errorf("expected base at %s, got %s", basePath, result.BasePackage)
	}

	// Check base keramos.yaml exists
	_, statErr := os.Stat(filepath.Join(basePath, "keramos.yaml"))
	if nil != statErr {
		t.Error("expected base/keramos.yaml to exist")
	}

	// Check base values.yaml exists
	_, statErr = os.Stat(filepath.Join(basePath, "values.yaml"))
	if nil != statErr {
		t.Error("expected base/values.yaml to exist")
	}
}

func TestScanPackageRewrite(t *testing.T) {
	tmpDir := t.TempDir()

	if err := copyDir(scanFixtureDir(), tmpDir); nil != err {
		t.Fatalf("failed to copy fixtures: %v", err)
	}

	result, err := Scan(tmpDir, tmpDir, false)
	if nil != err {
		t.Fatalf("expected no error, got %v", err)
	}

	if 0 == len(result.UpdatedPackages) {
		t.Fatal("expected at least one updated package")
	}

	// Verify updated packages reference base
	for _, pkgPath := range result.UpdatedPackages {
		keramosData, readErr := os.ReadFile(filepath.Join(pkgPath, "keramos.yaml"))
		if nil != readErr {
			t.Errorf("failed to read updated keramos.yaml for %s: %v", pkgPath, readErr)
			continue
		}
		content := string(keramosData)
		if !strings.Contains(content, "base") {
			t.Errorf("expected updated keramos.yaml at %s to reference 'base'", pkgPath)
		}
	}
}

func TestScanNotEnoughPackages(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "only-one")
	_ = os.MkdirAll(pkgDir, 0755)
	_ = os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte(`apiVersion: keramos/v1
name: only-one
version: 1.0.0
`), 0644)

	_, err := Scan(tmpDir, "", true)
	if nil == err {
		t.Fatal("expected error for single package, got nil")
	}
	if !strings.Contains(err.Error(), "at least 2") {
		t.Errorf("expected error about needing 2 packages, got: %v", err)
	}
}
