package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandReportsLdflagValues(t *testing.T) {
	Version, Commit, BuildDate = "v9.9.9", "abc123", "2026-01-01T00:00:00Z"
	t.Cleanup(func() { Version, Commit, BuildDate = "dev", "unknown", "unknown" })

	cmd := newVersionCommand()
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	if err := cmd.Execute(); nil != err {
		t.Fatalf("version command failed: %v", err)
	}

	want := "keramos version v9.9.9 (commit abc123, built 2026-01-01T00:00:00Z)"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("got %q, want it to contain %q", out.String(), want)
	}
}

func TestBuildMetadataFallsBackToBuildInfo(t *testing.T) {
	Version, Commit, BuildDate = "dev", "unknown", "unknown"

	version, commit, buildDate := buildMetadata()

	if "dev" == version && "unknown" == commit && "unknown" == buildDate {
		t.Skip("test binary carries no module version or vcs stamps")
	}

	if "" == version || "" == commit || "" == buildDate {
		t.Fatalf("empty metadata: version=%q commit=%q buildDate=%q", version, commit, buildDate)
	}
}
