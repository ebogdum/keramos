package hooks

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
)

func (m *mockKubeClient) Dynamic() (dynamic.Interface, error) {
	return dynfake.NewSimpleDynamicClient(runtime.NewScheme()), nil
}

// ---------------------------------------------------------------------------
// mockKubeClient implements kube.KubeClient for testing ExecuteHooks
// ---------------------------------------------------------------------------

type mockKubeClient struct {
	namespace       string
	applyErr        error
	deleteErr       error
	waitJobErr      error
	applyCalled     int
	deleteCalled    int
	waitJobCalled   int
}

func (m *mockKubeClient) Namespace() string                              { return m.namespace }
func (m *mockKubeClient) Clientset() kubernetes.Interface                { return nil }
func (m *mockKubeClient) GetCapabilities() (map[string]any, error)      { return nil, nil }
func (m *mockKubeClient) CreateNamespace(_ string) error                { return nil }
func (m *mockKubeClient) SetTimeout(_ time.Duration)                    {}
func (m *mockKubeClient) SetForce(_ bool)                               {}
func (m *mockKubeClient) DryRunApply(_ string) error                    { return nil }
func (m *mockKubeClient) WaitForReady(_ string, _ time.Duration) error  { return nil }
func (m *mockKubeClient) Lookup(_, _, _, _ string) (map[string]any, error) { return nil, nil }
func (m *mockKubeClient) ApplyCRDs(_ string, _ time.Duration) error          { return nil }
func (m *mockKubeClient) DeleteResources(_ string, _ map[string]bool) error  { return nil }
func (m *mockKubeClient) SnapshotResources(_ string) (map[string]bool, error) {
	return nil, nil
}
func (m *mockKubeClient) ResourcesNeedingForce(_ string) (map[string]bool, error) {
	return nil, nil
}

func (m *mockKubeClient) ApplyManifests(_ string) error {
	m.applyCalled++
	return m.applyErr
}

func (m *mockKubeClient) DeleteManifests(_ string) error {
	m.deleteCalled++
	return m.deleteErr
}

func (m *mockKubeClient) WaitForJob(_, _ string, _ time.Duration) error {
	m.waitJobCalled++
	return m.waitJobErr
}

// ---------------------------------------------------------------------------
// ExecuteHooks tests
// ---------------------------------------------------------------------------

