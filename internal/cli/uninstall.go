package cli

import (
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/spf13/cobra"
)

func newUninstallCommand() *cobra.Command {
	var (
		purge          bool
		noHooks        bool
		timeout        time.Duration
		output         string
		description    string
		ignoreNotFound bool
		keepHistory    bool
		noWaitU        bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall <release-name>",
		Short: "Uninstall a release",
		Long:  "Remove a release and its associated resources from the cluster. Release history is kept by default for auditing.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			releaseName := args[0]

			if err := validateOutputFlag(output); nil != err {
				return err
			}

			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}

			opts := &action.UninstallOptions{
				ReleaseName:    releaseName,
				Namespace:      namespace,
				KeepHistory:    keepHistory || !purge,
				Timeout:        timeout,
				NoHooks:        noHooks,
				Description:    description,
				IgnoreNotFound: ignoreNotFound,
				Wait:           !noWaitU,
			}

			rel, uninstallErr := action.Uninstall(client, opts)
			if nil != uninstallErr {
				return uninstallErr
			}

			return outputRelease(cmd.OutOrStdout(), rel, output, func() {
				logger.Log("release %s uninstalled", releaseName)
				if !purge {
					logger.Log("release history kept (use --purge to remove)")
				}
			})
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "delete release history (default: history is kept)")
	cmd.Flags().BoolVar(&keepHistory, "keep-history", false, "keep release history (default behaviour; explicit positive form)")
	cmd.Flags().BoolVar(&noHooks, "no-hooks", false, "skip lifecycle hooks for this operation")
	var explicitWait bool
	cmd.Flags().BoolVar(&explicitWait, "wait", true, "wait for resource deletion to complete (default)")
	_ = explicitWait // positive form; waiting is the default, toggled off by --no-wait
	cmd.Flags().BoolVar(&noWaitU, "no-wait", false, "do not wait for resource deletion")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for resource deletion")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
	cmd.Flags().StringVar(&description, "description", "", "description recorded against the uninstall revision")
	cmd.Flags().BoolVar(&ignoreNotFound, "ignore-not-found", false, "exit zero when the release is not found")

	return cmd
}
