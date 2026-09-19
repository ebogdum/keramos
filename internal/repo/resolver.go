package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/pkg"
)

// ResolvedDep describes a fully resolved dependency including its transitive children.
type ResolvedDep struct {
	Name         string
	Version      string
	Repository   string
	Digest       string
	Dependencies []string // names of direct dependencies
}

// ResolutionResult holds the flat list of resolved dependencies and any warnings.
type ResolutionResult struct {
	Resolved []ResolvedDep
	Warnings []string
}

// resolverState tracks state during transitive dependency resolution.
type resolverState struct {
	// resolved maps package name to its resolved dependency
	resolved map[string]*ResolvedDep
	// constraints maps package name to all constraints placed on it (by dependent name)
	constraints map[string][]constraintEntry
	// resolving tracks the current resolution stack for cycle detection
	resolving map[string]bool
	// stack tracks the resolution path for error reporting
	stack []string
	// warnings collects non-fatal issues
	warnings []string
}

type constraintEntry struct {
	constraint *semver.Constraints
	raw        string
	from       string // name of the package that declared this constraint
}

// ResolveTree performs transitive dependency resolution using Maximal Version Selection.
// It queues all direct dependencies, fetches their metadata, discovers transitive deps,
// and picks the highest version satisfying all constraints.
func ResolveTree(deps []pkg.Dependency) (*ResolutionResult, error) {
	state := &resolverState{
		resolved:    make(map[string]*ResolvedDep),
		constraints: make(map[string][]constraintEntry),
		resolving:   make(map[string]bool),
		stack:       make([]string, 0, 8),
		warnings:    make([]string, 0),
	}

	for _, dep := range deps {
		if err := resolveTransitive(state, dep, "root"); nil != err {
			return nil, err
		}
	}

	result := &ResolutionResult{
		Resolved: make([]ResolvedDep, 0, len(state.resolved)),
		Warnings: state.warnings,
	}
	for _, rd := range state.resolved {
		result.Resolved = append(result.Resolved, *rd)
	}

	return result, nil
}

func resolveTransitive(state *resolverState, dep pkg.Dependency, requiredBy string) error {
	// Cycle detection
	if state.resolving[dep.Name] {
		return buildCycleError(state.stack, dep.Name)
	}

	// Parse the constraint
	constraint, err := semver.NewConstraint(dep.Version)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"invalid version constraint for %s: %s", dep.Name, dep.Version)
	}

	// Record this constraint
	state.constraints[dep.Name] = append(state.constraints[dep.Name], constraintEntry{
		constraint: constraint,
		raw:        dep.Version,
		from:       requiredBy,
	})

	// If already resolved, check the existing version against the new constraint
	if existing, ok := state.resolved[dep.Name]; ok {
		return recheckExistingResolution(state, dep, existing, constraint, requiredBy)
	}

	// Mark as resolving for cycle detection
	state.resolving[dep.Name] = true
	state.stack = append(state.stack, dep.Name)
	defer func() {
		delete(state.resolving, dep.Name)
		if 0 < len(state.stack) {
			state.stack = state.stack[:len(state.stack)-1]
		}
	}()

	resolved, resolveErr := findAndResolveDep(state, dep)
	if nil != resolveErr {
		return resolveErr
	}

	state.resolved[dep.Name] = resolved

	return nil
}

func recheckExistingResolution(state *resolverState, dep pkg.Dependency, existing *ResolvedDep, constraint *semver.Constraints, requiredBy string) error {
	existingVer, err := semver.NewVersion(existing.Version)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to parse existing resolved version %s for %s", existing.Version, dep.Name)
	}

	if constraint.Check(existingVer) {
		return nil
	}

	// Existing version doesn't satisfy new constraint — try to find a higher version
	newResolved, err := findHighestCompatible(dep, state.constraints[dep.Name])
	if nil != err {
		return err
	}

	// Verify the new version satisfies ALL existing constraints
	newVer, parseErr := semver.NewVersion(newResolved.Version)
	if nil != parseErr {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, parseErr,
			"failed to parse version %s for %s", newResolved.Version, dep.Name)
	}

	for _, ce := range state.constraints[dep.Name] {
		if !ce.constraint.Check(newVer) {
			return keramoserr.NewErrorf(keramoserr.ErrConflict,
				"dependency conflict for %s: %s requires %s, %s requires %s — no version satisfies all constraints",
				dep.Name, ce.from, ce.raw, requiredBy, dep.Version)
		}
	}

	existing.Version = newResolved.Version
	existing.Digest = newResolved.Digest
	state.warnings = append(state.warnings,
		fmt.Sprintf("upgraded %s to %s to satisfy constraint from %s", dep.Name, newResolved.Version, requiredBy))

	// Re-resolve transitive dependencies for the upgraded version
	transitiveDeps, transitiveErr := resolveTransitiveDepsForVersion(state, dep, newResolved.Version)
	if nil != transitiveErr {
		return transitiveErr
	}
	existing.Dependencies = transitiveDeps

	return nil
}

