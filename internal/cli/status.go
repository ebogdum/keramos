package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	var (
		revision int
		output   string
	)

	cmd := &cobra.Command{
		Use:   "status <release-name>",
		Short: "Show release status",
		Long:  "Display the status of a deployed release including revision, package, and notes.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args[0], revision, output)
		},
	}

	cmd.Flags().IntVar(&revision, "revision", 0, "show status of a specific revision")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")

	return cmd
}

func runStatus(cmd *cobra.Command, releaseName string, revision int, output string) error {
	if err := validateOutputFormat(output); nil != err {
		return err
	}

	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return err
	}

	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())

	var rel *release.Release
	if 0 < revision {
		rel, err = storage.Get(releaseName, revision)
	} else {
		rel, err = storage.Last(releaseName)
	}
	if nil != err {
		return err
	}

	return outputStatus(cmd, rel, output)
}

func outputStatus(cmd *cobra.Command, rel *release.Release, output string) error {
	if "json" == output {
		out, err := FormatJSON(rel)
		if nil != err {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}
	if "yaml" == output {
		out, err := FormatYAML(rel)
		if nil != err {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "NAME:       %s\n", rel.Name)
	fmt.Fprintf(w, "NAMESPACE:  %s\n", rel.Namespace)
	fmt.Fprintf(w, "STATUS:     %s\n", rel.Status)
	fmt.Fprintf(w, "REVISION:   %d\n", rel.Revision)
	fmt.Fprintf(w, "PACKAGE:    %s-%s\n", rel.Package.Name, rel.Package.Version)
	fmt.Fprintf(w, "UPDATED:    %s\n", rel.Info.LastDeployed.Format("2006-01-02 15:04:05"))

	if "" != rel.Info.Description {
		fmt.Fprintf(w, "DESCRIPTION: %s\n", rel.Info.Description)
	}

	if "" != rel.Notes {
		fmt.Fprintf(w, "\nNOTES:\n%s\n", rel.Notes)
	}

	return nil
}
