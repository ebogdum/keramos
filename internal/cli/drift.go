package cli

import (
	"fmt"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	"github.com/ebogdum/keramos/v2/internal/diff"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/pkg"
	"github.com/spf13/cobra"
)

func newDriftCommand() *cobra.Command {
	var (
		releaseName string
		profile     string
		valueFiles  []string
		sets        []string
		setStrings  []string
		noColor     bool
		serverSide  bool
	)
	cmd := &cobra.Command{
		Use:   "drift [package-path]",
		Short: "Three-way compare: package vs recorded state vs live cluster",
		Long: `Compare three views of a release side by side and show, per resource and
field, where they disagree:

  package  — what the directory would render right now
  state    — what keramos last recorded (the stored manifest)
  running  — what is actually in the cluster

Two kinds of divergence are flagged:

  ⚠ cluster drift   — state ≠ running: something changed the cluster (kubectl
                      edit, a controller, a webhook) since the last apply.
  → pending apply   — package ≠ state: the directory has edits not yet applied
                      (this is also what 'keramos plan' previews).

The release is derived from the package's keramos.yaml name; use -r to override.
Comparison is limited to keramos-managed fields, so cluster-injected noise
(status, managedFields, server defaults) is ignored. Use 'keramos reconcile' to
push the stored state back onto a drifted cluster.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if 1 == len(args) {
				dir = args[0]
			}
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			if "" == releaseName {
				meta, mErr := pkg.LoadPackageMetadata(dir)
				if nil != mErr {
					return mErr
				}
				if "" == meta.Name {
					return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
						"package at %q has no 'name' in keramos.yaml; pass -r/--release", dir)
				}
				releaseName = meta.Name
			}
			pkgManifest, rErr := renderPkgManifest(dir, profile, valueFiles, sets, setStrings)
			if nil != rErr {
				return rErr
			}
			// Server-side mode: compare the LIVE cluster objects against what a
			// server-side apply of the package would actually produce, so
			// admission webhooks and API-server defaulting are reflected — the
			// change the cluster would really compute, not a client-side render.
			if serverSide {
				live, merged, ssErr := client.ServerSideDiff(pkgManifest)
				if nil != ssErr {
					return ssErr
				}
				changes, cErr := diff.Compute(live, merged, diff.Filters{})
				if nil != cErr {
					return cErr
				}
				w := cmd.OutOrStdout()
				if 0 == len(changes) {
					fmt.Fprintln(w, "In sync — the live cluster matches a server-side apply of the package.")
					return nil
				}
				fmt.Fprintf(w, "drift (server-side): live cluster → apply-dry-run   (release %s)\n\n", releaseName)
				fmt.Fprint(w, formatPlanChanges(changes, !noColor, nil, "live"))
				fmt.Fprint(w, changeSummary(changes))
				return nil
			}
			stateManifest, liveManifest, sErr := action.StateAndLiveManifests(client, releaseName)
			if nil != sErr {
				return sErr
			}
			resources, tErr := threeWay(pkgManifest, stateManifest, liveManifest)
			if nil != tErr {
				return tErr
			}
			w := cmd.OutOrStdout()
			if 0 == len(resources) {
				fmt.Fprintln(w, "In sync — package, state, and cluster agree on every managed field.")
				return nil
			}
			fmt.Fprintf(w, "drift: package ↔ state ↔ running   (release %s)\n\n", releaseName)
			fmt.Fprint(w, formatThreeWay(resources, !noColor))
			return nil
		},
	}
	cmd.Flags().StringVarP(&releaseName, "release", "r", "", "state/release name (default: derived from keramos.yaml)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile to apply when rendering the package side")
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file for the package side (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value for the package side (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "key=value (string) for the package side (repeatable)")
	cmd.Flags().BoolVar(&serverSide, "server-side", false, "compare the live cluster against a server-side apply dry-run (reflects admission/defaulting) instead of the three-way client-side view")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")
	return cmd
}

func newReconcileCommand() *cobra.Command {
	var (
		timeout time.Duration
		noWait  bool
	)
	cmd := &cobra.Command{
		Use:   "reconcile <release-name>",
		Short: "Re-apply the stored manifest of a release to converge cluster state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			effective := timeout
			if noWait {
				effective = 0
			}
			converged, err := action.Reconcile(client, args[0], effective)
			if nil != err {
				return err
			}
			if 0 == len(converged) {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift to reconcile.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reconciled %d resource(s):\n", len(converged))
			for _, r := range converged {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", r)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "readiness wait after apply")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "do not wait for resources to be ready after re-apply")
	return cmd
}
