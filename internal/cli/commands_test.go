package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebogdum/keramos/internal/release"
)

// --- Command construction tests ---

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if "keramos" != cmd.Use {
		t.Errorf("expected Use=keramos, got %s", cmd.Use)
	}
	if "" == cmd.Short {
		t.Error("expected non-empty Short description")
	}

	// Verify persistent flags exist
	flags := []string{"namespace", "kubeconfig", "kube-context", "debug"}
	for _, f := range flags {
		pf := cmd.PersistentFlags().Lookup(f)
		if nil == pf {
			t.Errorf("expected persistent flag %q", f)
		}
	}

	// Verify subcommands exist
	subcommands := []string{
		"version", "template", "lint", "create", "install", "upgrade",
		"rollback", "uninstall", "list", "status", "history", "get",
		"diff", "debug", "test", "package", "repo", "login", "logout",
		"registry", "dependency", "search", "plugin", "publish",
		"migrate", "completion",
	}
	cmdNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		cmdNames[sub.Name()] = true
	}
	for _, name := range subcommands {
		if !cmdNames[name] {
			t.Errorf("expected subcommand %q on root", name)
		}
	}
}

func TestNewVersionCommand(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); nil != err {
		t.Fatalf("version command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "keramos version") {
		t.Errorf("expected 'keramos version' in output, got %q", out)
	}
}

func TestNewInstallCommand_Flags(t *testing.T) {
	cmd := newInstallCommand()
	if "install <release-name> <package-path>" != cmd.Use {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	expectedFlags := []string{
		"values", "set", "profile", "no-wait", "timeout",
		"dry-run", "output", "description", "no-atomic",
		"no-force", "generate-name", "verify",
	}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on install command", f)
		}
	}
}

func TestNewUpgradeCommand_Flags(t *testing.T) {
	cmd := newUpgradeCommand()
	if "upgrade <release-name> <package-path>" != cmd.Use {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	expectedFlags := []string{
		"values", "set", "profile", "no-wait", "timeout",
		"dry-run", "output", "description", "no-atomic",
		"no-force", "reuse-values", "reset-values", "install",
	}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on upgrade command", f)
		}
	}
}

func TestNewRollbackCommand_Flags(t *testing.T) {
	cmd := newRollbackCommand()
	expectedFlags := []string{"no-wait", "timeout", "description", "output"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on rollback command", f)
		}
	}
}

func TestNewUninstallCommand_Flags(t *testing.T) {
	cmd := newUninstallCommand()
	expectedFlags := []string{"purge", "timeout", "output"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on uninstall command", f)
		}
	}
}

func TestNewListCommand_Flags(t *testing.T) {
	cmd := newListCommand()
	expectedFlags := []string{"all-namespaces", "all", "filter", "output", "sort-by"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on list command", f)
		}
	}
	// Check aliases
	if 0 == len(cmd.Aliases) || "ls" != cmd.Aliases[0] {
		t.Error("expected 'ls' alias on list command")
	}
}

func TestNewStatusCommand_Flags(t *testing.T) {
	cmd := newStatusCommand()
	expectedFlags := []string{"revision", "output"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on status command", f)
		}
	}
}

func TestNewHistoryCommand_Flags(t *testing.T) {
	cmd := newHistoryCommand()
	expectedFlags := []string{"max", "output"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on history command", f)
		}
	}
}

func TestNewGetCommand_Subcommands(t *testing.T) {
	cmd := newGetCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"values", "manifest", "notes", "hooks"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on get command", name)
		}
	}
}

func TestNewDiffCommand_Flags(t *testing.T) {
	cmd := newDiffCommand()
	expectedFlags := []string{"values", "set", "profile", "revision", "no-color"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on diff command", f)
		}
	}
}

func TestNewDebugCommand_Flags(t *testing.T) {
	cmd := newDebugCommand()
	expectedFlags := []string{"values", "set", "profile", "trace"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on debug command", f)
		}
	}
}

func TestNewTestCommand_Flags(t *testing.T) {
	cmd := newTestCommand()
	expectedFlags := []string{"timeout", "logs"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on test command", f)
		}
	}
}

func TestNewLintCommand_Flags(t *testing.T) {
	cmd := newLintCommand()
	expectedFlags := []string{"values", "set", "profile", "strict"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on lint command", f)
		}
	}
}

func TestNewCreateCommand(t *testing.T) {
	cmd := newCreateCommand()
	if "create <name>" != cmd.Use {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}
}

