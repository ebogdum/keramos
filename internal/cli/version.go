package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version, Commit, and BuildDate are set at build time via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the keramos version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "keramos version %s (commit %s, built %s)\n", Version, Commit, BuildDate)
			return nil
		},
	}
}
