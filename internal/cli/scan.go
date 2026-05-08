package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var outputDir string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "scan <directory>",
		Short: "Scan packages for common values and extract a base layer",
		Long: `Scan analyzes a directory of keramos packages, finds common values
and templates across packages, extracts them into a base layer,
and rewrites each package to reference the base.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			result, err := action.Scan(dir, outputDir, dryRun)
			if nil != err {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), result.Report)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory for generated base (default: same as input)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be changed without writing files")

	return cmd
}
