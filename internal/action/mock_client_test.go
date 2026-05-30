package action

import (
	"time"

	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/apimachinery/pkg/runtime"
)

func (m *mockKubeClient) Dynamic() (dynamic.Interface, error) {
	return dynfake.NewSimpleDynamicClient(runtime.NewScheme()), nil
}

type mockKubeClient struct {
	namespace        string
	applyErr         error
	deleteErr        error
	waitErr          error
	dryRunErr        error
	createNSErr      error
	capsErr          error
	appliedManifests []string
	deletedManifests []string
	dryRunManifests  []string
	clientset        kubernetes.Interface
	lookupFn         func(apiVersion, kind, namespace, name string) (map[string]any, error)
}

func newMockClient(ns string) *mockKubeClient {
	return &mockKubeClient{
		namespace: ns,
		clientset: fake.NewSimpleClientset(),
	}
}

func (m *mockKubeClient) Namespace() string { return m.namespace }

func (m *mockKubeClient) Clientset() kubernetes.Interface { return m.clientset }

func (m *mockKubeClient) ApplyManifests(manifests string) error {
	m.appliedManifests = append(m.appliedManifests, manifests)
	return m.applyErr
}

func (m *mockKubeClient) DeleteManifests(manifests string) error {
	m.deletedManifests = append(m.deletedManifests, manifests)
	return m.deleteErr
}

func (m *mockKubeClient) WaitForReady(_ string, _ time.Duration) error {
	return m.waitErr
}

func (m *mockKubeClient) GetCapabilities() (map[string]any, error) {
	if nil != m.capsErr {
		return nil, m.capsErr
	}
	return map[string]any{}, nil
}

func (m *mockKubeClient) CreateNamespace(_ string) error {
	return m.createNSErr
}

func (m *mockKubeClient) WaitForJob(_, _ string, _ time.Duration) error {
	return m.waitErr
}

func (m *mockKubeClient) DryRunApply(manifests string) error {
	m.dryRunManifests = append(m.dryRunManifests, manifests)
	return m.dryRunErr
}

func (m *mockKubeClient) Lookup(apiVersion, kind, namespace, name string) (map[string]any, error) {
	if nil != m.lookupFn {
		return m.lookupFn(apiVersion, kind, namespace, name)
	}
	return nil, nil
}

func (m *mockKubeClient) ApplyCRDs(manifests string, _ time.Duration) error {
	m.appliedManifests = append(m.appliedManifests, manifests)
	return m.applyErr
}

func (m *mockKubeClient) DeleteResources(manifests string, _ map[string]bool) error {
	m.deletedManifests = append(m.deletedManifests, manifests)
	return m.deleteErr
}

func (m *mockKubeClient) SnapshotResources(_ string) (map[string]bool, error) {
	return nil, nil
}

func (m *mockKubeClient) ResourcesNeedingForce(_ string) (map[string]bool, error) {
	return nil, nil
}

func (m *mockKubeClient) SetTimeout(_ time.Duration) {}

func (m *mockKubeClient) SetForce(_ bool) {}
