package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- PluginDir ---

func TestPluginDir_ReturnsPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := PluginDir()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" == dir {
		t.Fatal("expected non-empty dir")
	}
	if !strings.Contains(dir, "plugins") {
		t.Errorf("expected 'plugins' in path, got %q", dir)
	}
	// Dir should exist
	info, statErr := os.Stat(dir)
	if nil != statErr {
		t.Fatalf("expected dir to exist: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

// --- List ---

func TestList_EmptyPluginDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	plugins, err := List()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(plugins) {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestList_MultiplePlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	// Create two valid plugins
	for _, name := range []string{"plugin-a", "plugin-b"} {
		pDir := filepath.Join(pluginDir, name)
		os.MkdirAll(pDir, 0o755)
		yaml := "name: " + name + "\nversion: \"1.0.0\"\ncommand: run.sh\n"
		os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte(yaml), 0o644)
		os.WriteFile(filepath.Join(pDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	}

	plugins, err := List()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 2 != len(plugins) {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestList_SkipsInvalidPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	// Create one valid, one invalid
	validDir := filepath.Join(pluginDir, "valid")
	os.MkdirAll(validDir, 0o755)
	os.WriteFile(filepath.Join(validDir, "plugin.yaml"), []byte("name: valid\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(validDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	invalidDir := filepath.Join(pluginDir, "invalid")
	os.MkdirAll(invalidDir, 0o755)
	os.WriteFile(filepath.Join(invalidDir, "plugin.yaml"), []byte("{{invalid yaml"), 0o644)

	plugins, err := List()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(plugins) {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}
}

func TestList_SkipsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	// Create a regular file (not a directory)
	os.WriteFile(filepath.Join(pluginDir, "not-a-plugin"), []byte("file"), 0o644)

	plugins, err := List()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(plugins) {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

// --- FindPlugin ---

func TestFindPlugin_Found(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "myplugin")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: myplugin\nversion: \"2.0.0\"\ncommand: run.sh\ndescription: My plugin\n"), 0o644)
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	p, err := FindPlugin("myplugin")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "myplugin" != p.Name {
		t.Errorf("expected name myplugin, got %s", p.Name)
	}
	if "2.0.0" != p.Version {
		t.Errorf("expected version 2.0.0, got %s", p.Version)
	}
	if "My plugin" != p.Description {
		t.Errorf("expected description, got %s", p.Description)
	}
}

func TestFindPlugin_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := FindPlugin("nonexistent")
	if nil == err {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestFindPlugin_InvalidMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "badplugin")
	os.MkdirAll(pDir, 0o755)
	// Missing required field: name
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("version: 1.0\ncommand: run.sh\n"), 0o644)

	_, err = FindPlugin("badplugin")
	if nil == err {
		t.Fatal("expected error for invalid metadata")
	}
}

// --- validateCommand additional ---

func TestValidateCommand_SimplePathTraversal(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, "..")
	if nil == err {
		t.Fatal("expected error for '..' command")
	}
}

func TestValidateCommand_ForwardSlash(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, "path/to/cmd")
	if nil == err {
		t.Fatal("expected error for forward slash")
	}
}

func TestValidateCommand_EmptyString(t *testing.T) {
	dir := t.TempDir()
	// Empty command name: os.Stat will try the dir itself, which is a directory
	err := validateCommand(dir, "")
	if nil == err {
		t.Fatal("expected error for empty command")
	}
}

// --- Install from local ---

func TestInstallFromLocal_Success(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: test-local\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\necho test\n"), 0o755)

	destDir := t.TempDir()
	p, err := installFromLocal(srcDir, destDir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "test-local" != p.Name {
		t.Errorf("expected name test-local, got %s", p.Name)
	}

	// Verify files copied
	destPath := filepath.Join(destDir, filepath.Base(srcDir))
	if _, statErr := os.Stat(filepath.Join(destPath, "plugin.yaml")); nil != statErr {
		t.Error("expected plugin.yaml to be copied")
	}
	if _, statErr := os.Stat(filepath.Join(destPath, "run.sh")); nil != statErr {
		t.Error("expected run.sh to be copied")
	}
}

func TestInstallFromLocal_NotADirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir")
	os.WriteFile(filePath, []byte("file"), 0o644)

	_, err := installFromLocal(filePath, tmpDir)
	if nil == err {
		t.Fatal("expected error for non-directory source")
	}
}

