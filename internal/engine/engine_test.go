package engine

import (
	"strings"
	"testing"
)

func TestRenderFileSimple(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `apiVersion: v1
kind: Service
metadata:
  name: ${values.name}
spec:
  type: ClusterIP
`

	docs, err := e.RenderFile("service.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if !strings.Contains(docs[0], "name: myapp") {
		t.Errorf("expected 'name: myapp' in output, got:\n%s", docs[0])
	}
}

func TestRenderFileMultiDocument(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${values.name}-config
---
apiVersion: v1
kind: Service
metadata:
  name: ${values.name}-svc
`

	docs, err := e.RenderFile("multi.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(docs) {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if !strings.Contains(docs[0], "myapp-config") {
		t.Errorf("expected 'myapp-config' in first doc")
	}
	if !strings.Contains(docs[1], "myapp-svc") {
		t.Errorf("expected 'myapp-svc' in second doc")
	}
}

func TestRenderSkipsPartials(t *testing.T) {
	e := New()
	ctx := testContext()

	templates := map[string]string{
		"_helpers.yaml": "labels:\n  app: test\n",
		"service.yaml":  "apiVersion: v1\nkind: Service\n",
	}

	result, err := e.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "labels") {
		t.Error("partial content should not appear in output")
	}
	if !strings.Contains(result, "Service") {
		t.Error("service should appear in output")
	}
}

func TestRenderWithPartials(t *testing.T) {
	e := New()
	ctx := testContext()

	partials := map[string]any{
		"labels": map[string]any{
			"app":        "${values.name}",
			"managed-by": "keramos",
		},
	}

	content := `apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    $include: labels
`

	docs, err := e.RenderFile("configmap.yaml", content, partials, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if !strings.Contains(docs[0], "app: myapp") {
		t.Errorf("expected resolved label, got:\n%s", docs[0])
	}
	if !strings.Contains(docs[0], "managed-by: keramos") {
		t.Errorf("expected managed-by label, got:\n%s", docs[0])
	}
}

func TestRenderConditionalDocument(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"ingress": map[string]any{
				"enabled": false,
			},
		},
	}

	content := `$if: ${values.ingress.enabled}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
`

	docs, err := e.RenderFile("ingress.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(docs) {
		t.Errorf("expected 0 documents for disabled ingress, got %d", len(docs))
	}
}

func TestRenderConditionalDocumentEnabled(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"ingress": map[string]any{
				"enabled": true,
			},
		},
	}

	content := `$if: ${values.ingress.enabled}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
`

	docs, err := e.RenderFile("ingress.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if !strings.Contains(docs[0], "kind: Ingress") {
		t.Errorf("expected Ingress in output")
	}
}

func TestRenderFullPipeline(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"name":     "myapp",
			"replicas": 3,
			"image": map[string]any{
				"repository": "nginx",
				"tag":        "latest",
			},
			"autoscaling": map[string]any{
				"enabled": false,
			},
		},
		Release: map[string]any{
			"name":      "prod-release",
			"namespace": "production",
		},
		Package: map[string]any{
			"name":    "mypackage",
			"version": "1.0.0",
		},
	}

	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${values.name}
  namespace: ${release.namespace}
spec:
  replicas: ${values.replicas}
  template:
    spec:
      containers:
        - name: ${values.name}
          image: ${values.image.repository}:${values.image.tag}
`

	docs, err := e.RenderFile("deployment.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	output := docs[0]
	if !strings.Contains(output, "name: myapp") {
		t.Errorf("expected name substitution")
	}
	if !strings.Contains(output, "namespace: production") {
		t.Errorf("expected namespace from release")
	}
	if !strings.Contains(output, "replicas: 3") {
		t.Errorf("expected replicas=3")
	}
	if !strings.Contains(output, "image: nginx:latest") {
		t.Errorf("expected image substitution, got:\n%s", output)
	}
}

func TestRenderMultipleTemplates(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"name": "myapp",
		},
	}

	templates := map[string]string{
		"service.yaml": "apiVersion: v1\nkind: Service\nmetadata:\n  name: ${values.name}\n",
		"configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${values.name}-config\n",
	}

	result, err := e.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "kind: Service") {
		t.Error("expected Service in output")
	}
	if !strings.Contains(result, "kind: ConfigMap") {
		t.Error("expected ConfigMap in output")
	}
}

func TestRenderCleansKeramosDirectives(t *testing.T) {
	e := New()
	ctx := &RenderContext{
		Values: map[string]any{
			"name":    "myapp",
			"enabled": true,
		},
	}

	// Keramos's own directive keys ($if, $each, $switch, $include, ...)
	// must be stripped from the rendered output. Other $-prefixed keys
	// (JSON Schema's $ref, $schema, $defs, $id; CRD extensions) MUST be
	// preserved — stripping them by prefix corrupted documents that
	// legitimately use those names.
	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${values.name}
  $if: ${values.enabled}
data:
  schemaRef:
    $ref: '#/$defs/foo'
    $schema: 'https://json-schema.org/draft/2020-12/schema'
`

	docs, err := e.RenderFile("cm.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if strings.Contains(docs[0], "$if") {
		t.Error("$if directive should have been cleaned")
	}
	if !strings.Contains(docs[0], "$ref") {
		t.Error("$ref must be preserved (not a keramos directive)")
	}
	if !strings.Contains(docs[0], "$schema") {
		t.Error("$schema must be preserved (not a keramos directive)")
	}
}

func TestRenderWithSubdirectoryPartials(t *testing.T) {
	e := New()
	ctx := testContext()

	templates := map[string]string{
		"templates/_helpers.yaml": "labels:\n  app: test\n",
		"templates/svc.yaml":      "apiVersion: v1\nkind: Service\n",
	}

	result, err := e.Render(templates, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Service") {
		t.Error("expected Service in output")
	}
}

func TestRenderNilPartials(t *testing.T) {
	e := New()
	ctx := testContext()

	content := `apiVersion: v1
kind: Service
`

	docs, err := e.RenderFile("svc.yaml", content, nil, ctx)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(docs) {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
}
