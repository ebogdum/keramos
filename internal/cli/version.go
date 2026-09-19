package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func buildMetadata() (string, string, string) {
	version, commit, buildDate := Version, Commit, BuildDate

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, buildDate
	}

	if "dev" == version && "" != info.Main.Version && "(devel)" != info.Main.Version {
		version = info.Main.Version
	}

	for _, setting := range info.Settings {
		if "vcs.revision" == setting.Key && "unknown" == commit {
			commit = setting.Value
		}
		if "vcs.time" == setting.Key && "unknown" == buildDate {
			buildDate = setting.Value
		}
	}

	return version, commit, buildDate
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the keramos version",
		RunE: func(cmd *cobra.Command, args []string) error {
			version, commit, buildDate := buildMetadata()
			if "unknown" == commit && "unknown" == buildDate {
				fmt.Fprintf(cmd.OutOrStdout(), "keramos version %s\n", version)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "keramos version %s (commit %s, built %s)\n", version, commit, buildDate)
			return nil
		},
	}
}
