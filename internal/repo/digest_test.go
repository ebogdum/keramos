package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyDigestValid(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.tgz")
	content := []byte("test package content for digest verification")

	if err := os.WriteFile(filePath, content, 0644); nil != err {
		t.Fatalf("failed to write test file: %v", err)
	}

	h := sha256.Sum256(content)
	expectedDigest := hex.EncodeToString(h[:])

	err := VerifyDigest(filePath, expectedDigest)
	if nil != err {
		t.Fatalf("VerifyDigest should succeed for valid digest: %v", err)
	}

	// File should still exist
	if _, err := os.Stat(filePath); nil != err {
		t.Errorf("file should still exist after successful verification: %v", err)
	}
}

func TestVerifyDigestMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.tgz")
	content := []byte("actual content")

	if err := os.WriteFile(filePath, content, 0644); nil != err {
		t.Fatalf("failed to write test file: %v", err)
	}

	wrongDigest := "0000000000000000000000000000000000000000000000000000000000000000"

	err := VerifyDigest(filePath, wrongDigest)
	if nil == err {
		t.Fatal("VerifyDigest should fail on digest mismatch")
	}

	// File should be deleted on mismatch
	if _, statErr := os.Stat(filePath); !os.IsNotExist(statErr) {
		t.Error("file should be deleted after digest mismatch")
	}
}

func TestVerifyDigestMissingFile(t *testing.T) {
	err := VerifyDigest("/nonexistent/path/file.tgz", "abc123")
	if nil == err {
		t.Fatal("VerifyDigest should fail for missing file")
	}
}

func TestVerifyDigestEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.tgz")

	if err := os.WriteFile(filePath, []byte{}, 0644); nil != err {
		t.Fatalf("failed to write empty file: %v", err)
	}

	// SHA256 of empty content
	h := sha256.Sum256([]byte{})
	emptyDigest := hex.EncodeToString(h[:])

	err := VerifyDigest(filePath, emptyDigest)
	if nil != err {
		t.Fatalf("VerifyDigest should succeed for empty file with correct digest: %v", err)
	}
}

func TestVerifyDigestEmptyFileMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.tgz")

	if err := os.WriteFile(filePath, []byte{}, 0644); nil != err {
		t.Fatalf("failed to write empty file: %v", err)
	}

	err := VerifyDigest(filePath, "wrongdigest")
	if nil == err {
		t.Fatal("VerifyDigest should fail on mismatch even for empty file")
	}

	if _, statErr := os.Stat(filePath); !os.IsNotExist(statErr) {
		t.Error("empty file should be deleted after digest mismatch")
	}
}
