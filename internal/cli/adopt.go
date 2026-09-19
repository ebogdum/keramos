package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v2/internal/action"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/spf13/cobra"
)

func newAdoptCommand() *cobra.Command {
	var (
		description string
		createNS    bool
		labels      []string
	)
	cmd := &cobra.Command{
		Use:   "adopt <release-name> <resource-ref>...",
		Short: "Claim existing in-cluster resources as a keramos-managed release",
		Long: `Take ownership of resources that were created outside keramos (e.g., raw kubectl
apply, Terraform, hand-written manifests) so they become a tracked release.
After adoption you can keramos diff, keramos drift, keramos upgrade, keramos uninstall
them normally.

Resource references accept either of these forms:
  apps/v1/Deployment/myns/myapp
  v1/ConfigMap//cluster-scoped-cm
  kind=Deployment,name=myapp,ns=myns

Keramos fetches each referenced resource, strips server-side metadata, and
stores the result as revision 1 of the new release.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs := make([]action.ResourceRef, 0, len(args)-1)
			for _, raw := range args[1:] {
				ref, err := action.ParseResourceRef(raw)
				if nil != err {
					return err
				}
				refs = append(refs, ref)
			}
			labelMap, lErr := parseLabelFlags(labels)
			if nil != lErr {
				return lErr
			}
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			if createNS {
				if nsErr := client.CreateNamespace(namespace); nil != nsErr {
					return nsErr
				}
			}
			rel, err := action.Adopt(client, &action.AdoptOptions{
				ReleaseName: args[0],
				Namespace:   namespace,
				Description: description,
				Resources:   refs,
				Labels:      labelMap,
			})
			if nil != err {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Adopted %d resource(s) as release %q (revision 1, namespace %s).\n",
				len(refs), rel.Name, rel.Namespace)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "release description recorded in the audit trail")
	cmd.Flags().BoolVar(&createNS, "create-namespace", false, "create the release namespace if it does not exist")
	cmd.Flags().StringArrayVar(&labels, "labels", nil, "label key=value to attach to the release (repeatable)")
	return cmd
}
