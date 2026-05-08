package values

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_NilDefaults(t *testing.T) {
	result, err := Resolve(nil, nil, nil)
	if nil != err {
		t.Fatal(err)
	}
	// DeepMerge(nil, nil) returns nil, so Resolve with nil defaults returns nil
	// This is valid behavior - callers should provide at least empty map
	_ = result
}

func TestResolve_EmptyDefaults(t *testing.T) {
	result, err := Resolve(map[string]any{}, nil, nil)
	if nil != err {
		t.Fatal(err)
	}
	if nil == result {
		t.Fatal("expected non-nil result for empty defaults")
	}
}

func TestResolve_NonexistentValuesFile(t *testing.T) {
	defaults := map[string]any{"name": "app"}
	_, err := Resolve(defaults, []string{"/nonexistent/values.yaml"}, nil)
	if nil == err {
		t.Fatal("expected error for nonexistent values file")
	}
}

func TestResolve_InvalidYAMLValuesFile(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(badFile, []byte("{{invalid yaml[[\n"), 0o644)

	_, err := Resolve(map[string]any{}, []string{badFile}, nil)
	if nil == err {
		t.Fatal("expected error for invalid YAML values file")
	}
}

func TestResolve_EmptyValuesFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.yaml")
	os.WriteFile(emptyFile, []byte(""), 0o644)

	result, err := Resolve(map[string]any{"name": "app"}, []string{emptyFile}, nil)
	if nil != err {
		t.Fatal(err)
	}
	if "app" != result["name"] {
		t.Errorf("expected name=app preserved after empty file, got %v", result["name"])
	}
}

func TestResolve_InvalidSetFormat(t *testing.T) {
	_, err := Resolve(map[string]any{}, nil, []string{"no-equals-sign"})
	if nil == err {
		t.Fatal("expected error for invalid --set format")
	}
}

func TestResolve_SetOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "vals.yaml")
	os.WriteFile(filePath, []byte("name: fromfile\n"), 0o644)

	result, err := Resolve(map[string]any{"name": "default"}, []string{filePath}, []string{"name=fromset"})
	if nil != err {
		t.Fatal(err)
	}
	if "fromset" != result["name"] {
		t.Errorf("expected --set to win over file, got %v", result["name"])
	}
}

func TestResolve_ConflictingTypes(t *testing.T) {
	// defaults has image as a map, file overrides with a string
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "vals.yaml")
	os.WriteFile(filePath, []byte("image: simple-string\n"), 0o644)

	defaults := map[string]any{
		"image": map[string]any{
			"repository": "nginx",
			"tag":        "latest",
		},
	}
	result, err := Resolve(defaults, []string{filePath}, nil)
	if nil != err {
		t.Fatal(err)
	}
	// The string should replace the map
	s, ok := result["image"].(string)
	if !ok {
		t.Fatalf("expected image to be string after override, got %T", result["image"])
	}
	if "simple-string" != s {
		t.Errorf("expected 'simple-string', got %s", s)
	}
}

func TestResolve_MultipleSetsSameKey(t *testing.T) {
	result, err := Resolve(map[string]any{}, nil, []string{"key=first", "key=second"})
	if nil != err {
		t.Fatal(err)
	}
	if "second" != result["key"] {
		t.Errorf("expected last --set to win, got %v", result["key"])
	}
}
