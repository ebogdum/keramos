package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestSignCommandRequiresArgs(t *testing.T) {
	cmd := newSignCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// No args should fail
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); nil == err {
		t.Error("expected error when no arguments provided")
	}
}

func TestSignCommandRequiresKeyFlag(t *testing.T) {
	cmd := newSignCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	// Arg but no --key flag should fail
	cmd.SetArgs([]string{"some-archive.keramos.tgz"})
	if err := cmd.Execute(); nil == err {
		t.Error("expected error when --key flag is missing")
	}
}

func TestSignCommandWiring(t *testing.T) {
	root := &cobra.Command{Use: "keramos"}
	pkgCmd := newPackageCommand()
	root.AddCommand(pkgCmd)

	// Verify sign is a subcommand of package
	var found bool
	for _, sub := range pkgCmd.Commands() {
		if "sign" == sub.Name() {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'sign' to be a subcommand of 'package'")
	}
}