func TestExecuteHooks_NoMatchingHooks(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{Type: PreInstall, Weight: 1, Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"},
	}

	results, err := ExecuteHooks(client, hooks, PostInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != results {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestExecuteHooks_EmptyHooks(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	results, err := ExecuteHooks(client, nil, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if nil != results {
		t.Errorf("expected nil results, got %v", results)
	}
}

func TestExecuteHooks_ConfigMapHook(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: pre-install-cm
data:
  key: value
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if "succeeded" != results[0].Status {
		t.Errorf("expected succeeded, got %s", results[0].Status)
	}
	if "pre-install-cm" != results[0].Name {
		t.Errorf("expected pre-install-cm, got %s", results[0].Name)
	}
	if "ConfigMap" != results[0].Kind {
		t.Errorf("expected ConfigMap, got %s", results[0].Kind)
	}
	if 1 != client.applyCalled {
		t.Errorf("expected 1 apply call, got %d", client.applyCalled)
	}
}

func TestExecuteHooks_JobHook(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: batch/v1
kind: Job
metadata:
  name: pre-install-job
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: init
        image: busybox
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if "succeeded" != results[0].Status {
		t.Errorf("expected succeeded, got %s", results[0].Status)
	}
	if 1 != client.waitJobCalled {
		t.Errorf("expected 1 WaitForJob call, got %d", client.waitJobCalled)
	}
}

func TestExecuteHooks_JobHookNoNamespace(t *testing.T) {
	client := &mockKubeClient{namespace: "my-ns"}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: batch/v1
kind: Job
metadata:
  name: job-no-ns
spec:
  template:
    spec:
      containers:
      - name: init
        image: busybox
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if "succeeded" != results[0].Status {
		t.Errorf("expected succeeded, got %s", results[0].Status)
	}
}

func TestExecuteHooks_ApplyFails(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		applyErr:  fmt.Errorf("connection refused"),
	}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil == err {
		t.Fatal("expected error from apply failure")
	}

	if 0 == len(results) {
		t.Fatal("expected at least 1 result even on failure")
	}
	if "failed" != results[0].Status {
		t.Errorf("expected failed status, got %s", results[0].Status)
	}
}

func TestExecuteHooks_InvalidManifest(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:     PreInstall,
			Weight:   1,
			Manifest: "not: valid: yaml: [",
			Timeout:  5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil == err {
		t.Fatal("expected error for invalid manifest")
	}

	if 0 == len(results) {
		t.Fatal("expected result even on parse failure")
	}
	if "failed" != results[0].Status {
		t.Errorf("expected failed, got %s", results[0].Status)
	}
}

func TestExecuteHooks_WaitJobFails(t *testing.T) {
	client := &mockKubeClient{
		namespace:  "default",
		waitJobErr: fmt.Errorf("job timed out"),
	}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: batch/v1
kind: Job
metadata:
  name: failing-job
  namespace: default
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil == err {
		t.Fatal("expected error from job wait failure")
	}

	// Should have a failed result for the job
	found := false
	for _, r := range results {
		if "failed" == r.Status {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one failed result")
	}
}

func TestExecuteHooks_SortedByWeight(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 10,
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: second
`,
			Timeout: 5 * time.Minute,
		},
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: first
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 2 != len(results) {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Weight 1 should be first
	if "first" != results[0].Name {
		t.Errorf("expected first hook first (weight 1), got %s", results[0].Name)
	}
}

func TestExecuteHooks_BeforeHookCreationDeletePolicy(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:         PreInstall,
			Weight:       1,
			DeletePolicy: "before-hook-creation",
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if "succeeded" != results[0].Status {
		t.Errorf("expected succeeded, got %s", results[0].Status)
	}
	// Delete should have been called before apply
	if 1 != client.deleteCalled {
		t.Errorf("expected 1 delete call (before-hook-creation), got %d", client.deleteCalled)
	}
}

func TestExecuteHooks_HookSucceededDeletePolicy(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:         PreInstall,
			Weight:       1,
			DeletePolicy: "hook-succeeded",
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 1 != len(results) {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Delete should have been called after successful execution
	if 1 != client.deleteCalled {
		t.Errorf("expected 1 delete call (hook-succeeded), got %d", client.deleteCalled)
	}
}

func TestExecuteHooks_BeforeHookCreationDeleteFails(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		deleteErr: fmt.Errorf("permission denied"),
	}

	hooks := []Hook{
		{
			Type:         PreInstall,
			Weight:       1,
			DeletePolicy: "before-hook-creation",
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			Timeout: 5 * time.Minute,
		},
	}

	_, err := ExecuteHooks(client, hooks, PreInstall)
	if nil == err {
		t.Fatal("expected error when before-hook-creation delete fails")
	}
}

func TestExecuteHooks_HookSucceededDeleteFails(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		deleteErr: fmt.Errorf("permission denied"),
	}

	hooks := []Hook{
		{
			Type:         PreInstall,
			Weight:       1,
			DeletePolicy: "hook-succeeded",
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			Timeout: 5 * time.Minute,
		},
	}

	_, err := ExecuteHooks(client, hooks, PreInstall)
	if nil == err {
		t.Fatal("expected error when hook-succeeded delete fails")
	}
}

// ---------------------------------------------------------------------------
// deleteHookResources
// ---------------------------------------------------------------------------

func TestDeleteHookResources_Success(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	err := deleteHookResources(client, `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteHookResources_NotFoundIgnored(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		deleteErr: fmt.Errorf("resource not found"),
	}

	err := deleteHookResources(client, "manifest")
	if nil != err {
		t.Fatalf("expected nil for not-found error, got %v", err)
	}
}

func TestDeleteHookResources_NotFoundCamelCase(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		deleteErr: fmt.Errorf("resource NotFound in cluster"),
	}

	err := deleteHookResources(client, "manifest")
	if nil != err {
		t.Fatalf("expected nil for NotFound error, got %v", err)
	}
}

func TestDeleteHookResources_RealError(t *testing.T) {
	client := &mockKubeClient{
		namespace: "default",
		deleteErr: fmt.Errorf("permission denied"),
	}

	err := deleteHookResources(client, "manifest")
	if nil == err {
		t.Fatal("expected error for real failure")
	}
}

func TestExecuteHooks_MultipleResourcesInManifest(t *testing.T) {
	client := &mockKubeClient{namespace: "default"}

	hooks := []Hook{
		{
			Type:   PreInstall,
			Weight: 1,
			Manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
`,
			Timeout: 5 * time.Minute,
		},
	}

	results, err := ExecuteHooks(client, hooks, PreInstall)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 2 != len(results) {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if "cm1" != results[0].Name {
		t.Errorf("expected cm1, got %s", results[0].Name)
	}
	if "cm2" != results[1].Name {
		t.Errorf("expected cm2, got %s", results[1].Name)
	}
}
