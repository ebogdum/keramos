package cli

import (
	"fmt"
	"strings"

	"github.com/ebogdum/keramos/v3/internal/deptree"
	"github.com/ebogdum/keramos/v3/internal/layer"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"github.com/ebogdum/keramos/v3/internal/repo"
	"github.com/spf13/cobra"
)

func newDependencyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dependency",
		Aliases: []string{"dep"},
		Short:   "Manage package layers and dependencies",
		Long:    "List, update, and build layers and required packages declared in keramos.yaml.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newDependencyListCommand())
	cmd.AddCommand(newDependencyUpdateCommand())
	cmd.AddCommand(newDependencyBuildCommand())
	cmd.AddCommand(newDependencyTreeCommand())

	return cmd
}

func newDependencyListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <package-path>",
		Short: "List layers and required packages with their status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packagePath := args[0]
			return listLayersAndRequires(cmd, packagePath)
		},
	}
}

func listLayersAndRequires(cmd *cobra.Command, packagePath string) error {
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return err
	}

	layers := meta.EffectiveLayers()
	requires := meta.Requires

	if 0 == len(layers) && 0 == len(requires) {
		fmt.Fprintln(cmd.OutOrStdout(), "No layers or requires declared.")
		return nil
	}

	lf, _ := repo.LoadLockFile(packagePath)

	if 0 < len(layers) {
		fmt.Fprintln(cmd.OutOrStdout(), "LAYERS:")
		fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-12s %-40s %s\n",
			"NAME", "TYPE", "SOURCE", "STATUS")
		for _, ls := range layers {
			srcType, _, _ := repo.ParseSource(ls.Source)
			typeName := sourceTypeName(srcType)
			status := layerStatus(lf, ls, false)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-12s %-40s %s\n",
				ls.Name, typeName, truncate(ls.Source, 40), status)
		}
	}

	if 0 < len(requires) {
		if 0 < len(layers) {
			fmt.Fprintln(cmd.OutOrStdout())
		}
		fmt.Fprintln(cmd.OutOrStdout(), "REQUIRES:")
		fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-12s %-40s %s\n",
			"NAME", "TYPE", "SOURCE", "STATUS")
		for _, req := range requires {
			srcType, _, _ := repo.ParseSource(req.Source)
			typeName := sourceTypeName(srcType)
			status := layerStatus(lf, req, true)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-12s %-40s %s\n",
				req.Name, typeName, truncate(req.Source, 40), status)
		}
	}

	// Legacy dependencies (backward compat)
	if 0 < len(meta.Dependencies) && 0 == len(meta.Layers) {
		deps, listErr := repo.ListDependencies(packagePath)
		if nil != listErr {
			return listErr
		}
		if 0 < len(deps) {
			fmt.Fprintln(cmd.OutOrStdout(), "\nLEGACY DEPENDENCIES:")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-15s %-15s %-10s %-6s %s\n",
				"NAME", "DECLARED", "INSTALLED", "STATUS", "TYPE", "REPOSITORY")
			for _, d := range deps {
				installed := d.Installed
				if "" == installed {
					installed = "-"
				}
				depType := "direct"
				if d.Transitive {
					depType = "trans"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %-15s %-15s %-10s %-6s %s\n",
					d.Name, d.Declared, installed, d.Status, depType, d.Repository)
			}
		}
	}

	return nil
}

