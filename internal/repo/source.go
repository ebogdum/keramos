package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
)

// SourceType classifies the origin of a layer or required package.
type SourceType int

const (
	// SourceLocal is a relative or absolute filesystem path.
	SourceLocal SourceType = iota
	// SourceRegistry is an HTTPS registry URL.
	SourceRegistry
	// SourceGit is a git:: prefixed repository URL.
	SourceGit
	// SourceOCI is an oci:// prefixed registry reference.
	SourceOCI
)

const gitPrefix = "git::"
const ociPrefix = "oci://"

// ParseSource determines the source type, the base URL/path, and any subdirectory
// encoded with double-slash notation (git::https://host/repo.git//sub/dir).
func ParseSource(source string) (SourceType, string, string) {
	if strings.HasPrefix(source, gitPrefix) {
		raw := strings.TrimPrefix(source, gitPrefix)
		base, subdir := splitGitSubdir(raw)
		return SourceGit, base, subdir
	}
	if strings.HasPrefix(source, ociPrefix) {
		return SourceOCI, source, ""
	}
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		return SourceRegistry, source, ""
	}

	return SourceLocal, source, ""
}

// splitGitSubdir separates the repository URL from an optional sub-directory
// encoded after a double-slash.
func splitGitSubdir(raw string) (string, string) {
	// Find "//" that is NOT part of the scheme (e.g. https://).
	// Walk past the scheme portion first.
	searchStart := 0
	if idx := strings.Index(raw, "://"); -1 != idx {
		searchStart = idx + 3
	}

	rest := raw[searchStart:]
	dblIdx := strings.Index(rest, "//")
	if -1 == dblIdx {
		return raw, ""
	}

	repoEnd := searchStart + dblIdx
	return raw[:repoEnd], rest[dblIdx+2:]
}

// FetchSource resolves a source string to a local directory containing the package.
// For local sources it resolves relative to basePath.
// For git and registry sources it uses cacheDir for caching.
func FetchSource(source, ref, version, name, cacheDir, basePath string) (string, error) {
	srcType, url, subdir := ParseSource(source)

	switch srcType {
	case SourceLocal:
		return resolveLocalSource(url, basePath)
	case SourceGit:
		return FetchGitSource(url, ref, subdir, cacheDir)
	case SourceRegistry:
		return FetchRegistrySource(url, name, version, cacheDir)
	case SourceOCI:
		return FetchOCISource(url, version, cacheDir)
	default:
		return "", keramoserr.NewErrorf(keramoserr.ErrDependency, "unknown source type for %s", source)
	}
}

