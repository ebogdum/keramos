package migrate

import (
	"strings"
	"testing"
)

func convertSnippet(t *testing.T, content string) (string, *MigrateResult) {
	t.Helper()
	result := &MigrateResult{}
	c := newTemplateConverter(result)
	c.chartName = "demo"
	return c.convertFile(content, "test.yaml"), result
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output missing %q:\n%s", want, got)
	}
}

func TestValueAndReleaseReferences(t *testing.T) {
	got, result := convertSnippet(t, `apiVersion: v1
metadata:
  name: {{ .Release.Name }}-{{ .Chart.Name }}
  namespace: {{ .Release.Namespace }}
  version: {{ .Chart.Version }}
  image: {{ .Values.image.repository }}
`)
	requireContains(t, got, "name: ${release.name}-${package.name}")
	requireContains(t, got, "namespace: ${release.namespace}")
	requireContains(t, got, "image: ${values.image.repository}")
	if 0 != len(result.ManualReview) {
		t.Errorf("unexpected review items: %+v", result.ManualReview)
	}
}

func TestSubjectLastFunctionsPipeThrough(t *testing.T) {
	got, _ := convertSnippet(t, `name: {{ default "fallback" .Values.name | trunc 63 | trimSuffix "-" }}
`)
	requireContains(t, got, "${values.name | default 'fallback' | trunc 63 | trimSuffix '-'}")
}

func TestConditionalKeysBecomeMerge(t *testing.T) {
	got, _ := convertSnippet(t, `metadata:
  name: app
  {{- if .Values.annotations }}
  annotations:
    team: platform
  {{- end }}
`)
	requireContains(t, got, "$merge:")
	requireContains(t, got, "$if: ${values.annotations}")
	requireContains(t, got, "$then:")
	requireContains(t, got, "team: platform")
}

func TestIfElseBecomesBranches(t *testing.T) {
	got, _ := convertSnippet(t, `spec:
  {{- if .Values.highMem }}
  memory: 1Gi
  {{- else }}
  memory: 128Mi
  {{- end }}
`)
	requireContains(t, got, "$if: ${values.highMem}")
	requireContains(t, got, "$else:")
	requireContains(t, got, "memory: 128Mi")
}

func TestWithBlockRebindsDot(t *testing.T) {
	got, _ := convertSnippet(t, `spec:
  {{- with .Values.nodeSelector }}
  nodeSelector:
    kind: {{ .kind }}
  {{- end }}
`)
	requireContains(t, got, "$if: ${values.nodeSelector}")
	requireContains(t, got, "kind: ${values.nodeSelector.kind}")
}

func TestRangeBecomesEach(t *testing.T) {
	got, _ := convertSnippet(t, `spec:
  ports:
    {{- range .Values.ports }}
    - name: {{ .name }}
      port: {{ .port }}
    {{- end }}
`)
	requireContains(t, got, "$each: ${values.ports}")
	requireContains(t, got, "$as: item1")
	requireContains(t, got, "$yield:")
	requireContains(t, got, "name: ${item1.name}")
}

func TestRangeOverMapBindsKeyAndValue(t *testing.T) {
	got, _ := convertSnippet(t, `spec:
  env:
    {{- range $key, $value := .Values.config }}
    - name: {{ $key }}
      value: {{ $value }}
    {{- end }}
`)
	requireContains(t, got, "$each: ${values.config}")
	requireContains(t, got, "name: ${item1.key}")
	requireContains(t, got, "value: ${item1.value}")
}

func TestStandaloneToYamlBecomesMerge(t *testing.T) {
	got, _ := convertSnippet(t, `metadata:
  labels:
    {{- toYaml .Values.labels | nindent 4 }}
`)
	requireContains(t, got, "$merge: ${values.labels}")
}

func TestIncludeLineBecomesIncludeDirective(t *testing.T) {
	got, _ := convertSnippet(t, `metadata:
  labels:
    {{- include "demo.labels" . | nindent 4 }}
`)
	requireContains(t, got, "$include: demo.labels")
}

