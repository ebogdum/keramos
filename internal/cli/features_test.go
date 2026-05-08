package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Feature 10: --dry-run validation
func TestValidateDryRunFlag(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"", false},
		{"client", false},
		{"server", false},
		{"invalid", true},
		{"true", true},
		{"false", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateDryRunFlag(tt.value)
			if tt.wantErr && nil == err {
				t.Error("expected error")
			}
			if !tt.wantErr && nil != err {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Feature 11: --output validation
func TestValidateOutputFlag(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"table", false},
		{"json", false},
		{"yaml", false},
		{"csv", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateOutputFlag(tt.value)
			if tt.wantErr && nil == err {
				t.Error("expected error")
			}
			if !tt.wantErr && nil != err {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Feature 11: outputRelease
func TestOutputReleaseJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"name": "test-release"}

	err := outputRelease(&buf, data, "json", func() {
		t.Fatal("table function should not be called for json output")
	})
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if jsonErr := json.Unmarshal(buf.Bytes(), &parsed); nil != jsonErr {
		t.Fatalf("output is not valid JSON: %v", jsonErr)
	}
	if "test-release" != parsed["name"] {
		t.Errorf("expected name=test-release, got %s", parsed["name"])
	}
}

func TestOutputReleaseYAML(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"name": "test-release"}

	err := outputRelease(&buf, data, "yaml", func() {
		t.Fatal("table function should not be called for yaml output")
	})
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if yamlErr := yaml.Unmarshal(buf.Bytes(), &parsed); nil != yamlErr {
		t.Fatalf("output is not valid YAML: %v", yamlErr)
	}
	if "test-release" != parsed["name"] {
		t.Errorf("expected name=test-release, got %s", parsed["name"])
	}
}

func TestOutputReleaseTable(t *testing.T) {
	var buf bytes.Buffer
	called := false

	err := outputRelease(&buf, nil, "table", func() {
		called = true
	})
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("table function should be called for table output")
	}
}

// Feature 12: randomSuffix
func TestRandomSuffix(t *testing.T) {
	suffix, err := randomSuffix(5)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 5 != len(suffix) {
		t.Errorf("expected length 5, got %d", len(suffix))
	}
	for _, c := range suffix {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			t.Errorf("unexpected character %c in suffix", c)
		}
	}
}

func TestRandomSuffixUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		suffix, err := randomSuffix(5)
		if nil != err {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[suffix] = true
	}
	// With 36^5 possible values, 100 samples should all be unique
	if len(seen) < 95 {
		t.Errorf("expected at least 95 unique suffixes from 100 attempts, got %d", len(seen))
	}
}

func TestRandomSuffixZeroLength(t *testing.T) {
	suffix, err := randomSuffix(0)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" != suffix {
		t.Errorf("expected empty suffix for length 0, got %q", suffix)
	}
}

// Feature 12: resolveBaseName
func TestResolveBaseNameInvalidPath(t *testing.T) {
	_, err := resolveBaseName("/nonexistent/path")
	if nil == err {
		t.Fatal("expected error for nonexistent package path")
	}
}

func TestResolveBaseNameValidPackage(t *testing.T) {
	tmpDir := t.TempDir()
	keramosYaml := "apiVersion: keramos/v1\nname: my-app\nversion: 1.0.0\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "keramos.yaml"), []byte(keramosYaml), 0o644); nil != err {
		t.Fatalf("failed to write keramos.yaml: %v", err)
	}

	name, err := resolveBaseName(tmpDir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "my-app" != name {
		t.Errorf("expected my-app, got %s", name)
	}
}
