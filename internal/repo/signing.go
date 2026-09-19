package repo

import (
	"bytes"
	"crypto"
	"crypto/subtle"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
)

// SignPackage creates a PGP cleartext-signed provenance file for an archive.
// The provenance contains: package name, version, SHA256 digest, timestamp.
// Returns the path to the created .prov file (archivePath + ".prov").
func SignPackage(archivePath, privateKeyPath string) (string, error) {
	return SignPackageWithPassphrase(archivePath, privateKeyPath, "")
}

// SignPackageWithPassphrase is SignPackage for a passphrase-protected private
// key. An empty passphrase is fine for an unprotected key.
func SignPackageWithPassphrase(archivePath, privateKeyPath, passphrase string) (string, error) {
	digest, err := fileDigest(archivePath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to compute archive digest", err)
	}

	meta, err := readMetadataFromArchive(archivePath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to read archive metadata", err)
	}

	keyData, err := os.ReadFile(privateKeyPath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to read private key file", err)
	}

	entity, err := readPrivateKeyEntity(keyData)
	if nil != err {
		return "", err
	}
	if decErr := decryptEntity(entity, passphrase); nil != decErr {
		return "", decErr
	}

	provContent := formatProvenance(meta.Name, meta.Version, digest)

	signed, err := clearsignMessage(provContent, entity)
	if nil != err {
		return "", err
	}

	provPath := archivePath + ".prov"
	if err := os.WriteFile(provPath, signed, 0644); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to write provenance file", err)
	}

	logger.Debug("signed %s -> %s", archivePath, provPath)
	return provPath, nil
}

// SignFile produces a PGP cleartext-signed .prov sidecar for any file (used
// for index.yaml signing in addition to package archives).
func SignFile(filePath, privateKeyPath string) (string, error) {
	digest, err := fileDigest(filePath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to compute file digest", err)
	}

	keyData, keyErr := os.ReadFile(privateKeyPath)
	if nil != keyErr {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to read private key file", keyErr)
	}
	entity, entErr := readPrivateKeyEntity(keyData)
	if nil != entErr {
		return "", entErr
	}
	if decErr := decryptEntity(entity, ""); nil != decErr {
		return "", decErr
	}

	body := fmt.Sprintf("file: %s\ndigest: %s\nsigned: %s\n",
		filepath.Base(filePath), digest, time.Now().UTC().Format(time.RFC3339))
	signed, signErr := clearsignMessage(body, entity)
	if nil != signErr {
		return "", signErr
	}

	provPath := filePath + ".prov"
	if err := os.WriteFile(provPath, signed, 0644); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrSignature, "failed to write provenance file", err)
	}
	return provPath, nil
}

// VerifySignature checks the provenance file against the archive and a single public key.
func VerifySignature(archivePath, provFilePath, publicKeyPath string) error {
	keyData, err := os.ReadFile(publicKeyPath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to read public key file", err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(keyData))
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to parse public key", err)
	}

	return verifyProvenance(archivePath, provFilePath, keyring)
}

// VerifySignatureFromKeyring checks the provenance file against the archive
// using all public keys found in the keyring directory.
func VerifySignatureFromKeyring(archivePath, provFilePath, keyringDir string) error {
	keyring, err := loadKeyringDir(keyringDir)
	if nil != err {
		return err
	}

	if 0 == len(keyring) {
		return keramoserr.NewError(keramoserr.ErrSignature, "no public keys found in keyring directory")
	}

	return verifyProvenance(archivePath, provFilePath, keyring)
}

// VerifyCosign verifies an OCI artifact's cosign signature.
// For key-based verification, provide publicKeyPath. For keyless (Sigstore)
// verification, provide keylessOpts with explicit CertIdentity and CertIssuer.
func VerifyCosign(ref, publicKeyPath string, keylessOpts *CosignKeylessOpts) error {
	return verifyCosignExec(ref, publicKeyPath, keylessOpts)
}

func readPrivateKeyEntity(keyData []byte) (*openpgp.Entity, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(keyData))
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to parse private key", err)
	}

	if 0 == len(entities) {
		return nil, keramoserr.NewError(keramoserr.ErrSignature, "no key found in private key file")
	}

	entity := entities[0]
	if nil == entity.PrivateKey {
		return nil, keramoserr.NewError(keramoserr.ErrSignature, "key file does not contain a private key")
	}
	return entity, nil
}

