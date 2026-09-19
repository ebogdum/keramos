package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v3/internal/repo"
	"github.com/spf13/cobra"
)

func newSignCommand() *cobra.Command {
	var keyPath string

	cmd := &cobra.Command{
		Use:   "sign <archive.keramos.tgz>",
		Short: "Sign a package archive with a PGP private key",
		Long:  "Create a .keramos.tgz.prov provenance file for a keramos package archive using PGP cleartext signing.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := args[0]

			provPath, err := repo.SignPackage(archivePath, keyPath)
			if nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully signed: %s\n", provPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&keyPath, "key", "", "path to PGP private key file (required)")
	_ = cmd.MarkFlagRequired("key")

	return cmd
}
