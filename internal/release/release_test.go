package release

import (
	"testing"
	"time"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := &Release{
		Name:      "my-app",
		Namespace: "production",
		Revision:  3,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:       "my-package",
			Version:    "1.2.3",
			AppVersion: "4.5.6",
		},
		Values: map[string]any{
			"replicas": 3,
			"image": map[string]any{
				"tag": "latest",
			},
		},
		Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
		Notes:    "App deployed successfully",
		Hooks: []HookResult{
			{Name: "pre-install", Kind: "Job", Status: "succeeded"},
		},
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
			Description:   "initial install",
		},
	}

	encoded, err := Encode(original)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	if "" == encoded {
		t.Fatal("encoded string is empty")
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if original.Name != decoded.Name {
		t.Errorf("Name: expected %s, got %s", original.Name, decoded.Name)
	}
	if original.Namespace != decoded.Namespace {
		t.Errorf("Namespace: expected %s, got %s", original.Namespace, decoded.Namespace)
	}
	if original.Revision != decoded.Revision {
		t.Errorf("Revision: expected %d, got %d", original.Revision, decoded.Revision)
	}
	if original.Status != decoded.Status {
		t.Errorf("Status: expected %s, got %s", original.Status, decoded.Status)
	}
	if original.Package.Name != decoded.Package.Name {
		t.Errorf("Package.Name: expected %s, got %s", original.Package.Name, decoded.Package.Name)
	}
	if original.Package.Version != decoded.Package.Version {
		t.Errorf("Package.Version: expected %s, got %s", original.Package.Version, decoded.Package.Version)
	}
	if original.Manifest != decoded.Manifest {
		t.Errorf("Manifest mismatch")
	}
	if original.Notes != decoded.Notes {
		t.Errorf("Notes: expected %s, got %s", original.Notes, decoded.Notes)
	}
	if 1 != len(decoded.Hooks) {
		t.Fatalf("expected 1 hook result, got %d", len(decoded.Hooks))
	}
	if "succeeded" != decoded.Hooks[0].Status {
		t.Errorf("Hook status: expected succeeded, got %s", decoded.Hooks[0].Status)
	}
	if !original.Info.FirstDeployed.Equal(decoded.Info.FirstDeployed) {
		t.Errorf("FirstDeployed mismatch")
	}
}

func TestDecodeInvalidBase64(t *testing.T) {
	_, err := Decode("not-valid-base64!!!")
	if nil == err {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeInvalidGzip(t *testing.T) {
	// Valid base64 but not gzip data
	_, err := Decode("aGVsbG8gd29ybGQ=")
	if nil == err {
		t.Fatal("expected error for invalid gzip data")
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []Status{
		StatusDeployed,
		StatusSuperseded,
		StatusFailed,
		StatusUninstalling,
		StatusPendingInstall,
		StatusPendingUpgrade,
		StatusPendingRollback,
	}

	seen := make(map[Status]bool, len(statuses))
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
		if "" == string(s) {
			t.Error("status should not be empty")
		}
	}
}