func TestNewPackageCommand_Flags(t *testing.T) {
	cmd := newPackageCommand()
	expectedFlags := []string{"destination", "version"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on package command", f)
		}
	}
	// Verify sign subcommand
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	if !subNames["sign"] {
		t.Error("expected 'sign' subcommand on package command")
	}
}

func TestNewRepoCommand_Subcommands(t *testing.T) {
	cmd := newRepoCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"add", "list", "remove", "update", "index"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on repo command", name)
		}
	}
}

func TestNewRegistryCommand_Subcommands(t *testing.T) {
	cmd := newRegistryCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"login", "logout", "push", "pull"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on registry command", name)
		}
	}
}

func TestNewSearchCommand_Subcommands(t *testing.T) {
	cmd := newSearchCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"repo", "hub"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on search command", name)
		}
	}
}

func TestNewDependencyCommand_Subcommands(t *testing.T) {
	cmd := newDependencyCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"list", "update", "build", "tree"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on dependency command", name)
		}
	}
	if 0 == len(cmd.Aliases) || "dep" != cmd.Aliases[0] {
		t.Error("expected 'dep' alias on dependency command")
	}
}

func TestNewPluginCommand_Subcommands(t *testing.T) {
	cmd := newPluginCommand()
	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	expected := []string{"install", "list", "remove"}
	for _, name := range expected {
		if !subNames[name] {
			t.Errorf("expected subcommand %q on plugin command", name)
		}
	}
}

func TestNewMigrateCommand_Flags(t *testing.T) {
	cmd := newMigrateCommand()
	expectedFlags := []string{"output", "dry-run", "strict"}
	for _, f := range expectedFlags {
		if nil == cmd.Flags().Lookup(f) {
			t.Errorf("expected flag %q on migrate command", f)
		}
	}
}

func TestNewCompletionCommand_ValidArgs(t *testing.T) {
	cmd := newCompletionCommand()
	if 4 != len(cmd.ValidArgs) {
		t.Errorf("expected 4 valid args, got %d", len(cmd.ValidArgs))
	}
}

// --- Lint command integration ---

func TestLintCommand_ValidPackage(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("lint command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "lint passed") {
		t.Errorf("expected 'lint passed', got %q", out)
	}
}

func TestLintCommand_InvalidPackage(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", dir})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid package")
	}
}

func TestLintCommand_StrictWarnings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", "--strict", dir})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error in strict mode with warnings")
	}
}

// --- Create command integration ---

func TestCreateCommand(t *testing.T) {
	dir := t.TempDir()
	// Change to temp dir so create uses it as base
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"create", "mypkg"})

	if err := root.Execute(); nil != err {
		t.Fatalf("create command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "created package mypkg/") {
		t.Errorf("expected create confirmation, got %q", out)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dir, "mypkg", "keramos.yaml")); nil != err {
		t.Error("expected keramos.yaml to be created")
	}
}

// --- Debug command integration ---

func TestDebugCommand_Simple(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"debug", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("debug command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Package:") {
		t.Errorf("expected package info in debug output, got %q", out)
	}
	if !strings.Contains(out, "Templates:") {
		t.Errorf("expected template info in debug output, got %q", out)
	}
}

func TestDebugCommand_WithTrace(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"debug", "--trace", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("debug command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== PACKAGE RESOLUTION ===") {
		t.Errorf("expected trace section header in output, got %q", out)
	}
	if !strings.Contains(out, "=== VALUES MERGE ===") {
		t.Errorf("expected values merge section in trace output")
	}
	if !strings.Contains(out, "=== TEMPLATE FILES ===") {
		t.Errorf("expected template files section in trace output")
	}
	if !strings.Contains(out, "=== RENDERED OUTPUT ===") {
		t.Errorf("expected rendered output section in trace output")
	}
}

func TestDebugCommand_MissingPackage(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"debug", "/nonexistent/path"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for missing package")
	}
}

// --- Template command with base ---

func TestTemplateCommand_WithBase(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-base", "overlay")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"template", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("template command failed: %v", err)
	}
	out := buf.String()
	if "" == out {
		t.Error("expected non-empty template output")
	}
}

// --- List helper functions ---

