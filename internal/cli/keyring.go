package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/spf13/cobra"
)

// keyringDirEnsured calls keyringDir() (defined in verify.go) and ensures the
// directory exists.
func keyringDirEnsured() (string, error) {
	dir, err := keyringDir()
	if nil != err {
		return "", err
	}
	if mkErr := os.MkdirAll(dir, 0o700); nil != mkErr {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "cannot create keyring directory", mkErr)
	}
	return dir, nil
}

func newKeyringCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyring",
		Short: "Manage the PGP keyring used for provenance verification",
		Long:  "Add, list, and remove public keys used to verify .prov sidecars.",
	}
	cmd.AddCommand(newKeyringListCommand())
	cmd.AddCommand(newKeyringAddCommand())
	cmd.AddCommand(newKeyringRemoveCommand())
	return cmd
}

func newKeyringListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List keys in the keramos keyring",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := keyringDirEnsured()
			if nil != err {
				return err
			}
			entries, readErr := os.ReadDir(dir)
			if nil != readErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read keyring", readErr)
			}
			if 0 == len(entries) {
				fmt.Fprintln(cmd.OutOrStdout(), "No keys installed.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-44s %s\n", "FINGERPRINT", "FILE")
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				path := filepath.Join(dir, e.Name())
				fp, idErr := keyFingerprint(path)
				if nil != idErr {
					fp = "(unreadable)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-44s %s\n", fp, e.Name())
			}
			return nil
		},
	}
}

func newKeyringAddCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "add <key-file>",
		Short: "Install a public key into the keramos keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			data, readErr := os.ReadFile(src)
			if nil != readErr {
				return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, readErr, "failed to read key %s", src)
			}
			// Validate the file IS a recognisable PGP public key before
			// installing. Without this, `keramos keyring add /tmp/random.txt`
			// silently installs garbage and fingerprint-print fails
			// silently — leaving an unverifiable file in the trust store.
			fp, parseErr := parseKeyFingerprint(data)
			if nil != parseErr {
				return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, parseErr,
					"file %s is not a valid PGP public key", src)
			}
			if "" == fp {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"file %s contains no PGP entities", src)
			}
			dir, err := keyringDirEnsured()
			if nil != err {
				return err
			}
			base := filepath.Base(src)
			// Restrict the destination filename so it cannot accidentally
			// overwrite keramos config files even if the keyring directory is
			// misconfigured to share a path with credentials.json.
			if "credentials.json" == base || "repositories.yaml" == base {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"refusing to install key with reserved filename %q", base)
			}
			dest := filepath.Join(dir, base)
			if _, statErr := os.Lstat(dest); nil == statErr && !force {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"key %q already exists; use --force to overwrite", base)
			}
			if err := os.WriteFile(dest, data, 0o600); nil != err {
				return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "failed to install key to %s", dest)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed key %s (%s)\n", base, fp)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing key file with the same basename")
	return cmd
}

func newKeyringRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <fingerprint-or-filename>",
		Aliases: []string{"rm"},
		Short:   "Remove a key from the keramos keyring",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.ToLower(args[0])
			dir, err := keyringDirEnsured()
			if nil != err {
				return err
			}
			entries, readErr := os.ReadDir(dir)
			if nil != readErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read keyring", readErr)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.ToLower(e.Name()) == id {
					if rmErr := os.Remove(filepath.Join(dir, e.Name())); nil != rmErr {
						return keramoserr.WrapError(keramoserr.ErrInternal, "failed to remove key", rmErr)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Removed key %s\n", e.Name())
					return nil
				}
				fp, _ := keyFingerprint(filepath.Join(dir, e.Name()))
				if strings.ToLower(fp) == id {
					if rmErr := os.Remove(filepath.Join(dir, e.Name())); nil != rmErr {
						return keramoserr.WrapError(keramoserr.ErrInternal, "failed to remove key", rmErr)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Removed key %s (%s)\n", e.Name(), fp)
					return nil
				}
			}
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "no key matching %q in keyring", args[0])
		},
	}
}

// keyFingerprint reads a public key file and returns the primary key
// fingerprint as a hex string. Empty if the file is not a recognised key.
func keyFingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if nil != err {
		return "", err
	}
	defer f.Close()
	entities, readErr := openpgp.ReadArmoredKeyRing(f)
	if nil != readErr {
		// Try non-armored.
		_, _ = f.Seek(0, io.SeekStart)
		entities, readErr = openpgp.ReadKeyRing(f)
		if nil != readErr {
			return "", readErr
		}
	}
	if 0 == len(entities) {
		return "", nil
	}
	return fmt.Sprintf("%X", entities[0].PrimaryKey.Fingerprint), nil
}

// parseKeyFingerprint inspects an in-memory key blob (armoured or binary)
// and returns the primary key fingerprint. Used to validate keyring-add
// input before the file is written to disk.
func parseKeyFingerprint(data []byte) (string, error) {
	r := strings.NewReader(string(data))
	entities, readErr := openpgp.ReadArmoredKeyRing(r)
	if nil != readErr {
		r2 := strings.NewReader(string(data))
		entities, readErr = openpgp.ReadKeyRing(r2)
		if nil != readErr {
			return "", readErr
		}
	}
	if 0 == len(entities) {
		return "", nil
	}
	return fmt.Sprintf("%X", entities[0].PrimaryKey.Fingerprint), nil
}
