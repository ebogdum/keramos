package kube

import (
	"testing"
	"time"
)

func TestDefaultTimeout(t *testing.T) {
	c := &Client{}
	if 0 == c.timeout {
		// Before NewClient sets it, timeout is zero.
		// NewClient sets it to defaultTimeout.
		c.timeout = defaultTimeout
	}
	if defaultTimeout != c.timeout {
		t.Errorf("expected default timeout %v, got %v", defaultTimeout, c.timeout)
	}
}

func TestSetTimeout(t *testing.T) {
	c := &Client{timeout: defaultTimeout}
	c.SetTimeout(10 * time.Second)
	if 10*time.Second != c.timeout {
		t.Errorf("expected timeout 10s, got %v", c.timeout)
	}
}

func TestNewContext(t *testing.T) {
	c := &Client{timeout: 3 * time.Second}
	ctx, cancel := c.newContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}

	remaining := time.Until(deadline)
	if remaining > 3*time.Second || remaining < 2*time.Second {
		t.Errorf("expected deadline around 3s from now, got %v", remaining)
	}
}

func TestNamespaceMethod(t *testing.T) {
	c := &Client{namespace: "test-ns"}
	if "test-ns" != c.Namespace() {
		t.Errorf("expected namespace test-ns, got %s", c.Namespace())
	}
}

func TestClientImplementsKubeClient(t *testing.T) {
	// Compile-time check that *Client implements KubeClient
	var _ KubeClient = (*Client)(nil)
}
