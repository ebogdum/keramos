package cli

import (
	"fmt"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/spf13/cobra"
)

func newUpgradeCommand() *cobra.Command {
	var (
		valueFiles           []string
		sets                 []string
		setStrings           []string
		setFiles             []string
		setJSON              []string
		profile              string
		noWait               bool
		timeout              time.Duration
		dryRun               string
		reuseValues          bool
		resetValues          bool
		resetThenReuseValues bool
		install              bool
		description          string
		noAtomic             bool
		noForce              bool
		noHooks              bool
		createNamespace      bool
		includeCRDs          bool
		labels               []string
		apiVersions          []string
		kubeVersion          string
		postRenderer         string
		postRenderers        []string
		postRendererTimeout  time.Duration
		historyMax           int
		force                bool
		cleanupOnFail        bool
		recreatePods         bool
		waitForJobs          bool
		hookTimeout          time.Duration
		envName              string
		output               string
		only                 []string
	)

	cmd := &cobra.Command{
		Use:   "upgrade <release-name> <package-path>",
		Short: "Upgrade an existing release",
		Long:  "Upgrade an existing release to a new version of a keramos package.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			releaseName := args[0]
			packagePath := args[1]

			if err := validateDryRunFlag(dryRun); nil != err {
				return err
			}
			if err := validateOutputFlag(output); nil != err {
				return err
			}

			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			if noForce {
				client.SetForce(false)
			}

			labelMap, labelErr := parseLabelFlags(labels)
			if nil != labelErr {
				return labelErr
			}

			opts := &action.UpgradeOptions{
				ReleaseName:          releaseName,
				Namespace:            namespace,
				ValueFiles:           valueFiles,
				Sets:                 sets,
				SetStrings:           setStrings,
				SetFiles:             setFiles,
				SetJSON:              setJSON,
				Profile:              profile,
				Wait:                 !noWait,
				Timeout:              timeout,
				DryRun:               dryRun,
				ReuseValues:          reuseValues,
				ResetValues:          resetValues,
				ResetThenReuseValues: resetThenReuseValues,
				Install:              install,
				Description:          description,
				Atomic:               !noAtomic,
				NoHooks:              noHooks,
				CreateNamespace:      createNamespace,
				IncludeCRDs:          includeCRDs,
				Labels:               labelMap,
				APIVersions:          apiVersions,
				KubeVersion:          kubeVersion,
				PostRenderer:         postRenderer,
				PostRenderers:        postRenderers,
				PostRendererTimeout:  postRendererTimeout,
				HistoryMax:           historyMax,
				Force:                force,
				CleanupOnFail:        cleanupOnFail,
				RecreatePods:         recreatePods,
				WaitForJobs:          waitForJobs,
				HookTimeout:          hookTimeout,
				Environment:          envName,
				Only:                 only,
			}

			rel, err := action.Upgrade(client, packagePath, opts)
			if nil != err {
				return err
			}

			if "" != dryRun {
				return outputRelease(cmd.OutOrStdout(), rel, output, func() {
					fmt.Fprint(cmd.OutOrStdout(), rel.Manifest)
				})
			}

			return outputRelease(cmd.OutOrStdout(), rel, output, func() {
				logger.Log("release %s upgraded (revision %d)", rel.Name, rel.Revision)
				if "" != rel.Notes {
					fmt.Fprintf(cmd.OutOrStdout(), "\nNOTES:\n%s\n", rel.Notes)
				}
			})
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set key=value overrides forcing string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; the value is read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value is parsed as a JSON literal (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "don't wait for resources to be ready")
	var explicitWait bool
	cmd.Flags().BoolVar(&explicitWait, "wait", false, "wait for resources to be ready (default behaviour)")
	_ = explicitWait
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for readiness wait")
	cmd.Flags().StringVar(&dryRun, "dry-run", "", "dry-run mode: 'client' (local render) or 'server' (API validation)")
	cmd.Flags().BoolVar(&reuseValues, "reuse-values", false, "reuse values from previous release")
	cmd.Flags().BoolVar(&resetValues, "reset-values", false, "reset values to package defaults")
	cmd.Flags().BoolVar(&resetThenReuseValues, "reset-then-reuse-values", false, "reset to defaults then merge previous values")
	cmd.Flags().BoolVar(&install, "install", false, "install if release does not exist")
	cmd.Flags().StringVar(&description, "description", "", "release description")
	cmd.Flags().BoolVar(&noAtomic, "no-atomic", false, "don't roll back on failure")
	cmd.Flags().BoolVar(&noForce, "no-force", false, "don't force field ownership on server-side apply")
	cmd.Flags().BoolVar(&noHooks, "no-hooks", false, "skip lifecycle hooks for this operation")
	cmd.Flags().BoolVar(&createNamespace, "create-namespace", false, "create the release namespace if missing (with --install)")
	cmd.Flags().BoolVar(&includeCRDs, "include-crds", false, "include CRDs from crds/ in the rendered manifest")
	cmd.Flags().StringArrayVar(&labels, "labels", nil, "label key=value to attach to the release (repeatable)")
	cmd.Flags().StringArrayVar(&apiVersions, "api-versions", nil, "Kubernetes API version available for capability checks (repeatable)")
	cmd.Flags().StringVar(&kubeVersion, "kube-version", "", "override Kubernetes version reported in capabilities")
	cmd.Flags().StringVar(&postRenderer, "post-renderer", "", "command piped the rendered manifests on stdin (yields stdout)")
	cmd.Flags().StringArrayVar(&postRenderers, "post-renderers", nil, "chained post-renderers (repeatable; output of N feeds N+1)")
	cmd.Flags().DurationVar(&postRendererTimeout, "post-renderer-timeout", 5*time.Minute, "per-stage timeout for post-renderers")
	cmd.Flags().IntVar(&historyMax, "history-max", 0, "maximum revisions to retain in history (0 = unlimited)")
	cmd.Flags().BoolVar(&force, "force", false, "delete-and-recreate resources to force update of immutable fields")
	cmd.Flags().BoolVar(&cleanupOnFail, "cleanup-on-fail", false, "delete partially-applied resources if the upgrade fails")
	cmd.Flags().BoolVar(&recreatePods, "recreate-pods", false, "trigger a rolling restart of Deployments/StatefulSets/DaemonSets")
	cmd.Flags().BoolVar(&waitForJobs, "wait-for-jobs", false, "wait for Job resources to complete (in addition to --wait)")
	cmd.Flags().DurationVar(&hookTimeout, "hook-timeout", 0, "cap each hook's per-hook timeout (0 = use the chart-declared value)")
	cmd.Flags().StringVar(&envName, "env", "", "environment name declared in keramos.yaml's environments: section")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
	cmd.Flags().StringSliceVar(&only, "only", nil, "incremental upgrade: dotted value paths to update; all other keys retain their previous values (repeatable, comma-separated)")

	return cmd
}
