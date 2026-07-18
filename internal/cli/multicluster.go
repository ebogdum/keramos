package cli

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/spf13/cobra"
)

func newMultiInstallCommand() *cobra.Command {
	var (
		toContexts []string
		valueFiles []string
		sets       []string
		setStrings []string
		setFiles   []string
		setJSON    []string
		profile    string
		envName    string
		atomic     bool
		timeout    time.Duration
		parallel   int
		noWait     bool
	)
	cmd := &cobra.Command{
		Use:   "multi-install <release-name> <package-path>",
		Short: "Install a release into multiple clusters in one invocation",
		Long: `Install the same package into N clusters via N kubeconfig contexts. With
--atomic-cross-cluster, keramos verifies all installs succeed and rolls back any
that did not; otherwise installs are independent (eventual consistency).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if 0 == len(toContexts) {
				return fmt.Errorf("--to is required: comma-separated kubeconfig contexts")
			}
			if 1 > parallel {
				parallel = 1
			}
			if 0 == timeout {
				timeout = 5 * time.Minute
			}
			results := runMultiInstall(args[0], args[1], toContexts, valueFiles, sets, setStrings, setFiles, setJSON, profile, envName, parallel, atomic, timeout, !noWait)

			rows := make([][]string, 0, len(results))
			anyErr := false
			for _, r := range results {
				st := "ok"
				detail := ""
				if nil != r.Err {
					st = "FAIL"
					detail = r.Err.Error()
					anyErr = true
				} else if nil != r.Release {
					detail = fmt.Sprintf("revision %d, status %s", r.Release.Revision, r.Release.Status)
				}
				rows = append(rows, []string{r.Context, st, detail})
			}
			fmt.Fprint(cmd.OutOrStdout(), FormatTable([]string{"CONTEXT", "STATUS", "DETAIL"}, rows))

			if anyErr && atomic {
				fmt.Fprintln(cmd.OutOrStdout(), "Atomic cross-cluster install failed — rolling back successful installs.")
				rollbackMulti(args[0], results)
				return fmt.Errorf("cross-cluster install failed in at least one context")
			}
			if anyErr {
				return fmt.Errorf("cross-cluster install failed in at least one context (non-atomic)")
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&toContexts, "to", nil, "kubeconfig contexts (comma-separated)")
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "--set overrides")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "--set forcing string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "--set key=path; value read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "--set key=<json> (repeatable)")
	cmd.Flags().StringVar(&envName, "env", "", "environment name declared in keramos.yaml's environments: section")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "do not wait for resources to be ready in any cluster")
	cmd.Flags().StringVar(&profile, "profile", "", "profile to apply")
	cmd.Flags().BoolVar(&atomic, "atomic-cross-cluster", false, "if any cluster fails, roll back the rest")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "per-cluster wait timeout")
	cmd.Flags().IntVar(&parallel, "parallel", 4, "concurrent installs across clusters")
	return cmd
}

type multiResult struct {
	Context string
	Release *release.Release
	Err     error
}

func runMultiInstall(name, pkg string, contexts, valueFiles, sets, setStrings, setFiles, setJSON []string, profile, envName string, parallel int, _ bool, timeout time.Duration, wait bool) []multiResult {
	out := make([]multiResult, len(contexts))
	work := make(chan int, len(contexts))
	for i := range contexts {
		work <- i
	}
	close(work)
	var wg sync.WaitGroup
	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				ctx := contexts[i]
				client, err := kube.NewClient(kubeconfig, ctx, namespace)
				if nil != err {
					out[i] = multiResult{Context: ctx, Err: err}
					continue
				}
				rel, err := action.Install(client, pkg, &action.InstallOptions{
					ReleaseName: name,
					Namespace:   namespace,
					ValueFiles:  valueFiles,
					Sets:        sets,
					SetStrings:  setStrings,
					SetFiles:    setFiles,
					SetJSON:     setJSON,
					Profile:     profile,
					Environment: envName,
					Wait:        wait,
					Timeout:     timeout,
					Atomic:      true,
				})
				out[i] = multiResult{Context: ctx, Release: rel, Err: err}
			}
		}()
	}
	wg.Wait()
	return out
}

func rollbackMulti(name string, results []multiResult) {
	var wg sync.WaitGroup
	for _, r := range results {
		if nil != r.Err || nil == r.Release {
			continue
		}
		wg.Add(1)
		go func(ctx string) {
			defer wg.Done()
			client, err := kube.NewClient(kubeconfig, ctx, namespace)
			if nil != err {
				logger.Warn("rollback: cannot reach cluster %q to uninstall %q: %v", ctx, name, err)
				return
			}
			if _, uErr := action.Uninstall(client, &action.UninstallOptions{
				ReleaseName: name,
				Namespace:   namespace,
				KeepHistory: false,
			}); nil != uErr {
				logger.Warn("rollback: uninstall of %q on cluster %q failed (manual cleanup may be needed): %v", name, ctx, uErr)
			}
		}(r.Context)
	}
	wg.Wait()
}
