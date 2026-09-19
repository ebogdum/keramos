package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/action"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/spf13/cobra"
)

func newLintCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		profile    string
		strict     bool
	)

	cmd := &cobra.Command{
		Use:   "lint <package-path>",
		Short: "Validate a keramos package for correctness",
		Long:  "Validate a keramos package by checking metadata, values, schema, and rendering templates.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd, args[0], valueFiles, sets, profile, strict)
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as errors")

	return cmd
}

func runLint(cmd *cobra.Command, packagePath string, valueFiles []string, sets []string, profile string, strict bool) error {
	logger.Debug("linting package at %s", packagePath)

	result, err := action.Lint(packagePath, valueFiles, sets, profile, strict)
	if nil != err {
		return err
	}

	w := cmd.OutOrStdout()
	for _, msg := range result.Errors {
		prefix := "[ERROR]"
		if "" != msg.File {
			fmt.Fprintf(w, "%s %s: %s\n", prefix, msg.File, msg.Message)
		} else {
			fmt.Fprintf(w, "%s %s\n", prefix, msg.Message)
		}
	}
	for _, msg := range result.Warnings {
		prefix := "[WARNING]"
		if "" != msg.File {
			fmt.Fprintf(w, "%s %s: %s\n", prefix, msg.File, msg.Message)
		} else {
			fmt.Fprintf(w, "%s %s\n", prefix, msg.Message)
		}
	}

	hasErrors := !result.IsValid()
	hasWarnings := 0 < len(result.Warnings)

	if hasErrors || (strict && hasWarnings) {
		errorCount := len(result.Errors)
		warningCount := len(result.Warnings)
		return fmt.Errorf("lint failed: %d error(s), %d warning(s)", errorCount, warningCount)
	}

	fmt.Fprintln(w, "lint passed")
	return nil
}