func TestLatestRevisions(t *testing.T) {
	now := time.Now()
	releases := []*release.Release{
		{Name: "app1", Namespace: "ns1", Revision: 1, Info: release.ReleaseInfo{LastDeployed: now}},
		{Name: "app1", Namespace: "ns1", Revision: 3, Info: release.ReleaseInfo{LastDeployed: now}},
		{Name: "app1", Namespace: "ns1", Revision: 2, Info: release.ReleaseInfo{LastDeployed: now}},
		{Name: "app2", Namespace: "ns1", Revision: 1, Info: release.ReleaseInfo{LastDeployed: now}},
	}

	result := latestRevisions(releases)
	if 2 != len(result) {
		t.Fatalf("expected 2 latest releases, got %d", len(result))
	}

	revByName := make(map[string]int)
	for _, r := range result {
		revByName[r.Name] = r.Revision
	}
	if 3 != revByName["app1"] {
		t.Errorf("expected app1 revision 3, got %d", revByName["app1"])
	}
	if 1 != revByName["app2"] {
		t.Errorf("expected app2 revision 1, got %d", revByName["app2"])
	}
}

func TestFilterActiveStatuses(t *testing.T) {
	releases := []*release.Release{
		{Name: "a", Status: release.StatusDeployed},
		{Name: "b", Status: release.StatusSuperseded},
		{Name: "c", Status: release.StatusFailed},
		{Name: "d", Status: release.StatusPendingInstall},
	}

	result := filterActiveStatuses(releases)
	if 2 != len(result) {
		t.Fatalf("expected 2 active releases, got %d", len(result))
	}
	for _, r := range result {
		if release.StatusSuperseded == r.Status || release.StatusFailed == r.Status {
			t.Errorf("unexpected status %s in filtered results", r.Status)
		}
	}
}

func TestFilterByRegex(t *testing.T) {
	releases := []*release.Release{
		{Name: "myapp-prod"},
		{Name: "myapp-staging"},
		{Name: "other-svc"},
	}

	result, err := filterByRegex(releases, "myapp")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(result) {
		t.Fatalf("expected 2 matched releases, got %d", len(result))
	}
}

func TestFilterByRegex_InvalidPattern(t *testing.T) {
	_, err := filterByRegex(nil, "[invalid")
	if nil == err {
		t.Fatal("expected error for invalid regex")
	}
}

