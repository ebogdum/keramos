package plugin

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"gopkg.in/yaml.v3"
)

// validPluginName matches safe plugin names: alphanumeric start, then alphanumeric, dots, hyphens, underscores.
var validPluginName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// validatePluginName rejects names containing path separators, traversal sequences, or other unsafe characters.
func validatePluginName(name string) error {
	if !validPluginName.MatchString(name) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "invalid plugin name %q: must match [a-zA-Z0-9][a-zA-Z0-9._-]*", name)
	}
	return nil
}

// Plugin represents an installed keramos plugin.
type Plugin struct {
	Name        string       `yaml:"name"`
	Version     string       `yaml:"version"`
	Usage       string       `yaml:"usage,omitempty"`
	Description string       `yaml:"description"`
	Command     string       `yaml:"command"`
	IgnoreFlags bool         `yaml:"ignoreFlags,omitempty"`
	Hooks       PluginHooks  `yaml:"hooks,omitempty"`
	Downloaders []Downloader `yaml:"downloaders,omitempty"`
}

// PluginHooks declare commands run on plugin lifecycle events. Each is
// resolved relative to the plugin directory and executed with the plugin's
// environment (KERAMOS_PLUGIN_DIR, KERAMOS_BIN, HELM_*).
type PluginHooks struct {
	Install string `yaml:"install,omitempty"`
	Update  string `yaml:"update,omitempty"`
	Delete  string `yaml:"delete,omitempty"`
}

// Downloader declares a protocol handler implemented by the plugin. The
// `command` (relative to the plugin dir) is invoked as
//
//	command <command> <cert> <key> <ca> <full-url>
//
// where the four positional args are the TLS cert, key, CA file, and the
// full URL the user requested.
type Downloader struct {
	Command   string   `yaml:"command"`
	Protocols []string `yaml:"protocols"`
}

// PluginDir returns the plugin installation directory (~/.config/keramos/plugins/).
func PluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "cannot determine home directory", err)
	}
	dir := filepath.Join(home, ".config", "keramos", "plugins")
	if err := os.MkdirAll(dir, 0o755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "cannot create plugin directory", err)
	}
	return dir, nil
}

// Install installs a plugin from a git URL, archive URL, or local path.
func Install(source string) (*Plugin, error) {
	pluginDir, err := PluginDir()
	if nil != err {
		return nil, err
	}

	if isGitSource(source) {
		return installFromGit(source, pluginDir)
	}
	return installFromLocal(source, pluginDir)
}

// isGitSource recognises sources that point at a git repository: HTTPS git
// URLs, ssh URLs, file:// URLs, and any URL ending in `.git`.
func isGitSource(source string) bool {
	if strings.HasSuffix(source, ".git") {
		return true
	}
	for _, prefix := range []string{"git@", "git://", "ssh://", "file://"} {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}
	return false
}

func installFromGit(url, pluginDir string) (*Plugin, error) {
	// Reject dash-prefixed URLs that git would otherwise interpret as
	// flags. The `--` separator below is belt-and-braces.
	if strings.HasPrefix(url, "-") {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"invalid plugin URL %q: must not start with a dash", url)
	}

	// Derive directory name from git URL
	base := filepath.Base(url)
	name := strings.TrimSuffix(base, ".git")
	if err := validatePluginName(name); nil != err {
		return nil, err
	}
	dest := filepath.Join(pluginDir, name)

	if _, err := os.Stat(dest); nil == err {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin %q is already installed", name)
	}

	// `--` separates options from positional args so the URL cannot be
	// interpreted as a flag even if the dash-prefix check above is ever
	// loosened.
	cmd := exec.Command("git", "clone", "--depth", "1", "--", url, dest)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "git clone failed for %s", url)
	}

	p, err := loadPluginMetadata(dest)
	if nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if err := validatePluginCommand(dest, p.Command); nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if err := makeExecutableIfFile(dest, p.Command); nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if hookErr := runPluginHook(p, dest, p.Hooks.Install); nil != hookErr {
		_ = os.RemoveAll(dest)
		return nil, hookErr
	}

	return p, nil
}

