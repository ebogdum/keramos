package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/ebogdum/keramos/v3/internal/sbom"
	"github.com/spf13/cobra"
)

func newSBOMCommand() *cobra.Command {
	var revision int
	cmd := &cobra.Command{
		Use:   "sbom <release-name>",
		Short: "Emit a CycloneDX 1.5 SBOM for a release",
		Long: `Inspect every container image referenced by a release's manifest plus the
release's own package metadata, and emit a CycloneDX 1.5 JSON SBOM. The output
is suitable for cosign attestation, Grype/Trivy ingestion, and Dependency
Track upload.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
			var rel *release.Release
			if 0 < revision {
				rel, err = storage.Get(args[0], revision)
			} else {
				rel, err = storage.Last(args[0])
			}
			if nil != err {
				return err
			}
			version := "dev"
			doc, err := sbom.Generate(rel, version)
			if nil != err {
				return err
			}
			out, err := sbom.FormatJSON(doc)
			if nil != err {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&revision, "revision", 0, "release revision (default: latest)")
	return cmd
}