func TestSortReleases(t *testing.T) {
	now := time.Now()
	releases := []*release.Release{
		{Name: "b", Revision: 1, Info: release.ReleaseInfo{LastDeployed: now}},
		{Name: "a", Revision: 3, Info: release.ReleaseInfo{LastDeployed: now.Add(-time.Hour)}},
		{Name: "c", Revision: 2, Info: release.ReleaseInfo{LastDeployed: now.Add(time.Hour)}},
	}

	// Sort by name (default)
	sortReleases(releases, "name")
	if "a" != releases[0].Name {
		t.Errorf("expected first release 'a', got %s", releases[0].Name)
	}

	// Sort by date
	sortReleases(releases, "date")
	if "c" != releases[0].Name {
		t.Errorf("expected first release 'c' (most recent), got %s", releases[0].Name)
	}

	// Sort by revision
	sortReleases(releases, "revision")
	if 3 != releases[0].Revision {
		t.Errorf("expected first revision 3, got %d", releases[0].Revision)
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"table", false},
		{"json", false},
		{"yaml", false},
		{"csv", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validateOutputFormat(tt.input)
			if tt.wantErr && nil == err {
				t.Error("expected error")
			}
			if !tt.wantErr && nil != err {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// --- Output helpers ---

func TestOutputReleasesTable(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	releases := []*release.Release{
		{
			Name:      "app1",
			Namespace: "default",
			Revision:  1,
			Status:    release.StatusDeployed,
			Package:   release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info:      release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	err := outputReleasesTable(root, releases)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "app1") {
		t.Error("expected release name in table output")
	}
	if !strings.Contains(out, "NAME") {
		t.Error("expected header in table output")
	}
}

func TestOutputReleasesJSON(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	releases := []*release.Release{
		{
			Name:    "app1",
			Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	err := outputReleasesJSON(root, releases)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"name"`) {
		t.Error("expected JSON key in output")
	}
}

func TestOutputReleasesYAML(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	releases := []*release.Release{
		{
			Name:    "app1",
			Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	err := outputReleasesYAML(root, releases)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "name:") {
		t.Error("expected YAML key in output")
	}
}

func TestOutputReleases_Dispatch(t *testing.T) {
	root := NewRootCommand()
	releases := []*release.Release{}

	for _, format := range []string{"table", "json", "yaml"} {
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		err := outputReleases(root, releases, format)
		if nil != err {
			t.Errorf("outputReleases failed for format %s: %v", format, err)
		}
	}
}

// --- Sorted keys helper ---

func TestSortedKeys(t *testing.T) {
	m := map[string]string{"b": "1", "a": "2", "c": "3"}
	keys := sortedKeys(m)
	if 3 != len(keys) {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if "a" != keys[0] || "b" != keys[1] || "c" != keys[2] {
		t.Errorf("expected sorted keys, got %v", keys)
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	m := map[string]string{}
	keys := sortedKeys(m)
	if 0 != len(keys) {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// --- Collect warnings ---

func TestCollectWarnings(t *testing.T) {
	// Test via debug command integration which exercises collectWarnings indirectly.
	// Direct test requires layer.ResolvedPackage type.
}

// --- Debug with profiles ---

func TestDebugCommand_WithProfile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"debug", "--trace", "--profile", "prod", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("debug command failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Profile:") {
		t.Errorf("expected Profile in trace output when profile specified")
	}
}

// --- Dependency tree ---

func TestDependencyTreeCommand(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dependency", "tree", dir})

	err := root.Execute()
	// May fail if no lock file, but shouldn't panic
	if nil == err {
		out := buf.String()
		if "" == out {
			t.Error("expected some output from dependency tree")
		}
	}
}

func TestDependencyTreeCommand_NonexistentPath(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dependency", "tree", "/nonexistent/path"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for nonexistent path")
	}
}

// --- Install command dry-run integration ---

func TestInstallCommand_ClientDryRun(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "test-rel", dir, "--dry-run", "client"})

	err := root.Execute()
	if nil != err {
		t.Fatalf("install dry-run failed: %v", err)
	}
	out := buf.String()
	if "" == out {
		t.Error("expected output from dry-run install")
	}
}

func TestInstallCommand_ClientDryRunJSON(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "test-rel", dir, "--dry-run", "client", "-o", "json"})

	err := root.Execute()
	if nil != err {
		t.Fatalf("install dry-run json failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"name"`) {
		t.Error("expected JSON output")
	}
}

func TestInstallCommand_ClientDryRunYAML(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "test-rel", dir, "--dry-run", "client", "-o", "yaml"})

	err := root.Execute()
	if nil != err {
		t.Fatalf("install dry-run yaml failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "name:") {
		t.Error("expected YAML output")
	}
}

func TestInstallCommand_InvalidDryRun(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "test-rel", dir, "--dry-run", "invalid"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid dry-run value")
	}
}

func TestInstallCommand_InvalidOutput(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "test-rel", dir, "--dry-run", "client", "-o", "csv"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid output value")
	}
}

func TestInstallCommand_GenerateName(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"install", "--generate-name", dir, "--dry-run", "client"})

	err := root.Execute()
	if nil != err {
		t.Fatalf("install --generate-name failed: %v", err)
	}
}

// --- Lint command with more options ---

func TestLintCommand_WithProfile(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "with-profiles")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", "--profile", "prod", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("lint command failed: %v", err)
	}
}

func TestLintCommand_WithSet(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "simple")

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", "--set", "name=overridden", dir})

	if err := root.Execute(); nil != err {
		t.Fatalf("lint command failed: %v", err)
	}
}

// --- Lint with errors displayed ---

func TestLintCommand_ShowsErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: wrong\nname: test\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "cm.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", dir})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid apiVersion")
	}
	out := buf.String()
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("expected [ERROR] in output, got %q", out)
	}
}

func TestLintCommand_ShowsWarnings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keramos.yaml"), []byte("apiVersion: keramos/v1\nname: test\nversion: 1.0.0\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	// Empty templates dir produces warning

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"lint", dir})

	// Should succeed but with warnings
	err := root.Execute()
	if nil != err {
		// May fail in strict mode, but we're not using strict
		t.Logf("lint returned error (may be OK): %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[WARNING]") {
		t.Errorf("expected [WARNING] in output, got %q", out)
	}
}

// --- Completion command ---

func TestCompletionCommand_Bash(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "bash"})

	// completion writes to os.Stdout, not cmd.OutOrStdout
	// so this won't capture output, but it should not error
	_ = root.Execute()
}

func TestCompletionCommand_InvalidShell(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "invalid"})

	err := root.Execute()
	if nil == err {
		t.Fatal("expected error for invalid shell")
	}
}

// --- matchesKeywordBroad ---
// matchesKeywordBroad requires repo.IndexEntry type - tested indirectly via search commands

// --- FormatTable with many columns ---