// FetchOCISource pulls a keramos archive from an OCI registry, caches the
// extracted package under cacheDir/oci/<hash>/, and returns the package
// directory path. Resolves `oci://host/path:tag` or `oci://host/path` (uses
// `latest` when no tag).
func FetchOCISource(ociURL, version, cacheDir string) (string, error) {
	if "" == cacheDir {
		home, err := os.UserCacheDir()
		if nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrDependency, "cache dir", err)
		}
		cacheDir = filepath.Join(home, "keramos", "oci")
	}
	if err := os.MkdirAll(cacheDir, 0o755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrDependency, "create cache dir", err)
	}

	// Strip oci:// and decide on the tag. If `version` is supplied (from
	// keramos.yaml) it overrides any inline `:tag`. Otherwise use the inline
	// tag, and fall back to "latest".
	ref := strings.TrimPrefix(ociURL, ociPrefix)
	if "" != version {
		// Drop existing tag if present.
		if i := strings.LastIndex(ref, ":"); i > 0 && !strings.Contains(ref[i:], "/") {
			ref = ref[:i]
		}
		ref = ref + ":" + version
	} else if !strings.Contains(ref, ":") {
		ref = ref + ":latest"
	}

	cacheKey := hashString(ref)
	destDir := filepath.Join(cacheDir, cacheKey)
	if dirExists(destDir) {
		if pkgDir, err := findPackageDir(destDir); nil == err {
			logger.Debug("OCI source %s already cached", ref)
			return pkgDir, nil
		}
		// Cache directory exists but contains no recognisable package —
		// likely a partial extraction from a previous failed run. Wipe
		// and re-extract; without this, files from the OLD archive that
		// no longer appear in the new one would shadow the fresh
		// extraction (e.g. stale templates with deleted-upstream content).
		if rmErr := os.RemoveAll(destDir); nil != rmErr {
			return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, rmErr,
				"clear stale OCI cache at %s", destDir)
		}
	}

	if err := os.MkdirAll(destDir, 0o755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrDependency, "create OCI cache subdir", err)
	}

	// Allow plain-HTTP / TLS-skip via env knobs so this works against the
	// same registries `keramos registry pull` already supports without leaking
	// transport flags into keramos.yaml dependencies (which should stay
	// declarative).
	registry := &OCIRegistry{
		PlainHTTP:             "1" == os.Getenv("KERAMOS_OCI_PLAIN_HTTP"),
		InsecureSkipTLSVerify: "1" == os.Getenv("KERAMOS_OCI_INSECURE_SKIP_TLS"),
	}
	archivePath, pullErr := registry.Pull(ref, destDir)
	if nil != pullErr {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, pullErr,
			"pull OCI source %s", ref)
	}
	if err := ExtractArchive(archivePath, destDir); nil != err {
		return "", err
	}
	pkgDir, err := findPackageDir(destDir)
	if nil != err {
		return "", err
	}
	return pkgDir, nil
}

func resolveLocalSource(relPath, basePath string) (string, error) {
	resolved := relPath
	if !filepath.IsAbs(relPath) {
		resolved = filepath.Join(basePath, relPath)
	}

	abs, err := filepath.Abs(resolved)
	if nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, err, "failed to resolve local path %s", relPath)
	}

	info, err := os.Stat(abs)
	if nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, err, "local source not found: %s", abs)
	}
	if !info.IsDir() {
		return "", keramoserr.NewErrorf(keramoserr.ErrDependency, "local source is not a directory: %s", abs)
	}

	return abs, nil
}

// FetchGitSource clones or updates a git repository and returns the path to
// the package directory (applying subdir if specified).
func FetchGitSource(repoURL, ref, subdir, cacheDir string) (string, error) {
	if err := validateGitRepoURL(repoURL); nil != err {
		return "", err
	}
	if "" == cacheDir {
		home, err := os.UserCacheDir()
		if nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrDependency, "failed to determine cache directory", err)
		}
		cacheDir = filepath.Join(home, "keramos", "git")
	}

	if err := os.MkdirAll(cacheDir, 0755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrDependency, "failed to create git cache directory", err)
	}

	repoHash := hashString(repoURL)
	cloneDir := filepath.Join(cacheDir, repoHash)

	if dirExists(cloneDir) {
		if err := gitFetch(cloneDir); nil != err {
			return "", err
		}
	} else {
		if err := gitClone(repoURL, ref, cloneDir); nil != err {
			return "", err
		}
	}

	if err := gitCheckout(cloneDir, ref); nil != err {
		return "", err
	}

	result := cloneDir
	if "" != subdir {
		result = filepath.Join(cloneDir, subdir)
	}

	// Verify the resolved path stays within cloneDir to prevent path traversal.
	if !isSafePath(cloneDir, result) {
		return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
			"subdirectory %q escapes clone directory (path traversal)", subdir)
	}

	if !dirExists(result) {
		return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
			"subdirectory %s not found in git repo %s", subdir, repoURL)
	}

	return result, nil
}

