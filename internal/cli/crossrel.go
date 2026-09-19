package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/release"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// crossReleaseSpec defines keramos-releases.yaml: a list of releases with
// optional cross-release dependencies. Each entry maps to a single
// install/upgrade. Order is determined by dependsOn.
type crossReleaseSpec struct {
	Releases []crossReleaseEntry `yaml:"releases"`
}

type crossReleaseEntry struct {
	Name      string   `yaml:"name"`
	Package   string   `yaml:"package"`
	Namespace string   `yaml:"namespace,omitempty"`
	Profile   string   `yaml:"profile,omitempty"`
	Values    []string `yaml:"values,omitempty"`
	Set       []string `yaml:"set,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
}

// newReleasesCommand exposes `keramos releases plan|apply|status` for managing
// a graph of releases declared in keramos-releases.yaml.
func newReleasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "Manage cross-release dependencies declared in keramos-releases.yaml",
	}
	cmd.AddCommand(newReleasesPlanCommand())
	cmd.AddCommand(newReleasesStatusCommand())
	cmd.AddCommand(newReleasesInstallCommand())
	cmd.AddCommand(newReleasesUpgradeCommand())
	cmd.AddCommand(newReleasesUninstallCommand())
	return cmd
}

func newReleasesInstallCommand() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install every release in topological order",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleasesAction(cmd, path, "install")
		},
	}
	cmd.Flags().StringVar(&path, "file", "keramos-releases.yaml", "spec file path")
	return cmd
}

func newReleasesUpgradeCommand() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade every release in topological order (install if missing)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleasesAction(cmd, path, "upgrade")
		},
	}
	cmd.Flags().StringVar(&path, "file", "keramos-releases.yaml", "spec file path")
	return cmd
}

func newReleasesUninstallCommand() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall every release in reverse topological order",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleasesAction(cmd, path, "uninstall")
		},
	}
	cmd.Flags().StringVar(&path, "file", "keramos-releases.yaml", "spec file path")
	return cmd
}

func runReleasesAction(cmd *cobra.Command, path, action_ string) error {
	spec, err := loadCrossSpec(path)
	if nil != err {
		return err
	}
	order, sortErr := topoSortReleases(spec.Releases)
	if nil != sortErr {
		return sortErr
	}
	if "uninstall" == action_ {
		// reverse for uninstall
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	for _, e := range order {
		ns := e.Namespace
		if "" == ns {
			ns = namespace
		}
		client, kErr := kube.NewClient(kubeconfig, kubeContext, ns)
		if nil != kErr {
			return kErr
		}
		switch action_ {
		case "install":
			rel, iErr := action.Install(client, e.Package, &action.InstallOptions{
				ReleaseName: e.Name, Namespace: ns,
				ValueFiles: e.Values, Sets: e.Set,
				Profile: e.Profile,
				Atomic:  true, Wait: false, Timeout: 5 * time.Minute,
			})
			if nil != iErr {
				return fmt.Errorf("release %q install: %w", e.Name, iErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] installed (revision %d, ns %s)\n", e.Name, rel.Revision, rel.Namespace)
		case "upgrade":
			rel, uErr := action.Upgrade(client, e.Package, &action.UpgradeOptions{
				ReleaseName: e.Name, Namespace: ns,
				ValueFiles: e.Values, Sets: e.Set,
				Profile: e.Profile, Install: true,
				Atomic: true, Wait: false, Timeout: 5 * time.Minute,
			})
			if nil != uErr {
				return fmt.Errorf("release %q upgrade: %w", e.Name, uErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] upgraded (revision %d)\n", e.Name, rel.Revision)
		case "uninstall":
			_, dErr := action.Uninstall(client, &action.UninstallOptions{
				ReleaseName: e.Name, Namespace: ns,
				IgnoreNotFound: true, Timeout: 2 * time.Minute,
			})
			if nil != dErr {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] uninstall reported error: %v\n", e.Name, dErr)
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] uninstalled\n", e.Name)
		}
	}
	return nil
}

func newReleasesPlanCommand() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print the topological order in which releases should be applied",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := loadCrossSpec(path)
			if nil != err {
				return err
			}
			order, sortErr := topoSortReleases(spec.Releases)
			if nil != sortErr {
				return sortErr
			}
			for i, e := range order {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. %s (%s) ns=%s\n", i+1, e.Name, e.Package, e.Namespace)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "file", "keramos-releases.yaml", "spec file path")
	return cmd
}

func newReleasesStatusCommand() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current revision and status of every declared release",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := loadCrossSpec(path)
			if nil != err {
				return err
			}
			storage, sErr := storageFor()
			if nil != sErr {
				return sErr
			}
			for _, e := range spec.Releases {
				latest, lerr := storage.Last(e.Name)
				if nil != lerr {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: not deployed (%v)\n", e.Name, lerr)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: revision %d status=%s\n", e.Name, latest.Revision, latest.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "file", "keramos-releases.yaml", "spec file path")
	return cmd
}

func loadCrossSpec(path string) (*crossReleaseSpec, error) {
	data, err := os.ReadFile(path)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "read "+path, err)
	}
	var spec crossReleaseSpec
	if yErr := yaml.Unmarshal(data, &spec); nil != yErr {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse "+path, yErr)
	}
	return &spec, nil
}

// topoSortReleases performs Kahn's algorithm with cycle detection. Names
// that don't reference each other keep their declared order via stable sort.
func topoSortReleases(in []crossReleaseEntry) ([]crossReleaseEntry, error) {
	byName := make(map[string]crossReleaseEntry, len(in))
	indeg := make(map[string]int, len(in))
	for _, e := range in {
		if "" == e.Name {
			return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "release entry missing name")
		}
		byName[e.Name] = e
		indeg[e.Name] = 0
	}
	for _, e := range in {
		// Deduplicate dependsOn so duplicate entries don't inflate indegree
		// past the number of decrements in the topological loop below.
		seen := make(map[string]bool, len(e.DependsOn))
		for _, d := range e.DependsOn {
			if _, ok := byName[d]; !ok {
				return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"%s depends on unknown release %s", e.Name, d)
			}
			if seen[d] {
				continue
			}
			seen[d] = true
			indeg[e.Name]++
		}
	}
	queue := make([]string, 0, len(in))
	for _, e := range in {
		if 0 == indeg[e.Name] {
			queue = append(queue, e.Name)
		}
	}
	sort.Strings(queue)
	out := make([]crossReleaseEntry, 0, len(in))
	processed := 0
	for 0 < len(queue) {
		name := queue[0]
		queue = queue[1:]
		out = append(out, byName[name])
		processed++
		// Children: anyone depending on `name`.
		for _, e := range in {
			depended := false
			for _, d := range e.DependsOn {
				if d == name {
					depended = true
					break
				}
			}
			if !depended {
				continue
			}
			indeg[e.Name]--
			if 0 == indeg[e.Name] {
				queue = append(queue, e.Name)
				sort.Strings(queue)
			}
		}
	}
	if processed != len(in) {
		var stuck []string
		for n, d := range indeg {
			if 0 < d {
				stuck = append(stuck, n)
			}
		}
		sort.Strings(stuck)
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"cycle in releases involving: %s", strings.Join(stuck, ", "))
	}
	return out, nil
}

// _ keeps the release import used (status references release.StatusDeployed indirectly via storage).
var _ = release.StatusDeployed
