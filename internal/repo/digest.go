package repo

import (
	"crypto/subtle"
	"os"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
)

// VerifyDigest checks a file's SHA256 against an expected digest.
// On mismatch, the file is deleted and an ErrDigest error is returned.
func VerifyDigest(filePath, expectedDigest string) error {
	actual, err := fileDigest(filePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrDigest, "failed to compute file digest", err)
	}

	if 1 == subtle.ConstantTimeCompare([]byte(actual), []byte(expectedDigest)) {
		logger.Debug("digest verified for %s", filePath)
		return nil
	}

	logger.Warn("digest mismatch for %s: expected %s, got %s", filePath, expectedDigest, actual)

	if removeErr := os.Remove(filePath); nil != removeErr {
		logger.Warn("failed to remove file with mismatched digest: %s", removeErr)
	}

	return keramoserr.NewErrorf(keramoserr.ErrDigest,
		"digest mismatch for %s: expected %s, got %s", filePath, expectedDigest, actual)
}
