package kube

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ebogdum/keramos/v3/internal/engine"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func newFakeClient(handler http.Handler) (*Client, func()) {
	server := httptest.NewServer(handler)
	cfg := &rest.Config{Host: server.URL}
	cs, err := kubernetes.NewForConfig(cfg)
	if nil != err {
		panic(err)
	}
	c := &Client{
		clientset: cs,
		namespace: "default",
		timeout:   5 * time.Second,
		discovery: cs.Discovery(),
	}
	return c, server.Close
}

// ---------------------------------------------------------------------------
// NewClient
// ---------------------------------------------------------------------------

func TestNewClient_WithTempKubeconfig(t *testing.T) {
	server := httptest.NewServer(fullAPIHandler())
	defer server.Close()

	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ` + server.URL + `
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    namespace: test-ns
  name: test-context
current-context: test-context
`
	tmpFile := t.TempDir() + "/kubeconfig"
	if err := writeFile(tmpFile, kubeconfig); nil != err {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	c, err := NewClient(tmpFile, "test-context", "my-ns")
	if nil != err {
		t.Fatalf("NewClient failed: %v", err)
	}

	if "my-ns" != c.Namespace() {
		t.Errorf("expected my-ns, got %s", c.Namespace())
	}
	if nil == c.Clientset() {
		t.Error("expected non-nil clientset")
	}
	if defaultTimeout != c.timeout {
		t.Errorf("expected timeout %v, got %v", defaultTimeout, c.timeout)
	}
}

func TestNewClient_WithDefaultNamespace(t *testing.T) {
	server := httptest.NewServer(fullAPIHandler())
	defer server.Close()

	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: ` + server.URL + `
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    namespace: ctx-ns
  name: test-context
current-context: test-context
`
	tmpFile := t.TempDir() + "/kubeconfig"
	if err := writeFile(tmpFile, kubeconfig); nil != err {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	// Empty namespace — should use context namespace
	c, err := NewClient(tmpFile, "", "")
	if nil != err {
		t.Fatalf("NewClient failed: %v", err)
	}

	if "ctx-ns" != c.Namespace() {
		t.Errorf("expected ctx-ns, got %s", c.Namespace())
	}
}

func TestNewClient_InvalidKubeconfig(t *testing.T) {
	tmpFile := t.TempDir() + "/bad-kubeconfig"
	if err := writeFile(tmpFile, "not: valid: kubeconfig"); nil != err {
		t.Fatalf("failed to write: %v", err)
	}

	_, err := NewClient(tmpFile, "", "")
	if nil == err {
		t.Fatal("expected error for invalid kubeconfig")
	}
}

func TestNewClient_MissingKubeconfig(t *testing.T) {
	_, err := NewClient("/nonexistent/path/kubeconfig", "", "")
	if nil == err {
		t.Fatal("expected error for missing kubeconfig")
	}
}

func TestNewClient_EmptyKubeconfigPath(t *testing.T) {
	// With empty path and no KUBECONFIG env or ~/.kube/config, this might fail
	// depending on the environment. Just verify it doesn't panic.
	_, _ = NewClient("", "", "")
}

func writeFile(path, content string) error {
	f, err := os.Create(path)
	if nil != err {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// ---------------------------------------------------------------------------
// WaitForJob
// ---------------------------------------------------------------------------

func TestWaitForJob_Completed(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	err := c.WaitForJob("default", "test-job", 5*time.Second)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForJob_Failed(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "failed-job", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "BackoffLimitExceeded"},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	err := c.WaitForJob("default", "failed-job", 5*time.Second)
	if nil == err {
		t.Fatal("expected error for failed job")
	}
}

func TestWaitForJob_Timeout(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-job", Namespace: "default"},
		Status:     batchv1.JobStatus{},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	err := c.WaitForJob("default", "pending-job", 200*time.Millisecond)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForJob_NotFound(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
	}))
	defer cleanup()

	err := c.WaitForJob("default", "nonexistent", 200*time.Millisecond)
	if nil == err {
		t.Fatal("expected timeout error for nonexistent job")
	}
}

// ---------------------------------------------------------------------------
// CreateNamespace
// ---------------------------------------------------------------------------

func TestCreateNamespace_Success(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "new-ns"},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ns)
	}))
	defer cleanup()

	err := c.CreateNamespace("new-ns")
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCreateNamespace_AlreadyExists(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"AlreadyExists","code":409}`))
	}))
	defer cleanup()

	err := c.CreateNamespace("existing-ns")
	if nil != err {
		t.Fatalf("expected nil for already-existing namespace, got %v", err)
	}
}

func TestCreateNamespace_Error(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"Forbidden","code":403}`))
	}))
	defer cleanup()

	err := c.CreateNamespace("forbidden-ns")
	if nil == err {
		t.Fatal("expected error for forbidden namespace creation")
	}
}

