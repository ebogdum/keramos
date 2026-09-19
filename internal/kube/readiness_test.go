package kube

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

func int32Ptr(v int32) *int32 { return &v }

func objectOfKind(kind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace("default")
	return obj
}

func TestWaitForStatefulSetRejectsStaleStatus(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", Generation: 2},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32Ptr(1)},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 1,
			ReadyReplicas:      1,
			UpdatedReplicas:    1,
			CurrentRevision:    "db-1",
			UpdateRevision:     "db-2",
		},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(ss)}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := c.waitForStatefulSet(ctx, objectOfKind("StatefulSet", "db")); nil == err {
		t.Fatal("expected a timeout while the StatefulSet status still describes the previous generation")
	}
}

func TestWaitForStatefulSetAcceptsRolledOutStatus(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", Generation: 2},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32Ptr(2)},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 2,
			ReadyReplicas:      2,
			UpdatedReplicas:    2,
			CurrentRevision:    "db-2",
			UpdateRevision:     "db-2",
		},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(ss)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := c.waitForStatefulSet(ctx, objectOfKind("StatefulSet", "db")); nil != err {
		t.Fatalf("expected the fully rolled out StatefulSet to be ready, got %v", err)
	}
}

func TestWaitForDaemonSetWithNoSchedulableNodesIsReady(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", Generation: 1},
		Status:     appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 0},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(ds)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.waitForDaemonSet(ctx, objectOfKind("DaemonSet", "agent")); nil != err {
		t.Fatalf("a DaemonSet with no schedulable nodes must not block the wait, got %v", err)
	}
}

func TestWaitForDaemonSetRejectsPartialRollout(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", Generation: 2},
		Status: appsv1.DaemonSetStatus{
			ObservedGeneration:     2,
			DesiredNumberScheduled: 3,
			UpdatedNumberScheduled: 1,
			NumberReady:            3,
		},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(ds)}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := c.waitForDaemonSet(ctx, objectOfKind("DaemonSet", "agent")); nil == err {
		t.Fatal("expected a timeout while only one of three DaemonSet pods carries the new revision")
	}
}

func TestWaitForPVCRequiresBound(t *testing.T) {
	pending := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "default"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(pending)}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := c.waitForPVC(ctx, objectOfKind("PersistentVolumeClaim", "data")); nil == err {
		t.Fatal("expected a timeout while the PersistentVolumeClaim is still Pending")
	}
}

func TestWaitForPodRequiresReadyCondition(t *testing.T) {
	running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		},
	}

	c := &Client{namespace: "default", timeout: time.Second, clientset: fake.NewSimpleClientset(running)}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := c.waitForPod(ctx, objectOfKind("Pod", "app")); nil == err {
		t.Fatal("expected a timeout while the Pod is Running but not Ready")
	}
}
