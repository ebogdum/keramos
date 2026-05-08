package engine

import (
	"strings"
	"testing"
)

func TestSha256SumDeterministic(t *testing.T) {
	out, _ := fnSha256Sum("hello")
	const expected = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if out != expected {
		t.Errorf("sha256sum = %v, want %v", out, expected)
	}
}

func TestMd5Sum(t *testing.T) {
	out, _ := fnMd5Sum("hello")
	const expected = "5d41402abc4b2a76b9719d911017c592"
	if out != expected {
		t.Errorf("md5sum = %v", out)
	}
}

func TestRandAlphaNumLength(t *testing.T) {
	out, err := fnRandAlphaNum(16)
	if nil != err {
		t.Fatalf("randAlphaNum: %v", err)
	}
	s := out.(string)
	if 16 != len(s) {
		t.Errorf("randAlphaNum length = %d", len(s))
	}
}

func TestUUIDv4Format(t *testing.T) {
	out, _ := fnUUIDv4(nil)
	s := out.(string)
	// 36 chars, four dashes at fixed positions.
	if 36 != len(s) {
		t.Fatalf("uuid len = %d", len(s))
	}
	if '-' != s[8] || '-' != s[13] || '-' != s[18] || '-' != s[23] {
		t.Errorf("uuid layout = %s", s)
	}
}

func TestEncryptDecryptAESRoundTrip(t *testing.T) {
	plain := "hello world"
	enc, err := fnEncryptAES(plain, "passphrase")
	if nil != err {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := fnDecryptAES(enc, "passphrase")
	if nil != err {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != dec {
		t.Errorf("round-trip = %v", dec)
	}
}

func TestGenSelfSignedCert(t *testing.T) {
	out, err := fnGenSelfSignedCert("example.com")
	if nil != err {
		t.Fatalf("genSelfSignedCert: %v", err)
	}
	m := out.(map[string]any)
	if !strings.Contains(m["Cert"].(string), "BEGIN CERTIFICATE") {
		t.Error("cert missing PEM header")
	}
	if !strings.Contains(m["Key"].(string), "BEGIN RSA PRIVATE KEY") {
		t.Error("key missing PEM header")
	}
}
