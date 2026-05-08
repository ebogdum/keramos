package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseManifests(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantKinds []string
		wantErr   bool
	}{
		{
			name: "single deployment",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
`,
			wantCount: 1,
			wantKinds: []string{"Deployment"},
		},
		{
			name: "multi-document yaml",
			input: `apiVersion: v1
kind: Service
metadata:
  name: my-svc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-dep
`,
			wantCount: 2,
			wantKinds: []string{"Service", "Deployment"},
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "whitespace only",
			input:     "   \n\n  ",
			wantCount: 0,
		},
		{
			name: "with empty documents",
			input: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
---
---
`,
			wantCount: 1,
			wantKinds: []string{"ConfigMap"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := ParseManifests(tt.input)
			if tt.wantErr {
				if nil == err {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantCount != len(resources) {
				t.Fatalf("expected %d resources, got %d", tt.wantCount, len(resources))
			}
			for i, wantKind := range tt.wantKinds {
				if wantKind != resources[i].GetKind() {
					t.Errorf("resource %d: expected kind %s, got %s", i, wantKind, resources[i].GetKind())
				}
			}
		})
	}
}

func TestSortByInstallOrder(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("Deployment", "my-dep"),
		makeUnstructured("Namespace", "my-ns"),
		makeUnstructured("Service", "my-svc"),
		makeUnstructured("ConfigMap", "my-cm"),
		makeUnstructured("ClusterRole", "my-cr"),
		makeUnstructured("ServiceAccount", "my-sa"),
	}

	sorted := SortByInstallOrder(resources)

	expectedOrder := []string{"Namespace", "ServiceAccount", "ClusterRole", "ConfigMap", "Service", "Deployment"}

	if len(expectedOrder) != len(sorted) {
		t.Fatalf("expected %d resources, got %d", len(expectedOrder), len(sorted))
	}

	for i, expected := range expectedOrder {
		if expected != sorted[i].GetKind() {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i].GetKind())
		}
	}
}

func TestSortByUninstallOrder(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("Namespace", "my-ns"),
		makeUnstructured("Deployment", "my-dep"),
		makeUnstructured("Service", "my-svc"),
		makeUnstructured("ConfigMap", "my-cm"),
	}

	sorted := SortByUninstallOrder(resources)

	expectedOrder := []string{"Deployment", "Service", "ConfigMap", "Namespace"}

	if len(expectedOrder) != len(sorted) {
		t.Fatalf("expected %d resources, got %d", len(expectedOrder), len(sorted))
	}

	for i, expected := range expectedOrder {
		if expected != sorted[i].GetKind() {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i].GetKind())
		}
	}
}

func TestSortStability(t *testing.T) {
	// Unknown kinds should maintain relative order
	resources := []*unstructured.Unstructured{
		makeUnstructured("CustomResource", "cr-a"),
		makeUnstructured("CustomResource", "cr-b"),
		makeUnstructured("Namespace", "ns"),
	}

	sorted := SortByInstallOrder(resources)

	if "Namespace" != sorted[0].GetKind() {
		t.Errorf("expected Namespace first, got %s", sorted[0].GetKind())
	}
	if "cr-a" != sorted[1].GetName() {
		t.Errorf("expected cr-a before cr-b for stability, got %s", sorted[1].GetName())
	}
}

func TestKindOrder(t *testing.T) {
	if kindOrder("Namespace") >= kindOrder("Deployment") {
		t.Error("Namespace should come before Deployment")
	}
	if kindOrder("Service") >= kindOrder("Deployment") {
		t.Error("Service should come before Deployment")
	}
	if kindOrder("UnknownKind") != defaultInstallOrder {
		t.Error("unknown kinds should return defaultInstallOrder")
	}
}

func makeUnstructured(kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]any{
				"name": name,
			},
		},
	}
}
