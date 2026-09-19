package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/spf13/cobra"
)

// newDevCommand watches a package directory and re-renders on every change.
// On render failure it prints the error; on success it prints a unified diff
// vs the previous render. This is the inner loop for package authoring.
func newDevCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		profile    string
		interval   time.Duration
	)
	cmd := &cobra.Command{
		Use:   "dev <package-path>",
		Short: "Watch a package and re-render on changes (live development loop)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := args[0]
			if _, err := os.Stat(pkg); nil != err {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "stat package", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			lastSig := ""
			lastOut := ""
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				sig, sErr := dirSignature(pkg)
				if nil != sErr {
					logger.Warn("signature error: %v", sErr)
				}
				if sig != lastSig {
					lastSig = sig
					out, rErr := renderForDev(pkg, valueFiles, sets, profile)
					if nil != rErr {
						fmt.Fprintf(cmd.OutOrStdout(), "\n--- render error ---\n%v\n", rErr)
					} else {
						if "" == lastOut {
							fmt.Fprintln(cmd.OutOrStdout(), "--- initial render ---")
							fmt.Fprint(cmd.OutOrStdout(), out)
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), "--- diff ---")
							fmt.Fprint(cmd.OutOrStdout(), shortDiff(lastOut, out))
						}
						lastOut = out
					}
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	cmd.Flags().DurationVar(&interval, "interval", 500*time.Millisecond, "poll interval")
	return cmd
}

// dirSignature concatenates mtimes + sizes for every regular file under root.
// Cheap, deterministic, no FS-event-watcher dependency.
func dirSignature(root string) (string, error) {
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		if info.IsDir() {
			return nil
		}
		fmt.Fprintf(&b, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return b.String(), err
}

// renderForDev runs a client-side dry-run install to get the manifest.
func renderForDev(pkg string, vf, sets []string, profile string) (string, error) {
	rel, err := action.Install(nil, pkg, &action.InstallOptions{
		ReleaseName: "dev",
		Namespace:   "default",
		ValueFiles:  vf,
		Sets:        sets,
		Profile:     profile,
		DryRun:      "client",
	})
	if nil != err {
		return "", err
	}
	return rel.Manifest, nil
}

// shortDiff returns a unified-ish diff. Avoids pulling in a diff library.
func shortDiff(a, b string) string {
	if a == b {
		return "(no change)\n"
	}
	la := strings.Split(a, "\n")
	lb := strings.Split(b, "\n")
	var out strings.Builder
	max := len(la)
	if len(lb) > max {
		max = len(lb)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(la) {
			av = la[i]
		}
		if i < len(lb) {
			bv = lb[i]
		}
		if av != bv {
			if "" != av {
				fmt.Fprintf(&out, "- %s\n", av)
			}
			if "" != bv {
				fmt.Fprintf(&out, "+ %s\n", bv)
			}
		}
	}
	return out.String()
}
