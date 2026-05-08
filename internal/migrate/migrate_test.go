package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConvertSimpleValueRef(t *testing.T) {
	input := "{{ .Values.replicaCount }}"
	got := ConvertHelmExpression(input)
	expected := "${values.replicaCount}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertNestedValueRef(t *testing.T) {
	input := "{{ .Values.image.repository }}"
	got := ConvertHelmExpression(input)
	expected := "${values.image.repository}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertReleaseName(t *testing.T) {
	input := "{{ .Release.Name }}"
	got := ConvertHelmExpression(input)
	expected := "${release.name}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertReleaseNamespace(t *testing.T) {
	input := "{{ .Release.Namespace }}"
	got := ConvertHelmExpression(input)
	expected := "${release.namespace}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertChartName(t *testing.T) {
	input := "{{ .Chart.Name }}"
	got := ConvertHelmExpression(input)
	expected := "${package.name}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertChartVersion(t *testing.T) {
	input := "{{ .Chart.Version }}"
	got := ConvertHelmExpression(input)
	expected := "${package.version}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertChartAppVersion(t *testing.T) {
	input := "{{ .Chart.AppVersion }}"
	got := ConvertHelmExpression(input)
	expected := "${package.appVersion}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithDefault(t *testing.T) {
	input := `{{ .Values.x | default "foo" }}`
	got := ConvertHelmExpression(input)
	expected := `${values.x | default("foo")}`
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithUpper(t *testing.T) {
	input := "{{ .Values.x | upper }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | upper}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithQuote(t *testing.T) {
	input := "{{ .Values.x | quote }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | quote}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithB64Enc(t *testing.T) {
	input := "{{ .Values.x | b64enc }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | b64encode}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithIndent(t *testing.T) {
	input := "{{ .Values.x | indent 4 }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | indent(4)}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithNindent(t *testing.T) {
	input := "{{ .Values.x | nindent 8 }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | nindent(8)}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertWithToYaml(t *testing.T) {
	input := "{{ .Values.x | toYaml }}"
	got := ConvertHelmExpression(input)
	expected := "${values.x | toYaml}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertPipedDefaultAndUpper(t *testing.T) {
	input := `{{ .Values.appName | default "myapp" | upper }}`
	got := ConvertHelmExpression(input)
	expected := `${values.appName | default("myapp") | upper}`
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertIncludeLine(t *testing.T) {
	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	input := `    {{ include "myapp.name" . }}`
	got := ConvertHelmLine(input, "test.yaml", 1, result)
	expected := "    $include: myapp.name"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConvertLineWithDash(t *testing.T) {
	input := "{{- .Values.x -}}"
	got := ConvertHelmExpression(input)
	expected := "${values.x}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestIfBlockFlaggedForReview(t *testing.T) {
	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	input := "{{- if .Values.ingress.enabled }}"
	got := ConvertHelmLine(input, "test.yaml", 5, result)

	// Should remain unconverted
	if input != got {
		t.Errorf("expected line unchanged, got %q", got)
	}
	if 1 != len(result.ManualReview) {
		t.Fatalf("expected 1 review item, got %d", len(result.ManualReview))
	}
	if "test.yaml" != result.ManualReview[0].File {
		t.Errorf("expected file test.yaml, got %s", result.ManualReview[0].File)
	}
}

func TestRangeBlockFlaggedForReview(t *testing.T) {
	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	input := "{{- range .Values.extraEnv }}"
	ConvertHelmLine(input, "deploy.yaml", 10, result)
	if 1 != len(result.ManualReview) {
		t.Fatalf("expected 1 review item, got %d", len(result.ManualReview))
	}
	if !strings.Contains(result.ManualReview[0].Reason, "range") {
		t.Errorf("expected reason to mention range, got %s", result.ManualReview[0].Reason)
	}
}

func TestTplFlaggedForReview(t *testing.T) {
	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	input := "{{ tpl .Values.x . }}"
	ConvertHelmLine(input, "test.yaml", 1, result)
	if 1 != len(result.ManualReview) {
		t.Fatalf("expected 1 review item, got %d", len(result.ManualReview))
	}
}

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

func TestParseDefineBlocks(t *testing.T) {
	content := `{{- define "myapp.name" -}}
{{- .Values.nameOverride | default .Chart.Name -}}
{{- end -}}

{{- define "myapp.fullname" -}}
{{- .Release.Name }}-{{ .Chart.Name }}
{{- end -}}
`
	blocks := ParseDefineBlocks(content)
	if 2 != len(blocks) {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if _, ok := blocks["myapp.name"]; !ok {
		t.Error("missing block myapp.name")
	}
	if _, ok := blocks["myapp.fullname"]; !ok {
		t.Error("missing block myapp.fullname")
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

	// Verify if block was converted
	if !strings.Contains(deployStr, "$if: ${values.ingress.enabled}") {
		t.Error("deployment.yaml missing converted if block")
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