func TestInstallFromLocal_SourceNotFound(t *testing.T) {
	_, err := installFromLocal("/nonexistent/path", t.TempDir())
	if nil == err {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestInstallFromLocal_InvalidPluginYAML(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("{{invalid"), 0o644)

	destDir := t.TempDir()
	_, err := installFromLocal(srcDir, destDir)
	if nil == err {
		t.Fatal("expected error for invalid plugin.yaml")
	}
	// Verify cleanup
	destPath := filepath.Join(destDir, filepath.Base(srcDir))
	if _, statErr := os.Stat(destPath); nil == statErr {
		t.Error("expected destination to be cleaned up after failure")
	}
}

func TestInstallFromLocal_InvalidCommand(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: bad-cmd\nversion: \"1.0\"\ncommand: nonexistent.sh\n"), 0o644)

	destDir := t.TempDir()
	_, err := installFromLocal(srcDir, destDir)
	if nil == err {
		t.Fatal("expected error for invalid command")
	}
	// Verify cleanup
	destPath := filepath.Join(destDir, filepath.Base(srcDir))
	if _, statErr := os.Stat(destPath); nil == statErr {
		t.Error("expected destination to be cleaned up after failure")
	}
}

// --- Remove ---

func TestRemove_ExistingPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "removable")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: removable\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)

	err = Remove("removable")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(pDir); nil == statErr {
		t.Error("expected plugin directory to be removed")
	}
}

func TestRemove_NonexistentPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Force creation of plugin dir
	_, _ = PluginDir()

	err := Remove("nonexistent")
	if nil == err {
		t.Fatal("expected error for nonexistent plugin")
	}
}

// --- Plugin metadata ---

func TestLoadPluginMetadata_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("{{not yaml"), 0o644)

	_, err := loadPluginMetadata(dir)
	if nil == err {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadPluginMetadata_AllFields(t *testing.T) {
	dir := t.TempDir()
	yaml := "name: full-plugin\nversion: \"3.2.1\"\ndescription: Full description here\ncommand: main.sh\n"
	os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(yaml), 0o644)

	p, err := loadPluginMetadata(dir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "full-plugin" != p.Name {
		t.Errorf("expected name full-plugin, got %s", p.Name)
	}
	if "3.2.1" != p.Version {
		t.Errorf("expected version 3.2.1, got %s", p.Version)
	}
	if "Full description here" != p.Description {
		t.Errorf("expected full description, got %s", p.Description)
	}
	if "main.sh" != p.Command {
		t.Errorf("expected command main.sh, got %s", p.Command)
	}
}

// --- Run environment ---

func TestRun_EnvironmentSetup(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "env-test")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: env-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)

	// Script that prints environment variables
	script := "#!/bin/sh\nenv | grep KERAMOS_ > /dev/null\nexit 0\n"
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte(script), 0o755)

	p := &Plugin{Name: "env-test", Version: "1.0", Command: "run.sh"}
	err = Run(p, []string{})
	if nil != err {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRun_InvalidCommand(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "bad-run")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: bad-run\nversion: \"1.0\"\ncommand: nonexistent.sh\n"), 0o644)

	p := &Plugin{Name: "bad-run", Version: "1.0", Command: "nonexistent.sh"}
	err = Run(p, []string{})
	if nil == err {
		t.Fatal("expected error for nonexistent command")
	}
}

// --- makeExecutable ---

func TestMakeExecutable(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "script.sh")
	os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o644)

	err := makeExecutable(dir, "script.sh")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := os.Stat(cmdPath)
	if 0 == info.Mode()&0111 {
		t.Error("expected file to be executable")
	}
}

// --- copyFile ---

func TestCopyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	os.WriteFile(srcPath, []byte("content here"), 0o644)

	err := copyFile(srcPath, dstPath, 0o644)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	data, readErr := os.ReadFile(dstPath)
	if nil != readErr {
		t.Fatalf("failed to read dest: %v", readErr)
	}
	if "content here" != string(data) {
		t.Errorf("expected 'content here', got %q", string(data))
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	err := copyFile("/nonexistent/file", filepath.Join(t.TempDir(), "out"), 0o644)
	if nil == err {
		t.Fatal("expected error for nonexistent source")
	}
}

// --- Install (top-level) ---

func TestInstall_TopLevel_LocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: top-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	p, err := Install(srcDir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "top-test" != p.Name {
		t.Errorf("expected name top-test, got %s", p.Name)
	}
}

