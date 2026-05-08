package repo

import (
	"path/filepath"
	"regexp"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
)

var scopedNameRegex = regexp.MustCompile(`^(@[a-z0-9][a-z0-9-]*/)?[a-z0-9][a-z0-9.-]*$`)

// ValidateScopedName validates a package name, optionally scoped.
// Valid: "myapp", "@myorg/myapp", "@my-org/my.app"
// Invalid: "@/foo", "@@org/foo", "@org/", "@ORG/foo"
func ValidateScopedName(name string) error {
	if "" == name {
		return keramoserr.NewError(keramoserr.ErrPackageInvalid, "package name must not be empty")
	}

	if !scopedNameRegex.MatchString(name) {
		return keramoserr.NewErrorf(keramoserr.ErrPackageInvalid, "invalid package name %q: must match %s", name, scopedNameRegex.String())
	}

	return nil
}

// IsScoped returns true if the name has an @scope/ prefix.
func IsScoped(name string) bool {
	return strings.HasPrefix(name, "@") && strings.Contains(name, "/")
}

// ScopeAndName splits "@scope/name" into ("@scope", "name").
// For unscoped names, returns ("", name).
func ScopeAndName(name string) (string, string) {
	if !IsScoped(name) {
		return "", name
	}

	idx := strings.Index(name, "/")
	return name[:idx], name[idx+1:]
}

// ArchiveFileName returns the archive filename for a scoped or unscoped package.
// "@myorg/redis" version "1.0.0" -> "@myorg-redis-1.0.0.keramos.tgz"
// "redis" version "1.0.0" -> "redis-1.0.0.keramos.tgz"
func ArchiveFileName(name, version string) string {
	flat := name
	if IsScoped(name) {
		scope, base := ScopeAndName(name)
		flat = scope + "-" + base
	}
	return flat + "-" + version + ".keramos.tgz"
}

// PackageDir returns the dependency directory path for a scoped or unscoped package.
// "@myorg/redis" -> "packages/@myorg/redis"
// "redis" -> "packages/redis"
func PackageDir(name string) string {
	if IsScoped(name) {
		scope, base := ScopeAndName(name)
		return filepath.Join("packages", scope, base)
	}
	return filepath.Join("packages", name)
}