func installFromLocal(source, pluginDir string) (*Plugin, error) {
	info, err := os.Stat(source)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "source path %q not found", source)
	}
	if !info.IsDir() {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "source path %q is not a directory", source)
	}

	name := filepath.Base(source)
	dest := filepath.Join(pluginDir, name)

	if _, statErr := os.Stat(dest); nil == statErr {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin %q is already installed", name)
	}

	if err := copyDir(source, dest); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "failed to copy plugin directory", err)
	}

	p, err := loadPluginMetadata(dest)
	if nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if err := validatePluginCommand(dest, p.Command); nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if err := makeExecutableIfFile(dest, p.Command); nil != err {
		_ = os.RemoveAll(dest)
		return nil, err
	}

	if hookErr := runPluginHook(p, dest, p.Hooks.Install); nil != hookErr {
		_ = os.RemoveAll(dest)
		return nil, hookErr
	}

	return p, nil
}

// List returns all installed plugins.
func List() ([]*Plugin, error) {
	pluginDir, err := PluginDir()
	if nil != err {
		return nil, err
	}

	entries, err := os.ReadDir(pluginDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "cannot read plugin directory", err)
	}

	plugins := make([]*Plugin, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, loadErr := loadPluginMetadata(filepath.Join(pluginDir, entry.Name()))
		if nil != loadErr {
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// Remove uninstalls a plugin by name. Resolves the on-disk directory by
// matching against either the directory basename OR the plugin's
// `metadata.name` field — so `keramos plugin install ./foo` (which writes the
// directory `foo`, but whose plugin.yaml says `name: hello`) can be removed
// with either `keramos plugin remove foo` or `keramos plugin remove hello`.
func Remove(name string) error {
	if err := validatePluginName(name); nil != err {
		return err
	}

	pluginDir, err := PluginDir()
	if nil != err {
		return err
	}

	dest, resErr := resolvePluginDir(pluginDir, name)
	if nil != resErr {
		return resErr
	}

	// Best-effort delete hook before removing the directory.
	if p, loadErr := loadPluginMetadata(dest); nil == loadErr {
		if hookErr := runPluginHook(p, dest, p.Hooks.Delete); nil != hookErr {
			return hookErr
		}
	}

	if err := os.RemoveAll(dest); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "failed to remove plugin %q", name)
	}
	return nil
}

// Update refreshes a plugin in place. For git-tracked plugins it runs
// `git pull --ff-only`; otherwise it reloads metadata so a hand-edited plugin
// directory shows the latest version. Returns the (possibly updated) Plugin.
func Update(name string) (*Plugin, error) {
	if err := validatePluginName(name); nil != err {
		return nil, err
	}
	pluginDir, err := PluginDir()
	if nil != err {
		return nil, err
	}
	dest, resErr := resolvePluginDir(pluginDir, name)
	if nil != resErr {
		return nil, resErr
	}

	if _, gitErr := os.Stat(filepath.Join(dest, ".git")); nil == gitErr {
		cmd := exec.Command("git", "pull", "--ff-only")
		cmd.Dir = dest
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if runErr := cmd.Run(); nil != runErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, runErr, "git pull failed for plugin %s", name)
		}
	}

	p, loadErr := loadPluginMetadata(dest)
	if nil != loadErr {
		return nil, loadErr
	}
	if err := validatePluginCommand(dest, p.Command); nil != err {
		return nil, err
	}
	if err := makeExecutableIfFile(dest, p.Command); nil != err {
		return nil, err
	}
	if hookErr := runPluginHook(p, dest, p.Hooks.Update); nil != hookErr {
		return nil, hookErr
	}
	return p, nil
}

