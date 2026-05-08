package migrate

import (
	"strings"
	"testing"
)

func TestConvertWithToYamlBlock(t *testing.T) {
	input := `  {{- with .Values.podAnnotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${values.podAnnotations}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
	if !strings.Contains(got, "$merge: ${values.podAnnotations}") {
		t.Errorf("missing $merge, got:\n%s", got)
	}
}

func TestConvertWithToYamlOnlyBody(t *testing.T) {
	// When with block body is ONLY toYaml . | nindent (no wrapping key),
	// convert to just $merge
	input := `    {{- with .Values.nodeSelector }}
    {{- toYaml . | nindent 8 }}
    {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	if !strings.Contains(got, "$merge: ${values.nodeSelector}") {
		t.Errorf("expected $merge output, got:\n%s", got)
	}
}

func TestConvertSimpleIfBlock(t *testing.T) {
	input := `{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	if !strings.Contains(got, "$if: ${values.ingress.enabled}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
	if !strings.Contains(got, "apiVersion: networking.k8s.io/v1") {
		t.Errorf("missing body content, got:\n%s", got)
	}
}

func TestConvertIfElseBlock(t *testing.T) {
	input := `  {{- if .Values.autoscaling.enabled }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  {{- else }}
  replicas: {{ .Values.replicas }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${values.autoscaling.enabled}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
	if !strings.Contains(got, "$then:") {
		t.Errorf("missing $then, got:\n%s", got)
	}
	if !strings.Contains(got, "$else:") {
		t.Errorf("missing $else, got:\n%s", got)
	}
	if !strings.Contains(got, "${values.autoscaling.minReplicas}") {
		t.Errorf("missing minReplicas ref, got:\n%s", got)
	}
	if !strings.Contains(got, "${values.replicas}") {
		t.Errorf("missing replicas ref, got:\n%s", got)
	}
}

func TestConvertRangeBlock(t *testing.T) {
	input := `{{- range .Values.ingress.hosts }}
  - host: {{ .host | quote }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$each: ${values.ingress.hosts}") {
		t.Errorf("missing $each, got:\n%s", got)
	}
	if !strings.Contains(got, "$as: item") {
		t.Errorf("missing $as, got:\n%s", got)
	}
	if !strings.Contains(got, "$yield:") {
		t.Errorf("missing $yield, got:\n%s", got)
	}
	if !strings.Contains(got, "${item.host") {
		t.Errorf("missing item.host ref, got:\n%s", got)
	}
}

func TestConvertRangeWithDot(t *testing.T) {
	input := `{{- range .Values.args }}
  - {{ . }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	if !strings.Contains(got, "${item}") {
		t.Errorf("expected dot to become item, got:\n%s", got)
	}
}

func TestStandaloneToYaml(t *testing.T) {
	input := `    {{- toYaml .Values.resources | nindent 12 }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	expected := "    $merge: ${values.resources}"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStandaloneInclude(t *testing.T) {
	input := `    {{- include "app.labels" . | nindent 4 }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	expected := "    $include: app.labels"
	if expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestComplexTplFlaggedForReview(t *testing.T) {
	input := `{{- if .Values.x }}
  {{ tpl .Values.template . }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for tpl in body")
	}
}

func TestComplexDictFlaggedForReview(t *testing.T) {
	input := `{{- if .Values.x }}
  {{ include "foo" (dict "a" .Values.b "c" $) }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for dict in body")
	}
}

func TestComplexDollarVarFlaggedForReview(t *testing.T) {
	input := `{{- with .Values.x }}
  {{ $val := . }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for $var assignment")
	}
}

func TestNestedIfFlaggedForReview(t *testing.T) {
	input := `{{- if .Values.outer }}
  {{- if .Values.inner }}
  x: y
  {{- end }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for nested if blocks")
	}
}

func TestElseIfFlaggedForReview(t *testing.T) {
	input := `{{- if .Values.a }}
  x: 1
{{- else if .Values.b }}
  x: 2
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for else-if chain")
	}
}

func TestNegatedCondition(t *testing.T) {
	input := `{{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${!values.autoscaling.enabled}") {
		t.Errorf("missing negated condition, got:\n%s", got)
	}
}

func TestWithScalarBody(t *testing.T) {
	input := `  {{- with .Values.schedulerName }}
  schedulerName: {{ . }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${values.schedulerName}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
	if !strings.Contains(got, "schedulerName: ${values.schedulerName}") {
		t.Errorf("missing converted scalar, got:\n%s", got)
	}
}

func TestWithScalarQuoteBody(t *testing.T) {
	input := `  {{- with .Values.priorityClassName }}
  priorityClassName: {{ . | quote }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "${values.priorityClassName | quote}") {
		t.Errorf("missing converted scalar with quote, got:\n%s", got)
	}
}

func TestMixedContent(t *testing.T) {
	input := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  template:
    spec:
      containers:
        - name: app
          image: {{ .Values.image.repository }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "${release.name}") {
		t.Errorf("missing release.name, got:\n%s", got)
	}
	if !strings.Contains(got, "$if: ${!values.autoscaling.enabled}") {
		t.Errorf("missing if block, got:\n%s", got)
	}
	if !strings.Contains(got, "${values.image.repository}") {
		t.Errorf("missing image ref, got:\n%s", got)
	}
}

func TestWithKeyAndToYamlBlock(t *testing.T) {
	input := `      {{- with .Values.securityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${values.securityContext}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
	if !strings.Contains(got, "securityContext:") {
		t.Errorf("missing securityContext key, got:\n%s", got)
	}
	if !strings.Contains(got, "$merge: ${values.securityContext}") {
		t.Errorf("missing $merge, got:\n%s", got)
	}
}

func TestMatchBlocksNesting(t *testing.T) {
	lines := []string{
		"{{- if .Values.a }}",
		"  {{- if .Values.b }}",
		"  x: y",
		"  {{- end }}",
		"{{- end }}",
	}

	blocks := matchBlocks(lines)
	if 2 != len(blocks) {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	// Inner block should come first (stack order)
	inner := blocks[0]
	if 1 != inner.startLine || 3 != inner.endLine {
		t.Errorf("inner block: expected lines 1-3, got %d-%d", inner.startLine, inner.endLine)
	}

	outer := blocks[1]
	if 0 != outer.startLine || 4 != outer.endLine {
		t.Errorf("outer block: expected lines 0-4, got %d-%d", outer.startLine, outer.endLine)
	}
}

func TestConvertCondition(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{".Values.x", "values.x"},
		{".Values.ingress.enabled", "values.ingress.enabled"},
		{"not .Values.autoscaling.enabled", "!values.autoscaling.enabled"},
		{"eq .Values.type \"ClusterIP\"", `values.type == "ClusterIP"`},
		{".Release.Name", "release.name"},
		{"and .Values.a .Values.b", ""}, // complex
		{"tpl .Values.x .", ""},          // complex (tpl)
	}

	for _, tt := range tests {
		got := convertCondition(tt.input)
		if tt.expected != got {
			t.Errorf("convertCondition(%q): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestClassifyLine(t *testing.T) {
	tests := []struct {
		input    string
		expected directiveKind
	}{
		{"{{- if .Values.x }}", dirIf},
		{"{{- else }}", dirElse},
		{"{{- else if .Values.y }}", dirElseIf},
		{"{{- end }}", dirEnd},
		{"{{- range .Values.x }}", dirRange},
		{"{{- with .Values.x }}", dirWith},
		{"  name: {{ .Values.x }}", dirNone},
		{"plain text", dirNone},
	}

	for _, tt := range tests {
		got, _ := classifyLine(tt.input)
		if tt.expected != got {
			t.Errorf("classifyLine(%q): expected %d, got %d", tt.input, tt.expected, got)
		}
	}
}

func TestIsComplexExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"tpl .Values.x .", true},
		{".Values.x", false},
		{"$var := something", true},
		{"dict \"a\" .Values.b", true},
		{"printf \"%s-%s\" .Values.a .Values.b", true},
		{"index .Values.x 0", true},
		{"lookup \"v1\" \"ConfigMap\"", true},
		{".Values.simple.ref", false},
	}

	for _, tt := range tests {
		got := isComplexExpression(tt.input)
		if tt.expected != got {
			t.Errorf("isComplexExpression(%q): expected %v, got %v", tt.input, tt.expected, got)
		}
	}
}

func TestConvertIfWithComparisonCondition(t *testing.T) {
	input := `{{- if eq .Values.service.type "ClusterIP" }}
  clusterIP: None
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	if !strings.Contains(got, `$if: ${values.service.type == "ClusterIP"}`) {
		t.Errorf("missing comparison condition, got:\n%s", got)
	}
}

func TestRangeWithKeyValueFlaggedForReview(t *testing.T) {
	input := `{{- range $key, $value := .Values.env }}
  {{ $key }}: {{ $value }}
{{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	convertTemplateContentV2(input, "test.yaml", result)

	if 0 == len(result.ManualReview) {
		t.Error("expected review items for range with key-value assignment")
	}
}

func TestMultipleBlocksInSameFile(t *testing.T) {
	input := `  {{- with .Values.nodeSelector }}
  nodeSelector:
    {{- toYaml . | nindent 8 }}
  {{- end }}
  {{- with .Values.tolerations }}
  tolerations:
    {{- toYaml . | nindent 8 }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d", len(result.ManualReview))
	}
	if !strings.Contains(got, "${values.nodeSelector}") {
		t.Errorf("missing nodeSelector ref, got:\n%s", got)
	}
	if !strings.Contains(got, "${values.tolerations}") {
		t.Errorf("missing tolerations ref, got:\n%s", got)
	}
}

func TestIfWrappingUpdateStrategy(t *testing.T) {
	// Common pattern: if .Values.x, then a key with toYaml
	input := `  {{- if .Values.updateStrategy }}
  strategy: {{- toYaml .Values.updateStrategy | nindent 4 }}
  {{- end }}`

	result := &MigrateResult{ManualReview: make([]ReviewItem, 0)}
	got := convertTemplateContentV2(input, "test.yaml", result)

	if 0 != len(result.ManualReview) {
		t.Errorf("expected 0 review items, got %d: %+v", len(result.ManualReview), result.ManualReview)
	}
	if !strings.Contains(got, "$if: ${values.updateStrategy}") {
		t.Errorf("missing $if, got:\n%s", got)
	}
}
