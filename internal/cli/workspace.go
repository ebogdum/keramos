package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	"github.com/ebogdum/keramos/v2/internal/diff"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/release"
	"github.com/ebogdum/keramos/v2/internal/workspace"
	"github.com/spf13/cobra"
)

// workspaceResult records one member's outcome.
type workspaceResult struct {
	member  workspace.Member
	err     error
	started time.Time
	ended   time.Time
}

// workspaceRunOpts is shared across install/upgrade/uninstall.
type workspaceRunOpts struct {
	dir               string
	parallel          int
	continueOnError   bool
	atomic            bool
	dryRun            bool
	healthGate        bool
	progress          bool
	healthGateTimeout time.Duration
}

func (o *workspaceRunOpts) bindFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.dir, "dir", ".", "directory containing keramos-workspace.yaml")
	cmd.Flags().IntVar(&o.parallel, "parallel", 1, "max members to process concurrently within a topological level (1 = sequential)")
	cmd.Flags().BoolVar(&o.continueOnError, "continue-on-error", false, "keep processing remaining members past a failed one; report all failures at the end")
	cmd.Flags().BoolVar(&o.atomic, "atomic-workspace", false, "if any member fails, roll back every successful one (mutually exclusive with --continue-on-error)")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "render every member with client-side dry-run; do not apply to the cluster")
	cmd.Flags().BoolVar(&o.healthGate, "health-gate", false, "between levels, wait for ALL pods of every member in the level to be Ready (not just --wait)")
	cmd.Flags().DurationVar(&o.healthGateTimeout, "health-gate-timeout", 5*time.Minute, "per-level health-gate wait")
	cmd.Flags().BoolVar(&o.progress, "progress", false, "print live progress lines as members complete")
}

func newWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Orchestrate multiple keramos packages declared in keramos-workspace.yaml",
		Long: `A workspace declares a set of related keramos packages with optional
inter-package dependencies.

  keramos workspace plan
  keramos workspace install --parallel 4 --health-gate
  keramos workspace upgrade --continue-on-error
  keramos workspace uninstall

Members at the same topological depth (no dependency between each other)
are processed concurrently up to --parallel. Members at level N+1 wait for
every member of level N to finish. With --health-gate the wait extends until
every pod owned by level N is Ready, so level N+1 can rely on its
dependencies actually serving traffic, not merely "applied".`,
	}
	cmd.AddCommand(newWorkspacePlanCommand())
	cmd.AddCommand(newWorkspaceInstallCommand())
	cmd.AddCommand(newWorkspaceUpgradeCommand())
	cmd.AddCommand(newWorkspaceUninstallCommand())
	cmd.AddCommand(newWorkspaceDiffCommand())
	cmd.AddCommand(newWorkspaceStatusCommand())
	return cmd
}

func newWorkspacePlanCommand() *cobra.Command {
	var dir string
	var levels bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print the topological install order of the workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Load(absOrDot(dir))
			if nil != err {
				return err
			}
			if levels {
				lvls, lErr := ws.Levels()
				if nil != lErr {
					return lErr
				}
				for i, lv := range lvls {
					fmt.Fprintf(cmd.OutOrStdout(), "level %d (%d members, parallelisable):\n", i, len(lv))
					for _, m := range lv {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s (path=%s, ns=%s)\n",
							m.Name, m.Path, m.Namespace)
					}
				}
				return nil
			}
			order, err := ws.TopologicalOrder()
			if nil != err {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), workspace.FormatPlan(order))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "directory containing keramos-workspace.yaml")
	cmd.Flags().BoolVar(&levels, "levels", false, "group output by topological depth (parallelisable groups)")
	return cmd
}

func newWorkspaceInstallCommand() *cobra.Command {
	o := &workspaceRunOpts{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install every workspace member in topological order (parallel-where-safe)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceParallel(cmd, "install", o)
		},
	}
	o.bindFlags(cmd)
	return cmd
}

func newWorkspaceUpgradeCommand() *cobra.Command {
	o := &workspaceRunOpts{}
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade every workspace member in topological order (install if missing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceParallel(cmd, "upgrade", o)
		},
	}
	o.bindFlags(cmd)
	return cmd
}

func newWorkspaceUninstallCommand() *cobra.Command {
	o := &workspaceRunOpts{}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall every workspace member in REVERSE topological order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspaceParallel(cmd, "uninstall", o)
		},
	}
	o.bindFlags(cmd)
	return cmd
}

func newWorkspaceDiffCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show what would change if every workspace member were upgraded",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Load(absOrDot(dir))
			if nil != err {
				return err
			}
			order, err := ws.TopologicalOrder()
			if nil != err {
				return err
			}
			workspaceDir := absOrDot(dir)
			w := cmd.OutOrStdout()
			for _, m := range order {
				fmt.Fprintf(w, "\n=== %s (ns=%s) ===\n", m.Name, m.Namespace)
				rel, rErr := action.Install(nil, m.PackagePath(workspaceDir), &action.InstallOptions{
					ReleaseName: m.Name,
					Namespace:   m.Namespace,
					Profile:     m.Profile,
					DryRun:      "client",
				})
				if nil != rErr {
					fmt.Fprintf(w, "  render error: %v\n", rErr)
					continue
				}
				// Compare the render against the member's stored state, if any.
				base := ""
				if client, kErr := kube.NewClient(kubeconfig, kubeContext, m.Namespace); nil == kErr {
					storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
					if cur, sErr := storage.Last(m.Name); nil == sErr {
						base = cur.Manifest
					}
				}
				changes, dErr := diff.Compute(base, rel.Manifest, allShownFilters())
				if nil != dErr {
					fmt.Fprintf(w, "  diff error: %v\n", dErr)
					continue
				}
				if 0 == len(changes) {
					fmt.Fprintln(w, "  no changes")
					continue
				}
				fmt.Fprint(w, formatPlanChanges(changes, false, nil, "state"))
				fmt.Fprint(w, changeSummary(changes))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "directory containing keramos-workspace.yaml")
	return cmd
}

func newWorkspaceStatusCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current revision and status of every declared member",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Load(absOrDot(dir))
			if nil != err {
				return err
			}
			order, err := ws.TopologicalOrder()
			if nil != err {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-20s %-10s %-12s\n", "MEMBER", "NAMESPACE", "REVISION", "STATUS")
			for _, m := range order {
				client, kErr := kube.NewClient(kubeconfig, kubeContext, m.Namespace)
				if nil != kErr {
					fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-20s ?          (client error)\n", m.Name, m.Namespace)
					continue
				}
				storage := release.NewSecretStorage(client.Clientset(), m.Namespace)
				rel, lookErr := storage.Last(m.Name)
				if nil != lookErr || nil == rel {
					fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-20s -          not deployed\n", m.Name, m.Namespace)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-20s %-10d %-12s\n",
					m.Name, m.Namespace, rel.Revision, rel.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "directory containing keramos-workspace.yaml")
	return cmd
}

// runWorkspaceParallel is the level-by-level executor used by install,
// upgrade, and uninstall. Members within a level are processed concurrently
// up to opts.parallel. Levels are processed strictly serially.
func runWorkspaceParallel(cmd *cobra.Command, op string, opts *workspaceRunOpts) error {
	if opts.atomic && opts.continueOnError {
		return keramoserr.NewError(keramoserr.ErrCLIValidation,
			"--atomic-workspace and --continue-on-error are mutually exclusive")
	}
	if 1 > opts.parallel {
		opts.parallel = 1
	}
	ws, err := workspace.Load(absOrDot(opts.dir))
	if nil != err {
		return err
	}
	levels, err := ws.Levels()
	if nil != err {
		return err
	}
	if "uninstall" == op {
		// Reverse: start from level N (most-dependent), go down to level 0.
		for i, j := 0, len(levels)-1; i < j; i, j = i+1, j-1 {
			levels[i], levels[j] = levels[j], levels[i]
		}
	}
	workspaceDir := absOrDot(opts.dir)

	completed := make([]workspaceResult, 0)
	var compMu sync.Mutex
	var failures int

	totalMembers := 0
	for _, lv := range levels {
		totalMembers += len(lv)
	}
	progress := func(format string, args ...any) {
		if opts.progress {
			fmt.Fprintf(cmd.OutOrStdout(), format, args...)
		}
	}

	progress("workspace: %d members across %d level(s), parallel=%d, op=%s\n",
		totalMembers, len(levels), opts.parallel, op)

	for li, level := range levels {
		progress("\n[level %d/%d] %d member(s) starting concurrently\n",
			li, len(levels)-1, len(level))
		sem := make(chan struct{}, opts.parallel)
		var wg sync.WaitGroup
		for _, m := range level {
			m := m
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				started := time.Now()
				progress("  → %s (ns=%s) start\n", m.Name, m.Namespace)
				rerr := runWorkspaceMember(op, m, workspaceDir, opts)
				ended := time.Now()
				compMu.Lock()
				completed = append(completed, workspaceResult{member: m, err: rerr, started: started, ended: ended})
				if nil != rerr {
					failures++
				}
				compMu.Unlock()
				if nil != rerr {
					progress("  ✗ %s failed in %s: %v\n", m.Name, ended.Sub(started).Round(time.Millisecond), rerr)
				} else {
					progress("  ✓ %s done in %s\n", m.Name, ended.Sub(started).Round(time.Millisecond))
				}
			}()
		}
		wg.Wait()

		// Health gate before advancing to the next level.
		if opts.healthGate && "uninstall" != op {
			progress("[level %d] health-gate: waiting for member pods to be Ready\n", li)
			if hgErr := waitLevelHealthy(level, opts.healthGateTimeout); nil != hgErr {
				progress("  health-gate failed: %v\n", hgErr)
				if !opts.continueOnError {
					return keramoserr.WrapErrorf(keramoserr.ErrKube, hgErr,
						"health-gate at level %d", li)
				}
			}
		}

		// Atomic / fail-fast handling.
		if 0 < failures && !opts.continueOnError {
			if opts.atomic {
				progress("\n[ATOMIC] failure detected — rolling back every successful install\n")
				rollbackWorkspace(completed, op)
			}
			return collectErrors(completed)
		}
	}

	// Summary.
	if 0 < failures {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d member(s) failed:\n", failures)
		for _, r := range completed {
			if nil != r.err {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %v\n", r.member.Name, r.err)
			}
		}
		return collectErrors(completed)
	}
	if opts.progress {
		fmt.Fprintf(cmd.OutOrStdout(), "\nAll %d member(s) succeeded.\n", totalMembers)
	}
	return nil
}

