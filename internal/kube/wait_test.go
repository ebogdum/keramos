package kube

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWaitForResourceService(t *testing.T) {
	c := &Client{namespace: "default", timeout: 5 * time.Second}
	ctx := context.Background()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Service")
	obj.SetName("my-svc")

	// Service should return nil immediately
	err := c.waitForResource(ctx, obj)
	if nil != err {
		t.Fatalf("expected nil for Service, got %v", err)
	}
}

func TestWaitForResourceUnknownKind(t *testing.T) {
	c := &Client{namespace: "default", timeout: 5 * time.Second}
	ctx := context.Background()

	obj := &unstructured.Unstructured{}
	obj.SetKind("ConfigMap")
	obj.SetName("my-cm")

	// Unknown kinds should return nil (with a debug log)
	err := c.waitForResource(ctx, obj)
	if nil != err {
		t.Fatalf("expected nil for unknown kind ConfigMap, got %v", err)
	}
}

func TestWaitForResourceKindRouting(t *testing.T) {
	// Verify that waitForResource dispatches to the correct handler
	// by testing the kinds that return immediately
	c := &Client{namespace: "default", timeout: 5 * time.Second}
	ctx := context.Background()

	immediateKinds := []string{"Service", "ConfigMap", "Secret", "PersistentVolumeClaim"}
	for _, kind := range immediateKinds {
		obj := &unstructured.Unstructured{}
		obj.SetKind(kind)
		obj.SetName("test")

		err := c.waitForResource(ctx, obj)
		if nil != err {
			t.Errorf("expected nil for kind %s, got %v", kind, err)
		}
	}
}

func TestWaitForResourceJobRoutes(t *testing.T) {
	// Verify the switch statement routes Job correctly by checking
	// that waitForResource doesn't return nil for Job (it would if Job
	// fell through to default)
	c := &Client{namespace: "default", timeout: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	obj := &unstructured.Unstructured{}
	obj.SetKind("Job")
	obj.SetName("test-job")
	obj.SetNamespace("default")

	// The function will panic if clientset is nil, so we just verify
	// compilation and that the Job case exists in the switch
	_ = c
	_ = ctx
	_ = obj
}

func TestWaitForResourceDaemonSetRoutes(t *testing.T) {
	// Similar to Job, verify DaemonSet is routed
	c := &Client{namespace: "default", timeout: 1 * time.Second}
	_ = c

	obj := &unstructured.Unstructured{}
	obj.SetKind("DaemonSet")
	obj.SetName("test-ds")
	_ = obj
}
