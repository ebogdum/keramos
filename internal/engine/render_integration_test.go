package engine

import (
	"strings"
	"testing"
)

// Keramos's expression syntax uses lowercase namespace roots: values, release,
// package, capabilities. Function pipelines pipe the value through fns.

func TestRender_TplRecursive(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}
data:
  greeting: ${values.greeting | tpl}
`,
	}
	ctx := &RenderContext{
		Values:  map[string]any{"greeting": "hello ${release.name}"},
		Package: map[string]any{"name": "demo"},
		Release: map[string]any{"name": "myrelease"},
	}
	out, err := eng.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "hello myrelease") {
		t.Errorf("expected tpl-evaluated greeting in output:\n%s", out)
	}
}

func TestRender_LookupReturnsEmptyWhenNoClient(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}
`,
	}
	ctx := &RenderContext{
		Values:  map[string]any{},
		Package: map[string]any{"name": "p"},
		Release: map[string]any{"name": "r"},
	}
	out, err := eng.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "name: r") {
		t.Errorf("render did not produce ConfigMap:\n%s", out)
	}
}

func TestRender_FilesGet(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: x
data:
  body: ${values.filename | Files.Get}
`,
	}
	ctx := &RenderContext{
		Values: map[string]any{"filename": "config.txt"},
		Files:  map[string][]byte{"config.txt": []byte("hello world")},
	}
	out, err := eng.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("Files.Get output missing:\n%s", out)
	}
}

func TestRender_PrintfNumeric(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: x
data:
  msg: ${values.format | printf(8080)}
`,
	}
	ctx := &RenderContext{
		Values: map[string]any{"format": "port=%d"},
	}
	out, err := eng.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "port=8080") {
		t.Errorf("printf typed numeric failed:\n%s", out)
	}
}

func TestRender_DateStrftime(t *testing.T) {
	eng := New()
	templates := map[string]string{
		"cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: x
data:
  ts: ${values.stamp | toDate("2006-01-02") | date("%Y/%m/%d")}
`,
	}
	ctx := &RenderContext{
		Values: map[string]any{"stamp": "2025-06-15"},
	}
	out, err := eng.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "2025/06/15") {
		t.Errorf("strftime conversion failed:\n%s", out)
	}
}