// runPluginHook executes a lifecycle hook (install/update/delete) declared in
// plugin.yaml. POSIX systems use `/bin/sh -c`; Windows uses `cmd /C`. Hook
// stdout/stderr surface to the caller.
//
// SECURITY: the hook string is the plugin author's code and runs with the
// installing user's privileges. The user opted into this by installing the
// plugin, but we log the exact command at INFO level before running so a
// reviewer can see what they granted.
func runPluginHook(p *Plugin, dir, hook string) error {
	hook = strings.TrimSpace(hook)
	if "" == hook {
		return nil
	}
	if err := validatePluginName(p.Name); nil != err {
		return err
	}
	logger.Log("plugin %q: executing lifecycle hook: %s", p.Name, hook)
	var cmd *exec.Cmd
	if "windows" == runtime.GOOS {
		cmd = exec.Command("cmd", "/C", hook)
	} else {
		cmd = exec.Command("/bin/sh", "-c", hook)
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	selfBin, exeErr := os.Executable()
	if nil != exeErr || "" == selfBin {
		selfBin = os.Args[0]
	}
	ns := os.Getenv("KERAMOS_NAMESPACE")
	if "" == ns {
		ns = os.Getenv("HELM_NAMESPACE")
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KERAMOS_PLUGIN_DIR=%s", dir),
		fmt.Sprintf("KERAMOS_BIN=%s", selfBin),
		fmt.Sprintf("KERAMOS_NAMESPACE=%s", ns),
		fmt.Sprintf("KERAMOS_KUBECONFIG=%s", os.Getenv("KUBECONFIG")),
		fmt.Sprintf("HELM_PLUGIN_DIR=%s", dir),
		fmt.Sprintf("HELM_BIN=%s", selfBin),
		fmt.Sprintf("HELM_NAMESPACE=%s", ns),
	)
	if runErr := cmd.Run(); nil != runErr {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, runErr, "plugin %q hook failed", p.Name)
	}
	return nil
}

// Run executes a plugin with the given arguments.
// It sets KERAMOS_PLUGIN_DIR, KERAMOS_BIN, KERAMOS_NAMESPACE, KERAMOS_KUBECONFIG environment variables.
func Run(p *Plugin, args []string) error {
	if err := validatePluginName(p.Name); nil != err {
		return err
	}

	pluginDir, err := PluginDir()
	if nil != err {
		return err
	}

	dir, resErr := resolvePluginDir(pluginDir, p.Name)
	if nil != resErr {
		return resErr
	}

	// Plugin commands have two valid shapes:
	//
	//   1. A simple filename of an executable shipped inside the plugin
	//      directory (legacy keramos form): validateCommand enforces this.
	//   2. A shell expression evaluated against the host shell, with
	//      $KERAMOS_PLUGIN_DIR / $HELM_PLUGIN_DIR / $KERAMOS_BIN exported.
	//      Shape (2) lets one-liners like `command: "echo hello"` work
	//      without shipping an `echo` script. We pick (1) when the first
	//      whitespace-separated token resolves to a file inside the
	//      plugin directory, and (2) otherwise.
	cmd, cmdErr := buildPluginExec(dir, p.Command, args)
	if nil != cmdErr {
		return cmdErr
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir

	selfBin, exeErr := os.Executable()
	if nil != exeErr || "" == selfBin {
		selfBin = os.Args[0]
	}
	ns := os.Getenv("KERAMOS_NAMESPACE")
	if "" == ns {
		ns = os.Getenv("HELM_NAMESPACE")
	}
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KERAMOS_PLUGIN_DIR=%s", dir),
		fmt.Sprintf("KERAMOS_BIN=%s", selfBin),
		fmt.Sprintf("KERAMOS_NAMESPACE=%s", ns),
		fmt.Sprintf("KERAMOS_KUBECONFIG=%s", os.Getenv("KUBECONFIG")),
		// Legacy env-var aliases accepted for plugins that read them.
		fmt.Sprintf("HELM_PLUGIN_DIR=%s", dir),
		fmt.Sprintf("HELM_BIN=%s", selfBin),
		fmt.Sprintf("HELM_NAMESPACE=%s", ns),
	)

	if err := cmd.Run(); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "plugin %q exited with error", p.Name)
	}
	return nil
}

