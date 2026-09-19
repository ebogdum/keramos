package cli

import (
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/repo"
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
	if nil == lf {
		return nil
	}

	chartsDir := filepath.Join(packagePath, "packages")
	entries, err := os.ReadDir(chartsDir)
	if nil != err {
		if os.IsNotExist(err) {
			return nil
		}
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to read packages directory", err)
	}

	var errs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".keramos.tgz") {
			continue
		}

		archivePath := filepath.Join(chartsDir, entry.Name())
		if verifyErr := verifyArchiveSignatureWithKeyring(archivePath, keyring); nil != verifyErr {
			errs = append(errs, verifyErr.Error())
		}
	}

	if 0 < len(errs) {
		return keramoserr.NewErrorf(keramoserr.ErrSignature,
			"signature verification failed for %d package(s):\n  %s", len(errs), strings.Join(errs, "\n  "))
	}

	logger.Debug("all package signatures verified for %s", packagePath)
	return nil
}