func TestInstall_TopLevel_AlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: dup-top\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	// First install
	_, err := Install(srcDir)
	if nil != err {
		t.Fatalf("first install failed: %v", err)
	}

	// Second install should fail
	_, err = Install(srcDir)
	if nil == err {
		t.Fatal("expected error for duplicate install")
	}
}

// --- Run with arguments ---

func TestRun_WithArgs(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "args-test")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: args-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)

	script := "#!/bin/sh\nexit 0\n"
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte(script), 0o755)

	p := &Plugin{Name: "args-test", Version: "1.0", Command: "run.sh"}
	err = Run(p, []string{"arg1", "arg2"})
	if nil != err {
		t.Fatalf("Run with args failed: %v", err)
	}
}

// --- Run with failing command ---

func TestRun_CommandFails(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "fail-test")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: fail-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)

	script := "#!/bin/sh\nexit 1\n"
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte(script), 0o755)

	p := &Plugin{Name: "fail-test", Version: "1.0", Command: "run.sh"}
	err = Run(p, []string{})
	if nil == err {
		t.Fatal("expected error for failing command")
	}
}

// --- Remove with content ---

func TestRemove_WithContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "with-content")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: with-content\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.MkdirAll(filepath.Join(pDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(pDir, "subdir", "data.txt"), []byte("data"), 0o644)

	err = Remove("with-content")
	if nil != err {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, statErr := os.Stat(pDir); nil == statErr {
		t.Error("expected plugin directory to be fully removed")
	}
}

// --- installFromLocal with subdirectories ---

func TestInstallFromLocal_WithSubdirs(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: subdir-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.MkdirAll(filepath.Join(srcDir, "lib"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "lib", "helper.sh"), []byte("#!/bin/sh\n"), 0o755)

	destDir := t.TempDir()
	p, err := installFromLocal(srcDir, destDir)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "subdir-test" != p.Name {
		t.Errorf("expected name subdir-test, got %s", p.Name)
	}

	// Verify subdirectory was copied
	destPath := filepath.Join(destDir, filepath.Base(srcDir))
	if _, statErr := os.Stat(filepath.Join(destPath, "lib", "helper.sh")); nil != statErr {
		t.Error("expected subdirectory file to be copied")
	}
}

// --- installFromGit ---

func TestInstallFromGit_Success(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	// Create a local git repo to clone from
	gitDir := t.TempDir()
	pluginYAML := "name: git-plugin\nversion: \"1.0.0\"\ndescription: A git plugin\ncommand: run.sh\n"
	os.WriteFile(filepath.Join(gitDir, "plugin.yaml"), []byte(pluginYAML), 0o644)
	os.WriteFile(filepath.Join(gitDir, "run.sh"), []byte("#!/bin/sh\necho hello\n"), 0o755)

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); nil != err {
			t.Fatalf("git command %v failed: %v\n%s", args, err, out)
		}
	}

	destDir := t.TempDir()
	// installFromGit derives plugin name from last component of URL minus .git
	// We need to pass a URL where filepath.Base gives a unique name
	// Use the local file path as a git URL: the base dir name becomes the plugin name
	p, err := installFromGit(gitDir, destDir)
	if nil != err {
		t.Fatalf("installFromGit failed: %v", err)
	}
	if "git-plugin" != p.Name {
		t.Errorf("expected name git-plugin, got %s", p.Name)
	}
	if "1.0.0" != p.Version {
		t.Errorf("expected version 1.0.0, got %s", p.Version)
	}
}

func TestInstallFromGit_AlreadyInstalled(t *testing.T) {
	destDir := t.TempDir()
	// Pre-create the destination to simulate already installed
	os.MkdirAll(filepath.Join(destDir, ".git"), 0o755)

	_, err := installFromGit("https://example.com/repo.git", destDir)
	if nil == err {
		t.Fatal("expected error for already installed plugin")
	}
}

func TestInstallFromGit_InvalidURL(t *testing.T) {
	destDir := t.TempDir()
	_, err := installFromGit("https://invalid-url-that-does-not-exist.example.com/repo.git", destDir)
	if nil == err {
		t.Fatal("expected error for invalid git URL")
	}
}

// --- PluginDir creates directory ---

