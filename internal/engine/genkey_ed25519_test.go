package engine

import (
	"strings"
	"testing"
)

// TestGenPrivateKeyEd25519 proves ed25519 is implemented (was previously an
// unsupported-type error), emitting a PKCS#8 PEM private key.
func TestGenPrivateKeyEd25519(t *testing.T) {
	out, err := fnGenPrivateKey("ed25519")
	if nil != err {
		t.Fatalf("ed25519 keygen: %v", err)
	}
	pem, ok := out.(string)
	if !ok || !strings.Contains(pem, "BEGIN PRIVATE KEY") {
		t.Fatalf("expected PKCS#8 PEM, got: %v", out)
	}
}

func TestGenPrivateKeyUnknownStillErrors(t *testing.T) {
	if _, err := fnGenPrivateKey("bogus"); nil == err {
		t.Fatal("expected error for unknown key type")
	}
}