func TestKeyValueNindentKeepsStructure(t *testing.T) {
	got, _ := convertSnippet(t, `metadata:
  annotations: {{- toYaml .Values.annotations | nindent 4 }}
`)
	requireContains(t, got, "annotations: ${values.annotations}")
}

func TestInlineConditionBecomesTernary(t *testing.T) {
	got, _ := convertSnippet(t, `spec:
  policy: {{ if .Values.strict }}Always{{ else }}IfNotPresent{{ end }}
`)
	requireContains(t, got, "ternary 'Always' 'IfNotPresent'")
}

func TestDocumentLevelConditionWrapsDocument(t *testing.T) {
	got, _ := convertSnippet(t, `{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
{{- end }}
`)
	requireContains(t, got, "$if: ${values.ingress.enabled}")
	requireContains(t, got, "kind: Ingress")
}

func TestConditionalKeysInsideListItem(t *testing.T) {
	got, result := convertSnippet(t, `spec:
  containers:
    - name: app
      image: demo:1
{{- if .Values.probe }}
      livenessProbe:
        httpGet:
          path: /healthz
{{- end }}
`)
	if 0 != len(result.ManualReview) {
		t.Errorf("unexpected review items: %+v", result.ManualReview)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- $merge:") {
			t.Fatalf("conditional keys became a new list item:\n%s", got)
		}
	}
	requireContains(t, got, "$if: ${values.probe}")
	requireContains(t, got, "livenessProbe:")
}

func TestDefinesBecomePartials(t *testing.T) {
	result := &MigrateResult{}
	c := newTemplateConverter(result)
	c.chartName = "demo"
	c.convertFile(`{{- define "demo.name" -}}
{{- .Values.nameOverride | default .Chart.Name | trunc 63 -}}
{{- end -}}

{{- define "demo.labels" -}}
app: {{ include "demo.name" . }}
{{- end -}}
`, "_helpers.tpl")

	data, err := c.helpersDocument()
	if nil != err {
		t.Fatalf("helpers: %v", err)
	}
	out := string(data)
	requireContains(t, out, "demo.name:")
	requireContains(t, out, "values.nameOverride | default package.name | trunc 63")
	requireContains(t, out, "demo.labels:")
	requireContains(t, out, "app: ${include 'demo.name'}")
}

func TestCommentBlockWithBracesIsSkipped(t *testing.T) {
	got, result := convertSnippet(t, `{{/*
Usage: {{ include "demo.name" . }}
*/}}
kind: ConfigMap
name: {{ .Values.name }}
`)
	requireContains(t, got, "kind: ConfigMap")
	requireContains(t, got, "name: ${values.name}")
	if 0 != len(result.ManualReview) {
		t.Errorf("unexpected review items: %+v", result.ManualReview)
	}
}

func TestLiteralBlockKeepsExpressionsInline(t *testing.T) {
	got, _ := convertSnippet(t, `data:
  app.conf: |
    listen = {{ .Values.port }}
    {{- if .Values.tls }}
    tls = on
    {{- end }}
`)
	requireContains(t, got, "$text:")
	requireContains(t, got, "listen = ${values.port}")
	requireContains(t, got, "$if: ${values.tls}")
	requireContains(t, got, "tls = on")
}

func TestUnsupportedConstructIsReported(t *testing.T) {
	_, result := convertSnippet(t, `data:
  value: {{ .Values.items | fooBarUnknown }}
`)
	if 0 == len(result.ManualReview) {
		t.Fatal("expected a review item for an unknown function")
	}
	if !strings.Contains(result.ManualReview[0].Reason, "fooBarUnknown") {
		t.Errorf("unexpected reason: %s", result.ManualReview[0].Reason)
	}
}

func TestSubchartValuesArePrefixed(t *testing.T) {
	result := &MigrateResult{}
	c := newTemplateConverter(result)
	c.chartName = "redis"
	c.valuePrefix = "redis"
	got := c.convertFile(`spec:
  replicas: {{ .Values.replicaCount }}
  registry: {{ .Values.global.imageRegistry }}
`, "test.yaml")
	requireContains(t, got, "replicas: ${values.redis.replicaCount}")
	requireContains(t, got, "registry: ${values.global.imageRegistry}")
}
