package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/graph"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
)

func newGraphCommand() *cobra.Command {
	var (
		format   string
		revision int
	)
	cmd := &cobra.Command{
		Use:   "graph <release-name>",
		Short: "Render a dependency graph of a release's resources and hooks",
		Long: `Produce a visualisation of the resources and lifecycle hooks in a release.

Mermaid (default) is copy-paste compatible with Markdown. DOT works with
Graphviz (dot -Tpng). ASCII is for terminals without external tools.

Hook nodes are ordered by phase (pre-install, post-install, ...) and weight,
so the implicit ordering is made explicit.`,
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
			var rel *release.Release
			if 0 < revision {
				rel, err = storage.Get(args[0], revision)
			} else {
				rel, err = storage.Last(args[0])
			}
			if nil != err {
				return err
			}
			out, err := graph.Render(rel, graph.Format(format))
			if nil != err {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "mermaid", "format: mermaid, dot, ascii")
	cmd.Flags().IntVar(&revision, "revision", 0, "release revision (default: latest)")
	return cmd
}
