package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
)

func newAuditCommand() *cobra.Command {
	var (
		output   string
		revision int
	)
	cmd := &cobra.Command{
		Use:   "audit <release-name>",
		Short: "Show the chronological audit trail for a release",
		Long: `Print every revision of a release with the recorded provenance: who
initiated the operation, what action (install/upgrade/rollback), the kubeconfig
context, the CLI flags supplied, the value files referenced, and the timestamp.

This satisfies the SOC 2 / SLSA / general "who changed what when" question
without requiring an external audit pipeline.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			storage, storageErr := release.SelectStorage(client.Clientset(), client.Namespace())
			if nil != storageErr {
				return storageErr
			}
			history, err := storage.History(args[0])
			if nil != err {
				return err
			}
			if 0 < revision {
				filtered := history[:0]
				for _, rel := range history {
					if rel.Revision == revision {
						filtered = append(filtered, rel)
					}
				}
				history = filtered
			}
			if 0 == len(history) {
				fmt.Fprintf(cmd.OutOrStdout(), "no history for %s\n", args[0])
				return nil
			}
			records := make([]map[string]any, 0, len(history))
			for _, rel := range history {
				records = append(records, map[string]any{
					"revision":   rel.Revision,
					"action":     rel.Audit.Action,
					"user":       rel.Audit.User,
					"hostname":   rel.Audit.Hostname,
					"context":    rel.Audit.KubeContext,
					"flags":      rel.Audit.Flags,
					"valueFiles": rel.Audit.ValueFiles,
					"parentRev":  rel.Audit.ParentRev,
					"status":     string(rel.Status),
					"timestamp":  rel.Audit.Timestamp,
				})
			}
			if "json" == output {
				out, err := FormatJSON(records)
				if nil != err {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			if "yaml" == output {
				out, err := FormatYAML(records)
				if nil != err {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}

			rows := make([][]string, 0, len(history))
			for _, rel := range history {
				ts := ""
				if !rel.Audit.Timestamp.IsZero() {
					ts = rel.Audit.Timestamp.Format("2006-01-02 15:04:05")
				} else {
					ts = rel.Info.LastDeployed.Format("2006-01-02 15:04:05")
				}
				action := rel.Audit.Action
				if "" == action {
					action = "(legacy)"
				}
				user := rel.Audit.User
				if "" == user {
					user = "(unknown)"
				}
				rows = append(rows, []string{
					fmt.Sprintf("%d", rel.Revision),
					action,
					user,
					string(rel.Status),
					ts,
				})
			}
			fmt.Fprint(cmd.OutOrStdout(),
				FormatTable([]string{"REVISION", "ACTION", "USER", "STATUS", "TIMESTAMP"}, rows))
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
	cmd.Flags().IntVar(&revision, "revision", 0, "show only the named revision (0 = all)")
	return cmd
}
