package repo

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/pkg"
	"gopkg.in/yaml.v3"
)

const (
	maxFileSize               = 512 * 1024 * 1024      // 512 MB per file
	maxTotalExtractSize int64 = 2 * 1024 * 1024 * 1024 // 2 GB total (typed int64: the value overflows a 32-bit int on arm/386)
	// maxEntries caps the number of distinct entries (files + dirs +
	// special types). A tar with 1M tiny entries passes maxFileSize and
	// maxTotalExtractSize but exhausts FS metadata, fd quota, and inode
	// budgets on the operator's machine.
	maxEntries = 1 << 16
)

var defaultIgnorePatterns = []string{
	".git/",
	".git",
	".keramosignore",
	"*.keramos.tgz",
}

// PackageArchive creates a .keramos.tgz from a package directory.
// Wrapper around PackageArchiveOpts that preserves the legacy 3-arg signature.
func PackageArchive(packagePath, destDir string, versionOverride string) (string, error) {
	return PackageArchiveOpts(packagePath, destDir, PackageOpts{Version: versionOverride})
}

// PackageOpts customises PackageArchiveOpts. All fields are optional; an
// empty field leaves the corresponding keramos.yaml value unchanged.
type PackageOpts struct {
	Version      string // override metadata.version
	AppVersion   string // override metadata.appVersion
	Reproducible bool   // emit byte-identical output across machines (zero ModTime, deterministic walk order, gzip ModTime=0)
}

// reproducibleEpoch is the canonical timestamp used in reproducible-build
// mode. We pin to the SOURCE_DATE_EPOCH-equivalent of 0 to remove all
// timestamp variance.
var reproducibleEpoch = time.Unix(0, 0).UTC()

// PackageArchiveOpts is the option-rich form of PackageArchive.
func PackageArchiveOpts(packagePath, destDir string, opts PackageOpts) (string, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrArchive, "failed to resolve package path", err)
	}

	meta, err := pkg.LoadPackageMetadata(absPath)
	if nil != err {
		return "", err
	}

	if "" != opts.Version {
		meta.Version = opts.Version
	}
	if "" != opts.AppVersion {
		meta.AppVersion = opts.AppVersion
	}
	versionOverride := opts.Version
	repro := opts.Reproducible

	ignorePatterns, err := loadIgnorePatterns(absPath)
	if nil != err {
		return "", err
	}

	absDestDir, err := filepath.Abs(destDir)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrArchive, "failed to resolve destination path", err)
	}

	archiveName := ArchiveFileName(meta.Name, meta.Version)
	archivePath := filepath.Join(absDestDir, archiveName)

	logger.Debug("packaging %s version %s to %s", meta.Name, meta.Version, archivePath)

	// Use flat name as archive prefix to avoid nested directories from scoped names
	prefix := meta.Name
	if IsScoped(meta.Name) {
		scope, base := ScopeAndName(meta.Name)
		prefix = scope + "-" + base
	}

	if err := createArchive(absPath, archivePath, prefix, &meta, versionOverride, ignorePatterns, repro); nil != err {
		return "", err
	}

	return archivePath, nil
}

