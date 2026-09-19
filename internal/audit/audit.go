// Package audit collects provenance metadata for release operations.
package audit

import (
	"os"
	"os/user"
	"time"

	"github.com/ebogdum/keramos/v3/internal/release"
)

// Capture builds an AuditRecord for the current process. It reads a few
// environment markers and the OS user; callers add CLI-derived fields like
// flags and value files.
func Capture(action string, parentRev int) release.AuditRecord {
	rec := release.AuditRecord{
		Action:    action,
		Timestamp: time.Now().UTC(),
		ParentRev: parentRev,
	}
	if u, err := user.Current(); nil == err {
		rec.User = u.Username
	}
	if h, err := os.Hostname(); nil == err {
		rec.Hostname = h
	}
	rec.KeramosVersion = toolVersion
	rec.KubeContext = kubeContext
	if v := os.Getenv("KERAMOS_VERSION"); "" != v {
		rec.KeramosVersion = v
	}
	if ctx := os.Getenv("KERAMOS_KUBECONTEXT"); "" != ctx {
		rec.KubeContext = ctx
	}
	return rec
}

var (
	toolVersion string
	kubeContext string
)

func SetProvenance(version, context string) {
	toolVersion = version
	if "" != context {
		kubeContext = context
	}
}

func SetKubeContext(context string) {
	if "" != context {
		kubeContext = context
	}
}

// WithFlags attaches the CLI flag list (already redacted by the caller —
// callers are expected to scrub --password and similar before passing).
func WithFlags(rec release.AuditRecord, flags []string) release.AuditRecord {
	rec.Flags = append([]string{}, flags...)
	return rec
}

// WithValueFiles attaches the -f file paths.
func WithValueFiles(rec release.AuditRecord, files []string) release.AuditRecord {
	rec.ValueFiles = append([]string{}, files...)
	return rec
}
