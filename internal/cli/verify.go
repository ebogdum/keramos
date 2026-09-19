package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/repo"
)

// keyringDir returns the default keyring directory for keramos.
func keyringDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to determine config directory", err)
	}
	return filepath.Join(configDir, "keramos", "keyring"), nil
}

// verifyArchiveSignature checks the provenance file for a single archive.
// If --verify is set and no .prov file exists, it returns an error.
// If a .prov file exists, it verifies against the keyring directory.
func verifyArchiveSignature(archivePath string) error {
	return verifyArchiveSignatureWithKeyring(archivePath, "")
}

// verifyArchiveSignatureWithKeyring uses the given keyring path when non-empty,
// falling back to the default keyringDir().
func verifyArchiveSignatureWithKeyring(archivePath, keyring string) error {
	provPath := archivePath + ".prov"

	if _, err := os.Stat(provPath); nil != err {
		if os.IsNotExist(err) {
			return keramoserr.NewErrorf(keramoserr.ErrSignature,
				"no provenance file found for %s; expected %s", archivePath, provPath)
		}
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to check provenance file", err)
	}

	kDir := keyring
	if "" == kDir {
		def, err := keyringDir()
		if nil != err {
			return err
		}
		kDir = def
	}
	return repo.VerifySignatureFromKeyring(archivePath, provPath, kDir)
}

// verifyInstalledSignatures checks provenance files for all installed dependencies.
func verifyInstalledSignatures(packagePath string) error {
	return verifyInstalledSignaturesWithKeyring(packagePath, "")
}

// verifyInstalledSignaturesWithKeyring honors a custom keyring path.
func verifyInstalledSignaturesWithKeyring(packagePath, keyring string) error {
	lf, err := repo.LoadLockFile(packagePath)
	if nil != err {
		return err
	}
	if nil == lf || 0 == len(lf.Dependencies) {
		logger.Debug("no dependencies to verify for %s", packagePath)
		return nil
	}

	chartsDir := filepath.Join(packagePath, "packages")
	entries, err := os.ReadDir(chartsDir)
	if nil != err {
		if os.IsNotExist(err) {
			return keramoserr.NewErrorf(keramoserr.ErrSignature,
				"cannot verify: the lock file lists %d dependencies but %s does not exist",
				len(lf.Dependencies), chartsDir)
		}
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to read packages directory", err)
	}

	provenance := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".keramos.tgz") {
			continue
		}
		provenance[entry.Name()] = filepath.Join(chartsDir, entry.Name())
	}

	var errs []string
	for _, dep := range lf.Dependencies {
		archiveName := repo.ArchiveFileName(dep.Name, dep.Version)
		archivePath, ok := provenance[archiveName]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s@%s: no signed archive %s retained to verify against",
				dep.Name, dep.Version, archiveName))
			continue
		}
		if "" != dep.Digest {
			if digestErr := repo.VerifyDigest(archivePath, dep.Digest); nil != digestErr {
				errs = append(errs, fmt.Sprintf("%s@%s: %v", dep.Name, dep.Version, digestErr))
				continue
			}
		}
		if verifyErr := verifyArchiveSignatureWithKeyring(archivePath, keyring); nil != verifyErr {
			errs = append(errs, fmt.Sprintf("%s@%s: %v", dep.Name, dep.Version, verifyErr))
		}
	}

	if 0 < len(errs) {
		return keramoserr.NewErrorf(keramoserr.ErrSignature,
			"signature verification failed for %d of %d dependencies:\n  %s",
			len(errs), len(lf.Dependencies), strings.Join(errs, "\n  "))
	}

	logger.Debug("all package signatures verified for %s", packagePath)
	return nil
}
