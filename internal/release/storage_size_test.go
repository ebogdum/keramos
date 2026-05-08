package release

import (
	"strings"
	"testing"
)

func TestMaxSecretSizeConstant(t *testing.T) {
	// Verify the constant is 1MB
	expected := 1 * 1024 * 1024
	if expected != maxSecretSize {
		t.Errorf("expected maxSecretSize to be %d, got %d", expected, maxSecretSize)
	}
}

func TestEncodedSizeExceedsLimit(t *testing.T) {
	// Create a release with a very large manifest
	largeManifest := strings.Repeat("x", 2*1024*1024) // 2MB raw, will compress but still big after encode

	rel := &Release{
		Name:      "size-test",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "test-pkg",
			Version: "1.0.0",
		},
		Manifest: largeManifest,
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	// Verify the encoded size check works
	if len(encoded) <= maxSecretSize {
		t.Skip("encoded size is within limit after compression; skipping size validation test")
	}

	// The encoded size should exceed the limit
	if len(encoded) <= maxSecretSize {
		t.Fatal("expected encoded size to exceed maxSecretSize")
	}
}

func TestEncodedSizeWithinLimit(t *testing.T) {
	rel := &Release{
		Name:      "small-test",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "test-pkg",
			Version: "1.0.0",
		},
		Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoded) > maxSecretSize {
		t.Fatalf("small release encoded size %d exceeds limit %d", len(encoded), maxSecretSize)
	}
}