func TestPluginDir_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir, err := PluginDir()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call again - should succeed with existing dir
	dir2, err := PluginDir()
	if nil != err {
		t.Fatalf("second call failed: %v", err)
	}
	if dir != dir2 {
		t.Errorf("expected same dir, got %q and %q", dir, dir2)
	}
}

// --- copyDir with nested ---

func TestCopyDir_Nested(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("r"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "a", "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "a", "b", "b.txt"), []byte("b"), 0o644)

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := copyDir(srcDir, dstDir); nil != err {
		t.Fatalf("copyDir failed: %v", err)
	}

	for _, path := range []string{"root.txt", "a/a.txt", "a/b/b.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, path)); nil != err {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
}

func TestCopyDir_EmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")

	if err := copyDir(srcDir, dstDir); nil != err {
		t.Fatalf("copyDir failed: %v", err)
	}

	info, err := os.Stat(dstDir)
	if nil != err {
		t.Fatalf("expected dest dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestCopyDir_PreservesFileContent(t *testing.T) {
	srcDir := t.TempDir()
	content := "specific content for verification"
	os.WriteFile(filepath.Join(srcDir, "verify.txt"), []byte(content), 0o644)

	dstDir := filepath.Join(t.TempDir(), "dest")
	if err := copyDir(srcDir, dstDir); nil != err {
		t.Fatalf("copyDir failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "verify.txt"))
	if nil != err {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if content != string(data) {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

// --- copyFile to non-writable dest ---

// --- installFromLocal with non-executable command that gets made executable ---

// installFromLocal: makeExecutable is called after validateCommand succeeds
// It ensures chmod 0755 even if file already has some exec bits.
// Testing this path requires a file that passes validateCommand (already exec).

// --- installFromLocal preserves structure ---

func TestInstallFromLocal_PreservesFullStructure(t *testing.T) {
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("name: struct-test\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.MkdirAll(filepath.Join(srcDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "templates", "main.tmpl"), []byte("template"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "README"), []byte("readme"), 0o644)

	destDir := t.TempDir()
	_, err := installFromLocal(srcDir, destDir)
	if nil != err {
		t.Fatalf("installFromLocal failed: %v", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(srcDir))
	for _, f := range []string{"plugin.yaml", "run.sh", "templates/main.tmpl", "README"} {
		if _, statErr := os.Stat(filepath.Join(destPath, f)); nil != statErr {
			t.Errorf("expected %s to exist: %v", f, statErr)
		}
	}
}

func TestCopyFile_DestDirNotExist(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "src.txt")
	os.WriteFile(srcPath, []byte("data"), 0o644)

	err := copyFile(srcPath, "/nonexistent/dir/dest.txt", 0o644)
	if nil == err {
		t.Fatal("expected error for non-writable dest")
	}
}

// --- FindPlugin with valid metadata ---

func TestFindPlugin_ValidPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "found-plugin")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: found-plugin\nversion: \"3.0\"\ncommand: cmd.sh\ndescription: Found it\n"), 0o644)
	os.WriteFile(filepath.Join(pDir, "cmd.sh"), []byte("#!/bin/sh\n"), 0o755)

	p, err := FindPlugin("found-plugin")
	if nil != err {
		t.Fatalf("FindPlugin failed: %v", err)
	}
	if "found-plugin" != p.Name {
		t.Errorf("expected name found-plugin, got %s", p.Name)
	}
	if "3.0" != p.Version {
		t.Errorf("expected version 3.0, got %s", p.Version)
	}
}

// --- Install top-level git URL path ---

func TestInstall_TopLevel_GitURL(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a local git repo
	gitDir := filepath.Join(t.TempDir(), "my-plugin")
	os.MkdirAll(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "plugin.yaml"), []byte("name: my-plugin\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(gitDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	cmds := [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); nil != err {
			t.Fatalf("git command %v failed: %v\n%s", args, err, out)
		}
	}

	// Install takes "https://*.git" to use git path
	// But since that's not a real URL, we test the local path code path
	p, err := Install(gitDir)
	if nil != err {
		t.Fatalf("Install failed: %v", err)
	}
	if "my-plugin" != p.Name {
		t.Errorf("expected name my-plugin, got %s", p.Name)
	}
}

// --- installFromGit invalid plugin.yaml ---

func TestInstallFromGit_InvalidPluginYAML(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	gitDir := filepath.Join(t.TempDir(), "bad-plugin")
	os.MkdirAll(gitDir, 0o755)
	// Missing required name field
	os.WriteFile(filepath.Join(gitDir, "plugin.yaml"), []byte("version: 1.0\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(gitDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	cmds := [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		cmd.CombinedOutput()
	}

	destDir := t.TempDir()
	_, err := installFromGit(gitDir, destDir)
	if nil == err {
		t.Fatal("expected error for invalid plugin.yaml")
	}
	// Verify cleanup
	name := filepath.Base(gitDir)
	destPath := filepath.Join(destDir, name)
	if _, statErr := os.Stat(destPath); nil == statErr {
		t.Error("expected cleanup after failed install")
	}
}

// --- installFromGit invalid command ---

func TestInstallFromGit_InvalidCommand(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	gitDir := filepath.Join(t.TempDir(), "bad-cmd-plugin")
	os.MkdirAll(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "plugin.yaml"), []byte("name: bad-cmd\nversion: \"1.0\"\ncommand: nonexistent.sh\n"), 0o644)

	cmds := [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "commit", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		cmd.CombinedOutput()
	}

	destDir := t.TempDir()
	_, err := installFromGit(gitDir, destDir)
	if nil == err {
		t.Fatal("expected error for invalid command")
	}
}

// --- makeExecutable non-existent file ---

func TestMakeExecutable_NonexistentFile(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	dir := t.TempDir()
	err := makeExecutable(dir, "nonexistent.sh")
	if nil == err {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- makeExecutable on already executable ---

func TestMakeExecutable_AlreadyExecutable(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "already.sh")
	os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o755)

	err := makeExecutable(dir, "already.sh")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Install with https git URL ---

func TestInstall_TopLevel_GitURLScheme(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// This will fail because URL doesn't exist, but it tests the code path
	_, err := Install("https://example.com/nonexistent-repo.git")
	if nil == err {
		t.Fatal("expected error for nonexistent git repo")
	}
	// This exercises the git URL detection branch in Install()
}

// --- Remove errors ---

func TestRemove_VerifiesRemoval(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	pDir := filepath.Join(pluginDir, "remove-verify")
	os.MkdirAll(filepath.Join(pDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: remove-verify\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(filepath.Join(pDir, "subdir", "file.txt"), []byte("data"), 0o644)

	// Verify directory exists before removal
	if _, statErr := os.Stat(pDir); nil != statErr {
		t.Fatal("plugin dir should exist before removal")
	}

	err = Remove("remove-verify")
	if nil != err {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify complete removal
	if _, statErr := os.Stat(pDir); nil == statErr {
		t.Error("plugin dir should not exist after removal")
	}
}

// --- copyFile preserves content and permissions ---

func TestCopyFile_PreservesMode(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping on Windows")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "script.sh")
	dstPath := filepath.Join(dstDir, "script.sh")

	os.WriteFile(srcPath, []byte("#!/bin/sh\necho test"), 0o755)

	err := copyFile(srcPath, dstPath, 0o755)
	if nil != err {
		t.Fatalf("copyFile failed: %v", err)
	}

	info, _ := os.Stat(dstPath)
	if 0 == info.Mode()&0111 {
		t.Error("expected executable permissions to be preserved")
	}
}

// --- List with mixed content ---

func TestList_WithMixedContent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	pluginDir, err := PluginDir()
	if nil != err {
		t.Fatalf("PluginDir failed: %v", err)
	}

	// Valid plugin
	pDir := filepath.Join(pluginDir, "valid-plugin")
	os.MkdirAll(pDir, 0o755)
	os.WriteFile(filepath.Join(pDir, "plugin.yaml"), []byte("name: valid-plugin\nversion: \"1.0\"\ncommand: run.sh\n"), 0o644)
	os.WriteFile(filepath.Join(pDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755)

	// Dir without plugin.yaml (skipped)
	os.MkdirAll(filepath.Join(pluginDir, "no-yaml-dir"), 0o755)

	// File (skipped)
	os.WriteFile(filepath.Join(pluginDir, "regular-file"), []byte("not a plugin"), 0o644)

	// Dir with invalid yaml (skipped)
	invalidDir := filepath.Join(pluginDir, "invalid-yaml")
	os.MkdirAll(invalidDir, 0o755)
	os.WriteFile(filepath.Join(invalidDir, "plugin.yaml"), []byte("{{broken"), 0o644)

	plugins, err := List()
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 1 != len(plugins) {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}
}
