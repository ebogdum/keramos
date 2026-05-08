package release

import (
	"testing"
	"time"
)

func TestSecretName(t *testing.T) {
	tests := []struct {
		name     string
		release  string
		revision int
		expected string
	}{
		{"basic", "my-app", 1, "keramos.v1.my-app.v1"},
		{"multi-revision", "nginx", 5, "keramos.v1.nginx.v5"},
		{"with-dashes", "my-web-app", 10, "keramos.v1.my-web-app.v10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecretName(tt.release, tt.revision)
			if tt.expected != result {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSecretLabels(t *testing.T) {
	rel := &Release{
		Name:     "my-app",
		Revision: 3,
		Status:   StatusDeployed,
	}

	labels := SecretLabels(rel)

	if "keramos" != labels[labelOwner] {
		t.Errorf("expected owner=keramos, got %s", labels[labelOwner])
	}
	if "my-app" != labels[labelName] {
		t.Errorf("expected name=my-app, got %s", labels[labelName])
	}
	if "3" != labels[labelVersion] {
		t.Errorf("expected version=3, got %s", labels[labelVersion])
	}
	if "deployed" != labels[labelStatus] {
		t.Errorf("expected status=deployed, got %s", labels[labelStatus])
	}
}

func TestSecretLabelsAllStatuses(t *testing.T) {
	statuses := []Status{
		StatusDeployed, StatusSuperseded, StatusFailed,
		StatusUninstalling, StatusPendingInstall,
		StatusPendingUpgrade, StatusPendingRollback,
	}

	for _, s := range statuses {
		rel := &Release{Name: "test", Revision: 1, Status: s}
		labels := SecretLabels(rel)
		if string(s) != labels[labelStatus] {
			t.Errorf("status %s: expected label %s, got %s", s, s, labels[labelStatus])
		}
	}
}

func TestEncodeDecodeForStorage(t *testing.T) {
	rel := &Release{
		Name:      "storage-test",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "test-pkg",
			Version: "1.0.0",
		},
		Values: map[string]any{
			"key": "value",
		},
		Manifest: "apiVersion: v1\nkind: Service\nmetadata:\n  name: test\n",
		Info: ReleaseInfo{
			FirstDeployed: time.Now().UTC(),
			LastDeployed:  time.Now().UTC(),
		},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if rel.Name != decoded.Name {
		t.Errorf("Name mismatch: %s vs %s", rel.Name, decoded.Name)
	}
	if rel.Manifest != decoded.Manifest {
		t.Errorf("Manifest mismatch")
	}
}
