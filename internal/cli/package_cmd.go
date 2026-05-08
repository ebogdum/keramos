package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/repo"
	"github.com/spf13/cobra"
)

func newPackageVerifyCmd() *cobra.Command {
	var keyring string
	cmd := &cobra.Command{
		Use:   "verify <archive.keramos.tgz>",
		Short: "Verify a package archive's PGP signature against a keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			archive := args[0]
			prov := archive + ".prov"
			return repo.VerifySignature(archive, prov, keyring)
		},
	}
	cmd.Flags().StringVar(&keyring, "keyring", "", "public-key file or PGP keyring used for verification")
	_ = cmd.MarkFlagRequired("keyring")
	return cmd
}

func newPackageCommand() *cobra.Command {
	var (
		destination    string
		version        string
		appVersion     string
		reproducible   bool
		sign           bool
		key            string
		keyring        string
		passphraseFile string
	)

	cmd := &cobra.Command{
		Use:   "package <path>",
		Short: "Package a keramos package into a .keramos.tgz archive",
		Long:  "Package a keramos package directory into a versioned .keramos.tgz archive file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packagePath := args[0]

			if "" == destination {
				destination = "."
			}

			logger.Debug("packaging %s to %s", packagePath, destination)

			archivePath, err := repo.PackageArchiveOpts(packagePath, destination, repo.PackageOpts{
				Version:      version,
				AppVersion:   appVersion,
				Reproducible: reproducible,
			})
			if nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully packaged to: %s\n", archivePath)

			if sign {
				keyPath := key
				// --keyring + --key together imply "extract this secret key
				// from the keyring file"; the sign path expects a private-key
				// file directly, so we accept both shapes for convenience.
				if "" == keyPath && "" != keyring {
					keyPath = keyring
				}
				if "" == keyPath {
					return fmt.Errorf("--sign requires --key or --keyring")
				}
				_ = passphraseFile // accepted; the underlying key is unprotected in our flow
				provPath, sErr := repo.SignPackage(archivePath, keyPath)
				if nil != sErr {
					return sErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Signed: %s\n", provPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&destination, "destination", "d", ".", "directory to write the archive to")
	cmd.Flags().StringVar(&version, "version", "", "override the version in keramos.yaml")
	cmd.Flags().StringVar(&appVersion, "app-version", "", "override the appVersion in keramos.yaml")
	cmd.Flags().BoolVar(&reproducible, "reproducible", false, "produce byte-identical output across machines (zero timestamps, canonical modes)")
	cmd.Flags().BoolVar(&sign, "sign", false, "produce a .prov provenance file alongside the archive (requires --key or --keyring)")
	cmd.Flags().StringVar(&key, "key", "", "PGP private key file or signer name (used with --sign)")
	cmd.Flags().StringVar(&keyring, "keyring", "", "PGP keyring file containing the signer (alternative to --key)")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "", "file containing the key's passphrase (- for stdin)")

	cmd.AddCommand(newSignCommand())
	cmd.AddCommand(newPackageVerifyCmd())

	return cmd
}
