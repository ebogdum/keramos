package engine

import (
	"strings"
	"testing"
)

// TestEnvFuncsGatedByDefault proves that a template cannot read the host
// process environment unless the operator explicitly opts in. This closes the
// secret-exfiltration primitive where an untrusted chart copies host secrets
// (AWS_SECRET_ACCESS_KEY, VAULT_TOKEN, ...) into a rendered manifest.
func TestEnvFuncsGatedByDefault(t *testing.T) {
	t.Setenv("SECRET_TOKEN_UNDER_TEST", "s3cr3t-value")
	t.Setenv(renderEnvAccessEnv, "") // ensure gate is closed

	for _, name := range []string{"env", "expandenv"} {
		fn, ok := NewFuncRegistry().Get(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		out, err := fn("SECRET_TOKEN_UNDER_TEST")
		if nil == err {
			t.Fatalf("%s should be disabled by default, returned %v", name, out)
		}
		if s, _ := out.(string); strings.Contains(s, "s3cr3t-value") {
			t.Fatalf("%s leaked the secret while disabled: %q", name, s)
		}
	}
}

// TestEnvFuncsOptIn confirms the functions still work when explicitly enabled,
// so the gate does not silently break legitimate opt-in use.
func TestEnvFuncsOptIn(t *testing.T) {
	t.Setenv("SECRET_TOKEN_UNDER_TEST", "s3cr3t-value")
	t.Setenv(renderEnvAccessEnv, "1")

	fn, _ := NewFuncRegistry().Get("env")
	out, err := fn("SECRET_TOKEN_UNDER_TEST")
	if nil != err {
		t.Fatalf("env with opt-in should succeed: %v", err)
	}
	if "s3cr3t-value" != out {
		t.Fatalf("env opt-in = %v, want the value", out)
	}
}

// TestGetHostByNameGatedByDefault proves render-time DNS egress is disabled
// unless the network opt-in is set, closing the blind out-of-band exfiltration
// channel (pipe a secret into a lookup against an attacker-controlled resolver).
func TestGetHostByNameGatedByDefault(t *testing.T) {
	t.Setenv(renderNetworkEnv, "") // ensure gate is closed

	fn, ok := NewFuncRegistry().Get("getHostByName")
	if !ok {
		t.Fatal("getHostByName not registered")
	}
	// A resolvable name would still be refused before any lookup happens.
	if _, err := fn("example.com"); nil == err {
		t.Fatal("getHostByName should be disabled without network opt-in")
	}
}