// FetchRegistrySource downloads a package from an HTTP registry, resolving
// the version constraint against the registry index.
func FetchRegistrySource(registryURL, name, version, cacheDir string) (string, error) {
	if "" == cacheDir {
		home, err := os.UserCacheDir()
		if nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrDependency, "failed to determine cache directory", err)
		}
		cacheDir = filepath.Join(home, "keramos", "registry")
	}

	if err := os.MkdirAll(cacheDir, 0755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrDependency, "failed to create registry cache directory", err)
	}

	resolvedVersion, resolveErr := resolveRegistryVersion(registryURL, name, version)
	if nil != resolveErr {
		return "", resolveErr
	}

	destDir := filepath.Join(cacheDir, hashString(registryURL+"//"+name+"@"+resolvedVersion))
	if dirExists(destDir) {
		pkgDir, err := findPackageDir(destDir)
		if nil == err {
			logger.Debug("registry source %s@%s already cached", name, resolvedVersion)
			return pkgDir, nil
		}
	}

	archivePath, dlErr := DownloadPackage(registryURL, name, resolvedVersion)
	if nil != dlErr {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, dlErr,
			"failed to download %s@%s from %s", name, resolvedVersion, registryURL)
	}
	defer os.RemoveAll(filepath.Dir(archivePath))

	if err := ExtractArchive(archivePath, destDir); nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to extract %s@%s", name, resolvedVersion)
	}

	pkgDir, err := findPackageDir(destDir)
	if nil != err {
		return "", err
	}

	return pkgDir, nil
}

func resolveRegistryVersion(registryURL, name, version string) (string, error) {
	idx, err := FetchIndex(registryURL)
	if nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"failed to fetch index from %s", registryURL)
	}

	entries, ok := idx.Entries[name]
	if !ok {
		return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
			"package %s not found in registry %s", name, registryURL)
	}

	if "" == version {
		if 0 == len(entries) {
			return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
				"no versions of %s found in registry %s", name, registryURL)
		}
		return entries[0].Version, nil
	}

	constraint, parseErr := semver.NewConstraint(version)
	if nil != parseErr {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, parseErr,
			"invalid version constraint: %s", version)
	}

	// Entries are sorted descending; first match is the highest version.
	for _, entry := range entries {
		v, vErr := semver.NewVersion(entry.Version)
		if nil != vErr {
			continue
		}
		if constraint.Check(v) {
			return entry.Version, nil
		}
	}

	return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
		"no version of %s matching %s found in %s", name, version, registryURL)
}

func findPackageDir(dir string) (string, error) {
	if fileExists(filepath.Join(dir, "keramos.yaml")) {
		return dir, nil
	}

	entries, err := os.ReadDir(dir)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrDependency, "failed to read extracted directory", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(dir, entry.Name())
		if fileExists(filepath.Join(candidate, "keramos.yaml")) {
			return candidate, nil
		}
	}

	return "", keramoserr.NewErrorf(keramoserr.ErrDependency,
		"no keramos.yaml found in extracted package at %s", dir)
}

// validateGitRef rejects refs that could confuse git's argument parser or
// shell layer. Allowed: branch / tag / commit-SHA shapes —
// alphanumerics, dot, slash, dash, underscore, plus, at, tilde, caret.
// Rejected: dash-prefix (flag injection), control chars, NULL, range
// notation `..` (means something different than a single ref), longer
// than 256 chars (no legitimate ref is that long).
func validateGitRef(ref string) error {
	if "" == ref {
		return nil
	}
	if 256 < len(ref) {
		return keramoserr.NewErrorf(keramoserr.ErrDependency,
			"git ref %q exceeds 256-char limit", ref)
	}
	if strings.HasPrefix(ref, "-") {
		return keramoserr.NewErrorf(keramoserr.ErrDependency,
			"invalid git ref %q: must not start with a dash", ref)
	}
	if strings.Contains(ref, "..") {
		return keramoserr.NewErrorf(keramoserr.ErrDependency,
			"invalid git ref %q: range notation (..) not supported", ref)
	}
	for _, r := range ref {
		// 0-31 (control), 127 (DEL), and any rune outside the explicit
		// allowed set are rejected.
		if r < 0x20 || 0x7f == r {
			return keramoserr.NewErrorf(keramoserr.ErrDependency,
				"git ref %q contains control characters", ref)
		}
		switch {
		case 'a' <= r && r <= 'z':
		case 'A' <= r && r <= 'Z':
		case '0' <= r && r <= '9':
		case '.' == r || '/' == r || '-' == r || '_' == r:
		case '+' == r || '@' == r || '~' == r || '^' == r:
		default:
			return keramoserr.NewErrorf(keramoserr.ErrDependency,
				"git ref %q contains disallowed character %q", ref, r)
		}
	}
	return nil
}