// ---------------------------------------------------------------------------
// waitForDeployment
// ---------------------------------------------------------------------------

func TestWaitForDeployment_Available(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "dep", Namespace: "default", Generation: 1},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			Replicas:           1,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dep)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("dep")
	obj.SetNamespace("default")

	err := c.waitForDeployment(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForDeployment_NotAvailable_TimesOut(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "dep", Namespace: "default"},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dep)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("dep")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForDeployment(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForDeployment_NotFound(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("dep")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForDeployment(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

// ---------------------------------------------------------------------------
// waitForStatefulSet
// ---------------------------------------------------------------------------

func TestWaitForStatefulSet_Ready(t *testing.T) {
	replicas := int32(3)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ss", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ss)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("StatefulSet")
	obj.SetName("ss")
	obj.SetNamespace("default")

	err := c.waitForStatefulSet(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForStatefulSet_NilReplicas(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ss", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ss)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("StatefulSet")
	obj.SetName("ss")
	obj.SetNamespace("default")

	err := c.waitForStatefulSet(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForStatefulSet_NotReady_TimesOut(t *testing.T) {
	replicas := int32(3)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ss", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ss)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("StatefulSet")
	obj.SetName("ss")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForStatefulSet(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

// ---------------------------------------------------------------------------
// waitForPod
// ---------------------------------------------------------------------------

func TestWaitForPod_Running(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pod)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Pod")
	obj.SetName("pod")
	obj.SetNamespace("default")

	err := c.waitForPod(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForPod_Succeeded(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pod)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Pod")
	obj.SetName("pod")
	obj.SetNamespace("default")

	err := c.waitForPod(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForPod_Pending_TimesOut(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pod)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Pod")
	obj.SetName("pod")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForPod(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

// ---------------------------------------------------------------------------
// waitForDaemonSet
// ---------------------------------------------------------------------------

func TestWaitForDaemonSet_Ready(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ds", Namespace: "default"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			NumberReady:            3,
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ds)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("DaemonSet")
	obj.SetName("ds")
	obj.SetNamespace("default")

	err := c.waitForDaemonSet(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForDaemonSet_NotReady_TimesOut(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ds", Namespace: "default"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 3,
			NumberReady:            1,
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ds)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("DaemonSet")
	obj.SetName("ds")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForDaemonSet(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

func TestWaitForDaemonSet_ZeroDesired_TimesOut(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ds", Namespace: "default"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 0,
			NumberReady:            0,
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ds)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("DaemonSet")
	obj.SetName("ds")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := c.waitForDaemonSet(ctx, obj)
	if nil == err {
		t.Fatal("expected timeout error")
	}
}

// ---------------------------------------------------------------------------
// waitForResource routing — with real sub-calls
// ---------------------------------------------------------------------------

func TestWaitForResource_DeploymentRoute(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "dep", Namespace: "default", Generation: 1},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			Replicas:           1,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dep)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("dep")
	obj.SetNamespace("default")

	err := c.waitForResource(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForResource_StatefulSetRoute(t *testing.T) {
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ss", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ss)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("StatefulSet")
	obj.SetName("ss")
	obj.SetNamespace("default")

	err := c.waitForResource(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForResource_PodRoute(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pod)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Pod")
	obj.SetName("pod")
	obj.SetNamespace("default")

	err := c.waitForResource(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForResource_DaemonSetRoute(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "ds", Namespace: "default"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1,
			NumberReady:            1,
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ds)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("DaemonSet")
	obj.SetName("ds")
	obj.SetNamespace("default")

	err := c.waitForResource(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForResource_JobRoute_WithDeadline(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Job")
	obj.SetName("job")
	obj.SetNamespace("default")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := c.waitForResource(ctx, obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWaitForResource_JobRoute_ExpiredDeadline(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Job")
	obj.SetName("job")
	obj.SetNamespace("default")

	// Create a context with deadline already passed
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	// remaining <= 0 so uses defaultTimeout; job is already complete
	err := c.waitForResource(ctx, obj)
	if nil != err {
		t.Fatalf("expected nil (job is complete), got %v", err)
	}
}

func TestWaitForResource_JobRoute_NoDeadline(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
	}))
	defer cleanup()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Job")
	obj.SetName("job")
	obj.SetNamespace("default")

	// Context without deadline
	err := c.waitForResource(context.Background(), obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCapabilities
// ---------------------------------------------------------------------------

func TestGetCapabilities_Success(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version":
			json.NewEncoder(w).Encode(map[string]any{
				"major":      "1",
				"minor":      "28",
				"gitVersion": "v1.28.0",
			})
		case "/apis":
			json.NewEncoder(w).Encode(map[string]any{
				"kind":       "APIGroupList",
				"apiVersion": "v1",
				"groups": []map[string]any{
					{"name": "apps", "versions": []map[string]any{{"groupVersion": "apps/v1", "version": "v1"}}},
				},
			})
		case "/api":
			json.NewEncoder(w).Encode(map[string]any{
				"kind":     "APIVersions",
				"versions": []string{"v1"},
			})
		case "/api/v1":
			json.NewEncoder(w).Encode(map[string]any{
				"kind":         "APIResourceList",
				"groupVersion": "v1",
				"resources":    []map[string]any{},
			})
		case "/apis/apps/v1":
			json.NewEncoder(w).Encode(map[string]any{
				"kind":         "APIResourceList",
				"groupVersion": "apps/v1",
				"resources":    []map[string]any{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cleanup()

	caps, err := c.GetCapabilities()
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}

	kv, ok := caps["kubeVersion"].(*engine.KubeVersion)
	if !ok {
		t.Fatal("missing kubeVersion")
	}
	if "1" != kv.Major {
		t.Errorf("expected major 1, got %v", kv.Major)
	}
	if "28" != kv.Minor {
		t.Errorf("expected minor 28, got %v", kv.Minor)
	}

	if nil == caps["apiVersions"] {
		t.Error("missing apiVersions")
	}
	if nil == caps["groups"] {
		t.Error("missing groups")
	}
}

func TestGetCapabilities_VersionError(t *testing.T) {
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer cleanup()

	_, err := c.GetCapabilities()
	if nil == err {
		t.Fatal("expected error when version endpoint fails")
	}
}

// ---------------------------------------------------------------------------
// Full fake API server for testing apply/delete/dryrun
// ---------------------------------------------------------------------------

// fullAPIHandler handles discovery, dynamic resource operations
func fullAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case "/version" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"major": "1", "minor": "28", "gitVersion": "v1.28.0",
			})

		case "/api" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"kind": "APIVersions", "versions": []string{"v1"},
			})

		case "/api/v1" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"kind":         "APIResourceList",
				"groupVersion": "v1",
				"resources": []map[string]any{
					{"name": "configmaps", "singularName": "configmap", "namespaced": true, "kind": "ConfigMap", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
					{"name": "services", "singularName": "service", "namespaced": true, "kind": "Service", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
					{"name": "pods", "singularName": "pod", "namespaced": true, "kind": "Pod", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
					{"name": "secrets", "singularName": "secret", "namespaced": true, "kind": "Secret", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
					{"name": "namespaces", "singularName": "namespace", "namespaced": false, "kind": "Namespace", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
				},
			})

		case "/apis" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"kind":       "APIGroupList",
				"apiVersion": "v1",
				"groups": []map[string]any{
					{
						"name": "apps",
						"versions": []map[string]any{
							{"groupVersion": "apps/v1", "version": "v1"},
						},
						"preferredVersion": map[string]any{"groupVersion": "apps/v1", "version": "v1"},
					},
					{
						"name": "batch",
						"versions": []map[string]any{
							{"groupVersion": "batch/v1", "version": "v1"},
						},
						"preferredVersion": map[string]any{"groupVersion": "batch/v1", "version": "v1"},
					},
				},
			})

		case "/apis/apps/v1" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"kind":         "APIResourceList",
				"groupVersion": "apps/v1",
				"resources": []map[string]any{
					{"name": "deployments", "singularName": "deployment", "namespaced": true, "kind": "Deployment", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
				},
			})

		case "/apis/batch/v1" == path:
			json.NewEncoder(w).Encode(map[string]any{
				"kind":         "APIResourceList",
				"groupVersion": "batch/v1",
				"resources": []map[string]any{
					{"name": "jobs", "singularName": "job", "namespaced": true, "kind": "Job", "verbs": []string{"get", "list", "create", "update", "patch", "delete"}},
				},
			})

		default:
			// Handle PATCH (apply) and DELETE requests for any resource
			if "PATCH" == r.Method {
				body, _ := io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
				w.Write(body)
				return
			}
			if "DELETE" == r.Method {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"kind": "Status", "apiVersion": "v1", "status": "Success",
				})
				return
			}
			// GET requests for resources
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{})
		}
	})
}

func newFullFakeClient(handler http.Handler) (*Client, func()) {
	server := httptest.NewServer(handler)
	cfg := &rest.Config{Host: server.URL}
	cs, err := kubernetes.NewForConfig(cfg)
	if nil != err {
		panic(err)
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if nil != err {
		panic(err)
	}
	c := &Client{
		clientset:  cs,
		dynamic:    dynClient,
		config:     cfg,
		namespace:  "default",
		discovery:  cs.Discovery(),
		timeout:    5 * time.Second,
		forceApply: true,
	}
	return c, server.Close
}

// ---------------------------------------------------------------------------
// applyResource
// ---------------------------------------------------------------------------

func TestApplyResource_ConfigMap(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test-cm",
				"namespace": "default",
			},
			"data": map[string]any{
				"key": "value",
			},
		},
	}

	err := c.applyResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestApplyResource_ClusterScoped(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "test-ns",
			},
		},
	}

	err := c.applyResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestApplyManifests_FullFlow(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	manifests := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  key: value
`
	err := c.ApplyManifests(manifests)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestApplyManifests_MultipleResources(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	manifests := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
`
	err := c.ApplyManifests(manifests)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// deleteResource
// ---------------------------------------------------------------------------

func TestDeleteResource_ConfigMap(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	err := c.deleteResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDeleteResource_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		// Serve discovery endpoints
		h := fullAPIHandler()
		if !strings.HasPrefix(path, "/api/v1/namespaces/default/configmaps") || "DELETE" != r.Method {
			h.ServeHTTP(w, r)
			return
		}

		// Return NotFound for DELETE
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"kind": "Status", "apiVersion": "v1", "status": "Failure",
			"reason": "NotFound", "code": 404,
		})
	})

	c, cleanup := newFullFakeClient(handler)
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "nonexistent",
				"namespace": "default",
			},
		},
	}

	err := c.deleteResource(obj)
	// NotFound is treated as success
	if nil != err {
		t.Fatalf("expected nil for not-found, got %v", err)
	}
}