// FindPlugin looks up a plugin by name.
func FindPlugin(name string) (*Plugin, error) {
	if err := validatePluginName(name); nil != err {
		return nil, err
	}

	pluginDir, err := PluginDir()
	if nil != err {
		return nil, err
	}

	dir, resErr := resolvePluginDir(pluginDir, name)
	if nil != resErr {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin %q not found", name)
	}
	return loadPluginMetadata(dir)
}

func loadPluginMetadata(dir string) (*Plugin, error) {
	metaPath := filepath.Join(dir, "plugin.yaml")
	data, err := os.ReadFile(metaPath)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "plugin.yaml not found in %s", dir)
	}

	// Strict decode so unknown top-level keys are rejected at install time
	// rather than silently ignored.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var p Plugin
	if err := dec.Decode(&p); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrParse, "invalid plugin.yaml", err)
	}

	if err := validatePluginMetadata(&p); nil != err {
		return nil, err
	}
	return &p, nil
}

// validatePluginMetadata enforces required fields and reasonable bounds on
// downloader declarations. Returns the first violation found.
func validatePluginMetadata(p *Plugin) error {
	if "" == p.Name {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "plugin.yaml missing required field: name")
	}
	if err := validatePluginName(p.Name); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "plugin.yaml contains invalid name %q", p.Name)
	}
	if "" == p.Command {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "plugin.yaml missing required field: command")
	}
	for i, dl := range p.Downloaders {
		if "" == dl.Command {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin.yaml downloader[%d] missing 'command'", i)
		}
		if 0 == len(dl.Protocols) {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin.yaml downloader[%d] declares no protocols", i)
		}
	}
	return nil
}

