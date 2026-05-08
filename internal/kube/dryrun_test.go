package kube

import (
	"testing"
)

func TestClientImplementsDryRunApply(t *testing.T) {
	// Compile-time check that *Client has the DryRunApply method
	var c KubeClient = (*Client)(nil)
	_ = c
}

func TestDryRunApplyEmptyManifest(t *testing.T) {
	c := &Client{namespace: "default"}
	// Empty manifest should parse fine (no resources to apply)
	err := c.DryRunApply("")
	if nil != err {
		t.Fatalf("expected nil for empty manifest, got %v", err)
	}
}

func TestDryRunApplyInvalidYAML(t *testing.T) {
	c := &Client{namespace: "default"}
	err := c.DryRunApply("not: valid: yaml: [")
	// Should fail at parse stage
	if nil == err {
		t.Fatal("expected error for invalid YAML manifest")
	}
}