func TestDeleteResource_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		h := fullAPIHandler()
		if !strings.HasPrefix(path, "/api/v1/namespaces/default/configmaps") || "DELETE" != r.Method {
			h.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"kind": "Status", "apiVersion": "v1", "status": "Failure",
			"reason": "Forbidden", "code": 403,
		})
	})

	c, cleanup := newFullFakeClient(handler)
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}

	err := c.deleteResource(obj)
	if nil == err {
		t.Fatal("expected error for forbidden delete")
	}
}

func TestDeleteManifests_FullFlow(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	manifests := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
`
	err := c.DeleteManifests(manifests)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDeleteResource_ClusterScoped(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "test-ns",
			},
		},
	}

	err := c.deleteResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// dryRunApplyResource
// ---------------------------------------------------------------------------

func TestDryRunApplyResource_ConfigMap(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	err := c.dryRunApplyResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDryRunApplyResource_ClusterScoped(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "test-ns",
			},
		},
	}

	err := c.dryRunApplyResource(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDryRunApply_FullFlow(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	manifests := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  key: value
`
	err := c.DryRunApply(manifests)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDryRunApplyResource_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		h := fullAPIHandler()
		if !strings.HasPrefix(path, "/api/v1/namespaces/default/configmaps") || "PATCH" != r.Method {
			h.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"kind": "Status", "apiVersion": "v1", "status": "Failure",
			"reason": "Invalid", "code": 422,
		})
	})

	c, cleanup := newFullFakeClient(handler)
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}

	err := c.dryRunApplyResource(obj)
	if nil == err {
		t.Fatal("expected error for invalid dry-run")
	}
}

