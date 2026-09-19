package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newShowCommand exposes the package's metadata, default values, README,
// CRDs, or all of the above through subcommands.
func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show information about a package",
		Long:  "Show package metadata, default values, README, or CRDs without installing.",
	}
	cmd.AddCommand(newShowChartCommand())
	cmd.AddCommand(newShowValuesCommand())
	cmd.AddCommand(newShowReadmeCommand())
	cmd.AddCommand(newShowCRDsCommand())
	cmd.AddCommand(newShowAllCommand())
	return cmd
}

func newShowChartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "chart <package-path>",
		Short: "Show package metadata (keramos.yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readPackageFile(args[0], "keramos.yaml")
			if nil != err {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), data)
			return nil
		},
	}
}

func newShowValuesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "values <package-path>",
		Short: "Show default values (values.yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readPackageFile(args[0], "values.yaml")
			if nil != err {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), data)
			return nil
		},
	}
}

func newShowReadmeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "readme <package-path>",
		Short: "Show package README",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range []string{"README.md", "README.txt", "README"} {
				data, err := readPackageFile(args[0], name)
				if nil == err {
					fmt.Fprint(cmd.OutOrStdout(), data)
					return nil
				}
				if !os.IsNotExist(err) {
					return err
				}
			}
			return keramoserr.NewError(keramoserr.ErrCLIValidation, "no README found in package")
		},
	}
}

func newShowCRDsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "crds <package-path>",
		Short: "Show CRDs declared by the package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			crdDir := filepath.Join(args[0], "crds")
			info, statErr := os.Stat(crdDir)
			if nil != statErr || !info.IsDir() {
				fmt.Fprintln(cmd.OutOrStdout(), "(no crds/ directory)")
				return nil
			}

			files := make([]string, 0)
			walkErr := filepath.Walk(crdDir, func(path string, fi os.FileInfo, e error) error {
				if nil != e {
					return e
				}
				// Reject symlinks: they could exfiltrate arbitrary host files.
				lstat, lstatErr := os.Lstat(path)
				if nil == lstatErr && 0 != lstat.Mode()&os.ModeSymlink {
					return nil
				}
				if fi.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ".yaml" == ext || ".yml" == ext {
					files = append(files, path)
				}
				return nil
			})
			if nil != walkErr {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "failed to walk crds directory", walkErr)
			}
			sort.Strings(files)
			for i, path := range files {
				if 0 < i {
					fmt.Fprintln(cmd.OutOrStdout(), "---")
				}
				data, readErr := os.ReadFile(path)
				if nil != readErr {
					return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, readErr, "failed to read CRD file %s", path)
				}
				fmt.Fprint(cmd.OutOrStdout(), string(data))
			}
			return nil
		},
	}
}

func newShowAllCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "all <package-path>",
		Short: "Show chart metadata, values, and README in one document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, metaErr := pkg.LoadPackageMetadata(args[0])
			if nil != metaErr {
				return metaErr
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "# Chart")
			metaBytes, marshalErr := yaml.Marshal(meta)
			if nil != marshalErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "failed to marshal package metadata", marshalErr)
			}
			fmt.Fprint(out, string(metaBytes))

			fmt.Fprintln(out, "\n# Values")
			if vals, err := readPackageFile(args[0], "values.yaml"); nil == err {
				fmt.Fprint(out, vals)
			} else if !os.IsNotExist(err) {
				return err
			}

			for _, name := range []string{"README.md", "README.txt", "README"} {
				if data, err := readPackageFile(args[0], name); nil == err {
					fmt.Fprintln(out, "\n# README")
					fmt.Fprint(out, data)
					break
				}
			}
			return nil
		},
	}
}

func readPackageFile(packagePath, name string) (string, error) {
	// Accept either a directory (read packagePath/name) or a keramos archive
	// (.keramos.tgz / .tgz / .tar.gz) — read the file from inside the tarball.
	if info, err := os.Stat(packagePath); nil == err && !info.IsDir() {
		if isKeramosArchive(packagePath) {
			return readFromArchive(packagePath, name)
		}
	}
	full := filepath.Join(packagePath, name)
	data, err := os.ReadFile(full)
	if nil != err {
		if os.IsNotExist(err) {
			return "", err
		}
		return "", keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "failed to read %s", name)
	}
	return string(data), nil
}

func isKeramosArchive(p string) bool {
	lower := strings.ToLower(p)
	return strings.HasSuffix(lower, ".keramos.tgz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.gz")
}

func readFromArchive(archivePath, name string) (string, error) {
	f, err := os.Open(archivePath)
	if nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "open archive %s", archivePath)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrCLIValidation, "decompress archive", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if nil != err {
			break
		}
		// Tar entries are typically prefixed with the package directory:
		// e.g. nginx-web/keramos.yaml. Match either bare name or any
		// single-directory-prefixed form.
		base := filepath.Base(hdr.Name)
		stripped := hdr.Name
		if idx := strings.Index(stripped, "/"); idx >= 0 {
			stripped = stripped[idx+1:]
		}
		if base == name || stripped == name || hdr.Name == name {
			// Cap declared entry size against a sane upper bound (16 MiB)
			// before allocating: a hostile archive could declare a giant
			// hdr.Size to OOM the renderer. Use io.LimitReader so the
			// actual transferred bytes are also bounded even if the size
			// field is honest but the gzip stream is a bomb.
			const maxEntrySize = 16 * 1024 * 1024
			if hdr.Size > maxEntrySize {
				return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"archive entry %q size %d exceeds %d bytes", name, hdr.Size, maxEntrySize)
			}
			data, rErr := io.ReadAll(io.LimitReader(tr, maxEntrySize+1))
			if nil != rErr {
				return "", keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, rErr, "read %s from archive", name)
			}
			if int64(len(data)) > maxEntrySize {
				return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"archive entry %q exceeded %d-byte cap during read", name, maxEntrySize)
			}
			return string(data), nil
		}
	}
	return "", os.ErrNotExist
}
