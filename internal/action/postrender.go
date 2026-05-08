package action

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
)

const defaultPostRendererTimeout = 5 * time.Minute

// RunPostRendererPublic exposes the post-renderer pipe for cli/template.
func RunPostRendererPublic(cmdLine, manifest string) (string, error) {
	return runPostRenderer(cmdLine, manifest, defaultPostRendererTimeout)
}

// RunPostRenderersPublic runs a chain of post-renderers, piping each output
// into the next stage's stdin.
func RunPostRenderersPublic(cmdLines []string, manifest string, timeout time.Duration) (string, error) {
	return runPostRenderers(cmdLines, manifest, timeout)
}

// runPostRenderers pipes manifest through each post-renderer in order.
func runPostRenderers(cmdLines []string, manifest string, timeout time.Duration) (string, error) {
	if 0 == timeout {
		timeout = defaultPostRendererTimeout
	}
	current := manifest
	for _, line := range cmdLines {
		out, err := runPostRenderer(line, current, timeout)
		if nil != err {
			return "", err
		}
		current = out
	}
	return current, nil
}

// runPostRenderer pipes the rendered manifest through an external command and
// returns the post-rendered output. The command path may be absolute, relative
// (resolved against PATH), or contain whitespace-separated args.
func runPostRenderer(cmdLine, manifest string, timeout time.Duration) (string, error) {
	cmdLine = strings.TrimSpace(cmdLine)
	if "" == cmdLine {
		return manifest, nil
	}
	parts := strings.Fields(cmdLine)
	bin := parts[0]
	args := parts[1:]

	// Disallow shell metacharacters in the binary path; require an exact path.
	if strings.ContainsAny(bin, ";|&`$<>") {
		return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"post-renderer command %q contains shell metacharacters", bin)
	}
	if !filepath.IsAbs(bin) && !strings.HasPrefix(bin, "./") && !strings.HasPrefix(bin, "../") {
		resolved, lookErr := exec.LookPath(bin)
		if nil != lookErr {
			return "", keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, lookErr,
				"post-renderer %q not found on PATH", bin)
		}
		bin = resolved
	}

	if 0 == timeout {
		timeout = defaultPostRendererTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(manifest)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); nil != runErr {
		if ctx.Err() == context.DeadlineExceeded {
			return "", keramoserr.NewErrorf(keramoserr.ErrInternal,
				"post-renderer %q timed out after %s", bin, timeout)
		}
		return "", keramoserr.WrapErrorf(keramoserr.ErrInternal, runErr,
			"post-renderer %q failed: %s", bin, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