func newDependencyUpdateCommand() *cobra.Command {
	var skipRefresh bool
	cmd := &cobra.Command{
		Use:   "update <package-path> [name]",
		Short: "Re-resolve layer and dependency versions",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			packagePath := args[0]
			if skipRefresh {
				logger.Debug("--skip-refresh: skipping repository index refresh before resolution")
			}

			meta, err := pkg.LoadPackageMetadata(packagePath)
			if nil != err {
				return err
			}

			// New layers-based workflow
			layers := meta.EffectiveLayers()
			if 0 < len(layers) {
				if err := updateLayers(packagePath, meta); nil != err {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Layers updated successfully.")
				return nil
			}

			// Legacy dependencies workflow
			if 2 == len(args) {
				depName := args[1]
				if err := repo.UpdateSingleDependency(packagePath, depName); nil != err {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dependency %s updated successfully.\n", depName)
				return nil
			}

			if err := repo.UpdateDependencies(packagePath); nil != err {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Dependencies updated successfully.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipRefresh, "skip-refresh", false, "skip repository index refresh before resolving")
	return cmd
}

func updateLayers(packagePath string, meta pkg.PackageMetadata) error {
	layers := meta.EffectiveLayers()
	lockedLayers := make([]repo.LockedLayer, 0, len(layers))

	for _, ls := range layers {
		locked, err := resolveLayerLock(ls)
		if nil != err {
			return err
		}
		lockedLayers = append(lockedLayers, locked)
	}

	lockedRequires := make([]repo.LockedLayer, 0, len(meta.Requires))
	for _, req := range meta.Requires {
		locked, err := resolveLayerLock(req)
		if nil != err {
			return err
		}
		lockedRequires = append(lockedRequires, locked)
	}

	lf := repo.BuildLayerLockFile(lockedLayers, lockedRequires)
	return repo.SaveLockFile(lf, packagePath)
}

func resolveLayerLock(ls pkg.LayerSource) (repo.LockedLayer, error) {
	locked := repo.LockedLayer{
		Name:   ls.Name,
		Source: ls.Source,
		Ref:    ls.Ref,
	}

	srcType, _, _ := repo.ParseSource(ls.Source)
	switch srcType {
	case repo.SourceGit:
		cacheDir := ""
		localPath, err := repo.FetchGitSource(extractGitURL(ls.Source), ls.Ref, extractGitSubdir(ls.Source), cacheDir)
		if nil != err {
			return locked, err
		}
		commit, err := repo.GitResolveCommit(localPath)
		if nil != err {
			return locked, err
		}
		locked.ResolvedCommit = commit
	case repo.SourceRegistry:
		locked.ResolvedVersion = ls.Version
	}

	return locked, nil
}

func extractGitURL(source string) string {
	_, url, _ := repo.ParseSource(source)
	return url
}

func extractGitSubdir(source string) string {
	_, _, subdir := repo.ParseSource(source)
	return subdir
}

func newDependencyBuildCommand() *cobra.Command {
	var noCache bool
	var verify bool

	cmd := &cobra.Command{
		Use:   "build <package-path>",
		Short: "Resolve and download all layers and dependencies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packagePath := args[0]

			if noCache {
				cache, cacheErr := repo.NewIndexCache()
				if nil == cacheErr {
					if invalidateErr := cache.InvalidateAll(); nil != invalidateErr {
						return invalidateErr
					}
				}
			}

			if err := repo.ResolveDependencies(packagePath); nil != err {
				return err
			}

			if verify {
				if err := verifyInstalledDigests(packagePath); nil != err {
					return err
				}
				if err := verifyInstalledSignatures(packagePath); nil != err {
					return err
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Dependencies resolved successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&noCache, "no-cache", false, "clear index cache before resolving")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify digests of installed dependencies")

	return cmd
}

func newDependencyTreeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tree <package-path>",
		Short: "Display the layer composition chain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packagePath := args[0]
			return printLayerTree(cmd, packagePath)
		},
	}
}

func printLayerTree(cmd *cobra.Command, packagePath string) error {
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return err
	}

	// Use explicit layers/requires if present; otherwise fall back to legacy tree
	if 0 == len(meta.Layers) && 0 == len(meta.Requires) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s@%s\n", meta.Name, meta.Version)
		return printLegacyDependencyTree(cmd, packagePath, meta)
	}

	root, buildErr := layer.BuildTree(packagePath)
	if nil != buildErr {
		return buildErr
	}

	fmt.Fprint(cmd.OutOrStdout(), deptree.PrintTree(root))
	return nil
}

func printLegacyDependencyTree(cmd *cobra.Command, packagePath string, meta pkg.PackageMetadata) error {
	lf, lockErr := repo.LoadLockFile(packagePath)
	if nil != lockErr {
		return lockErr
	}
	if nil == lf {
		return nil
	}

	lockedByName := make(map[string]*repo.LockedDependency, len(lf.Dependencies))
	for i := range lf.Dependencies {
		lockedByName[lf.Dependencies[i].Name] = &lf.Dependencies[i]
	}

	seen := make(map[string]bool, len(lf.Dependencies))
	depCount := len(meta.Dependencies)
	for i, dep := range meta.Dependencies {
		isLast := (i == depCount-1)
		printTreeNode(cmd, dep.Name, lockedByName, seen, "", isLast)
	}

	return nil
}

func printTreeNode(cmd *cobra.Command, name string, locked map[string]*repo.LockedDependency, seen map[string]bool, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	ld, ok := locked[name]
	if !ok {
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s (not resolved)\n", prefix, connector, name)
		return
	}

	if seen[name] {
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s@%s (deduped)\n", prefix, connector, name, ld.Version)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s@%s\n", prefix, connector, name, ld.Version)
	seen[name] = true

	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	childCount := len(ld.Dependencies)
	for i, childName := range ld.Dependencies {
		childIsLast := (i == childCount-1)
		printTreeNode(cmd, childName, locked, seen, childPrefix, childIsLast)
	}
}

func sourceTypeName(st repo.SourceType) string {
	switch st {
	case repo.SourceLocal:
		return "local"
	case repo.SourceGit:
		return "git"
	case repo.SourceRegistry:
		return "registry"
	default:
		return "unknown"
	}
}

func layerStatus(lf *repo.LockFile, ls pkg.LayerSource, isRequire bool) string {
	if nil == lf {
		return "unlocked"
	}

	entries := lf.Layers
	if isRequire {
		entries = lf.Requires
	}

	for _, locked := range entries {
		if locked.Name == ls.Name {
			return "locked"
		}
	}

	return "unlocked"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func verifyInstalledDigests(packagePath string) error {
	lf, err := repo.LoadLockFile(packagePath)
	if nil != err {
		return err
	}
	if nil == lf {
		return nil
	}

	var errs []string
	for _, ld := range lf.Dependencies {
		installed := repo.CheckInstalledVersion(packagePath, ld.Name)
		if "" == installed {
			continue
		}
		if installed != ld.Version {
			errs = append(errs, fmt.Sprintf("%s: installed %s does not match locked %s", ld.Name, installed, ld.Version))
		}
	}

	if 0 < len(errs) {
		return fmt.Errorf("digest verification failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}
