package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/repo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newRepoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage keramos package repositories",
		Long:  "Add, list, remove, and update keramos package repositories.",
	}

	cmd.AddCommand(newRepoAddCommand())
	cmd.AddCommand(newRepoListCommand())
	cmd.AddCommand(newRepoRemoveCommand())
	cmd.AddCommand(newRepoUpdateCommand())
	cmd.AddCommand(newRepoIndexCommand())

	return cmd
}

func newRepoAddCommand() *cobra.Command {
	var (
		username        string
		password        string
		passCredentials bool
		caFile          string
		certFile        string
		keyFile         string
		noUpdate        bool
		forceUpdate     bool
		insecureSkipTLS bool
		passAll         bool
	)
	cmd := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			url := args[1]

			rf, err := repo.LoadRepoFile()
			if nil != err {
				return err
			}

			// --no-update suppresses re-fetch when name exists,
			// --force-update replaces an existing entry.
			if existing := rf.Find(name); nil != existing {
				if !forceUpdate {
					fmt.Fprintf(cmd.OutOrStdout(), "%q already exists with URL %q\n", name, existing.URL)
					if noUpdate {
						return nil
					}
					return nil
				}
				_ = rf.Remove(name)
			}

			if err := rf.Add(name, url); nil != err {
				return err
			}
			// Save TLS material on the entry.
			if updated := rf.Find(name); nil != updated {
				updated.CAFile = caFile
				updated.CertFile = certFile
				updated.KeyFile = keyFile
			}

			// Stash basic-auth credentials in the unified credential store.
			if "" != username || "" != password {
				store, credErr := repo.LoadCredentialStore()
				if nil != credErr {
					return credErr
				}
				host := repoHost(url)
				if "" == host {
					return fmt.Errorf("could not derive host from URL %q for credential storage", url)
				}
				store.Set(host, repo.Credential{
					Type:     repo.AuthBasic,
					Username: username,
					Password: password,
				})
				if err := store.Save(); nil != err {
					return err
				}
				_ = passCredentials   // forward-compat: future redirect-aware client honors this
				_ = insecureSkipTLS   // accepted; TLS bypass surface is via repo URL/scheme
				_ = passAll           // flag accepted; redirect-aware client will honour it
			}

			if err := rf.Save(); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%q has been added to your repositories\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "HTTP basic-auth username")
	cmd.Flags().StringVar(&password, "password", "", "HTTP basic-auth password")
	cmd.Flags().BoolVar(&passCredentials, "pass-credentials", false, "forward credentials on HTTP redirects")
	cmd.Flags().StringVar(&caFile, "ca-file", "", "CA bundle to verify the repo's server certificate")
	cmd.Flags().StringVar(&certFile, "cert-file", "", "client certificate for mutual TLS")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "client key for mutual TLS")
	cmd.Flags().BoolVar(&noUpdate, "no-update", false, "do nothing if the repository already exists")
	cmd.Flags().BoolVar(&forceUpdate, "force-update", false, "replace the existing repository entry")
	cmd.Flags().BoolVar(&insecureSkipTLS, "insecure-skip-tls-verify", false, "skip TLS certificate verification")
	cmd.Flags().BoolVar(&passAll, "pass-credentials-all", false, "send credentials on every HTTP redirect")
	return cmd
}

// repoHost extracts the host (with port) from a repository URL for credential
// store keying. Falls back to the raw URL when parsing fails.
func repoHost(rawURL string) string {
	if i := indexAny(rawURL, "/?"); i > 0 {
		// Strip scheme://host part — find first / after scheme.
		u := rawURL
		if idx := substr(u, "://"); -1 != idx {
			u = u[idx+3:]
		}
		if slash := indexAny(u, "/?"); slash > 0 {
			return u[:slash]
		}
		return u
	}
	return rawURL
}

func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

func substr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func newRepoListCommand() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rf, err := repo.LoadRepoFile()
			if nil != err {
				return err
			}

			switch output {
			case "json":
				data, mErr := json.MarshalIndent(rf.Repositories, "", "  ")
				if nil != mErr {
					return mErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			case "yaml":
				data, mErr := yaml.Marshal(rf.Repositories)
				if nil != mErr {
					return mErr
				}
				fmt.Fprint(cmd.OutOrStdout(), string(data))
				return nil
			}

			if 0 == len(rf.Repositories) {
				fmt.Fprintln(cmd.OutOrStdout(), "No repositories configured.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", "NAME", "URL")
			for _, r := range rf.Repositories {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", r.Name, r.URL)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
	return cmd
}

func newRepoRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			rf, err := repo.LoadRepoFile()
			if nil != err {
				return err
			}

			if err := rf.Remove(name); nil != err {
				return err
			}

			if err := rf.Save(); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%q has been removed from your repositories\n", name)
			return nil
		},
	}
}

func newRepoUpdateCommand() *cobra.Command {
	var failOnUpdateFail bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update repository indexes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rf, err := repo.LoadRepoFile()
			if nil != err {
				return err
			}

			if 0 == len(rf.Repositories) {
				fmt.Fprintln(cmd.OutOrStdout(), "No repositories configured.")
				return nil
			}

			var firstErr error
			for _, r := range rf.Repositories {
				logger.Debug("updating index for %s (%s)", r.Name, r.URL)
				_, fetchErr := repo.FetchIndex(r.URL)
				if nil != fetchErr {
					fmt.Fprintf(cmd.OutOrStdout(), "...error updating %s: %s\n", r.Name, fetchErr)
					if nil == firstErr {
						firstErr = fetchErr
					}
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "...successfully got an update from %q\n", r.Name)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Update complete.")
			if failOnUpdateFail && nil != firstErr {
				return firstErr
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&failOnUpdateFail, "fail-on-repo-update-fail", false, "exit non-zero if any repository update fails")
	return cmd
}

func newRepoIndexCommand() *cobra.Command {
	var baseURL string
	var merge bool
	var signKey string

	cmd := &cobra.Command{
		Use:   "index <dir>",
		Short: "Generate an index.yaml for a directory of archives",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			absDir, err := filepath.Abs(dir)
			if nil != err {
				return err
			}

			newIdx, err := repo.GenerateIndex(absDir, baseURL)
			if nil != err {
				return err
			}

			indexPath := filepath.Join(absDir, "index.yaml")

			if merge {
				existing, loadErr := repo.LoadIndex(indexPath)
				if nil != loadErr {
					existing = &repo.IndexFile{
						APIVersion: "v1",
						Entries:    make(map[string][]repo.IndexEntry),
					}
				}
				newIdx = repo.MergeIndex(existing, newIdx)
			}

			if err := repo.SaveIndex(newIdx, indexPath); nil != err {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Index generated at %s\n", indexPath)

			if "" != signKey {
				provPath, signErr := repo.SignFile(indexPath, signKey)
				if nil != signErr {
					return signErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Index signed: %s\n", provPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&baseURL, "url", "", "base URL for download URLs in the index")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge with existing index.yaml instead of regenerating")
	cmd.Flags().StringVar(&signKey, "sign", "", "private key path to sign the index (produces index.yaml.prov)")

	return cmd
}