// buildPluginExec resolves the plugin's `command:` to an *exec.Cmd. If the
// first token is a file inside the plugin directory the command runs that
// binary directly; otherwise the entire string is invoked through the host
// shell so `command: "echo hello"` and other one-liners just work.
func buildPluginExec(dir, command string, args []string) (*exec.Cmd, error) {
	first := command
	if idx := strings.IndexAny(first, " \t"); idx >= 0 {
		first = first[:idx]
	}
	if !strings.ContainsAny(first, `/\`) && !strings.Contains(first, "..") {
		candidate := filepath.Join(dir, first)
		if _, statErr := os.Lstat(candidate); nil == statErr {
			if err := validateCommand(dir, first); nil != err {
				return nil, err
			}
			rest := strings.TrimSpace(command[len(first):])
			cmdArgs := args
			if "" != rest {
				cmdArgs = append([]string{rest}, args...)
			}
			return exec.Command(candidate, cmdArgs...), nil
		}
	}
	if "windows" == runtime.GOOS {
		var sb strings.Builder
		sb.WriteString(command)
		for _, a := range args {
			sb.WriteByte(' ')
			sb.WriteString(a)
		}
		return exec.Command("cmd", "/C", sb.String()), nil
	}
	var sb strings.Builder
	sb.WriteString(command)
	for _, a := range args {
		sb.WriteByte(' ')
		sb.WriteString(shellQuote(a))
	}
	return exec.Command("/bin/sh", "-c", sb.String()), nil
}

func shellQuote(s string) string {
	if "" == s {
		return "''"
	}
	if !strings.ContainsAny(s, ` $"'\\` + "\t\n") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// resolvePluginDir looks up an installed plugin by either directory basename
// or by `metadata.name` from its plugin.yaml. Returns the absolute directory.
func resolvePluginDir(pluginRoot, query string) (string, error) {
	// Direct hit on directory name
	direct := filepath.Join(pluginRoot, query)
	if _, statErr := os.Stat(direct); nil == statErr {
		return direct, nil
	}
	// Otherwise scan every installed plugin for one whose plugin.yaml `name`
	// matches the query.
	entries, readErr := os.ReadDir(pluginRoot)
	if nil != readErr {
		return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin %q is not installed", query)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(pluginRoot, e.Name())
		p, loadErr := loadPluginMetadata(candidate)
		if nil != loadErr {
			continue
		}
		if p.Name == query {
			return candidate, nil
		}
	}
	return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin %q is not installed", query)
}

// validatePluginCommand accepts both forms:
//   - simple-filename: a single token with no whitespace/path-separators that
//     resolves to a file inside the plugin directory (strict legacy form);
//   - shell-form: a string containing whitespace or a shell metachar, which
//     is exec'd via the host shell as-is.
//
// A simple-filename that does NOT resolve to a file in the plugin dir is an
// error — the operator clearly intended a binary lookup, and silently falling
// through to shell exec hides typos like `command: nonexistnt.sh`.
func validatePluginCommand(pluginDir, command string) error {
	if "" == strings.TrimSpace(command) {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "plugin command is empty")
	}
	if strings.ContainsAny(command, " \t") || isShellExpression(command) {
		return nil
	}
	return validateCommand(pluginDir, command)
}

// isShellExpression returns true when the string contains characters that
// only make sense to a shell (pipes, redirects, command substitution, env
// var expansion). When present we let /bin/sh do the parsing.
func isShellExpression(s string) bool {
	return strings.ContainsAny(s, "|&;<>$`(){}*?")
}

// validateCommand validates that a plugin command is safe to execute:
// - Must not contain path separators (simple filename only)
// - Must exist within the plugin directory
// - Must be executable (on non-Windows)
func validateCommand(pluginDir, command string) error {
	// Reject path separators to prevent directory traversal
	if strings.ContainsAny(command, `/\`) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin command %q must not contain path separators", command)
	}

	// Reject path components like ".."
	if strings.Contains(command, "..") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin command %q must be a simple filename", command)
	}

	cmdPath := filepath.Join(pluginDir, command)

	info, err := os.Lstat(cmdPath)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "plugin command %q not found in plugin directory", command)
	}

	if 0 != info.Mode()&os.ModeSymlink {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin command %q is a symlink, which is not allowed", command)
	}

	if info.IsDir() {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin command %q is a directory, not a file", command)
	}

	// On non-Windows, verify the file is executable
	if "windows" != runtime.GOOS {
		mode := info.Mode()
		if 0 == mode&0111 {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "plugin command %q is not executable", command)
		}
	}

	return nil
}

// makeExecutableIfFile is the dual of validatePluginCommand: only chmod the
// command's first token when it actually refers to a file in the plugin dir.
// Shell-form commands ("echo hello") have no file to chmod and must skip.
func makeExecutableIfFile(dir, command string) error {
	first := command
	if idx := strings.IndexAny(first, " \t"); idx >= 0 {
		first = first[:idx]
	}
	if strings.ContainsAny(first, `/\`) || strings.Contains(first, "..") {
		return nil
	}
	candidate := filepath.Join(dir, first)
	if _, statErr := os.Lstat(candidate); nil != statErr {
		return nil
	}
	return makeExecutable(dir, first)
}

func makeExecutable(dir, command string) error {
	if "windows" == runtime.GOOS {
		return nil
	}
	cmdPath := filepath.Join(dir, command)
	if err := os.Chmod(cmdPath, 0o755); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot make %s executable", command)
	}
	return nil
}

// copyDir copies a directory tree refusing to follow symlinks. A symlink
// inside `src` would otherwise let an attacker copy `/etc/shadow`, ssh keys,
// or other host files into the plugin install directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if nil != err {
			return err
		}

		// Re-stat without following symlinks; refuse anything that resolves
		// to a symlink at this level (filepath.Walk's `info` may already be
		// the symlink-resolved target on some platforms).
		lstat, lstatErr := os.Lstat(path)
		if nil != lstatErr {
			return lstatErr
		}
		if 0 != lstat.Mode()&os.ModeSymlink {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"plugin source contains symlink %q; symlinks are refused during install", path)
		}

		relPath, relErr := filepath.Rel(src, path)
		if nil != relErr {
			return relErr
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		return copyFile(path, destPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if nil != err {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if nil != err {
		return err
	}

	if _, err = io.Copy(out, in); nil != err {
		out.Close()
		return err
	}
	return out.Close()
}
