package cli

import (
	"os"
	"testing"
)

// TestTransportFlagsSetEnv verifies the global transport opt-in flags set the
// environment variables the fetch/registry code reads, so a flag is exactly
// equivalent to exporting the variable.
func TestTransportFlagsSetEnv(t *testing.T) {
	cases := []struct{ flag, env string }{
		{"allow-plaintext-auth", "KERAMOS_ALLOW_PLAINTEXT_AUTH"},
		{"oci-plain-http", "KERAMOS_OCI_PLAIN_HTTP"},
		{"oci-insecure-skip-tls-verify", "KERAMOS_OCI_INSECURE_SKIP_TLS"},
	}
	for _, c := range cases {
		t.Run(c.flag, func(t *testing.T) {
			t.Setenv(c.env, "") // snapshot for restore; start unset
			cmd := NewRootCommand()
			if err := cmd.PersistentFlags().Set(c.flag, "true"); nil != err {
				t.Fatalf("set flag: %v", err)
			}
			if err := cmd.PersistentPreRunE(cmd, nil); nil != err {
				t.Fatalf("prerun: %v", err)
			}
			if "1" != os.Getenv(c.env) {
				t.Fatalf("flag --%s did not set %s=1 (got %q)", c.flag, c.env, os.Getenv(c.env))
			}
		})
	}
}

// TestTransportFlagsDefaultLeaveEnvAlone verifies an unset flag never clears an
// existing export.
func TestTransportFlagsDefaultLeaveEnvAlone(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "1")
	allowPlaintextAuth = false
	cmd := NewRootCommand()
	if err := cmd.PersistentPreRunE(cmd, nil); nil != err {
		t.Fatalf("prerun: %v", err)
	}
	if "1" != os.Getenv("KERAMOS_ALLOW_PLAINTEXT_AUTH") {
		t.Fatal("an unset flag must not clear an existing env export")
	}
}
