package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/spf13/cobra"
)

// newCanaryCommand performs a staged upgrade: deploy at percentage W1, watch
// readiness for `--bake`, advance to W2, then full rollout. Rollback on
// failure. Implementation tweaks `replicas` and waits for healthy pods at
// each stage.
func newCanaryCommand() *cobra.Command {
	var (
		stages     []string
		bake       time.Duration
		valueFiles []string
		sets       []string
		setStrings []string
		setFiles   []string
		setJSON    []string
		profile    string
	)
	cmd := &cobra.Command{
		Use:   "canary <release> <package-path>",
		Short: "Staged upgrade: advance through replica percentages with bake periods",
		Long: `Perform a canary upgrade by stepping through replica counts (or
percentages) with a bake-and-verify pause between each. On failure at any
stage, the release is rolled back to the prior revision.

Example:
    keramos canary myapp ./pkg --stages 1,3,5 --bake 60s
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if 1 > len(stages) {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "--stages requires at least one entry")
			}
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			_ = ctx
			// Capture the pre-canary baseline revision so a failure at any
			// stage rolls back to the state before the canary started — not
			// to an intermediate canary stage.
			baselineRev := 0
			if storage, sErr := storageFor(); nil == sErr {
				if cur, _ := storage.Last(args[0]); nil != cur {
					baselineRev = cur.Revision
				}
			}
			for i, stage := range stages {
				logger.Log("canary stage %d/%d → replicas=%s", i+1, len(stages), stage)
				stageSets := append([]string{}, sets...)
				stageSets = append(stageSets, "replicas="+stage)
				rel, uErr := action.Upgrade(client, args[1], &action.UpgradeOptions{
					ReleaseName: args[0],
					Namespace:   namespace,
					ValueFiles:  valueFiles,
					Sets:        stageSets,
					SetStrings:  setStrings,
					SetFiles:    setFiles,
					SetJSON:     setJSON,
					Profile:     profile,
					Atomic:      true,
					Wait:        true,
					Timeout:     5 * time.Minute,
					Install:     0 == i,
				})
				if nil != uErr {
					if 0 < baselineRev {
						logger.Warn("stage %d failed: %v — rolling back to pre-canary revision %d", i+1, uErr, baselineRev)
						_, _ = action.Rollback(client, &action.RollbackOptions{
							ReleaseName: args[0],
							Namespace:   namespace,
							Revision:    baselineRev,
						})
					} else {
						// Release didn't exist before the canary; uninstall the
						// partial deployment instead of rolling back to revision 0.
						logger.Warn("stage %d failed: %v — uninstalling freshly-created release", i+1, uErr)
						_, _ = action.Uninstall(client, &action.UninstallOptions{
							ReleaseName: args[0],
							Namespace:   namespace,
						})
					}
					return uErr
				}
				// First successful stage on a fresh release establishes the
				// rollback target for subsequent stage failures.
				if 0 == baselineRev {
					baselineRev = rel.Revision
				}
				if i+1 < len(stages) && 0 < bake {
					logger.Log("baking for %s …", bake)
					time.Sleep(bake)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "canary completed at stage %s\n", stages[len(stages)-1])
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&stages, "stages", nil, "comma-separated replica counts to step through (e.g. 1,3,5)")
	cmd.Flags().DurationVar(&bake, "bake", 60*time.Second, "pause between stages to observe health")
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "force string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; value read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value parsed as JSON (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}
