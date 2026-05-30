package helmcompat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeChart(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); nil != err {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); nil != err {
			t.Fatal(err)
		}
	}
}

func TestRenderRealisticChart(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: web\nversion: 1.2.3\nappVersion: \"4.5.6\"\n",
		"values.yaml": "replicas: 2\nimage:\n  repo: nginx\n  tag: stable\nconfig:\n  a: 1\n  b: two\n",
		"templates/_helpers.tpl": `{{- define "web.fullname" -}}
{{ .Release.Name }}-web
{{- end -}}
{{- define "web.labels" -}}
app: {{ include "web.fullname" . }}
chart: {{ .Chart.name }}-{{ .Chart.version }}
{{- end -}}`,
		"templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "web.fullname" . }}
  labels:
    {{- include "web.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicas }}
  template:
    spec:
      containers:
        - name: web
          image: "{{ .Values.image.repo }}:{{ required "tag required" .Values.image.tag }}"
          env:
            - name: SVC
              value: {{ .Release.Service }}
`,
		"templates/cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "web.fullname" . }}-cfg
data:
{{ toYaml .Values.config | indent 2 }}
`,
		"templates/NOTES.txt": "Thanks for installing {{ .Chart.name }}!\n",
	})

	out, err := Render(dir, Options{
		Release: ReleaseMeta{Name: "prod", Namespace: "default", Revision: 1, IsInstall: true},
	})
	if nil != err {
		t.Fatalf("render: %v", err)
	}

	dep := find(t, out, "deployment.yaml")
	for _, want := range []string{"name: prod-web", "app: prod-web", "chart: web-1.2.3", "replicas: 2", "image: \"nginx:stable\"", "value: Helm"} {
		if !strings.Contains(dep, want) {
			t.Errorf("deployment missing %q:\n%s", want, dep)
		}
	}

	cm := find(t, out, "cm.yaml")
	if !strings.Contains(cm, "a: 1") || !strings.Contains(cm, "b: two") {
		t.Errorf("configmap toYaml wrong:\n%s", cm)
	}

	// NOTES.txt must not be emitted as a manifest.
	for name := range out {
		if strings.HasSuffix(name, "NOTES.txt") {
			t.Errorf("NOTES.txt should not be rendered as a manifest")
		}
	}
}

func TestRenderRequiredFails(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":         "apiVersion: v2\nname: x\nversion: 0.1.0\n",
		"values.yaml":        "name: \"\"\n",
		"templates/svc.yaml": `name: {{ required "name is required" .Values.name }}`,
	})
	_, err := Render(dir, Options{Release: ReleaseMeta{Name: "r", Namespace: "default", Revision: 1}})
	if nil == err {
		t.Fatal("expected required to fail on empty value")
	}
	if !strings.Contains(err.Error(), "required value missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSubchartScoping(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: parent\nversion: 1.0.0\n",
		"values.yaml": "child:\n  message: from-parent\nglobal:\n  env: prod\n",
		"templates/p.yaml": `kind: Parent
env: {{ .Values.global.env }}`,
		"charts/child/Chart.yaml":  "apiVersion: v2\nname: child\nversion: 0.1.0\n",
		"charts/child/values.yaml": "message: default\n",
		"charts/child/templates/c.yaml": `kind: Child
msg: {{ .Values.message }}
genv: {{ .Values.global.env }}`,
	})
	out, err := Render(dir, Options{Release: ReleaseMeta{Name: "r", Namespace: "default", Revision: 1}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	child := find(t, out, "charts/child/templates/c.yaml")
	if !strings.Contains(child, "msg: from-parent") {
		t.Errorf("subchart value not overridden by parent:\n%s", child)
	}
	if !strings.Contains(child, "genv: prod") {
		t.Errorf("global not propagated to subchart:\n%s", child)
	}
}

func TestRenderTplFunction(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":       "apiVersion: v2\nname: t\nversion: 1.0.0\n",
		"values.yaml":      "tpl: \"hello {{ .Release.Name }}\"\n",
		"templates/x.yaml": `msg: {{ tpl .Values.tpl . }}`,
	})
	out, err := Render(dir, Options{Release: ReleaseMeta{Name: "world", Namespace: "default", Revision: 1}})
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	x := find(t, out, "x.yaml")
	if !strings.Contains(x, "msg: hello world") {
		t.Errorf("tpl did not render: %s", x)
	}
}

func TestRenderTplRecursionGuard(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":       "apiVersion: v2\nname: loop\nversion: 1.0.0\n",
		"values.yaml":      "self: \"{{ tpl .Values.self . }}\"\n",
		"templates/x.yaml": `v: {{ tpl .Values.self . }}`,
	})
	_, err := Render(dir, Options{Release: ReleaseMeta{Name: "r", Namespace: "default", Revision: 1}})
	if nil == err {
		t.Fatal("expected a recursion-depth error from self-referential tpl, not a hang")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected depth error, got: %v", err)
	}
}

func TestRenderIncludeSelfLoopGuard(t *testing.T) {
	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":             "apiVersion: v2\nname: loop\nversion: 1.0.0\n",
		"templates/_helpers.tpl": `{{- define "x" -}}{{ include "x" . }}{{- end -}}`,
		"templates/y.yaml":       `v: {{ include "x" . }}`,
	})
	_, err := Render(dir, Options{Release: ReleaseMeta{Name: "r", Namespace: "default", Revision: 1}})
	if nil == err {
		t.Fatal("expected a recursion-depth error from self-including define, not a crash")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected depth error, got: %v", err)
	}
}

func find(t *testing.T, out map[string]string, suffix string) string {
	t.Helper()
	for name, body := range out {
		if strings.HasSuffix(name, suffix) {
			return body
		}
	}
	t.Fatalf("no rendered template ending in %q (have %v)", suffix, keys(out))
	return ""
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