// ---------------------------------------------------------------------------
// applyResource error
// ---------------------------------------------------------------------------

func TestApplyResource_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		h := fullAPIHandler()
		if !strings.HasPrefix(path, "/api/v1/namespaces/default/configmaps") || "PATCH" != r.Method {
			h.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{
			"kind": "Status", "apiVersion": "v1", "status": "Failure",
			"reason": "Forbidden", "code": 403,
		})
	})

	c, cleanup := newFullFakeClient(handler)
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test",
				"namespace": "default",
			},
		},
	}

	err := c.applyResource(obj)
	if nil == err {
		t.Fatal("expected error for forbidden apply")
	}
}

// ---------------------------------------------------------------------------
// resourceForObj
// ---------------------------------------------------------------------------

func TestResourceForObj_Success(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "test"},
		},
	}

	gvr, err := c.resourceForObj(obj)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
	if "configmaps" != gvr.Resource {
		t.Errorf("expected configmaps, got %s", gvr.Resource)
	}
}

func TestResourceForObj_UnknownKind(t *testing.T) {
	c, cleanup := newFullFakeClient(fullAPIHandler())
	defer cleanup()

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "UnknownResource",
			"metadata":   map[string]any{"name": "test"},
		},
	}

	_, err := c.resourceForObj(obj)
	if nil == err {
		t.Fatal("expected error for unknown resource kind")
	}
}

// ---------------------------------------------------------------------------
// WaitForReady — end-to-end with waitForResource routing
// ---------------------------------------------------------------------------

func TestWaitForReady_MultipleResourceTypes(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "dep", Namespace: "default", Generation: 1},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			Replicas:           1,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// This handler is simple — it returns dep or pod depending on the URL
	c, cleanup := newFakeClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/apis/apps/v1/namespaces/default/deployments/dep" {
			json.NewEncoder(w).Encode(dep)
		} else if r.URL.Path == "/api/v1/namespaces/default/pods/pod" {
			json.NewEncoder(w).Encode(pod)
		} else {
			// For services — return a generic OK
			json.NewEncoder(w).Encode(map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}})
		}
	}))
	defer cleanup()

	manifests := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep
  namespace: default
---
apiVersion: v1
kind: Pod
metadata:
  name: pod
  namespace: default
---
apiVersion: v1
kind: Service
metadata:
  name: svc
  namespace: default
`

	err := c.WaitForReady(manifests, 5*time.Second)
	if nil != err {
		t.Fatalf("expected nil, got %v", err)
	}
}
