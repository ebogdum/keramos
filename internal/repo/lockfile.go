package repo

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"gopkg.in/yaml.v3"
)

const lockFileName = "keramos.lock"

// LockFile represents the keramos.lock dependency lock file.
type LockFile struct {
	APIVersion   string             `yaml:"apiVersion"`
	Generated    time.Time          `yaml:"generated"`
	Layers       []LockedLayer      `yaml:"layers,omitempty"`
	Requires     []LockedLayer      `yaml:"requires,omitempty"`
	Dependencies []LockedDependency `yaml:"dependencies,omitempty"` // legacy
}

// LockedLayer represents a locked layer or required package entry.
type LockedLayer struct {
	Name            string `yaml:"name"`
	Source          string `yaml:"source"`
	ResolvedVersion string `yaml:"resolvedVersion,omitempty"`
	Ref             string `yaml:"ref,omitempty"`
	ResolvedCommit  string `yaml:"resolvedCommit,omitempty"`
	Digest          string `yaml:"digest,omitempty"`
}

// LockedDependency represents a single locked dependency entry (legacy format).
type LockedDependency struct {
	Name         string   `yaml:"name"`
	Version      string   `yaml:"version"`
	Repository   string   `yaml:"repository"`
	Digest       string   `yaml:"digest"`
	Dependencies []string `yaml:"dependencies,omitempty"`
}

// LoadLockFile reads and parses keramos.lock from the given package directory.
// Returns nil and no error if the lock file does not exist.
func LoadLockFile(packageDir string) (*LockFile, error) {
	lockPath := filepath.Join(packageDir, lockFileName)
	data, err := os.ReadFile(lockPath)
	if nil != err {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrLockFile, "failed to read keramos.lock", err)
	}

	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrLockFile, "failed to parse keramos.lock", err)
	}

	logger.Debug("loaded lock file with %d layers, %d requires, %d legacy dependencies",
		len(lf.Layers), len(lf.Requires), len(lf.Dependencies))
	return &lf, nil
}

// SaveLockFile writes a LockFile to keramos.lock in the given package directory.
func SaveLockFile(lf *LockFile, packageDir string) error {
	lockPath := filepath.Join(packageDir, lockFileName)

	data, err := yaml.Marshal(lf)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrLockFile, "failed to marshal keramos.lock", err)
	}

	if err := os.WriteFile(lockPath, data, 0644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrLockFile, "failed to write keramos.lock", err)
	}

	logger.Debug("saved lock file with %d layers, %d requires",
		len(lf.Layers), len(lf.Requires))
	return nil
}

// BuildLayerLockFile creates a LockFile from layers and requires.
func BuildLayerLockFile(layers []LockedLayer, requires []LockedLayer) *LockFile {
	return &LockFile{
		APIVersion: "keramos/v1",
		Generated:  time.Now(),
		Layers:     layers,
		Requires:   requires,
	}
}

// IsLockFileCurrent checks that every dependency in keramos.yaml has a corresponding
// locked entry whose version satisfies the declared constraint.
// Supports both new layers format and legacy dependencies.
func IsLockFileCurrent(lf *LockFile, deps []pkg.Dependency) bool {
	if nil == lf {
		return false
	}

	// No dependencies declared - any lock file (even empty) is current
	if 0 == len(deps) {
		return true
	}

	// Legacy path: check Dependencies field
	return isLegacyLockFileCurrent(lf, deps)
}

func isLegacyLockFileCurrent(lf *LockFile, deps []pkg.Dependency) bool {
	lockedByName := make(map[string]*LockedDependency, len(lf.Dependencies))
	for i := range lf.Dependencies {
		lockedByName[lf.Dependencies[i].Name] = &lf.Dependencies[i]
	}

	for _, dep := range deps {
		locked, ok := lockedByName[dep.Name]
		if !ok {
			logger.Debug("lock file missing entry for %s", dep.Name)
			return false
		}

		constraint, err := semver.NewConstraint(dep.Version)
		if nil != err {
			logger.Debug("invalid constraint %s for %s, treating as stale", dep.Version, dep.Name)
			return false
		}

		v, err := semver.NewVersion(locked.Version)
		if nil != err {
			logger.Debug("invalid locked version %s for %s, treating as stale", locked.Version, dep.Name)
			return false
		}

		if !constraint.Check(v) {
			logger.Debug("locked version %s of %s does not satisfy %s", locked.Version, dep.Name, dep.Version)
			return false
		}
	}

	return true
}

// buildLockFile creates a LockFile from a ResolutionResult (legacy format).
func buildLockFile(result *ResolutionResult) *LockFile {
	deps := make([]LockedDependency, 0, len(result.Resolved))
	for _, rd := range result.Resolved {
		deps = append(deps, LockedDependency(rd))
	}

	return &LockFile{
		APIVersion:   "v1",
		Generated:    time.Now(),
		Dependencies: deps,
	}
}
