package repo

import (
	"os/exec"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
)

// CosignKeylessOpts holds required identity parameters for keyless (Sigstore) verification.
type CosignKeylessOpts struct {
	CertIdentity string
	CertIssuer   string
}

func verifyCosignExec(ref, publicKeyPath string, keylessOpts *CosignKeylessOpts) error {
	cosignPath, err := exec.LookPath("cosign")
	if nil != err {
		return keramoserr.NewError(keramoserr.ErrSignature,
			"cosign is not installed; install it from https://docs.sigstore.dev/cosign/system_config/installation/")
	}

	args, buildErr := buildCosignArgs(ref, publicKeyPath, keylessOpts)
	if nil != buildErr {
		return buildErr
	}
	logger.Debug("running: %s %v", cosignPath, args)

	cmd := exec.Command(cosignPath, args...)
	output, err := cmd.CombinedOutput()
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrSignature, err,
			"cosign verification failed for %s: %s", ref, string(output))
	}

	logger.Debug("cosign verification passed for %s", ref)
	return nil
}

func buildCosignArgs(ref, publicKeyPath string, keylessOpts *CosignKeylessOpts) ([]string, error) {
	// Reject dash-prefixed values: cosign's flag parser would otherwise
	// reinterpret a value like `--allow-insecure-registry` as a flag and
	// silently weaken verification.
	for _, v := range []string{ref, publicKeyPath} {
		if strings.HasPrefix(v, "-") {
			return nil, keramoserr.NewErrorf(keramoserr.ErrSignature,
				"cosign argument %q must not start with a dash", v)
		}
	}
	if nil != keylessOpts {
		if strings.HasPrefix(keylessOpts.CertIdentity, "-") || strings.HasPrefix(keylessOpts.CertIssuer, "-") {
			return nil, keramoserr.NewError(keramoserr.ErrSignature,
				"cosign CertIdentity / CertIssuer must not start with a dash")
		}
	}

	if "" != publicKeyPath {
		return []string{"verify", "--key", publicKeyPath, ref}, nil
	}

	// Keyless verification requires explicit identity and issuer — wildcards are unsafe.
	if nil == keylessOpts || "" == keylessOpts.CertIdentity || "" == keylessOpts.CertIssuer {
		return nil, keramoserr.NewError(keramoserr.ErrSignature,
			"keyless cosign verification requires explicit CertIdentity and CertIssuer values")
	}

	return []string{
		"verify",
		"--certificate-identity", keylessOpts.CertIdentity,
		"--certificate-oidc-issuer", keylessOpts.CertIssuer,
		ref,
	}, nil
}
