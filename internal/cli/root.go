package cli

import (
	"os"
	"strings"

	"github.com/ebogdum/keramos/internal/logger"
	"github.com/spf13/cobra"
)

var (
	namespace   string
	kubeconfig  string
	kubeContext string
	debugFlag   bool
)

// NewRootCommand creates and returns the top-level keramos CLI command.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keramos",
		Short: "A Kubernetes package manager",
		Long:  "Keramos is a Kubernetes package manager built around expression-based templating, layered composition, and dependency-aware orchestration.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logger.Init(false, debugFlag)
			logger.Debug("debug mode enabled")
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file")
	cmd.PersistentFlags().StringVar(&kubeContext, "kube-context", "", "Kubernetes context to use")
	cmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug output")

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newTemplateCommand())
	cmd.AddCommand(newLintCommand())
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newInstallCommand())
	cmd.AddCommand(newUpgradeCommand())
	cmd.AddCommand(newRollbackCommand())
	cmd.AddCommand(newUninstallCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newHistoryCommand())
	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newDiffCommand())
	cmd.AddCommand(newDebugCommand())
	cmd.AddCommand(newTestCommand())
	cmd.AddCommand(newPackageCommand())
	cmd.AddCommand(newRepoCommand())
	cmd.AddCommand(newLoginCommand())
	cmd.AddCommand(newLogoutCommand())
	cmd.AddCommand(newRegistryCommand())
	cmd.AddCommand(newDependencyCommand())
	cmd.AddCommand(newSearchCommand())
	cmd.AddCommand(newPluginCommand())
	cmd.AddCommand(newPublishCommand())
	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newCompletionCommand())
	cmd.AddCommand(newEnvCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newKeyringCommand())
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newPolicyCommand())
	cmd.AddCommand(newWorkspaceCommand())
	cmd.AddCommand(newDriftCommand())
	cmd.AddCommand(newReconcileCommand())
	cmd.AddCommand(newValuesCommand())
	cmd.AddCommand(newAuditCommand())
	cmd.AddCommand(newSBOMCommand())
	cmd.AddCommand(newGraphCommand())
	cmd.AddCommand(newAdoptCommand())
	cmd.AddCommand(newMultiInstallCommand())
	cmd.AddCommand(newPlanCommand())
	cmd.AddCommand(newApplyCommand())
	cmd.AddCommand(newRenameCommand())
	cmd.AddCommand(newPruneCommand())
	cmd.AddCommand(newCanaryCommand())
	cmd.AddCommand(newReleasesCommand())
	cmd.AddCommand(newDevCommand())
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newMarketplaceCommand())
	cmd.AddCommand(newHelmCompatCommand())
	cmd.AddCommand(newControllerCommand())
	cmd.AddCommand(newMetricsCommand())
	cmd.AddCommand(newPurgeCommand())

	return cmd
}

// Execute runs the root command. This is the main entry point.
// If the command is unknown, it attempts to run it as a plugin.
func Execute() error {
	root := NewRootCommand()
	err := root.Execute()
	if nil == err {
		return nil
	}

	// Only fall back to plugin lookup when cobra reports an unknown command.
	// Other errors (flag parsing, missing args, command run errors) must surface
	// to the user rather than be silently masked by an installed plugin.
	if !strings.HasPrefix(err.Error(), "unknown command") {
		return err
	}

	args := os.Args[1:]
	if 0 == len(args) {
		return err
	}
	if strings.HasPrefix(args[0], "-") {
		return err
	}

	pluginErr := runPluginIfExists(args[0], args[1:])
	if nil != pluginErr {
		return err // return original cobra error, not plugin lookup error
	}
	return nil
}
