package action

import (
	"strings"
	"testing"
)

// TestInstallRecordsProvenance proves a dry-run install records where each
// value came from onto the release, so the state carries the origin chain.
func TestInstallRecordsProvenance(t *testing.T) {
	dir := t.TempDir()
	createPackageWithTemplate(t, dir)

	rel, err := Install(nil, dir, &InstallOptions{
		ReleaseName: "prov-test",
		DryRun:      "client",
		Sets:        []string{"replicas=9"},
	})
	if nil != err {
		t.Fatalf("install: %v", err)
	}
	if nil == rel.Provenance {
		t.Fatal("release recorded no provenance")
	}
	// The package default 'name' should be attributed to values.yaml.
	if got := rel.Provenance["name"]; !strings.Contains(got, "package-default") {
		t.Fatalf("name provenance = %q, want package-default", got)
	}
	// The --set override must be attributed to the set expression.
	if got := rel.Provenance["replicas"]; !strings.Contains(got, "set") || !strings.Contains(got, "replicas=9") {
		t.Fatalf("replicas provenance = %q, want set (replicas=9)", got)
	}
}
