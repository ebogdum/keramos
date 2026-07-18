package cli

import (
	"fmt"
	"sort"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/spf13/cobra"
)

// newRenameCommand renames a release. Implementation: copy every revision of
// `<old>` to `<new>` (rewriting `Name`), then delete the old revisions. Done
// in two phases so partial failure leaves the old release intact.
func newRenameCommand() *cobra.Command {
	var keepOld bool
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename an existing release (copy revisions, delete originals)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			if oldName == newName {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "old and new names are identical")
			}
			storage, err := storageFor()
			if nil != err {
				return err
			}
			existingHist, _ := storage.History(newName)
			if 0 < len(existingHist) {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"release %s already has %d revision(s); refusing to overwrite",
					newName, len(existingHist))
			}
			history, err := storage.History(oldName)
			if nil != err {
				return err
			}
			if 0 == len(history) {
				return keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", oldName)
			}
			sort.Slice(history, func(i, j int) bool { return history[i].Revision < history[j].Revision })
			// Track copies so a partial failure can roll the new release back
			// to a clean slate rather than leaving orphaned revisions.
			copied := make([]int, 0, len(history))
			for _, rev := range history {
				clone := *rev
				clone.Name = newName
				if cErr := storage.Create(&clone); nil != cErr {
					for _, r := range copied {
						_ = storage.Delete(newName, r)
					}
					return keramoserr.WrapErrorf(keramoserr.ErrRelease, cErr,
						"copy %s revision %d to %s (rolled back %d already-copied revisions)",
						oldName, rev.Revision, newName, len(copied))
				}
				copied = append(copied, rev.Revision)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "copied %d revisions from %s to %s\n", len(history), oldName, newName)
			if keepOld {
				return nil
			}
			for _, rev := range history {
				if dErr := storage.Delete(oldName, rev.Revision); nil != dErr {
					return keramoserr.WrapErrorf(keramoserr.ErrRelease, dErr,
						"delete %s revision %d after rename (new release at %s is intact)",
						oldName, rev.Revision, newName)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %d revisions of %s\n", len(history), oldName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&keepOld, "keep-old", false, "leave the old release revisions in place after copying")
	return cmd
}

// newPruneCommand removes superseded release revisions, retaining the latest
// `--keep` plus any revision currently the deployed one. This implements
// sealed-revision pruning so storage doesn't grow unbounded.
func newPruneCommand() *cobra.Command {
	var (
		keep     int
		release_ string
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune superseded release revisions, keeping only the most recent N",
		RunE: func(cmd *cobra.Command, args []string) error {
			if 1 > keep {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "--keep must be >= 1")
			}
			storage, err := storageFor()
			if nil != err {
				return err
			}
			names := []string{release_}
			if "" == release_ {
				releases, lErr := storage.List(namespace)
				if nil != lErr {
					return lErr
				}
				seen := map[string]struct{}{}
				names = names[:0]
				for _, r := range releases {
					if _, ok := seen[r.Name]; ok {
						continue
					}
					seen[r.Name] = struct{}{}
					names = append(names, r.Name)
				}
			}
			pruned := 0
			for _, n := range names {
				history, hErr := storage.History(n)
				if nil != hErr {
					return hErr
				}
				sort.Slice(history, func(i, j int) bool { return history[i].Revision > history[j].Revision })
				deployedRev := -1
				for _, r := range history {
					if release.StatusDeployed == r.Status {
						deployedRev = r.Revision
						break
					}
				}
				kept := 0
				for _, r := range history {
					if kept < keep || r.Revision == deployedRev {
						kept++
						continue
					}
					if dryRun {
						fmt.Fprintf(cmd.OutOrStdout(), "would prune %s revision %d (status=%s)\n", n, r.Revision, r.Status)
						pruned++
						continue
					}
					if dErr := storage.Delete(n, r.Revision); nil != dErr {
						return keramoserr.WrapErrorf(keramoserr.ErrRelease, dErr,
							"prune %s revision %d", n, r.Revision)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "pruned %s revision %d\n", n, r.Revision)
					pruned++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d revision(s) pruned\n", pruned)
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 10, "number of recent revisions to retain per release")
	cmd.Flags().StringVar(&release_, "release", "", "prune a single release; empty means every release in the namespace")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list revisions that would be deleted without deleting them")
	return cmd
}

// storageFor opens release storage using the global namespace + kube flags.
func storageFor() (release.Storage, error) {
	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return nil, err
	}
	return release.SelectStorage(client.Clientset(), namespace)
}
