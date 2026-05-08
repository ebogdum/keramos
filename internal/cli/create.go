package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Scaffold a new keramos package",
		Long:  "Create a new keramos package with default templates, values, and metadata.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, args[0])
		},
	}
}

func runCreate(cmd *cobra.Command, name string) error {
	logger.Debug("creating package %s", name)

	if err := action.Create(name, "."); nil != err {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "created package %s/\n", name)
	return nil
}
