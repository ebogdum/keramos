package release

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
)

// Status represents the state of a release.
type Status string

const (
	StatusDeployed        Status = "deployed"
	StatusSuperseded      Status = "superseded"
	StatusFailed          Status = "failed"
	StatusUninstalling    Status = "uninstalling"
	StatusPendingInstall  Status = "pending-install"
	StatusPendingUpgrade  Status = "pending-upgrade"
	StatusPendingRollback Status = "pending-rollback"
)

// Release represents a deployed keramos package.
type Release struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Revision      int               `json:"revision"`
	Status        Status            `json:"status"`
	Package       PackageRef        `json:"package"`
	Values        map[string]any    `json:"values"`
	UserValues    map[string]any    `json:"userValues,omitempty"`
	Manifest      string            `json:"manifest"`
	Hooks         []HookResult      `json:"hooks,omitempty"`
	HookTemplates map[string]string `json:"hookTemplates,omitempty"` // rendered hook manifests (filename -> body) for rollback re-execution
	Tests         map[string]string `json:"tests,omitempty"`         // rendered test manifests (filename -> body)
	Notes         string            `json:"notes,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Provenance    map[string]string `json:"provenance,omitempty"` // dotted value key -> "source (origin)": where each value was resolved from
	Audit         AuditRecord       `json:"audit,omitempty"`      // who/what/when for this revision
	Info          ReleaseInfo       `json:"info"`
}

// AuditRecord captures provenance metadata for a release revision so
// operators can answer "who applied what, with what flags?" months later.
type AuditRecord struct {
	Action      string    `json:"action,omitempty"`      // install, upgrade, rollback, uninstall
	User        string    `json:"user,omitempty"`        // kubeconfig user / OS username
	Hostname    string    `json:"hostname,omitempty"`    // machine that initiated the operation
	KeramosVersion string    `json:"keramosVersion,omitempty"` // keramos binary version
	KubeContext string    `json:"kubeContext,omitempty"` // active kubeconfig context
	Flags       []string  `json:"flags,omitempty"`       // CLI flags as passed
	ValueFiles  []string  `json:"valueFiles,omitempty"`  // -f files supplied
	Timestamp   time.Time `json:"timestamp,omitempty"`   // operation time
	ParentRev   int       `json:"parentRev,omitempty"`   // previous revision (for upgrades/rollbacks)
}

// PackageRef identifies the package used in a release.
type PackageRef struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion,omitempty"`
}

// ReleaseInfo holds timing and description metadata for a release.
type ReleaseInfo struct {
	FirstDeployed time.Time `json:"firstDeployed"`
	LastDeployed  time.Time `json:"lastDeployed"`
	Description   string    `json:"description,omitempty"`
}

// HookResult records the outcome of a hook execution.
type HookResult struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Status string `json:"status"` // "succeeded", "failed"
}

// Encode serializes a Release to gzip-compressed, base64-encoded JSON.
func Encode(rel *Release) (string, error) {
	jsonData, err := json.Marshal(rel)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRelease, "failed to encode release to JSON", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRelease, "failed to gzip release data", err)
	}
	if err := gz.Close(); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRelease, "failed to close gzip writer", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Decode deserializes a Release from gzip-compressed, base64-encoded JSON.
func Decode(data string) (*Release, error) {
	compressed, err := base64.StdEncoding.DecodeString(data)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to base64 decode release", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to create gzip reader", err)
	}
	defer gz.Close()

	// Bound decompression: refuse gzip bombs that would expand into
	// gigabytes from a small Secret payload. The cap is generous (50 MB)
	// versus the 1 MB encoded-payload limit, so legitimate releases pass.
	const maxDecompressedRelease = 50 * 1024 * 1024
	jsonData, err := io.ReadAll(io.LimitReader(gz, maxDecompressedRelease+1))
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to decompress release data", err)
	}
	if int64(len(jsonData)) > maxDecompressedRelease {
		return nil, keramoserr.NewErrorf(keramoserr.ErrRelease,
			"release decompressed payload exceeds %d-byte safety cap (possible gzip bomb)", maxDecompressedRelease)
	}

	var rel Release
	if err := json.Unmarshal(jsonData, &rel); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to unmarshal release JSON", err)
	}

	return &rel, nil
}
