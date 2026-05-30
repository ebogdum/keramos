package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/internal/repo"
	"github.com/spf13/cobra"
)

func newRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage OCI registries for keramos packages",
		Long:  "Login, logout, push, and pull keramos packages to/from OCI-compliant registries.",
	}

	cmd.AddCommand(newRegistryLoginCommand())
	cmd.AddCommand(newRegistryLogoutCommand())
	cmd.AddCommand(newRegistryPushCommand())
	cmd.AddCommand(newRegistryPullCommand())

	return cmd
}

func newRegistryLoginCommand() *cobra.Command {
	var (
		username string
		password string
	)

	cmd := &cobra.Command{
		Use:        "login <host>",
		Short:      "Log in to an OCI registry",
		Deprecated: "use 'keramos login' instead",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			registry := &repo.OCIRegistry{}

			if err := registry.Login(host, username, password); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Login succeeded for %s\n", host)
			return nil
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "registry username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "registry password")

	return cmd
}

func newRegistryLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:        "logout <host>",
		Short:      "Log out from an OCI registry",
		Deprecated: "use 'keramos logout' instead",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			registry := &repo.OCIRegistry{}

			if err := registry.Logout(host); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logout succeeded for %s\n", host)
			return nil
		},
	}
}

func newRegistryPushCommand() *cobra.Command {
	var (
		plainHTTP bool
		insecure  bool
	)
	cmd := &cobra.Command{
		Use:   "push <archive> <ref>",
		Short: "Push a keramos archive to an OCI registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := args[0]
			ref := args[1]
			registry := &repo.OCIRegistry{
				PlainHTTP:             plainHTTP,
				InsecureSkipTLSVerify: insecure,
			}

			if err := registry.Push(archivePath, ref); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pushed %s to %s\n", archivePath, ref)
			return nil
		},
	}
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "use plaintext HTTP (no TLS)")
	cmd.Flags().BoolVar(&insecure, "insecure-skip-tls-verify", false, "skip TLS certificate verification")
	return cmd
}

func newRegistryPullCommand() *cobra.Command {
	var (
		destDir        string
		plainHTTP      bool
		insecure       bool
		cosignKey      string
		cosignIdentity string
		cosignIssuer   string
	)

	cmd := &cobra.Command{
		Use:   "pull <ref>",
		Short: "Pull a keramos package from an OCI registry",
		Long: "Pull a keramos package from an OCI registry.\n\n" +
			"Supply --cosign-key (key-based) or --cosign-identity together with " +
			"--cosign-issuer (keyless/Sigstore) to require a valid cosign " +
			"signature on the artifact BEFORE it is pulled. Verification is " +
			"fail-closed: an unsigned or wrongly-signed artifact is not pulled.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			// Fail-closed cosign verification before the artifact is fetched.
			if "" != cosignKey || "" != cosignIdentity || "" != cosignIssuer {
				var keyless *repo.CosignKeylessOpts
				if "" == cosignKey {
					keyless = &repo.CosignKeylessOpts{CertIdentity: cosignIdentity, CertIssuer: cosignIssuer}
				}
				if err := repo.VerifyCosign(ref, cosignKey, keyless); nil != err {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "cosign signature verified for %s\n", ref)
			}

			registry := &repo.OCIRegistry{
				PlainHTTP:             plainHTTP,
				InsecureSkipTLSVerify: insecure,
			}

			archivePath, err := registry.Pull(ref, destDir)
			if nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s to %s\n", ref, archivePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&destDir, "destination", "d", ".", "directory to save the pulled package")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "use plaintext HTTP (no TLS)")
	cmd.Flags().BoolVar(&insecure, "insecure-skip-tls-verify", false, "skip TLS certificate verification")
	cmd.Flags().StringVar(&cosignKey, "cosign-key", "", "verify the artifact's cosign signature with this public key before pulling")
	cmd.Flags().StringVar(&cosignIdentity, "cosign-identity", "", "keyless cosign: required certificate identity (use with --cosign-issuer)")
	cmd.Flags().StringVar(&cosignIssuer, "cosign-issuer", "", "keyless cosign: required certificate OIDC issuer (use with --cosign-identity)")

	return cmd
}
