package action

import (
	"regexp"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
)

var releaseNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

const maxReleaseNameLen = 53

// ValidateReleaseName checks that a release name is a valid DNS-1123 label subset.
// It returns an error if the name is empty, too long, or contains invalid characters.
func ValidateReleaseName(name string) error {
	if "" == name {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "release name is required")
	}

	if len(name) > maxReleaseNameLen {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "release name %q exceeds maximum length of %d characters", name, maxReleaseNameLen)
	}

	if !releaseNameRegex.MatchString(name) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "release name %q is invalid: must match [a-z0-9]([a-z0-9-]*[a-z0-9])?", name)
	}

	return nil
}