// isSafePath checks that targetPath is safely contained within destDir.
func isSafePath(destDir, targetPath string) bool {
	rel, err := filepath.Rel(destDir, targetPath)
	if nil != err {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// ExtractArchive extracts a .keramos.tgz to a directory.
func ExtractArchive(archivePath, destDir string) error {
	absDestDir, err := filepath.Abs(destDir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to resolve destination path", err)
	}

	f, err := os.Open(archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to open archive", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to create gzip reader", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var totalExtracted int64
	var entryCount int
	for {
		header, err := tr.Next()
		if io.EOF == err {
			break
		}
		if nil != err {
			return keramoserr.WrapError(keramoserr.ErrArchive, "failed to read tar entry", err)
		}
		entryCount++
		if entryCount > maxEntries {
			return keramoserr.NewErrorf(keramoserr.ErrArchive,
				"archive contains more than %d entries", maxEntries)
		}

		// Reject absolute paths in tar entries
		if filepath.IsAbs(header.Name) {
			return keramoserr.NewErrorf(keramoserr.ErrArchive, "archive contains absolute path: %s", header.Name)
		}

		cleanName := filepath.Clean(header.Name)
		targetPath := filepath.Join(absDestDir, cleanName)

		// Verify resolved path stays within destination directory
		if !isSafePath(absDestDir, targetPath) {
			return keramoserr.NewErrorf(keramoserr.ErrArchive, "archive contains path traversal: %s", header.Name)
		}

		// Reject symlinks that could escape the destination
		if tar.TypeSymlink == header.Typeflag || tar.TypeLink == header.Typeflag {
			linkTarget := header.Linkname
			if !filepath.IsAbs(linkTarget) {
				linkTarget = filepath.Join(filepath.Dir(targetPath), linkTarget)
			}
			linkTarget = filepath.Clean(linkTarget)
			if !isSafePath(absDestDir, linkTarget) {
				return keramoserr.NewErrorf(keramoserr.ErrArchive, "archive contains symlink escaping destination: %s -> %s", header.Name, header.Linkname)
			}
			continue // skip creating symlinks for security
		}

		if tar.TypeDir == header.Typeflag {
			if err := os.MkdirAll(targetPath, 0755); nil != err {
				return keramoserr.WrapError(keramoserr.ErrArchive, "failed to create directory", err)
			}
			continue
		}

		if tar.TypeReg == header.Typeflag {
			// Reject files that declare a size exceeding the per-file limit
			if header.Size > maxFileSize {
				return keramoserr.NewErrorf(keramoserr.ErrArchive, "file %s exceeds maximum size (%d bytes)", header.Name, header.Size)
			}

			written, extractErr := extractFile(tr, targetPath, header.Mode)
			if nil != extractErr {
				return extractErr
			}
			totalExtracted += written
			if totalExtracted > maxTotalExtractSize {
				return keramoserr.NewErrorf(keramoserr.ErrArchive, "archive exceeds maximum total extraction size (%d bytes)", maxTotalExtractSize)
			}
		}
	}

	return nil
}

func extractFile(tr *tar.Reader, targetPath string, mode int64) (written int64, err error) {
	parentDir := filepath.Dir(targetPath)
	if mkdirErr := os.MkdirAll(parentDir, 0o750); nil != mkdirErr {
		return 0, keramoserr.WrapError(keramoserr.ErrArchive, "failed to create parent directory", mkdirErr)
	}

	// Sanitise the mode taken from the (potentially hostile) tar header:
	// strip setuid/setgid/sticky bits, clamp to the 9 standard rwx bits, and
	// constrain to a safe int range to avoid overflow on the int64→FileMode
	// cast. Default to 0o644 if the header carries no usable mode.
	safeMode := os.FileMode(mode & 0o777) //nolint:gosec // masked above
	if 0 == safeMode {
		safeMode = 0o644
	}

	outFile, openErr := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, safeMode)
	if nil != openErr {
		return 0, keramoserr.WrapError(keramoserr.ErrArchive, "failed to create file", openErr)
	}

	written, err = io.Copy(outFile, io.LimitReader(tr, maxFileSize+1))
	if nil != err {
		outFile.Close()
		return 0, keramoserr.WrapError(keramoserr.ErrArchive, "failed to write file contents", err)
	}

	if written > maxFileSize {
		outFile.Close()
		return 0, keramoserr.NewErrorf(keramoserr.ErrArchive, "file %s exceeds maximum size during extraction (%d bytes)", targetPath, maxFileSize)
	}

	if closeErr := outFile.Close(); nil != closeErr {
		return 0, keramoserr.WrapError(keramoserr.ErrArchive, "failed to close extracted file", closeErr)
	}

	return written, nil
}

func createArchive(srcDir, archivePath, prefix string, meta *pkg.PackageMetadata, versionOverride string, ignorePatterns []string, reproducible bool) (err error) {
	outFile, err := os.Create(archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to create archive file", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	if reproducible {
		// gzip header carries a ModTime by default; pin it for byte-identical output.
		gzWriter.ModTime = reproducibleEpoch
		gzWriter.OS = 255 // unknown OS
	}
	tw := tar.NewWriter(gzWriter)

	walkErr := filepath.Walk(srcDir, func(filePath string, info os.FileInfo, walkErr error) error {
		if nil != walkErr {
			return keramoserr.WrapError(keramoserr.ErrArchive, "walk error", walkErr)
		}

		relPath, err := filepath.Rel(srcDir, filePath)
		if nil != err {
			return keramoserr.WrapError(keramoserr.ErrArchive, "failed to compute relative path", err)
		}

		if "." == relPath {
			return nil
		}

		// Refuse to follow symlinks during packaging — a symlink in the
		// package directory could otherwise embed the target file's contents
		// (e.g. /etc/shadow, ssh keys) into the archive.
		lstat, lstatErr := os.Lstat(filePath)
		if nil != lstatErr {
			return keramoserr.WrapError(keramoserr.ErrArchive, "lstat failed", lstatErr)
		}
		if 0 != lstat.Mode()&os.ModeSymlink {
			logger.Warn("archive: skipping symlink %s (target outside package boundary)", relPath)
			return nil
		}

		if shouldIgnore(relPath, info.IsDir(), ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		archName := filepath.Join(prefix, relPath)
		modTime := info.ModTime()
		mode := info.Mode()
		if reproducible {
			modTime = reproducibleEpoch
			// Mask out variable bits, keep canonical 0644 / 0755.
			if 0 != mode&os.ModeDir {
				mode = 0o755 | os.ModeDir
			} else {
				mode = 0o644
			}
		}

		if info.IsDir() {
			header := &tar.Header{
				Name:     archName + "/",
				Typeflag: tar.TypeDir,
				Mode:     int64(mode),
				ModTime:  modTime,
			}
			return tw.WriteHeader(header)
		}

		// If version override is set and this is keramos.yaml, write the modified version
		if "" != versionOverride && "keramos.yaml" == relPath {
			return writeModifiedMetadataAt(tw, archName, meta, mode, modTime)
		}

		return writeFileToTarAt(tw, filePath, archName, mode, modTime)
	})

	if twErr := tw.Close(); nil != twErr && nil == walkErr {
		walkErr = keramoserr.WrapError(keramoserr.ErrArchive, "failed to close tar writer", twErr)
	}
	if gzErr := gzWriter.Close(); nil != gzErr && nil == walkErr {
		walkErr = keramoserr.WrapError(keramoserr.ErrArchive, "failed to close gzip writer", gzErr)
	}

	return walkErr
}

func writeModifiedMetadataAt(tw *tar.Writer, archName string, meta *pkg.PackageMetadata, mode os.FileMode, modTime time.Time) error {
	data, err := yaml.Marshal(meta)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to marshal modified keramos.yaml", err)
	}

	header := &tar.Header{
		Name:     archName,
		Size:     int64(len(data)),
		Mode:     int64(mode),
		ModTime:  modTime,
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to write tar header", err)
	}

	if _, err := tw.Write(data); nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to write modified keramos.yaml", err)
	}

	return nil
}

func writeFileToTarAt(tw *tar.Writer, filePath, archName string, mode os.FileMode, modTime time.Time) error {
	stat, err := os.Stat(filePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to stat source file", err)
	}
	header := &tar.Header{
		Name:     archName,
		Size:     stat.Size(),
		Mode:     int64(mode),
		ModTime:  modTime,
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to write tar header", err)
	}

	f, err := os.Open(filePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to open source file", err)
	}
	defer f.Close()

	if _, err := io.Copy(tw, f); nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to copy file to archive", err)
	}

	return nil
}

