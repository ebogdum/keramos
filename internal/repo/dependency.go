package repo

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/pkg"
)

// DependencyStatus describes the state of a single dependency.
type DependencyStatus struct {
	Name       string
	Declared   string // version constraint
	Installed  string // installed version (empty if missing)
	Repository string
	Status     string // "ok", "missing", "outdated"
	Transitive bool   // true if this is a transitive (indirect) dependency
}

// ResolveDependencies resolves and downloads all dependencies declared in keramos.yaml
// into the packages/ subdirectory. If a current keramos.lock exists, it uses locked
// versions for reproducible resolution.
func ResolveDependencies(packagePath string) error {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to resolve package path", err)
	}

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return err
	}

	if 0 == len(meta.Dependencies) {
		logger.Debug("no dependencies declared")
		return nil
	}

	// Check lock file first
	lf, lockErr := LoadLockFile(absPath)
	if nil != lockErr {
		return lockErr
	}

	if IsLockFileCurrent(lf, meta.Dependencies) {
		logger.Debug("lock file is current, using locked versions")
		return installFromLockFile(absPath, lf)
	}

	// Lock file is stale or missing — resolve transitively
	logger.Debug("resolving dependencies transitively")
	result, resolveErr := ResolveTree(meta.Dependencies)
	if nil != resolveErr {
		return resolveErr
	}

	if err := installResolved(absPath, result); nil != err {
		return err
	}

	newLock := buildLockFile(result)
	return SaveLockFile(newLock, absPath)
}

// UpdateDependencies re-resolves all dependencies via ResolveTree and rewrites the lock file.
func UpdateDependencies(packagePath string) error {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to resolve package path", err)
	}

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return err
	}

	if 0 == len(meta.Dependencies) {
		logger.Debug("no dependencies declared")
		return nil
	}

	result, resolveErr := ResolveTree(meta.Dependencies)
	if nil != resolveErr {
		return resolveErr
	}

	packagesDir := filepath.Join(absPath, "packages")
	if err := os.RemoveAll(packagesDir); nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to clean packages directory", err)
	}

	if err := installResolved(absPath, result); nil != err {
		return err
	}

	newLock := buildLockFile(result)
	return SaveLockFile(newLock, absPath)
}

// UpdateSingleDependency re-resolves only the named dependency and its transitive
// tree, leaving other dependencies at their locked versions.
func UpdateSingleDependency(packagePath, depName string) error {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to resolve package path", err)
	}

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return err
	}

	var targetDep *pkg.Dependency
	for i := range meta.Dependencies {
		if meta.Dependencies[i].Name == depName {
			targetDep = &meta.Dependencies[i]
			break
		}
	}

	if nil == targetDep {
		return keramoserr.NewErrorf(keramoserr.ErrDependency, "dependency %s not declared in keramos.yaml", depName)
	}

	// Resolve just this dep's tree
	result, resolveErr := ResolveTree([]pkg.Dependency{*targetDep})
	if nil != resolveErr {
		return resolveErr
	}

	// Load existing lock file and merge
	lf, lockErr := LoadLockFile(absPath)
	if nil != lockErr {
		return lockErr
	}

	// Build set of newly resolved names
	newResolved := make(map[string]*ResolvedDep, len(result.Resolved))
	for i := range result.Resolved {
		newResolved[result.Resolved[i].Name] = &result.Resolved[i]
	}

	// Install newly resolved packages
	for _, rd := range result.Resolved {
		if err := installSingleResolved(absPath, &rd); nil != err {
			return err
		}
	}

	// Merge into lock file
	mergedLock := mergeLockFile(lf, result)
	return SaveLockFile(mergedLock, absPath)
}

// ListDependencies lists all declared and transitive dependencies with their status.
func ListDependencies(packagePath string) ([]DependencyStatus, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrDependency, "failed to resolve package path", err)
	}

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return nil, err
	}

	// Direct dependencies
	directNames := make(map[string]bool, len(meta.Dependencies))
	results := make([]DependencyStatus, 0, len(meta.Dependencies))
	for _, dep := range meta.Dependencies {
		directNames[dep.Name] = true
		status := checkDependencyStatus(absPath, dep)
		results = append(results, status)
	}

	// Include transitive dependencies from lock file
	lf, lockErr := LoadLockFile(absPath)
	if nil != lockErr {
		return results, nil
	}
	if nil == lf {
		return results, nil
	}

	for _, ld := range lf.Dependencies {
		if directNames[ld.Name] {
			continue
		}
		results = append(results, DependencyStatus{
			Name:       ld.Name,
			Declared:   ld.Version,
			Installed:  CheckInstalledVersion(absPath, ld.Name),
			Repository: ld.Repository,
			Status:     checkLockedStatus(absPath, ld),
			Transitive: true,
		})
	}

	return results, nil
}

