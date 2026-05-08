package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/repo"
	"github.com/spf13/cobra"
)

// newPullCommand fetches a package from an HTTP/index repository or an OCI
// registry. For OCI references (oci://...) the registry pull path is used.
// For HTTP repositories, the latest version (or a specific --version) is
// downloaded from the index.yaml URL set.
// hasNonStandardScheme returns true when ref begins with a URL scheme that is
// neither one of keramos's built-in transports (oci://, http://, https://) nor a
// chart name. The plugin-downloader dispatch is tried for these.
func hasNonStandardScheme(ref string) bool {
	idx := strings.Index(ref, "://")
	if idx <= 0 {
		return false
	}
	scheme := ref[:idx]
	switch scheme {
	case "oci", "http", "https":
		return false
	}
	return true
}

func newPullCommand() *cobra.Command {
	var (
		version   string
		destDir   string
		repoURL   string
		untar     bool
		untarDir  string
		verifySig bool
		fetchProv bool
		caFile    string
		certFile  string
		keyFile   string
	)
	cmd := &cobra.Command{
		Use:   "pull <chart>",
		Short: "Download a chart from a repository",
		Long: `Download a chart from a repository and (optionally) unpack it locally.

Accepts an OCI reference (oci://registry/name) or a chart name with --repo set
to an HTTP repository URL.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]

			if "" == destDir {
				destDir = "."
			}
			if absDest, err := filepath.Abs(destDir); nil == err {
				destDir = absDest
			}

			if strings.HasPrefix(ref, "oci://") {
				registry := &repo.OCIRegistry{}
				fullRef := ref
				if "" != version {
					fullRef = ref + ":" + version
				}
				archivePath, pullErr := registry.Pull(fullRef, destDir)
				if nil != pullErr {
					return pullErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s to %s\n", fullRef, archivePath)
				return nil
			}

			// Plugin-downloader dispatch: if a registered plugin advertises
			// the URL's scheme, run it. The downloader writes the archive
			// bytes to stdout (or, if its first line is `file://<path>`,
			// to that path) — see internal/repo/downloader.go.
			if hasNonStandardScheme(ref) {
				data, ok, dlErr := repo.TryDownloaderFetchPublic(ref)
				if ok {
					if nil != dlErr {
						return dlErr
					}
					out := filepath.Join(destDir, filepath.Base(ref))
					if "" == filepath.Ext(out) {
						out += ".tgz"
					}
					if wErr := os.WriteFile(out, data, 0o644); nil != wErr {
						return keramoserr.WrapError(keramoserr.ErrInternal, "write downloader output", wErr)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Pulled %s to %s\n", ref, out)
					return nil
				}
			}

			if "" == repoURL {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "--repo is required for non-OCI charts")
			}

			tlsClient, tlsErr := repo.NewClientWithTLS(caFile, certFile, keyFile)
			if nil != tlsErr {
				return tlsErr
			}
			idx, idxErr := repo.FetchIndexWith(tlsClient, repoURL)
			if nil != idxErr {
				return idxErr
			}
			entries, ok := idx.Entries[ref]
			if !ok || 0 == len(entries) {
				return keramoserr.NewErrorf(keramoserr.ErrRepo, "chart %q not found in repository %s", ref, repoURL)
			}

			selected := entries[0]
			if "" != version {
				found := false
				for _, e := range entries {
					if e.Version == version {
						selected = e
						found = true
						break
					}
				}
				if !found {
					return keramoserr.NewErrorf(keramoserr.ErrRepo, "chart %q version %q not found", ref, version)
				}
			}

			if 0 == len(selected.URLs) {
				return keramoserr.NewErrorf(keramoserr.ErrRepo, "chart %q has no download URLs", ref)
			}
			downloadURL := selected.URLs[0]
			if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
				downloadURL = strings.TrimRight(repoURL, "/") + "/" + downloadURL
			}

			archive, err := repo.DownloadArchive(downloadURL)
			if nil != err {
				return err
			}

			if verifySig {
				if vErr := repo.VerifyArchive(archive, downloadURL); nil != vErr {
					_ = os.Remove(archive)
					return vErr
				}
			}

			// Path-traversal guard: ref and version may contain `/` or `..`
			// crafted by an attacker controlling index.yaml. Reject the
			// download if the joined path escapes destDir.
			finalName := fmt.Sprintf("%s-%s.tgz", filepath.Base(ref), filepath.Base(selected.Version))
			finalPath := filepath.Join(destDir, finalName)
			cleanDest := filepath.Clean(destDir) + string(filepath.Separator)
			if !strings.HasPrefix(filepath.Clean(finalPath)+string(filepath.Separator), cleanDest) {
				_ = os.Remove(archive)
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"computed archive path %q escapes destination %q", finalPath, destDir)
			}
			// If destination is a symlink, refuse to follow it (prevents
			// overwriting an attacker-chosen target).
			if existing, err := os.Lstat(finalPath); nil == err && 0 != existing.Mode()&os.ModeSymlink {
				_ = os.Remove(archive)
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"refusing to overwrite symlink at %q", finalPath)
			}
			if mvErr := os.Rename(archive, finalPath); nil != mvErr {
				return keramoserr.WrapErrorf(keramoserr.ErrInternal, mvErr, "failed to move archive to %s", finalPath)
			}

			if fetchProv {
				provPath := finalPath + ".prov"
				provLocal, provErr := repo.DownloadArchive(downloadURL + ".prov")
				if nil != provErr {
					fmt.Fprintf(cmd.OutOrStdout(), "warning: failed to fetch provenance: %v\n", provErr)
				} else if mvErr := os.Rename(provLocal, provPath); nil != mvErr {
					_ = os.Remove(provLocal)
					fmt.Fprintf(cmd.OutOrStdout(), "warning: failed to save provenance to %s: %v\n", provPath, mvErr)
				}
			}

			if untar {
				if "" == untarDir {
					untarDir = filepath.Join(destDir, ref)
				}
				if err := repo.ExtractArchive(finalPath, untarDir); nil != err {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pulled and extracted: %s\n", untarDir)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pulled: %s\n", finalPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "specific version to pull (default: latest)")
	cmd.Flags().StringVar(&destDir, "destination", ".", "directory to save the archive in")
	cmd.Flags().StringVar(&repoURL, "repo", "", "HTTP repository URL containing index.yaml")
	cmd.Flags().BoolVar(&untar, "untar", false, "extract the archive after downloading")
	cmd.Flags().StringVar(&untarDir, "untardir", "", "extraction directory (default: <dest>/<chart>)")
	cmd.Flags().BoolVar(&verifySig, "verify", false, "verify provenance signature before saving")
	cmd.Flags().BoolVar(&fetchProv, "prov", false, "also download the .prov provenance sidecar")
	cmd.Flags().StringVar(&caFile, "ca-file", "", "CA bundle for HTTPS")
	cmd.Flags().StringVar(&certFile, "cert-file", "", "client certificate for HTTPS")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "client key for HTTPS")

	return cmd
}
