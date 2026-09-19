package action

import (
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
)

// LoadCRDsForRender is the exported variant of loadCRDs for use by cli/template.
func LoadCRDsForRender(packagePath string) (string, error) {
	return loadCRDs(packagePath)
}

// loadCRDs reads every YAML file under <packagePath>/crds/ (recursively) and
// concatenates them into a single multi-document manifest. CRDs are emitted
// untemplated and applied before any other resources in the package.
func loadCRDs(packagePath string) (string, error) {
	crdDir := filepath.Join(packagePath, "crds")
	info, err := os.Stat(crdDir)
	if nil != err {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to stat crds directory", err)
	}
	if !info.IsDir() {
		return "", nil
	}

	docs := make([]string, 0, 8)
	walkErr := filepath.Walk(crdDir, func(path string, fi os.FileInfo, walkErr error) error {
		if nil != walkErr {
			return walkErr
		}
		// Refuse to follow symlinks under crds/ — they could read arbitrary
		// host files (admin.conf, ssh keys) and embed them as CRDs.
		lstat, lstatErr := os.Lstat(path)
		if nil == lstatErr && 0 != lstat.Mode()&os.ModeSymlink {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ".yaml" != ext && ".yml" != ext {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrPackageInvalid, readErr, "failed to read CRD file %s", path)
		}
		docs = append(docs, string(data))
		return nil
	})
	if nil != walkErr {
		return "", walkErr
	}

	return strings.Join(docs, "---\n"), nil
}
