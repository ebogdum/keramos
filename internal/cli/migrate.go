package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v2/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	var (
		outputDir string
		dryRun    bool
		strict    bool
	)

	cmd := &cobra.Command{
		Use:   "migrate <helm-chart-path>",
		Short: "Convert a Helm chart to a keramos package",
		Long:  "Migrate a Helm chart directory to keramos format, converting templates and metadata.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartPath := args[0]

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run mode: showing conversion plan")
			}

			result, err := migrate.Migrate(chartPath, outputDir, strict, dryRun)
			if nil != err && nil == result {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Output: %s\n", result.PackagePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Converted %d files:\n", len(result.ConvertedFiles))
			for _, f := range result.ConvertedFiles {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", f)
			}

			if len(result.ManualReview) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nItems requiring manual review (%d):\n", len(result.ManualReview))
				for _, item := range result.ManualReview {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s:%d — %s\n", item.File, item.Line, item.Reason)
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", item.Original)
				}
			}

			if len(result.Warnings) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nWarnings:\n")
				for _, w := range result.Warnings {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", w)
				}
			}

			if nil != err {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\nMigration complete.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory (default: <chart-name>-keramos/)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be converted without writing")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail on any template that cannot be fully auto-converted")

	return cmd
}