// --- git helpers using os/exec ---

// validateGitRepoURL restricts git source URLs to safe forms. Allowed:
//   - https:// (TLS-protected remote)
//   - ssh:// or git@host:... (authenticated remote)
//   - absolute local filesystem paths (for testing and local-only workflows)
//
// Rejected: file://, http://, git:// (plaintext), and dash-prefixed strings
// (which git would interpret as flags). This blocks SSRF via attacker-supplied
// keramos.yaml dependencies pointing at internal services.
func validateGitRepoURL(repoURL string) error {
	if "" == repoURL {
		return keramoserr.NewError(keramoserr.ErrDependency, "git repository URL must not be empty")
	}
	if strings.HasPrefix(repoURL, "-") {
		return keramoserr.NewErrorf(keramoserr.ErrDependency,
			"invalid git URL %q: must not start with a dash", repoURL)
	}
	if strings.HasPrefix(repoURL, "https://") {
		return nil
	}
	if strings.HasPrefix(repoURL, "ssh://") || strings.HasPrefix(repoURL, "git@") {
		return nil
	}
	if strings.HasPrefix(repoURL, "/") {
		return nil
	}
	return keramoserr.NewErrorf(keramoserr.ErrDependency,
		"unsupported git URL scheme: %q (only https://, ssh://, and absolute local paths are allowed)", repoURL)
}

func gitClone(repoURL, ref, destDir string) error {
	if err := validateGitRef(ref); nil != err {
		return err
	}

	args := []string{"clone"}

	if !isCommitSHA(ref) {
		args = append(args, "--depth", "1")
		if "" != ref {
			args = append(args, "--branch", ref)
		}
	}

	// `--` separates options from positional args so a future change in URL
	// validation (or a slip in the dash-prefix check above) cannot cause git
	// to interpret the URL as a flag.
	args = append(args, "--", repoURL, destDir)

	logger.Debug("git clone %s -> %s", repoURL, destDir)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"git clone failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func gitFetch(repoDir string) error {
	logger.Debug("git fetch in %s", repoDir)
	cmd := exec.Command("git", "fetch", "--all", "--prune")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"git fetch failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func gitCheckout(repoDir, ref string) error {
	if "" == ref {
		ref = "HEAD"
	}

	if err := validateGitRef(ref); nil != err {
		return err
	}

	logger.Debug("git checkout %s in %s", ref, repoDir)
	// We deliberately do NOT use `git checkout -- <ref>` here: the `--`
	// would force git to treat the argument as a pathspec rather than a
	// branch/tag/SHA. validateGitRef above already rejects dash-prefixed
	// values, which is the only flag-injection vector for this call.
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"git checkout %s failed: %s", ref, strings.TrimSpace(string(output)))
	}
	return nil
}

// GitResolveCommit returns the full commit SHA that HEAD points to.
func GitResolveCommit(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrDependency, err,
			"git rev-parse failed: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// isCommitSHA returns true if s looks like a hex commit SHA (>= 7 chars, all hex).
func isCommitSHA(s string) bool {
	if 7 > len(s) {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if nil != err {
		return false
	}
	return info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if nil != err {
		return false
	}
	return !info.IsDir()
}
