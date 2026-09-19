package plugin

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
)

// FindDownloader scans installed plugins for one that handles the given URL
// protocol. Compound schemes like `s3+https://...` are tried as `s3`, `https`,
// and the full string. Plugins are scanned in deterministic name order so
// repeat runs give the same result on different filesystems.
func FindDownloader(rawURL string) (*Plugin, string, bool) {
	u, err := url.Parse(rawURL)
	if nil != err {
		return nil, "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if "" == scheme {
		return nil, "", false
	}
	candidates := []string{scheme}
	if strings.Contains(scheme, "+") {
		candidates = append(candidates, strings.Split(scheme, "+")...)
	}

	plugins, listErr := List()
	if nil != listErr {
		return nil, "", false
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })

	for _, p := range plugins {
		for _, dl := range p.Downloaders {
			for _, proto := range dl.Protocols {
				for _, want := range candidates {
					if strings.EqualFold(proto, want) {
						return p, dl.Command, true
					}
				}
			}
		}
	}
	return nil, "", false
}

// RunDownloader invokes a downloader plugin to fetch the given URL. The
// plugin invocation contract is:
//
//	command <command> <cert> <key> <ca> <full-url>
//
// where command is one of "getter" (download bytes) or other plugin-specific
// verbs. Returns the bytes the plugin wrote to stdout.
func RunDownloader(rawURL string, certFile, keyFile, caFile string) ([]byte, error) {
	p, downloadCmd, found := FindDownloader(rawURL)
	if !found {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "no downloader plugin handles URL %q", rawURL)
	}
	pluginDir, dirErr := PluginDir()
	if nil != dirErr {
		return nil, dirErr
	}
	dir, resErr := resolvePluginDir(pluginDir, p.Name)
	if nil != resErr {
		return nil, resErr
	}

	// Downloader commands canonically appear as `./script.sh`; strip the
	// leading `./` and confine the result to the plugin directory.
	cleanCmd := strings.TrimPrefix(downloadCmd, "./")
	if strings.ContainsAny(cleanCmd, `/\`) || strings.Contains(cleanCmd, "..") {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"downloader command %q must be a simple filename (or ./<filename>) inside the plugin directory", downloadCmd)
	}
	if err := validateCommand(dir, cleanCmd); nil != err {
		return nil, err
	}
	cmdPath := filepath.Join(dir, cleanCmd)

	args := []string{certFile, keyFile, caFile, rawURL}
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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
		fmt.Sprintf("HELM_PLUGIN_DIR=%s", dir),
		fmt.Sprintf("HELM_BIN=%s", selfBin),
		fmt.Sprintf("HELM_NAMESPACE=%s", ns),
	)
	if runErr := cmd.Run(); nil != runErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, runErr,
			"downloader %q failed: %s", p.Name, strings.TrimSpace(stderr.String()))
	}

	// "File-mode" contract: if the plugin emits a single line beginning
	// with `file://`, the rest is a path on disk that holds the actual bytes.
	// SECURITY: the path is RESTRICTED to the system temp directory so a
	// compromised downloader plugin cannot exfiltrate `/etc/passwd`, SSH keys,
	// or any host file via `file:///etc/passwd`-style payloads.
	out := stdout.Bytes()
	trimmed := strings.TrimSpace(string(out))
	if rest, ok := strings.CutPrefix(trimmed, "file://"); ok {
		clean, evalErr := filepath.EvalSymlinks(rest)
		if nil != evalErr {
			// Fall back to the cleaned path; ReadFile will still error if it does not exist.
			clean = filepath.Clean(rest)
		}
		tempBase := filepath.Clean(os.TempDir())
		// Also accept the per-plugin directory itself for plugins that stage files there.
		pluginBase := filepath.Clean(dir)
		if !pathInside(clean, tempBase) && !pathInside(clean, pluginBase) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"downloader %q file-mode: path %q escapes allowed staging roots (%s, %s)",
				p.Name, rest, tempBase, pluginBase)
		}
		data, readErr := os.ReadFile(clean)
		if nil != readErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr,
				"downloader %q file-mode: cannot read %q", p.Name, rest)
		}
		return data, nil
	}

	return out, nil
}

// pathInside reports whether `target` is the same as or below `root`.
// Both are expected to be cleaned absolute paths.
func pathInside(target, root string) bool {
	rel, err := filepath.Rel(root, target)
	if nil != err {
		return false
	}
	if ".." == rel || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