func runWorkspaceMember(op string, m workspace.Member, workspaceDir string, opts *workspaceRunOpts) error {
	client, err := kube.NewClient(kubeconfig, kubeContext, m.Namespace)
	if nil != err {
		return err
	}
	timeout := 5 * time.Minute
	wait := m.MemberWait()
	dryRun := ""
	if opts.dryRun {
		dryRun = "client"
	}
	pkgPath := m.PackagePath(workspaceDir)
	switch op {
	case "install":
		_, ierr := action.Install(client, pkgPath, &action.InstallOptions{
			ReleaseName: m.Name, Namespace: m.Namespace,
			Profile: m.Profile,
			Atomic:  m.MemberAtomic(),
			Wait:    wait, Timeout: timeout,
			DryRun: dryRun,
		})
		return ierr
	case "upgrade":
		_, uerr := action.Upgrade(client, pkgPath, &action.UpgradeOptions{
			ReleaseName: m.Name, Namespace: m.Namespace,
			Profile: m.Profile,
			Install: true, Atomic: m.MemberAtomic(),
			Wait: wait, Timeout: timeout,
			DryRun: dryRun,
		})
		return uerr
	case "uninstall":
		if opts.dryRun {
			return nil
		}
		_, derr := action.Uninstall(client, &action.UninstallOptions{
			ReleaseName:    m.Name,
			Namespace:      m.Namespace,
			IgnoreNotFound: true,
			Timeout:        timeout,
		})
		return derr
	}
	return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "unknown op %q", op)
}

func waitLevelHealthy(level []workspace.Member, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for _, m := range level {
		client, err := kube.NewClient(kubeconfig, kubeContext, m.Namespace)
		if nil != err {
			return err
		}
		storage := release.NewSecretStorage(client.Clientset(), m.Namespace)
		rel, gerr := storage.Last(m.Name)
		if nil != gerr || nil == rel {
			continue // member not installed (e.g. dry-run)
		}
		if remain := time.Until(deadline); remain > 0 {
			if werr := client.WaitForReady(rel.Manifest, remain); nil != werr {
				return keramoserr.WrapErrorf(keramoserr.ErrKube, werr,
					"member %q not Ready", m.Name)
			}
		}
	}
	return nil
}

func rollbackWorkspace(completed []workspaceResult, op string) {
	// Iterate the successes in reverse order and uninstall each.
	type entry struct {
		name, ns string
	}
	successes := make([]entry, 0)
	for _, r := range completed {
		if nil == r.err {
			successes = append(successes, entry{name: r.member.Name, ns: r.member.Namespace})
		}
	}
	// Reverse-order rollback so deepest-installed comes off first.
	for i := len(successes) - 1; i >= 0; i-- {
		s := successes[i]
		client, err := kube.NewClient(kubeconfig, kubeContext, s.ns)
		if nil != err {
			logger.Warn("atomic rollback: %s/%s: %v", s.ns, s.name, err)
			continue
		}
		if _, derr := action.Uninstall(client, &action.UninstallOptions{
			ReleaseName: s.name, Namespace: s.ns, IgnoreNotFound: true,
		}); nil != derr {
			logger.Warn("atomic rollback uninstall %s/%s: %v", s.ns, s.name, derr)
		}
	}
}

// collectErrors aggregates per-member errors into one.
func collectErrors(results []workspaceResult) error {
	failed := make([]string, 0)
	for _, r := range results {
		if nil != r.err {
			failed = append(failed, fmt.Sprintf("%s: %v", r.member.Name, r.err))
		}
	}
	if 0 == len(failed) {
		return nil
	}
	sort.Strings(failed)
	return keramoserr.NewErrorf(keramoserr.ErrInternal,
		"workspace had %d failure(s): %v", len(failed), failed)
}

func absOrDot(dir string) string {
	if "" == dir {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if nil != err {
		return dir
	}
	return abs
}
