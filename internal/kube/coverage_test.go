package kube

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ---------------------------------------------------------------------------
// ParseManifests — additional edge cases
// ---------------------------------------------------------------------------

func TestParseManifests_InvalidYAML(t *testing.T) {
	_, err := ParseManifests("not: valid: yaml: [")
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseManifests_InvalidJSON(t *testing.T) {
	// YAML that converts to JSON but isn't a valid K8s object structure
	input := `- just
- a
- list
`
	_, err := ParseManifests(input)
	if nil == err {
		t.Fatal("expected error for non-object YAML")
	}
}

func TestParseManifests_DocumentSeparatorOnly(t *testing.T) {
	resources, err := ParseManifests("---\n---\n---")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(resources) {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestParseManifests_NullDocument(t *testing.T) {
	input := `---
null
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`
	resources, err := ParseManifests(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(resources) {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
}

func TestParseManifests_MissingKind(t *testing.T) {
	input := `apiVersion: v1
metadata:
  name: no-kind
`
	// K8s unmarshalling requires Kind, so this should error
	_, err := ParseManifests(input)
	if nil == err {
		t.Fatal("expected error for object without kind")
	}
}

func TestParseManifests_MultipleNamespacedResources(t *testing.T) {
	input := `apiVersion: v1
kind: Service
metadata:
  name: svc-1
  namespace: ns-a
---
apiVersion: v1
kind: Service
metadata:
  name: svc-2
  namespace: ns-b
`
	resources, err := ParseManifests(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(resources) {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	if "ns-a" != resources[0].GetNamespace() {
		t.Errorf("expected ns-a, got %s", resources[0].GetNamespace())
	}
	if "ns-b" != resources[1].GetNamespace() {
		t.Errorf("expected ns-b, got %s", resources[1].GetNamespace())
	}
}

func TestParseManifests_SpecialCharactersInValues(t *testing.T) {
	input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: special-chars
data:
  key: "value with \"quotes\" and\nnewlines"
  unicode: "日本語テスト"
`
	resources, err := ParseManifests(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(resources) {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
}

func TestParseManifests_LargeManifest(t *testing.T) {
	// Build a multi-doc YAML with many resources
	var b strings.Builder
	for i := 0; i < 50; i++ {
		if 0 < i {
			b.WriteString("---\n")
		}
		b.WriteString("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm-")
		b.WriteString(strings.Repeat("x", 5))
		b.WriteString("\n")
	}

	resources, err := ParseManifests(b.String())
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 50 != len(resources) {
		t.Errorf("expected 50 resources, got %d", len(resources))
	}
}

func TestParseManifests_CommentsAndWhitespace(t *testing.T) {
	input := `# This is a comment
apiVersion: v1
kind: ConfigMap
metadata:
  name: with-comments
# Another comment
data:
  key: value
`
	resources, err := ParseManifests(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(resources) {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// SortByInstallOrder / SortByUninstallOrder — additional cases
// ---------------------------------------------------------------------------

func TestSortByInstallOrder_AllKnownKinds(t *testing.T) {
	kinds := []string{
		"Ingress", "CronJob", "Job", "DaemonSet", "StatefulSet",
		"Deployment", "Service", "PersistentVolumeClaim", "Secret",
		"ConfigMap", "RoleBinding", "Role", "ClusterRoleBinding",
		"ClusterRole", "ServiceAccount", "Namespace",
	}

	resources := make([]*unstructured.Unstructured, 0, len(kinds))
	for _, k := range kinds {
		resources = append(resources, makeUnstructured(k, "test-"+strings.ToLower(k)))
	}

	sorted := SortByInstallOrder(resources)

	expectedOrder := []string{
		"Namespace", "ServiceAccount", "ClusterRole", "ClusterRoleBinding",
		"Role", "RoleBinding", "ConfigMap", "Secret", "PersistentVolumeClaim",
		"Service", "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Ingress",
	}

	for i, expected := range expectedOrder {
		if expected != sorted[i].GetKind() {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i].GetKind())
		}
	}
}

func TestSortByUninstallOrder_AllKnownKinds(t *testing.T) {
	kinds := []string{
		"Namespace", "ServiceAccount", "ClusterRole", "ClusterRoleBinding",
		"Role", "RoleBinding", "ConfigMap", "Secret", "PersistentVolumeClaim",
		"Service", "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Ingress",
	}

	resources := make([]*unstructured.Unstructured, 0, len(kinds))
	for _, k := range kinds {
		resources = append(resources, makeUnstructured(k, "test-"+strings.ToLower(k)))
	}

	sorted := SortByUninstallOrder(resources)

	// Uninstall = reverse of install
	expectedOrder := []string{
		"Ingress", "CronJob", "Job", "DaemonSet", "StatefulSet",
		"Deployment", "Service", "PersistentVolumeClaim", "Secret",
		"ConfigMap", "RoleBinding", "Role", "ClusterRoleBinding",
		"ClusterRole", "ServiceAccount", "Namespace",
	}

	for i, expected := range expectedOrder {
		if expected != sorted[i].GetKind() {
			t.Errorf("position %d: expected %s, got %s", i, expected, sorted[i].GetKind())
		}
	}
}

func TestSortByInstallOrder_Empty(t *testing.T) {
	sorted := SortByInstallOrder(nil)
	if 0 != len(sorted) {
		t.Errorf("expected empty, got %d", len(sorted))
	}
}

func TestSortByUninstallOrder_Empty(t *testing.T) {
	sorted := SortByUninstallOrder(nil)
	if 0 != len(sorted) {
		t.Errorf("expected empty, got %d", len(sorted))
	}
}

func TestSortByInstallOrder_SingleElement(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("Deployment", "single"),
	}
	sorted := SortByInstallOrder(resources)
	if 1 != len(sorted) {
		t.Fatalf("expected 1, got %d", len(sorted))
	}
	if "Deployment" != sorted[0].GetKind() {
		t.Errorf("expected Deployment, got %s", sorted[0].GetKind())
	}
}

func TestSortByInstallOrder_UnknownKindsGroupedAtEnd(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("MyCustomResource", "cr1"),
		makeUnstructured("Namespace", "ns"),
		makeUnstructured("AnotherCRD", "cr2"),
		makeUnstructured("Deployment", "dep"),
	}

	sorted := SortByInstallOrder(resources)
	if "Namespace" != sorted[0].GetKind() {
		t.Errorf("expected Namespace first, got %s", sorted[0].GetKind())
	}
	if "Deployment" != sorted[1].GetKind() {
		t.Errorf("expected Deployment second, got %s", sorted[1].GetKind())
	}
	// Unknown kinds (100) should come after Deployment (10)
	for _, r := range sorted[2:] {
		order := kindOrder(r.GetKind())
		if defaultInstallOrder != order {
			t.Errorf("expected unknown kinds at end, got kind %s with order %d", r.GetKind(), order)
		}
	}
}

func TestSortByInstallOrder_DoesNotMutateInput(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("Deployment", "dep"),
		makeUnstructured("Namespace", "ns"),
	}
	original0 := resources[0].GetKind()

	_ = SortByInstallOrder(resources)

	if original0 != resources[0].GetKind() {
		t.Error("SortByInstallOrder mutated the input slice")
	}
}

// ---------------------------------------------------------------------------
// resolveNamespace
// ---------------------------------------------------------------------------

func TestResolveNamespace_ObjectHasNamespace(t *testing.T) {
	c := &Client{namespace: "default"}
	obj := makeUnstructured("Deployment", "dep")
	obj.SetNamespace("custom-ns")

	ns := c.resolveNamespace(obj)
	if "custom-ns" != ns {
		t.Errorf("expected custom-ns, got %s", ns)
	}
}

func TestResolveNamespace_ClusterScopedKinds(t *testing.T) {
	c := &Client{namespace: "default"}

	clusterKinds := []string{
		"Namespace", "ClusterRole", "ClusterRoleBinding",
		"PersistentVolume", "CustomResourceDefinition",
	}

	for _, kind := range clusterKinds {
		obj := makeUnstructured(kind, "test")
		ns := c.resolveNamespace(obj)
		if "" != ns {
			t.Errorf("kind %s: expected empty namespace for cluster-scoped, got %s", kind, ns)
		}
	}
}

func TestResolveNamespace_NamespacedKindFallsBackToClient(t *testing.T) {
	c := &Client{namespace: "my-ns"}

	namespacedKinds := []string{
		"Deployment", "Service", "ConfigMap", "Secret",
		"Pod", "Job", "StatefulSet", "DaemonSet", "CronJob",
	}

	for _, kind := range namespacedKinds {
		obj := makeUnstructured(kind, "test")
		ns := c.resolveNamespace(obj)
		if "my-ns" != ns {
			t.Errorf("kind %s: expected client namespace my-ns, got %s", kind, ns)
		}
	}
}

func TestResolveNamespace_ClusterScopedWithExplicitNS(t *testing.T) {
	// If a cluster-scoped resource happens to have a namespace set on the object,
	// the object's namespace wins (resolveNamespace checks obj ns first).
	c := &Client{namespace: "default"}
	obj := makeUnstructured("Namespace", "my-ns")
	obj.SetNamespace("explicit")

	ns := c.resolveNamespace(obj)
	if "explicit" != ns {
		t.Errorf("expected explicit, got %s", ns)
	}
}

// ---------------------------------------------------------------------------
// boolPtr
// ---------------------------------------------------------------------------

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	if nil == trueVal || !*trueVal {
		t.Error("expected *true")
	}

	falseVal := boolPtr(false)
	if nil == falseVal || *falseVal {
		t.Error("expected *false")
	}
}

// ---------------------------------------------------------------------------
// SetForce
// ---------------------------------------------------------------------------

func TestSetForce(t *testing.T) {
	c := &Client{forceApply: true}
	c.SetForce(false)
	if c.forceApply {
		t.Error("expected forceApply to be false")
	}
	c.SetForce(true)
	if !c.forceApply {
		t.Error("expected forceApply to be true")
	}
}

// ---------------------------------------------------------------------------
// newContext
// ---------------------------------------------------------------------------

func TestNewContext_DeadlineSet(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"default", defaultTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{timeout: tt.timeout}
			ctx, cancel := c.newContext()
			defer cancel()

			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected context to have deadline")
			}

			remaining := time.Until(deadline)
			if remaining > tt.timeout || remaining < tt.timeout-1*time.Second {
				t.Errorf("expected ~%v remaining, got %v", tt.timeout, remaining)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// waitForResource routing
// ---------------------------------------------------------------------------

func TestWaitForResource_AllImmediateKinds(t *testing.T) {
	c := &Client{namespace: "default", timeout: 5 * time.Second}
	ctx := context.Background()

	// These kinds return nil immediately (Service, or fall to default)
	immediateKinds := []string{
		"Service", "ConfigMap", "Secret", "Ingress", "CronJob",
		"PersistentVolumeClaim", "ClusterRole", "RoleBinding",
		"CustomResource", "NetworkPolicy", "HorizontalPodAutoscaler",
	}

	for _, kind := range immediateKinds {
		obj := &unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetName("test-" + strings.ToLower(kind))

		err := c.waitForResource(ctx, obj)
		if nil != err {
			t.Errorf("kind %s: expected nil, got %v", kind, err)
		}
	}
}

// ---------------------------------------------------------------------------
// ApplyManifests / DeleteManifests / DryRunApply — empty/invalid input paths
// ---------------------------------------------------------------------------

func TestApplyManifests_EmptyInput(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.ApplyManifests("")
	if nil != err {
		t.Fatalf("expected nil for empty manifests, got %v", err)
	}
}

func TestApplyManifests_InvalidYAML(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.ApplyManifests("not: valid: yaml: [")
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDeleteManifests_EmptyInput(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.DeleteManifests("")
	if nil != err {
		t.Fatalf("expected nil for empty manifests, got %v", err)
	}
}

func TestDeleteManifests_InvalidYAML(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.DeleteManifests("not: valid: yaml: [")
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestWaitForReady_EmptyInput(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.WaitForReady("", 5*time.Second)
	if nil != err {
		t.Fatalf("expected nil for empty manifests, got %v", err)
	}
}

func TestWaitForReady_InvalidYAML(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.WaitForReady("not: valid: yaml: [", 5*time.Second)
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestWaitForReady_ServiceOnly(t *testing.T) {
	c := &Client{namespace: "default"}
	input := `apiVersion: v1
kind: Service
metadata:
  name: my-svc
`
	err := c.WaitForReady(input, 5*time.Second)
	if nil != err {
		t.Fatalf("expected nil for Service-only manifests, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// kindOrder
// ---------------------------------------------------------------------------

func TestKindOrder_AllDefinedKinds(t *testing.T) {
	// Order updated to put CRDs first so charts that ship CRDs in
	// templates/ get them applied before any custom-resource instances.
	expectedOrders := map[string]int{
		"CustomResourceDefinition": 0,
		"Namespace":                1,
		"ServiceAccount":           2,
		"ClusterRole":              3,
		"ClusterRoleBinding":       4,
		"Role":                     5,
		"RoleBinding":              6,
		"ConfigMap":                7,
		"Secret":                   8,
		"PersistentVolumeClaim":    9,
		"Service":                  10,
		"Deployment":               11,
		"StatefulSet":              12,
		"DaemonSet":                13,
		"Job":                      14,
		"CronJob":                  15,
		"Ingress":                  16,
	}

	for kind, expected := range expectedOrders {
		got := kindOrder(kind)
		if expected != got {
			t.Errorf("kindOrder(%s): expected %d, got %d", kind, expected, got)
		}
	}
}

func TestKindOrder_Unknown(t *testing.T) {
	unknowns := []string{"FooBar", "MyResource", "", "deployment"}
	for _, kind := range unknowns {
		got := kindOrder(kind)
		if defaultInstallOrder != got {
			t.Errorf("kindOrder(%q): expected %d, got %d", kind, defaultInstallOrder, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Clientset accessor
// ---------------------------------------------------------------------------

func TestClientsetAccessor(t *testing.T) {
	c := &Client{}
	// Clientset returns the underlying *kubernetes.Clientset (may be nil for uninitialized client)
	_ = c.Clientset()
}
