package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/repo"
	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	var (
		username      string
		password      string
		passwordStdin bool
		token         string
		apiKey        string
		insecure      bool
	)

	cmd := &cobra.Command{
		Use:   "login <host>",
		Short: "Store credentials for a package registry",
		Long: `Authenticate to a package registry using basic auth, bearer token, or API key.

Examples:
  keramos login registry.example.com -u myuser -p mypass
  keramos login registry.example.com --token eyJhbG...
  keramos login registry.example.com --api-key abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]

			if passwordStdin {
				if "" != password {
					return keramoserr.NewError(keramoserr.ErrCLIValidation,
						"--password and --password-stdin are mutually exclusive")
				}
				r := bufio.NewReader(os.Stdin)
				line, err := r.ReadString('\n')
				if nil != err && "" == strings.TrimSpace(line) {
					return keramoserr.WrapError(keramoserr.ErrCLIValidation, "read password from stdin", err)
				}
				password = strings.TrimRight(line, "\r\n")
			}
			cred, err := buildCredential(username, password, token, apiKey)
			if nil != err {
				return err
			}
			// Record the operator's opt-in to reach this host over an untrusted
			// transport; the registry/HTTP clients consult it per host.
			cred.Insecure = insecure

			store, err := repo.LoadCredentialStore()
			if nil != err {
				return err
			}

			store.Set(host, cred)

			if err := store.Save(); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Login succeeded for %s\n", host)
			return nil
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "registry username (basic auth)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "registry password (basic auth)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (mutually exclusive with --password)")
	cmd.Flags().StringVar(&token, "token", "", "bearer token")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "reach this host over an untrusted transport (skip TLS verification / allow plain HTTP)")

	return cmd
}

func newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout <host>",
		Short: "Remove stored credentials for a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]

			store, err := repo.LoadCredentialStore()
			if nil != err {
				return err
			}

			if _, ok := store.Get(host); !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Not logged in to %s\n", host)
				return nil
			}

			store.Remove(host)

			if err := store.Save(); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logout succeeded for %s\n", host)
			return nil
		},
	}
}

func buildCredential(username, password, token, apiKey string) (repo.Credential, error) {
	if "" != token {
		return repo.Credential{Type: repo.AuthBearer, Token: token}, nil
	}
	if "" != apiKey {
		return repo.Credential{Type: repo.AuthAPIKey, APIKey: apiKey}, nil
	}
	if "" != username {
		return repo.Credential{Type: repo.AuthBasic, Username: username, Password: password}, nil
	}
	return repo.Credential{}, fmt.Errorf("provide --username/-u, --token, or --api-key")
}
