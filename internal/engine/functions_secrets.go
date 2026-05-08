package engine

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
)

// sopsKeyPattern bounds keypaths to safe identifiers so the bracket-quoted
// argument we synthesise for sops --extract cannot be coerced into addressing
// fields the template author did not intend.
var sopsKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*(\.[A-Za-z0-9_][A-Za-z0-9_-]*)*$`)

// validateSopsPath rejects empty paths, absolute paths, and any traversal
// sequence (\.\. on any component). Templates are expected to ship encrypted
// material inside the package; pointing at /etc or ../host-state is a path
// traversal that leaks plaintext through keramos's process.
func validateSopsPath(path string) error {
	if "" == path {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "sops requires a file path")
	}
	if filepath.IsAbs(path) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"sops path %q must be relative to the package directory", path)
	}
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"sops path %q escapes the package directory", path)
	}
	return nil
}

// registerSecretFuncs adds the secrets-bridge engine functions:
//
//   ${sops "path/db.enc.yaml"}                 decrypted plaintext (string/yaml)
//   ${sopsKey "path/db.enc.yaml" "key.path"}   single decrypted key
//   ${externalSecret "name" "store" "key"}     emits an ExternalSecret CR
//   ${sealedSecret "name" "namespace" "data"}  emits a SealedSecret stub
//
// The functions shell out to the host `sops` binary for decryption. When the
// binary is absent the function returns a structured error rather than a panic.
func registerSecretFuncs(r *FuncRegistry) {
	r.Register("sops", fnSops)
	r.Register("sopsKey", fnSopsKey)
	r.Register("externalSecret", fnExternalSecret)
	r.Register("sealedSecret", fnSealedSecret)
}

// fnSops decrypts the file at `value` (the pipeline input or first arg) using
// the `sops` binary and returns the plaintext. If sops is unavailable the
// function emits a non-fatal error so callers can fall back to a stub value
// during template rendering.
func fnSops(value any, args ...any) (any, error) {
	path := coerceString(value)
	if "" == path && 0 < len(args) {
		path = coerceString(args[0])
	}
	if vErr := validateSopsPath(path); nil != vErr {
		return nil, vErr
	}
	if _, err := exec.LookPath("sops"); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal,
			"sops binary not found in PATH (install https://github.com/getsops/sops)", err)
	}
	out, err := exec.Command("sops", "--decrypt", path).Output()
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "sops decrypt %s", path)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// fnSopsKey decrypts the file then walks a dotted path into the YAML/JSON
// document. Useful for `${sopsKey "secrets.enc.yaml" "database.password"}`.
func fnSopsKey(value any, args ...any) (any, error) {
	if 1 > len(args) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "sopsKey requires a key path argument")
	}
	path := coerceString(value)
	if vErr := validateSopsPath(path); nil != vErr {
		return nil, vErr
	}
	keyPath := coerceString(args[0])
	if !sopsKeyPattern.MatchString(keyPath) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"sopsKey: keyPath %q must match %s", keyPath, sopsKeyPattern.String())
	}
	if _, err := exec.LookPath("sops"); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "sops binary not found in PATH", err)
	}
	out, err := exec.Command("sops", "--decrypt", "--extract", "[\""+strings.ReplaceAll(keyPath, ".", "\"][\"")+"\"]", path).Output()
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "sops extract %s from %s", keyPath, path)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// fnExternalSecret renders an external-secrets.io ExternalSecret manifest
// referencing `secretStore`/`remoteKey`. Output is YAML and may be embedded
// directly into a template document.
func fnExternalSecret(value any, args ...any) (any, error) {
	name := coerceString(value)
	if "" == name && 0 < len(args) {
		name = coerceString(args[0])
		args = args[1:]
	}
	if "" == name {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "externalSecret requires a name")
	}
	if 2 > len(args) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation,
			"externalSecret requires (name, secretStore, remoteKey [, refreshInterval])")
	}
	store := coerceString(args[0])
	remoteKey := coerceString(args[1])
	refresh := "1h"
	if 3 <= len(args) {
		refresh = coerceString(args[2])
	}
	manifest := fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: %s
spec:
  refreshInterval: %s
  secretStoreRef:
    name: %s
    kind: SecretStore
  target:
    name: %s
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: %s
`, name, refresh, store, name, remoteKey)
	return manifest, nil
}

// fnSealedSecret produces a placeholder SealedSecret manifest. The encrypted
// material is computed at keramos-package author time (via `kubeseal` invoked
// out-of-band); this helper just wraps the supplied ciphertext map into the
// expected CR shape.
func fnSealedSecret(value any, args ...any) (any, error) {
	name := coerceString(value)
	if "" == name && 0 < len(args) {
		name = coerceString(args[0])
		args = args[1:]
	}
	if "" == name {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "sealedSecret requires a name")
	}
	if 2 > len(args) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation,
			"sealedSecret requires (name, namespace, encryptedData map)")
	}
	namespace := coerceString(args[0])
	data, ok := args[1].(map[string]any)
	if !ok {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation,
			"sealedSecret encryptedData must be a map[string]string")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret\nmetadata:\n  name: %s\n  namespace: %s\nspec:\n  encryptedData:\n", name, namespace)
	for k, v := range data {
		fmt.Fprintf(&b, "    %s: %s\n", k, coerceString(v))
	}
	b.WriteString("  template:\n    metadata:\n      name: ")
	b.WriteString(name)
	b.WriteString("\n      namespace: ")
	b.WriteString(namespace)
	b.WriteString("\n")
	return b.String(), nil
}
