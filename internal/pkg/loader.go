package pkg

import (
	"os"
	"path/filepath"
	"regexp"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

var scopedNameRegex = regexp.MustCompile(`^(@[a-z0-9][a-z0-9-]*/)?[a-z0-9][a-z0-9.-]*$`)

const (
	packageFileName = "keramos.yaml"
	valuesFileName  = "values.yaml"
)

// LoadMetadata is an alias for LoadPackageMetadata for concise usage.
func LoadMetadata(dirPath string) (PackageMetadata, error) {
	return LoadPackageMetadata(dirPath)
}

// LoadPackageMetadata reads and parses keramos.yaml from the given directory path.
func LoadPackageMetadata(dirPath string) (PackageMetadata, error) {
	fullPath := filepath.Join(dirPath, packageFileName)
	data, err := os.ReadFile(fullPath)
	if nil != err {
		return PackageMetadata{}, keramoserr.PackageError("failed to read keramos.yaml", fullPath, err)
	}

	var meta PackageMetadata
	if err := yaml.Unmarshal(data, &meta); nil != err {
		return PackageMetadata{}, keramoserr.PackageError("failed to parse keramos.yaml", fullPath, err)
	}

	if validationErr := validateMetadata(&meta, fullPath); nil != validationErr {
		return PackageMetadata{}, validationErr
	}

	return meta, nil
}

// LoadValues reads and parses values.yaml from the given directory path.
func LoadValues(dirPath string) (Values, error) {
	fullPath := filepath.Join(dirPath, valuesFileName)
	data, err := os.ReadFile(fullPath)
	if nil != err {
		return nil, keramoserr.PackageError("failed to read values.yaml", fullPath, err)
	}

	var vals Values
	if err := yaml.Unmarshal(data, &vals); nil != err {
		return nil, keramoserr.PackageError("failed to parse values.yaml", fullPath, err)
	}

	return vals, nil
}

func validateMetadata(meta *PackageMetadata, filePath string) *keramoserr.KeramosError {
	if "" == meta.APIVersion {
		return keramoserr.NewError(keramoserr.ErrPackageInvalid, "apiVersion is required").
			WithFile(filePath, 0, 0)
	}
	if "" == meta.Name {
		return keramoserr.NewError(keramoserr.ErrPackageInvalid, "name is required").
			WithFile(filePath, 0, 0)
	}
	if !scopedNameRegex.MatchString(meta.Name) {
		return keramoserr.NewErrorf(keramoserr.ErrPackageInvalid, "invalid package name %q: must match %s", meta.Name, scopedNameRegex.String()).
			WithFile(filePath, 0, 0)
	}
	if "" == meta.Version {
		return keramoserr.NewError(keramoserr.ErrPackageInvalid, "version is required").
			WithFile(filePath, 0, 0)
	}
	return nil
}