// resolveTransitiveDepsForVersion downloads the given version of a dependency,
// extracts its metadata, and resolves its transitive dependencies. It returns
// the list of direct dependency names for the new version.
func resolveTransitiveDepsForVersion(state *resolverState, dep pkg.Dependency, version string) ([]string, error) {
	archivePath, dlErr := DownloadPackage(dep.Repository, dep.Name, version)
	if nil != dlErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, dlErr,
			"failed to download dependency %s@%s for transitive re-resolution", dep.Name, version)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	meta, metaErr := extractMetadataFromArchive(archivePath)
	if nil != metaErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, metaErr,
			"failed to extract metadata from %s@%s during transitive re-resolution", dep.Name, version)
	}

	transitiveDeps := make([]string, 0, len(meta.Dependencies))
	for _, td := range meta.Dependencies {
		transitiveDeps = append(transitiveDeps, td.Name)
	}

	for _, td := range meta.Dependencies {
		if resolveErr := resolveTransitive(state, td, dep.Name); nil != resolveErr {
			return nil, resolveErr
		}
	}

	return transitiveDeps, nil
}

func findAndResolveDep(state *resolverState, dep pkg.Dependency) (*ResolvedDep, error) {
	resolved, err := findHighestCompatible(dep, state.constraints[dep.Name])
	if nil != err {
		return nil, err
	}

	// Download archive to extract transitive dependencies
	archivePath, dlErr := DownloadPackage(dep.Repository, dep.Name, resolved.Version)
	if nil != dlErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, dlErr,
			"failed to download dependency %s@%s", dep.Name, resolved.Version)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	// Compute digest
	digest, digestErr := fileDigest(archivePath)
	if nil != digestErr {
		return nil, digestErr
	}
	resolved.Digest = digest

	// Extract metadata for transitive deps
	meta, metaErr := extractMetadataFromArchive(archivePath)
	if nil != metaErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, metaErr,
			"failed to extract metadata from %s@%s", dep.Name, resolved.Version)
	}

	transitiveDeps := make([]string, 0, len(meta.Dependencies))
	for _, td := range meta.Dependencies {
		transitiveDeps = append(transitiveDeps, td.Name)
	}
	resolved.Dependencies = transitiveDeps

	// Resolve transitive dependencies
	for _, td := range meta.Dependencies {
		if err := resolveTransitive(state, td, dep.Name); nil != err {
			return nil, err
		}
	}

	return resolved, nil
}

func findHighestCompatible(dep pkg.Dependency, allConstraints []constraintEntry) (*ResolvedDep, error) {
	idx, err := FetchIndex(dep.Repository)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to fetch index for dependency %s", dep.Name)
	}

	entries, ok := idx.Entries[dep.Name]
	if !ok {
		return nil, keramoserr.NewErrorf(keramoserr.ErrDependency,
			"dependency %s not found in repository %s", dep.Name, dep.Repository)
	}

	var bestVersion *semver.Version
	var bestEntry *IndexEntry

	for i := range entries {
		v, parseErr := semver.NewVersion(entries[i].Version)
		if nil != parseErr {
			continue
		}
		if !satisfiesAll(v, allConstraints) {
			continue
		}
		if nil == bestVersion || v.GreaterThan(bestVersion) {
			bestVersion = v
			bestEntry = &entries[i]
		}
	}

	if nil == bestEntry {
		return nil, keramoserr.NewErrorf(keramoserr.ErrDependency,
			"no version of %s matching all constraints found in %s", dep.Name, dep.Repository)
	}

	logger.Debug("resolved %s to %s", dep.Name, bestEntry.Version)

	return &ResolvedDep{
		Name:       dep.Name,
		Version:    bestEntry.Version,
		Repository: dep.Repository,
		Digest:     bestEntry.Digest,
	}, nil
}

func satisfiesAll(v *semver.Version, constraints []constraintEntry) bool {
	for _, ce := range constraints {
		if !ce.constraint.Check(v) {
			return false
		}
	}
	return true
}

func buildCycleError(stack []string, repeated string) error {
	cycleStart := -1
	stackLen := len(stack)
	for i := 0; i < stackLen; i++ {
		if stack[i] == repeated {
			cycleStart = i
			break
		}
	}

	var cyclePath strings.Builder
	if 0 <= cycleStart {
		for i := cycleStart; i < stackLen; i++ {
			cyclePath.WriteString(stack[i])
			cyclePath.WriteString(" -> ")
		}
	}
	cyclePath.WriteString(repeated)

	return keramoserr.NewErrorf(keramoserr.ErrCycle,
		"dependency cycle detected: %s", cyclePath.String())
}

// extractMetadataFromArchive extracts keramos.yaml from a package archive without
// fully extracting all files. It extracts to a temp dir, reads the metadata,
// then cleans up.
func extractMetadataFromArchive(archivePath string) (*pkg.PackageMetadata, error) {
	tmpDir, err := os.MkdirTemp("", "keramos-resolve-*")
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrDependency,
			"failed to create temp directory for metadata extraction", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := ExtractArchive(archivePath, tmpDir); nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to extract archive for metadata")
	}

	dirEntries, err := os.ReadDir(tmpDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrDependency,
			"failed to read extracted archive", err)
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(tmpDir, entry.Name())
		meta, loadErr := pkg.LoadPackageMetadata(metaPath)
		if nil == loadErr {
			return &meta, nil
		}
	}

	return nil, keramoserr.NewError(keramoserr.ErrDependency,
		"archive does not contain a valid keramos.yaml")
}
