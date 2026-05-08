package cli

import (
	"fmt"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/spf13/cobra"
)

func newDriftCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "drift <release-name>",
		Short: "Detect drift between a release's stored manifest and live cluster state",
		Long: `Compare the manifest that keramos installed against what is currently in the cluster.

This is the primitive Argo CD users rely on: a list of resources that were
mutated (kubectl edit, controller mutation, hand-written kustomize) since
install or last upgrade. Use 'keramos reconcile' to re-apply the stored manifest.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			items, err := action.Drift(client, args[0])
			if nil != err {
				return err
			}
			if 0 == len(items) {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift detected.")
				return nil
			}
			if "json" == output {
				out, fmtErr := FormatJSON(items)
				if nil != fmtErr {
					return fmtErr
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			if "yaml" == output {
				out, fmtErr := FormatYAML(items)
				if nil != fmtErr {
					return fmtErr
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				ns := it.Namespace
				if "" == ns {
					ns = "(cluster)"
				}
				rows = append(rows, []string{
					it.Kind.String(),
					it.ResourceKind,
					ns,
					it.Name,
					fmt.Sprintf("%d", len(it.FieldDiffs)),
				})
			}
			fmt.Fprint(cmd.OutOrStdout(),
				FormatTable([]string{"DRIFT", "KIND", "NAMESPACE", "NAME", "FIELDS"}, rows))
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
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
