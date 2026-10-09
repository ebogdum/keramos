package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeChart(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["Chart.yaml"] = "apiVersion: v2\nname: demo\nversion: 0.1.0\n"
	if _, ok := files["values.yaml"]; !ok {
		files["values.yaml"] = "replicas: 1\n"
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); nil != err {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); nil != err {
			t.Fatal(err)
		}
	}
	return root
}

func TestTextBlockBecomesTextDirective(t *testing.T) {
	chart := writeChart(t, map[string]string{
		"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
data:
  app.conf: |
    {{- range $key, $value := .Values.config }}
    {{ $key }} = {{ $value }}
    {{- end }}
`,
	})
	out := filepath.Join(t.TempDir(), "pkg")
	result, err := Migrate(chart, out, false)
	if nil != err {
		t.Fatalf("migrate: %v", err)
	}
	if 0 != len(result.ManualReview) {
		t.Errorf("expected no manual review items, got %+v", result.ManualReview)
	}

	converted, readErr := os.ReadFile(filepath.Join(out, "templates", "configmap.yaml"))
	if nil != readErr {
		t.Fatalf("converted template missing: %v", readErr)
	}
	body := string(converted)
	if strings.Contains(body, "{{") {
		t.Errorf("converted template still contains Helm syntax:\n%s", body)
	}
	for _, want := range []string{"$text:", "$each: ${values.config}", "${item1.key} = ${item1.value}"} {
		if !strings.Contains(body, want) {
			t.Errorf("converted template missing %q:\n%s", want, body)
		}
	}
}

func TestCommentWithTrimmedTerminator(t *testing.T) {
	got, result := convertSnippet(t, `{{- /*
Usage: {{ include "demo.name" . }}
*/ -}}
kind: ConfigMap
name: {{ .Values.name }}
`)
	if 0 != len(result.ManualReview) {
		t.Errorf("unexpected review items: %+v", result.ManualReview)
	}
	requireContains(t, got, "kind: ConfigMap")
	requireContains(t, got, "name: ${values.name}")
}

func TestFullnameHelperCompilesWithCorrectPrecedence(t *testing.T) {
	result := &MigrateResult{}
	c := newTemplateConverter(result)
	c.chartName = "demo"
	c.convertFile(`{{- define "demo.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
`, "_helpers.tpl")

	data, err := c.helpersDocument()
	if nil != err {
		t.Fatalf("helpers: %v", err)
	}
	out := string(data)
	if strings.Contains(out, `"\n"`) {
		t.Errorf("helper kept literal newlines:\n%s", out)
	}
	want := "printf release.name (values.nameOverride | default package.name)"
	if !strings.Contains(out, want) {
		t.Errorf("helper = %s\nwant it to contain %q", out, want)
	}
}