// decryptEntity decrypts a passphrase-protected private key (and its subkeys)
// in place. An unencrypted key is left as-is. An encrypted key with an empty
// passphrase is a clear error rather than a cryptic failure at signing time.
func decryptEntity(entity *openpgp.Entity, passphrase string) error {
	pw := []byte(passphrase)
	if nil != entity.PrivateKey && entity.PrivateKey.Encrypted {
		if 0 == len(pw) {
			return keramoserr.NewError(keramoserr.ErrSignature,
				"private key is passphrase-protected; provide --passphrase-file")
		}
		if err := entity.PrivateKey.Decrypt(pw); nil != err {
			return keramoserr.WrapError(keramoserr.ErrSignature, "failed to decrypt private key (wrong passphrase?)", err)
		}
	}
	for _, sub := range entity.Subkeys {
		if nil != sub.PrivateKey && sub.PrivateKey.Encrypted && 0 < len(pw) {
			_ = sub.PrivateKey.Decrypt(pw)
		}
	}
	return nil
}

func formatProvenance(name, version, digest string) string {
	return fmt.Sprintf("name: %s\nversion: %s\ndigest: sha256:%s\ntimestamp: %s",
		name, version, digest, time.Now().UTC().Format(time.RFC3339))
}

func clearsignMessage(message string, entity *openpgp.Entity) ([]byte, error) {
	var buf bytes.Buffer

	signingKey, ok := entity.SigningKey(time.Now())
	if !ok {
		return nil, keramoserr.NewError(keramoserr.ErrSignature, "entity has no valid signing key")
	}

	cfg := &packet.Config{
		DefaultHash: crypto.SHA256,
	}

	writer, err := clearsign.Encode(&buf, signingKey.PrivateKey, cfg)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to create clearsign encoder", err)
	}

	if _, err := writer.Write([]byte(message)); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to write message to clearsign encoder", err)
	}

	if err := writer.Close(); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to finalize clearsign encoding", err)
	}

	return buf.Bytes(), nil
}

func verifyProvenance(archivePath, provFilePath string, keyring openpgp.EntityList) error {
	provData, err := os.ReadFile(provFilePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to read provenance file", err)
	}

	block, _ := clearsign.Decode(provData)
	if nil == block {
		return keramoserr.NewError(keramoserr.ErrSignature, "provenance file does not contain a valid clearsigned message")
	}

	if _, err := block.VerifySignature(keyring, nil); nil != err {
		return keramoserr.WrapError(keramoserr.ErrSignature, "signature verification failed", err)
	}

	return verifyProvenanceContent(archivePath, block.Plaintext)
}

func verifyProvenanceContent(archivePath string, plaintext []byte) error {
	digest, err := fileDigest(archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrSignature, "failed to compute archive digest for verification", err)
	}

	expectedDigest := "sha256:" + digest
	lines := strings.Split(string(plaintext), "\n")

	var foundDigest string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "digest: ") {
			foundDigest = strings.TrimPrefix(trimmed, "digest: ")
			break
		}
	}

	if "" == foundDigest {
		return keramoserr.NewError(keramoserr.ErrSignature, "provenance file does not contain a digest field")
	}

	if 1 != subtle.ConstantTimeCompare([]byte(foundDigest), []byte(expectedDigest)) {
		return keramoserr.NewErrorf(keramoserr.ErrSignature,
			"provenance digest mismatch: provenance has %s, archive has %s", foundDigest, expectedDigest)
	}

	logger.Debug("provenance verification passed for %s", archivePath)
	return nil
}

func loadKeyringDir(dir string) (openpgp.EntityList, error) {
	entries, err := os.ReadDir(dir)
	if nil != err {
		if os.IsNotExist(err) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrSignature, "keyring directory does not exist: %s", dir)
		}
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to read keyring directory", err)
	}

	var keyring openpgp.EntityList
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		keyPath := filepath.Join(dir, entry.Name())
		entities, readErr := readKeyFile(keyPath)
		if nil != readErr {
			logger.Debug("skipping key file %s: %v", keyPath, readErr)
			continue
		}
		keyring = append(keyring, entities...)
	}

	return keyring, nil
}

func readKeyFile(path string) (openpgp.EntityList, error) {
	data, err := os.ReadFile(path)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to read key file", err)
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrSignature, "failed to parse key file", err)
	}

	return entities, nil
}
