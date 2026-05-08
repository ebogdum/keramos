package repo

import (
	"crypto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// generateTestKeyPair creates a PGP key pair for testing and writes them to files.
// Returns (privateKeyPath, publicKeyPath, error).
func generateTestKeyPair(dir string) (string, string, error) {
	cfg := &packet.Config{
		DefaultHash: crypto.SHA256,
	}

	entity, err := openpgp.NewEntity("Test User", "test", "test@example.com", cfg)
	if nil != err {
		return "", "", err
	}

	privPath := filepath.Join(dir, "test-private.asc")
	privFile, err := os.Create(privPath)
	if nil != err {
		return "", "", err
	}

	privWriter, err := armor.Encode(privFile, openpgp.PrivateKeyType, nil)
	if nil != err {
		privFile.Close()
		return "", "", err
	}
	if err := entity.SerializePrivate(privWriter, cfg); nil != err {
		privWriter.Close()
		privFile.Close()
		return "", "", err
	}
	privWriter.Close()
	privFile.Close()

	pubPath := filepath.Join(dir, "test-public.asc")
	pubFile, err := os.Create(pubPath)
	if nil != err {
		return "", "", err
	}

	pubWriter, err := armor.Encode(pubFile, openpgp.PublicKeyType, nil)
	if nil != err {
		pubFile.Close()
		return "", "", err
	}
	if err := entity.Serialize(pubWriter); nil != err {
		pubWriter.Close()
		pubFile.Close()
		return "", "", err
	}
	pubWriter.Close()
	pubFile.Close()

	return privPath, pubPath, nil
}

// createTestArchive creates a minimal .keramos.tgz for signing tests.
func createTestArchive(t *testing.T, dir string) string {
	t.Helper()

	pkgDir := filepath.Join(dir, "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); nil != err {
		t.Fatal(err)
	}

	keramosYaml := `apiVersion: keramos/v1
name: testpkg
version: 1.0.0
description: test package
`
	if err := os.WriteFile(filepath.Join(pkgDir, "keramos.yaml"), []byte(keramosYaml), 0644); nil != err {
		t.Fatal(err)
	}

	archivePath, err := PackageArchive(pkgDir, dir, "")
	if nil != err {
		t.Fatal(err)
	}
	return archivePath
}

func TestSignAndVerify(t *testing.T) {
	dir := t.TempDir()

	archivePath := createTestArchive(t, dir)
	privKey, pubKey, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	provPath, err := SignPackage(archivePath, privKey)
	if nil != err {
		t.Fatalf("SignPackage failed: %v", err)
	}

	if !strings.HasSuffix(provPath, ".prov") {
		t.Errorf("expected .prov suffix, got %s", provPath)
	}

	if _, statErr := os.Stat(provPath); nil != statErr {
		t.Fatalf("provenance file not created: %v", statErr)
	}

	// Verify with public key
	if err := VerifySignature(archivePath, provPath, pubKey); nil != err {
		t.Fatalf("VerifySignature failed: %v", err)
	}
}

func TestVerifyTamperedArchive(t *testing.T) {
	dir := t.TempDir()

	archivePath := createTestArchive(t, dir)
	privKey, pubKey, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	provPath, err := SignPackage(archivePath, privKey)
	if nil != err {
		t.Fatal(err)
	}

	// Tamper with the archive by appending data
	f, err := os.OpenFile(archivePath, os.O_APPEND|os.O_WRONLY, 0644)
	if nil != err {
		t.Fatal(err)
	}
	if _, writeErr := f.Write([]byte("tampered")); nil != writeErr {
		f.Close()
		t.Fatal(writeErr)
	}
	f.Close()

	// Verification should fail due to digest mismatch
	verifyErr := VerifySignature(archivePath, provPath, pubKey)
	if nil == verifyErr {
		t.Fatal("expected verification to fail for tampered archive")
	}
	if !strings.Contains(verifyErr.Error(), "digest mismatch") {
		t.Errorf("expected digest mismatch error, got: %v", verifyErr)
	}
}

func TestVerifyWrongKey(t *testing.T) {
	dir := t.TempDir()

	archivePath := createTestArchive(t, dir)
	privKey, _, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	provPath, err := SignPackage(archivePath, privKey)
	if nil != err {
		t.Fatal(err)
	}

	// Generate a different key pair
	dir2 := t.TempDir()
	_, wrongPubKey, err := generateTestKeyPair(dir2)
	if nil != err {
		t.Fatal(err)
	}

	// Verification should fail with wrong key
	verifyErr := VerifySignature(archivePath, provPath, wrongPubKey)
	if nil == verifyErr {
		t.Fatal("expected verification to fail with wrong key")
	}
}

func TestVerifyMissingProvFile(t *testing.T) {
	dir := t.TempDir()

	archivePath := createTestArchive(t, dir)
	_, pubKey, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	missingProv := archivePath + ".prov"
	verifyErr := VerifySignature(archivePath, missingProv, pubKey)
	if nil == verifyErr {
		t.Fatal("expected error for missing provenance file")
	}
}

func TestVerifyFromKeyring(t *testing.T) {
	dir := t.TempDir()
	keyringPath := filepath.Join(dir, "keyring")
	if err := os.MkdirAll(keyringPath, 0755); nil != err {
		t.Fatal(err)
	}

	archivePath := createTestArchive(t, dir)
	privKey, pubKey, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	// Copy public key to keyring directory
	pubData, err := os.ReadFile(pubKey)
	if nil != err {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyringPath, "test.asc"), pubData, 0644); nil != err {
		t.Fatal(err)
	}

	provPath, err := SignPackage(archivePath, privKey)
	if nil != err {
		t.Fatal(err)
	}

	if err := VerifySignatureFromKeyring(archivePath, provPath, keyringPath); nil != err {
		t.Fatalf("VerifySignatureFromKeyring failed: %v", err)
	}
}

func TestVerifyFromEmptyKeyring(t *testing.T) {
	dir := t.TempDir()
	keyringPath := filepath.Join(dir, "keyring")
	if err := os.MkdirAll(keyringPath, 0755); nil != err {
		t.Fatal(err)
	}

	archivePath := createTestArchive(t, dir)
	privKey, _, err := generateTestKeyPair(dir)
	if nil != err {
		t.Fatal(err)
	}

	provPath, err := SignPackage(archivePath, privKey)
	if nil != err {
		t.Fatal(err)
	}

	verifyErr := VerifySignatureFromKeyring(archivePath, provPath, keyringPath)
	if nil == verifyErr {
		t.Fatal("expected error for empty keyring")
	}
}
