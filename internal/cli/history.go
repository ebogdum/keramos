package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
)

func newHistoryCommand() *cobra.Command {
	var (
		maxRevisions int
		output       string
	)

	cmd := &cobra.Command{
		Use:   "history <release-name>",
		Short: "Show release history",
		Long:  "Display the revision history for a release.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, args[0], maxRevisions, output)
		},
	}

	cmd.Flags().IntVar(&maxRevisions, "max", 0, "maximum number of revisions to show (0 = all)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")

	return cmd
}

func runHistory(cmd *cobra.Command, releaseName string, maxRevisions int, output string) error {
	if err := validateOutputFormat(output); nil != err {
		return err
	}

	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return err
	}

	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
	history, err := storage.History(releaseName)
	if nil != err {
		return err
	}

	if 0 == len(history) {
		fmt.Fprintf(cmd.OutOrStdout(), "no history found for release %s\n", releaseName)
		return nil
	}

	if 0 < maxRevisions && maxRevisions < len(history) {
		history = history[len(history)-maxRevisions:]
	}

	return outputHistory(cmd, history, output)
}

func outputHistory(cmd *cobra.Command, history []*release.Release, output string) error {
	if "json" == output {
		out, err := FormatJSON(history)
		if nil != err {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}
	if "yaml" == output {
		out, err := FormatYAML(history)
		if nil != err {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}

	headers := []string{"REVISION", "STATUS", "PACKAGE", "UPDATED", "DESCRIPTION"}
	rows := make([][]string, 0, len(history))

	for _, rel := range history {
		pkg := fmt.Sprintf("%s-%s", rel.Package.Name, rel.Package.Version)
		rows = append(rows, []string{
			fmt.Sprintf("%d", rel.Revision),
			string(rel.Status),
			pkg,
			rel.Info.LastDeployed.Format("2006-01-02 15:04:05"),
			rel.Info.Description,
		})
	}

	fmt.Fprint(cmd.OutOrStdout(), FormatTable(headers, rows))
	return nil
}