func installFromLockFile(absPath string, lf *LockFile) error {
	packagesDir := filepath.Join(absPath, "packages")
	if err := os.MkdirAll(packagesDir, 0755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to create packages directory", err)
	}

	for _, ld := range lf.Dependencies {
		destDir := filepath.Join(packagesDir, ld.Name)

		// Skip if already installed at correct version
		installed := CheckInstalledVersion(absPath, ld.Name)
		if installed == ld.Version {
			logger.Debug("dependency %s@%s already installed", ld.Name, ld.Version)
			continue
		}

		if err := downloadAndExtract(ld.Repository, ld.Name, ld.Version, ld.Digest, destDir); nil != err {
			return err
		}
	}

	return nil
}

func installResolved(absPath string, result *ResolutionResult) error {
	packagesDir := filepath.Join(absPath, "packages")
	if err := os.MkdirAll(packagesDir, 0755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to create packages directory", err)
	}

	for _, rd := range result.Resolved {
		destDir := filepath.Join(packagesDir, rd.Name)
		if err := downloadAndExtract(rd.Repository, rd.Name, rd.Version, rd.Digest, destDir); nil != err {
			return err
		}
	}

	return nil
}

func installSingleResolved(absPath string, rd *ResolvedDep) error {
	packagesDir := filepath.Join(absPath, "packages")
	if err := os.MkdirAll(packagesDir, 0755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to create packages directory", err)
	}

	destDir := filepath.Join(packagesDir, rd.Name)
	return downloadAndExtract(rd.Repository, rd.Name, rd.Version, rd.Digest, destDir)
}

func downloadAndExtract(repoURL, name, version, expectedDigest, destDir string) error {
	archivePath, err := DownloadPackage(repoURL, name, version)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to download dependency %s@%s", name, version)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	// Verify digest if available
	if "" != expectedDigest {
		if verifyErr := VerifyDigest(archivePath, expectedDigest); nil != verifyErr {
			return verifyErr
		}
	}

	// Clean destination and extract
	if err := os.RemoveAll(destDir); nil != err {
		return keramoserr.WrapError(keramoserr.ErrDependency, "failed to clean dependency directory", err)
	}

	if err := ExtractArchive(archivePath, destDir); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to extract dependency %s", name)
	}

	logger.Debug("installed dependency %s version %s", name, version)
	return nil
}

func checkDependencyStatus(absPath string, dep pkg.Dependency) DependencyStatus {
	ds := DependencyStatus{
		Name:       dep.Name,
		Declared:   dep.Version,
		Repository: dep.Repository,
		Status:     "missing",
	}

	depDir := filepath.Join(absPath, "packages", dep.Name)
	entries, err := os.ReadDir(depDir)
	if nil != err {
		return ds
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, loadErr := pkg.LoadPackageMetadata(filepath.Join(depDir, entry.Name()))
		if nil != loadErr {
			continue
		}
		ds.Installed = meta.Version
		ds.Status = determineDepStatus(dep.Version, meta.Version)
		return ds
	}

	return ds
}

// CheckInstalledVersion returns the installed version of a dependency, or empty string if not found.
func CheckInstalledVersion(absPath, depName string) string {
	depDir := filepath.Join(absPath, "packages", depName)
	entries, err := os.ReadDir(depDir)
	if nil != err {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, loadErr := pkg.LoadPackageMetadata(filepath.Join(depDir, entry.Name()))
		if nil != loadErr {
			continue
		}
		return meta.Version
	}

	return ""
}

func checkLockedStatus(absPath string, ld LockedDependency) string {
	installed := CheckInstalledVersion(absPath, ld.Name)
	if "" == installed {
		return "missing"
	}
	if installed == ld.Version {
		return "ok"
	}
	return "outdated"
}

func determineDepStatus(constraint, installed string) string {
	c, err := semver.NewConstraint(constraint)
	if nil != err {
		return "ok"
	}

	v, err := semver.NewVersion(installed)
	if nil != err {
		return "ok"
	}

	if c.Check(v) {
		return "ok"
	}

	return "outdated"
}

func mergeLockFile(existing *LockFile, newResult *ResolutionResult) *LockFile {
	newResolved := make(map[string]LockedDependency, len(newResult.Resolved))
	for _, rd := range newResult.Resolved {
		newResolved[rd.Name] = LockedDependency(rd)
	}

	merged := make([]LockedDependency, 0)

	// Keep existing entries that weren't re-resolved
	if nil != existing {
		for _, ld := range existing.Dependencies {
			if _, replaced := newResolved[ld.Name]; !replaced {
				merged = append(merged, ld)
			}
		}
	}

	// Add new/updated entries
	for _, ld := range newResolved {
		merged = append(merged, ld)
	}

	var generated time.Time
	if nil != existing {
		generated = existing.Generated
	}

	return &LockFile{
		APIVersion:   "v1",
		Generated:    generated,
		Dependencies: merged,
	}
}