func loadIgnorePatterns(packagePath string) ([]string, error) {
	patterns := make([]string, len(defaultIgnorePatterns))
	copy(patterns, defaultIgnorePatterns)

	ignorePath := filepath.Join(packagePath, ".keramosignore")
	f, err := os.Open(ignorePath)
	if nil != err {
		if os.IsNotExist(err) {
			return patterns, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrArchive, "failed to open .keramosignore", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if "" == line || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrArchive, "failed to read .keramosignore", err)
	}

	return patterns, nil
}

func shouldIgnore(relPath string, isDir bool, patterns []string) bool {
	// Check negation patterns first — if a negation matches, do not ignore
	for _, pattern := range patterns {
		if !strings.HasPrefix(pattern, "!") {
			continue
		}
		negated := pattern[1:]
		if matchPattern(relPath, isDir, negated) {
			return false
		}
	}

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		if matchPattern(relPath, isDir, pattern) {
			return true
		}
	}

	return false
}

func matchPattern(relPath string, isDir bool, pattern string) bool {
	// Directory-only pattern (ends with /)
	if strings.HasSuffix(pattern, "/") {
		if !isDir {
			return false
		}
		dirPattern := strings.TrimSuffix(pattern, "/")
		return matchGlob(relPath, dirPattern)
	}

	// Wildcard pattern
	if strings.ContainsAny(pattern, "*?[") {
		matched, _ := filepath.Match(pattern, filepath.Base(relPath))
		return matched
	}

	// Exact match or prefix match for directories
	if relPath == pattern {
		return true
	}
	if strings.HasPrefix(relPath, pattern+"/") {
		return true
	}

	return filepath.Base(relPath) == pattern
}

func matchGlob(relPath, pattern string) bool {
	if relPath == pattern {
		return true
	}
	if strings.HasPrefix(relPath, pattern+"/") {
		return true
	}
	matched, _ := filepath.Match(pattern, relPath)
	return matched
}