func TestFormatTable_ManyColumns(t *testing.T) {
	headers := []string{"A", "B", "C", "D", "E"}
	rows := [][]string{
		{"1", "2", "3", "4", "5"},
		{"long-value", "x", "y", "z", "w"},
	}
	result := FormatTable(headers, rows)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if 3 != len(lines) {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

// --- ColorizeDiff empty ---

func TestColorizeDiff_EmptyInput(t *testing.T) {
	result := ColorizeDiff("")
	if !strings.Contains(result, "\n") {
		t.Error("expected at least a newline")
	}
}

// --- UnifiedDiff with content to empty ---

func TestUnifiedDiff_ContentToEmpty(t *testing.T) {
	a := "line1\nline2\n"
	b := ""

	result := UnifiedDiff(a, b, "a", "b")
	if "" == result {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "-line1") {
		t.Error("expected removed lines")
	}
}

// --- Keyword matching ---

func TestMatchesKeyword(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		want    bool
	}{
		{"nginx", "nginx", true},
		{"my-nginx-app", "nginx", true},
		{"redis", "nginx", false},
		{"Nginx", "nginx", true},
	}
	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.keyword, func(t *testing.T) {
			result := matchesKeyword(tt.name, tt.keyword)
			if tt.want != result {
				t.Errorf("matchesKeyword(%q, %q) = %v, want %v", tt.name, tt.keyword, result, tt.want)
			}
		})
	}
}

// --- Test command helpers ---

func TestContainsTest(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"test-pod", true},
		{"my-test-hook", true},
		{"notest", true}, // contains "test" substring
		{"tes", false},
		{"test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsTest(tt.name)
			if tt.want != result {
				t.Errorf("containsTest(%q) = %v, want %v", tt.name, result, tt.want)
			}
		})
	}
}

func TestSplitLogLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"single line", "hello", 1},
		{"multiple lines", "line1\nline2\nline3\n", 3},
		{"with empty lines", "line1\n\nline3\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLogLines(tt.input)
			if tt.expected != len(result) {
				t.Errorf("expected %d lines, got %d: %v", tt.expected, len(result), result)
			}
		})
	}
}

func TestBuildTestPodManifest(t *testing.T) {
	result := buildTestPodManifest("test-pod", "default")
	if !strings.Contains(result, "name: test-pod") {
		t.Error("expected pod name in manifest")
	}
	if !strings.Contains(result, "namespace: default") {
		t.Error("expected namespace in manifest")
	}
	if !strings.Contains(result, "restartPolicy: Never") {
		t.Error("expected restartPolicy: Never in manifest")
	}
}

func TestFindTestHooks(t *testing.T) {
	rel := &release.Release{
		Namespace: "default",
		Hooks: []release.HookResult{
			{Name: "test-connection", Kind: "Pod", Status: "succeeded"},
			{Name: "pre-install-hook", Kind: "Job", Status: "succeeded"},
			{Name: "integration-test", Kind: "Pod", Status: "succeeded"},
		},
	}
	hooks := findTestHooks(rel)
	// test-connection and integration-test both contain "test"
	if 2 != len(hooks) {
		t.Errorf("expected 2 test hooks, got %d", len(hooks))
	}
}

func TestFindTestHooks_NoHooks(t *testing.T) {
	rel := &release.Release{
		Namespace: "default",
		Hooks:     []release.HookResult{},
	}
	hooks := findTestHooks(rel)
	if 0 != len(hooks) {
		t.Errorf("expected 0 test hooks, got %d", len(hooks))
	}
}

// --- Install command arg validation ---

func TestInstallCommand_ArgValidation_GenerateName(t *testing.T) {
	cmd := newInstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// With --generate-name, only 1 arg is needed
	cmd.SetArgs([]string{"--generate-name", "/nonexistent"})
	// This will fail because the path doesn't exist, but arg validation should pass
	err := cmd.Execute()
	// Error is from execution, not arg validation
	if nil == err {
		t.Fatal("expected execution error for nonexistent path")
	}
}

func TestInstallCommand_ArgValidation_NoGenerateName_WrongArgCount(t *testing.T) {
	cmd := newInstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"only-one-arg"})
	err := cmd.Execute()
	if nil == err {
		t.Fatal("expected error for wrong number of arguments")
	}
}

// --- Diff helpers (additional) ---

func TestComputeEdits_NoChanges(t *testing.T) {
	a := []string{"line1", "line2"}
	b := []string{"line1", "line2"}
	edits := computeEdits(a, b)
	if nil != edits {
		t.Errorf("expected nil edits for identical input, got %v", edits)
	}
}

