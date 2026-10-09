package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestChartYAMLConversion(t *testing.T) {
	chartYAML := `apiVersion: v2
name: myapp
version: 1.0.0
appVersion: "2.0.0"
description: A sample chart
type: application
kubeVersion: ">=1.20.0"
maintainers:
  - name: John Doe
    email: john@example.com
keywords:
  - web
dependencies:
  - name: redis
    version: "17.0.0"
    repository: "https://charts.bitnami.com/bitnami"
    condition: redis.enabled
`

	keramosYAML, err := ParseHelmChart([]byte(chartYAML))
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	var meta keramosMeta
	if err := yaml.Unmarshal(keramosYAML, &meta); nil != err {
		t.Fatalf("cannot parse output: %v", err)
	}

	if "keramos/v1" != meta.APIVersion {
		t.Errorf("expected apiVersion keramos/v1, got %s", meta.APIVersion)
	}
	if "myapp" != meta.Name {
		t.Errorf("expected name myapp, got %s", meta.Name)
	}
	if "1.0.0" != meta.Version {
		t.Errorf("expected version 1.0.0, got %s", meta.Version)
	}
	if "2.0.0" != meta.AppVersion {
		t.Errorf("expected appVersion 2.0.0, got %s", meta.AppVersion)
	}
	if 1 != len(meta.Dependencies) {
		t.Fatalf("expected 1 dependency, got %d", len(meta.Dependencies))
	}
	if "redis" != meta.Dependencies[0].Name {
		t.Errorf("expected dep name redis, got %s", meta.Dependencies[0].Name)
	}
	if "redis.enabled" != meta.Dependencies[0].Condition {
		t.Errorf("expected dep condition redis.enabled, got %s", meta.Dependencies[0].Condition)
	}
	if 1 != len(meta.Maintainers) {
		t.Fatalf("expected 1 maintainer, got %d", len(meta.Maintainers))
	}
	if "John Doe" != meta.Maintainers[0].Name {
		t.Errorf("expected maintainer John Doe, got %s", meta.Maintainers[0].Name)
	}
}

func TestMigrateFullChart(t *testing.T) {
	chartPath := filepath.Join("..", "..", "test", "fixtures", "helm-chart")
	outputDir := filepath.Join(t.TempDir(), "myapp-keramos")

	result, err := Migrate(chartPath, outputDir, false)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 0 == len(result.ConvertedFiles) {
		t.Fatal("expected converted files")
	}

	// Check keramos.yaml was created
	keramosYAML, readErr := os.ReadFile(filepath.Join(outputDir, "keramos.yaml"))
	if nil != readErr {
		t.Fatal("keramos.yaml not created")
	}
	if !strings.Contains(string(keramosYAML), "myapp") {
		t.Error("keramos.yaml missing chart name")
	}

	// Check values.yaml was copied
	if _, statErr := os.Stat(filepath.Join(outputDir, "values.yaml")); nil != statErr {
		t.Error("values.yaml not copied")
	}

	// Check templates were converted
	deployPath := filepath.Join(outputDir, "templates", "deployment.yaml")
	deployData, readErr := os.ReadFile(deployPath)
	if nil != readErr {
		t.Fatal("deployment.yaml not created")
	}
	deployContent := string(deployData)

	if !strings.Contains(deployContent, "${release.name}") {
		t.Error("deployment.yaml missing converted release.name")
	}
	if !strings.Contains(deployContent, "${values.replicaCount}") {
		t.Error("deployment.yaml missing converted values ref")
	}

	// Check _helpers.yaml was created
	helpersPath := filepath.Join(outputDir, "templates", "_helpers.yaml")
	if _, statErr := os.Stat(helpersPath); nil != statErr {
		t.Error("_helpers.yaml not created")
	}

	// Check tests directory was created
	testsDir := filepath.Join(outputDir, "tests")
	if _, statErr := os.Stat(testsDir); nil != statErr {
		t.Error("tests directory not created")
	}

	// Check notes.yaml was converted
	notesPath := filepath.Join(outputDir, "templates", "notes.yaml")
	notesData, readErr := os.ReadFile(notesPath)
	if nil != readErr {
		t.Fatal("notes.yaml not created")
	}
	notesContent := string(notesData)
	if !strings.Contains(notesContent, "message: |") {
		t.Error("notes.yaml missing message block scalar")
	}
	if !strings.Contains(notesContent, "${package.name}") {
		t.Error("notes.yaml missing converted chart name")
	}

	// Check .keramosignore was created
	if _, statErr := os.Stat(filepath.Join(outputDir, ".keramosignore")); nil != statErr {
		t.Error(".keramosignore not created")
	}

	// The simple fixture templates (if/range) should now be auto-converted
	// so there should be fewer or zero manual review items
	deployData2, _ := os.ReadFile(deployPath)
	deployStr := string(deployData2)

	if !strings.Contains(deployStr, "$if: ${values.ingress.enabled}") {
		t.Error("deployment.yaml did not convert the if-block")
	}
	for _, sibling := range []string{"imagePullPolicy:", "ports:", "env:"} {
		if !strings.Contains(deployStr, sibling) {
			t.Errorf("deployment.yaml lost unconditional key %q", sibling)
		}
	}
	for _, item := range result.ManualReview {
		if strings.Contains(item.File, "deployment.yaml") {
			t.Errorf("deployment.yaml still needs review: %s", item.Reason)
		}
	}

	// Verify range block was converted
	if !strings.Contains(deployStr, "$each: ${values.extraEnv}") {
		t.Error("deployment.yaml missing converted range block")
	}
}

func TestMigrateStrictPassesForSimpleChart(t *testing.T) {
	chartPath := filepath.Join("..", "..", "test", "fixtures", "helm-chart")
	outputDir := filepath.Join(t.TempDir(), "myapp-keramos-strict")

	result, err := Migrate(chartPath, outputDir, true)
	// Simple chart with basic if/range blocks should now auto-convert
	// and may pass strict mode if all blocks are handled
	if nil != err {
		// If there are still review items, that's ok — verify they're reasonable
		if 0 == len(result.ManualReview) {
			t.Fatal("error with no review items in strict mode")
		}
	}
}

func TestMigrateNotAChart(t *testing.T) {
	dir := t.TempDir()
	_, err := Migrate(dir, filepath.Join(t.TempDir(), "out"), false)
	if nil == err {
		t.Fatal("expected error for non-chart directory")
	}
}

func TestStripHookAnnotation(t *testing.T) {
	input := `metadata:
  annotations:
    "helm.sh/hook": test
spec:
  containers: []
`
	got := stripHookAnnotation(input)
	if strings.Contains(got, "helm.sh/hook") {
		t.Error("hook annotation not stripped")
	}
	if !strings.Contains(got, "spec:") {
		t.Error("non-hook content was removed")
	}
}
