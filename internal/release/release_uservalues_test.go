package release

import (
	"testing"
	"time"
)

func TestEncodeDecodeWithUserValues(t *testing.T) {
	original := &Release{
		Name:      "user-vals-test",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "test-pkg",
			Version: "1.0.0",
		},
		Values: map[string]any{
			"replicas": 3,
			"image": map[string]any{
				"repository": "nginx",
				"tag":        "latest",
			},
		},
		UserValues: map[string]any{
			"replicas": 3,
		},
		Manifest: "apiVersion: v1\nkind: ConfigMap\n",
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	encoded, err := Encode(original)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if nil == decoded.UserValues {
		t.Fatal("expected UserValues to be non-nil after decode")
	}

	replicas, ok := decoded.UserValues["replicas"]
	if !ok {
		t.Fatal("expected 'replicas' in UserValues")
	}

	// JSON numbers decode as float64
	if float64(3) != replicas {
		t.Errorf("expected UserValues replicas=3, got %v (%T)", replicas, replicas)
	}
}

func TestEncodeDecodeWithNilUserValues(t *testing.T) {
	original := &Release{
		Name:      "no-user-vals",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "test-pkg",
			Version: "1.0.0",
		},
		Values:   map[string]any{"replicas": 1},
		Manifest: "apiVersion: v1\n",
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	encoded, err := Encode(original)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if nil != decoded.UserValues {
		t.Errorf("expected UserValues to be nil, got %v", decoded.UserValues)
	}
}