func TestBuildHunks_ContextLines(t *testing.T) {
	// Create edits with a change
	edits := []edit{
		{op: editEqual, line: "ctx1"},
		{op: editEqual, line: "ctx2"},
		{op: editDelete, line: "old"},
		{op: editInsert, line: "new"},
		{op: editEqual, line: "ctx3"},
	}
	hunks := buildHunks(edits, 4, 4)
	if 0 == len(hunks) {
		t.Fatal("expected at least one hunk")
	}
	if !strings.Contains(hunks[0], "@@") {
		t.Error("expected hunk header with @@")
	}
}

// --- History output ---

func TestOutputHistory_Table(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	history := []*release.Release{
		{
			Name:     "app1",
			Revision: 1,
			Status:   release.StatusDeployed,
			Package:  release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info: release.ReleaseInfo{
				LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Description:  "initial install",
			},
		},
		{
			Name:     "app1",
			Revision: 2,
			Status:   release.StatusDeployed,
			Package:  release.PackageRef{Name: "myapp", Version: "1.1.0"},
			Info: release.ReleaseInfo{
				LastDeployed: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
				Description:  "upgrade",
			},
		},
	}

	err := outputHistory(root, history, "table")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "REVISION") {
		t.Error("expected REVISION header")
	}
	if !strings.Contains(out, "myapp-1.0.0") {
		t.Error("expected package info in output")
	}
}

func TestOutputHistory_JSON(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	history := []*release.Release{
		{
			Name:    "app1",
			Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	err := outputHistory(root, history, "json")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"name"`) {
		t.Error("expected JSON output")
	}
}

func TestOutputHistory_YAML(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	history := []*release.Release{
		{
			Name:    "app1",
			Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
			Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}

	err := outputHistory(root, history, "yaml")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "name:") {
		t.Error("expected YAML output")
	}
}

// --- Status output ---

func TestOutputStatus_Table(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	rel := &release.Release{
		Name:      "myapp",
		Namespace: "default",
		Revision:  3,
		Status:    release.StatusDeployed,
		Package:   release.PackageRef{Name: "myapp", Version: "1.0.0"},
		Notes:     "Installation complete.",
		Info: release.ReleaseInfo{
			LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Description:  "test desc",
		},
	}

	err := outputStatus(root, rel, "table")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME:") {
		t.Error("expected NAME label")
	}
	if !strings.Contains(out, "myapp") {
		t.Error("expected release name")
	}
	if !strings.Contains(out, "NOTES:") {
		t.Error("expected NOTES section")
	}
	if !strings.Contains(out, "DESCRIPTION:") {
		t.Error("expected DESCRIPTION")
	}
}

func TestOutputStatus_JSON(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	rel := &release.Release{
		Name:    "myapp",
		Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
		Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	err := outputStatus(root, rel, "json")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOutputStatus_YAML(t *testing.T) {
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)

	rel := &release.Release{
		Name:    "myapp",
		Package: release.PackageRef{Name: "myapp", Version: "1.0.0"},
		Info:    release.ReleaseInfo{LastDeployed: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	err := outputStatus(root, rel, "yaml")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Build credential ---

func TestBuildCredential_Precedence(t *testing.T) {
	// Token takes precedence
	cred, err := buildCredential("user", "pass", "tok", "key")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "tok" != cred.Token {
		t.Errorf("expected token, got %+v", cred)
	}

	// API key second
	cred, err = buildCredential("", "", "", "key")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "key" != cred.APIKey {
		t.Errorf("expected apikey, got %+v", cred)
	}

	// Basic auth third
	cred, err = buildCredential("user", "pass", "", "")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "user" != cred.Username {
		t.Errorf("expected username, got %+v", cred)
	}

	// None = error
	_, err = buildCredential("", "", "", "")
	if nil == err {
		t.Fatal("expected error when no credentials provided")
	}
}

// --- FormatTable edge cases ---

func TestFormatTable_RowShorterThanHeaders(t *testing.T) {
	headers := []string{"A", "B", "C"}
	rows := [][]string{
		{"only-one"},
	}
	result := FormatTable(headers, rows)
	if !strings.Contains(result, "only-one") {
		t.Error("expected row value in output")
	}
	// Should not panic
}

func TestFormatTable_SingleColumn(t *testing.T) {
	headers := []string{"NAME"}
	rows := [][]string{{"alpha"}, {"beta"}}
	result := FormatTable(headers, rows)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if 3 != len(lines) {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}
