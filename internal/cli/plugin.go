package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/plugin"
	"github.com/spf13/cobra"
)

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage keramos plugins",
		Long:  "Install, list, and remove keramos plugins that extend keramos with custom commands.",
	}

	cmd.AddCommand(newPluginInstallCommand())
	cmd.AddCommand(newPluginListCommand())
	cmd.AddCommand(newPluginRemoveCommand())
	cmd.AddCommand(newPluginUpdateCommand())

	return cmd
}

func newPluginUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "update <name>",
		Short:   "Update an installed plugin",
		Aliases: []string{"up", "upgrade"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := plugin.Update(args[0])
			if nil != err {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated plugin: %s v%s\n", p.Name, p.Version)
			return nil
		},
	}
}

func newPluginInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install <source>",
		Short: "Install a plugin from a git URL or local path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := plugin.Install(args[0])
			if nil != err {
				return err
			}
			logger.Log("installed plugin %s (%s)", p.Name, p.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Installed plugin: %s v%s\n", p.Name, p.Version)
			return nil
		},
	}
}

func newPluginListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List installed plugins",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := plugin.List()
			if nil != err {
				return err
			}

			if 0 == len(plugins) {
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins installed.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
			for _, p := range plugins {
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Version, p.Description)
			}
			return w.Flush()
		},
	}
}

func newPluginRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Short:   "Remove an installed plugin",
		Aliases: []string{"rm", "uninstall"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := plugin.Remove(args[0]); nil != err {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed plugin: %s\n", args[0])
			return nil
		},
	}
}

// runPluginIfExists checks if an unknown command matches an installed plugin and runs it.
func runPluginIfExists(name string, args []string) error {
	p, err := plugin.FindPlugin(name)
	if nil != err {
		return err
	}
	return plugin.Run(p, args)
}
