package values

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_DefaultsOnly(t *testing.T) {
	defaults := map[string]any{
		"name":     "myapp",
		"replicas": 3,
	}
	result, err := Resolve(defaults, nil, nil)
	if nil != err {
		t.Fatal(err)
	}
	if "myapp" != result["name"] {
		t.Errorf("expected name=myapp, got %v", result["name"])
	}
	if 3 != result["replicas"] {
		t.Errorf("expected replicas=3, got %v", result["replicas"])
	}
}

func TestResolve_ValuesFileOverridesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	overridePath := filepath.Join(tmpDir, "override.yaml")
	os.WriteFile(overridePath, []byte("replicas: 10\nenv: prod\n"), 0o644)

	defaults := map[string]any{
		"name":     "myapp",
		"replicas": 3,
	}
	result, err := Resolve(defaults, []string{overridePath}, nil)
	if nil != err {
		t.Fatal(err)
	}
	if "myapp" != result["name"] {
		t.Errorf("expected name=myapp, got %v", result["name"])
	}
	if 10 != result["replicas"] {
		t.Errorf("expected replicas=10, got %v", result["replicas"])
	}
	if "prod" != result["env"] {
		t.Errorf("expected env=prod, got %v", result["env"])
	}
}

func TestResolve_MultipleValuesFilesLeftToRight(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "a.yaml")
	file2 := filepath.Join(tmpDir, "b.yaml")
	os.WriteFile(file1, []byte("replicas: 5\n"), 0o644)
	os.WriteFile(file2, []byte("replicas: 10\n"), 0o644)

	defaults := map[string]any{"replicas": 1}
	result, err := Resolve(defaults, []string{file1, file2}, nil)
	if nil != err {
		t.Fatal(err)
	}
	if 10 != result["replicas"] {
		t.Errorf("expected replicas=10 (last file wins), got %v", result["replicas"])
	}
}

func TestResolve_SetOverridesAll(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "vals.yaml")
	os.WriteFile(filePath, []byte("replicas: 10\n"), 0o644)

	defaults := map[string]any{"replicas": 1}
	result, err := Resolve(defaults, []string{filePath}, []string{"replicas=99"})
	if nil != err {
		t.Fatal(err)
	}
	if 99 != result["replicas"] {
		t.Errorf("expected replicas=99 (--set wins), got %v", result["replicas"])
	}
}

func TestResolve_MissingValuesFile(t *testing.T) {
	_, err := Resolve(map[string]any{}, []string{"/nonexistent/file.yaml"}, nil)
	if nil == err {
		t.Fatal("expected error for missing values file")
	}
}

func TestResolve_DeepMergeFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "vals.yaml")
	os.WriteFile(filePath, []byte("image:\n  tag: v2\n"), 0o644)

	defaults := map[string]any{
		"image": map[string]any{
			"repository": "nginx",
			"tag":        "v1",
		},
	}
	result, err := Resolve(defaults, []string{filePath}, nil)
	if nil != err {
		t.Fatal(err)
	}
	image := result["image"].(map[string]any)
	if "nginx" != image["repository"] {
		t.Errorf("expected repository=nginx, got %v", image["repository"])
	}
	if "v2" != image["tag"] {
		t.Errorf("expected tag=v2, got %v", image["tag"])
	}
}
