package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runVersionCommand(t *testing.T) string {
	t.Helper()

	cmd := newVersionCommand()
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	if err := cmd.Execute(); nil != err {
		t.Fatalf("version command failed: %v", err)
	}

	return out.String()
}

func TestVersionCommandReportsLdflagValues(t *testing.T) {
	Version, Commit, BuildDate = "v9.9.9", "abc123", "2026-01-01T00:00:00Z"
	t.Cleanup(func() { Version, Commit, BuildDate = "dev", "unknown", "unknown" })

	got := runVersionCommand(t)

	want := "keramos version v9.9.9 (commit abc123, built 2026-01-01T00:00:00Z)"
	if !strings.Contains(got, want) {
		t.Fatalf("got %q, want it to contain %q", got, want)
	}
}

func TestVersionCommandOmitsUnknownStamps(t *testing.T) {
	Version, Commit, BuildDate = "v9.9.9", "unknown", "unknown"
	t.Cleanup(func() { Version, Commit, BuildDate = "dev", "unknown", "unknown" })

	got := strings.TrimSpace(runVersionCommand(t))

	if "keramos version v9.9.9" != got {
		t.Fatalf("got %q, want %q", got, "keramos version v9.9.9")
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
