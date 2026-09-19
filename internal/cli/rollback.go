package cli

import (
	"strconv"
	"time"

	"github.com/ebogdum/keramos/v3/internal/action"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/spf13/cobra"
)

func newRollbackCommand() *cobra.Command {
	var (
		noWait        bool
		explicitWait  bool
		timeout       time.Duration
		description   string
		noHooks       bool
		historyMax    int
		force         bool
		cleanupOnFail bool
		recreatePods  bool
		output        string
	)

	cmd := &cobra.Command{
		Use:   "rollback <release-name> [revision]",
		Short: "Roll back a release to a previous revision",
		Long:  "Roll back a release to a previous revision. If no revision is specified, rolls back to the previous revision.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			releaseName := args[0]
			revision := 0

			if err := validateOutputFlag(output); nil != err {
				return err
			}

			if 2 == len(args) {
				parsed, err := strconv.Atoi(args[1])
				if nil != err {
					return err
				}
				revision = parsed
			}

			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}

			opts := &action.RollbackOptions{
				ReleaseName:   releaseName,
				Namespace:     namespace,
				Revision:      revision,
				Wait:          explicitWait || !noWait,
				Timeout:       timeout,
				Description:   description,
				NoHooks:       noHooks,
				HistoryMax:    historyMax,
				Force:         force,
				CleanupOnFail: cleanupOnFail,
				RecreatePods:  recreatePods,
			}

			rel, err := action.Rollback(client, opts)
			if nil != err {
				return err
			}

			return outputRelease(cmd.OutOrStdout(), rel, output, func() {
				logger.Log("release %s rolled back to revision %d (new revision %d)", rel.Name, revision, rel.Revision)
			})
		},
	}

	cmd.Flags().BoolVar(&noWait, "no-wait", false, "don't wait for resources to be ready")
	cmd.Flags().BoolVar(&explicitWait, "wait", false, "wait for resources to be ready (default)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for readiness wait")
	cmd.Flags().StringVar(&description, "description", "", "rollback description")
	cmd.Flags().BoolVar(&noHooks, "no-hooks", false, "skip lifecycle hooks for this operation")
	cmd.Flags().IntVar(&historyMax, "history-max", 10, "maximum revisions to retain in history (0 = unlimited)")
	cmd.Flags().BoolVar(&force, "force", false, "delete and recreate resources to force update of immutable fields")
	cmd.Flags().BoolVar(&cleanupOnFail, "cleanup-on-fail", false, "delete partially-applied resources if rollback fails")
	cmd.Flags().BoolVar(&recreatePods, "recreate-pods", false, "trigger a rolling restart of Deployments/StatefulSets/DaemonSets")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")

	return cmd
}
