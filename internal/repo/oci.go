package repo

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bytes"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/netguard"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	keramosMediaType       = "application/vnd.keramos.package.v1.tar+gzip"
	keramosConfigMediaType = "application/vnd.keramos.config.v1+json"
)

// OCIRegistry handles push/pull to OCI-compliant registries.
type OCIRegistry struct {
	// PlainHTTP forces plaintext (http://) instead of TLS — for local
	// registries during development.
	PlainHTTP bool
	// InsecureSkipTLSVerify disables certificate validation; surfaced via
	// `--insecure-skip-tls-verify` on the relevant CLI commands.
	InsecureSkipTLSVerify bool
}

// OCIRegistryForRef builds an OCIRegistry for an oci:// reference, honoring the
// operator's opt-ins: env vars (KERAMOS_OCI_PLAIN_HTTP / KERAMOS_OCI_INSECURE_SKIP_TLS)
// and a per-host insecure flag recorded via `keramos login --insecure`.
func OCIRegistryForRef(ref string) *OCIRegistry {
	reg := &OCIRegistry{
		PlainHTTP:             "1" == os.Getenv("KERAMOS_OCI_PLAIN_HTTP"),
		InsecureSkipTLSVerify: "1" == os.Getenv("KERAMOS_OCI_INSECURE_SKIP_TLS"),
	}
	host := ociHost(ref)
	if store, err := LoadCredentialStore(); nil == err && "" != host {
		if cred, ok := store.GetForHost(host); ok && cred.Insecure {
			reg.InsecureSkipTLSVerify = true
		}
	}
	return reg
}

// ociHost extracts the registry host from an oci:// reference.
func ociHost(ref string) string {
	trimmed := strings.TrimPrefix(ref, "oci://")
	if i := strings.IndexAny(trimmed, "/"); i >= 0 {
		trimmed = trimmed[:i]
	}
	return trimmed
}

// crossHostRedirectBlock refuses redirects that change host so credentials and
// mTLS material cannot leak to an attacker-chosen target.
func crossHostRedirectBlock(req *http.Request, via []*http.Request) error {
	if 0 == len(via) {
		return nil
	}
	if req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("redirect to different host %q blocked (original: %q)", req.URL.Host, via[0].URL.Host)
	}
	return nil
}

// ociTransport builds an http.Transport whose dialer enforces the SSRF
// blocklist (metadata/loopback blocked, private registries allowed) and pins
// TLS 1.2 as the floor. tlsConf supplies cert verification settings.
func ociTransport(tlsConf *tls.Config) *http.Transport {
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         netguard.DialContext(netguard.BlockMetadata, "KERAMOS_ALLOW_INTERNAL_FETCH", defaultConnectTimeout),
		TLSClientConfig:     tlsConf,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

// secureRetryClient is the default OCI client: SSRF-guarded dialer, TLS 1.2
// floor, ORAS retry semantics, and a cross-host-redirect block.
func secureRetryClient() *http.Client {
	return &http.Client{
		Transport:     retry.NewTransport(ociTransport(&tls.Config{MinVersion: tls.VersionTLS12})),
		Timeout:       defaultOverallTimeout,
		CheckRedirect: crossHostRedirectBlock,
	}
}

// insecureRetryClient skips TLS certificate verification (explicit opt-in via
// --insecure-skip-tls-verify) but keeps every other safety property: the
// SSRF-guarded dialer, TLS 1.2 floor, retry semantics, timeouts, and the
// cross-host-redirect block.
func insecureRetryClient() *http.Client {
	tlsConf := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} //nolint:gosec // explicit opt-in via --insecure-skip-tls-verify
	return &http.Client{
		Transport:     retry.NewTransport(ociTransport(tlsConf)),
		Timeout:       defaultOverallTimeout,
		CheckRedirect: crossHostRedirectBlock,
	}
}

// noCredentials returns an auth.CredentialFunc that always returns empty
// credentials, used to suppress Basic Auth over plaintext HTTP unless the
// operator explicitly opts in via KERAMOS_ALLOW_PLAINTEXT_AUTH.
func noCredentials() auth.CredentialFunc {
	return func(_ context.Context, _ string) (auth.Credential, error) {
		return auth.Credential{}, nil
	}
}

// Login authenticates to an OCI registry by storing credentials in the unified credential store.
func (o *OCIRegistry) Login(host, username, password string) error {
	store, err := LoadCredentialStore()
	if nil != err {
		return err
	}

	store.Set(host, Credential{
		Type:     AuthBasic,
		Username: username,
		Password: password,
	})

	if err := store.Save(); nil != err {
		return err
	}

	logger.Debug("logged in to %s", host)
	return nil
}

// Logout removes credentials for an OCI registry from the unified credential store.
func (o *OCIRegistry) Logout(host string) error {
	store, err := LoadCredentialStore()
	if nil != err {
		return err
	}

	if _, ok := store.Get(host); !ok {
		return keramoserr.NewErrorf(keramoserr.ErrRegistry, "not logged in to %s", host)
	}

	store.Remove(host)

	if err := store.Save(); nil != err {
		return err
	}

	logger.Debug("logged out of %s", host)
	return nil
}

// Push pushes a .keramos.tgz to an OCI registry.
func (o *OCIRegistry) Push(archivePath, ref string) error {
	if _, err := os.Stat(archivePath); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrRegistry, err, "archive not found: %s", archivePath)
	}

	// Bound the entire push so a hostile or stalled registry cannot hang
	// keramos indefinitely. ORAS propagates the context through retries.
	ctx, cancel := context.WithTimeout(context.Background(), defaultOverallTimeout)
	defer cancel()

	repo, err := remote.NewRepository(ref)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to parse OCI reference", err)
	}
	repo.PlainHTTP = o.PlainHTTP

	store, err := LoadCredentialStore()
	if nil != err {
		return err
	}

	httpClient := secureRetryClient()
	if o.InsecureSkipTLSVerify {
		httpClient = insecureRetryClient()
	}
	credFn := credentialFuncFromStore(store)
	if o.PlainHTTP {
		// Refuse to send Basic Auth over plaintext HTTP. Operators who
		// genuinely want this must opt in via env (KERAMOS_ALLOW_PLAINTEXT_AUTH).
		if "1" != os.Getenv("KERAMOS_ALLOW_PLAINTEXT_AUTH") {
			credFn = noCredentials()
			logger.Warn("OCI plain-http: credentials suppressed (set KERAMOS_ALLOW_PLAINTEXT_AUTH=1 to override)")
		}
	}
	repo.Client = &auth.Client{
		Client:     httpClient,
		Credential: credFn,
	}

	fs, err := file.New("")
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to create file store", err)
	}
	defer fs.Close()

	fileDesc, err := fs.Add(ctx, filepath.Base(archivePath), keramosMediaType, archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to add file to store", err)
	}

	// Push the file (layer), config blob, and manifest into the same
	// in-memory store. file.Store keeps blobs keyed by filename, not by
	// digest, which breaks oras.Copy's resolve-by-digest contract; using
	// memory.Store avoids that and lets the single oras.Copy call upload
	// the entire content tree to the remote repository in one shot.
	memStore := memory.New()
	fileBytes, readErr := os.ReadFile(archivePath)
	if nil != readErr {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "read archive bytes", readErr)
	}
	layerDesc := ocispec.Descriptor{
		MediaType: keramosMediaType,
		Digest:    fileDesc.Digest,
		Size:      fileDesc.Size,
		Annotations: map[string]string{
			ocispec.AnnotationTitle: filepath.Base(archivePath),
		},
	}
	if pushErr := memStore.Push(ctx, layerDesc, bytes.NewReader(fileBytes)); nil != pushErr {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "stage layer", pushErr)
	}
	configBytes := []byte("{}")
	configDesc, err := pushBlob(ctx, keramosConfigMediaType, configBytes, memStore)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to push config blob", err)
	}

	manifest, err := oras.PackManifest(ctx, memStore, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		Layers:              []ocispec.Descriptor{layerDesc},
		ConfigDescriptor:    &configDesc,
		ManifestAnnotations: map[string]string{},
	})
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to pack manifest", err)
	}
	// Tag the manifest in the source store so oras.Copy can resolve the
	// reference name; without this it would only know the manifest by
	// digest, which memory.Store does not Resolve by default.
	if tagErr := memStore.Tag(ctx, manifest, repo.Reference.Reference); nil != tagErr {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "tag manifest in source store", tagErr)
	}

	if _, err := oras.Copy(ctx, memStore, repo.Reference.Reference, repo, repo.Reference.Reference, oras.DefaultCopyOptions); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRegistry, "failed to push to registry", err)
	}

	logger.Debug("pushed %s to %s", archivePath, ref)
	return nil
}

func pushBlob(ctx context.Context, mediaType string, blob []byte, target oras.Target) (ocispec.Descriptor, error) {
	return oras.PushBytes(ctx, target, mediaType, blob)
}

// Pull pulls a package from an OCI registry.
func (o *OCIRegistry) Pull(ref, destDir string) (string, error) {
	// Bound the entire pull so a hostile or stalled registry cannot hang
	// keramos indefinitely. ORAS propagates the context through retries.
	ctx, cancel := context.WithTimeout(context.Background(), defaultOverallTimeout)
	defer cancel()

	repo, err := remote.NewRepository(ref)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to parse OCI reference", err)
	}
	repo.PlainHTTP = o.PlainHTTP

	store, err := LoadCredentialStore()
	if nil != err {
		return "", err
	}

	httpClient := secureRetryClient()
	if o.InsecureSkipTLSVerify {
		httpClient = insecureRetryClient()
	}
	credFn := credentialFuncFromStore(store)
	if o.PlainHTTP {
		// Refuse to send Basic Auth over plaintext HTTP. Operators who
		// genuinely want this must opt in via env (KERAMOS_ALLOW_PLAINTEXT_AUTH).
		if "1" != os.Getenv("KERAMOS_ALLOW_PLAINTEXT_AUTH") {
			credFn = noCredentials()
			logger.Warn("OCI plain-http: credentials suppressed (set KERAMOS_ALLOW_PLAINTEXT_AUTH=1 to override)")
		}
	}
	repo.Client = &auth.Client{
		Client:     httpClient,
		Credential: credFn,
	}

	absDestDir, err := filepath.Abs(destDir)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to resolve destination path", err)
	}

	if err := os.MkdirAll(absDestDir, 0755); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to create destination directory", err)
	}

	fs, err := file.New(absDestDir)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to create file store", err)
	}
	defer fs.Close()

	// Never let the ORAS file store unpack pulled layers. The unpack path is
	// driven by the untrusted remote manifest (io.deis.oras.content.unpack),
	// and its tar extraction is vulnerable to hardlink/symlink escapes outside
	// the destination dir (GHSA hardlink-escape, no fixed oras-go release as of
	// 2.6.1). Keramos never relies on ORAS unpacking — it pulls a single .tgz and
	// extracts it through the hardened archive.go extractor, which rejects link
	// entries that escape the destination. SkipUnpack overrides the annotation.
	fs.SkipUnpack = true

	tag := repo.Reference.Reference
	if "" == tag {
		tag = "latest"
	}

	// Bound each blob: a hostile registry could otherwise declare and serve a
	// huge layer and exhaust the disk (ORAS verifies digest/size but sets no
	// ceiling). Reject oversized descriptors before they are fetched.
	copyOpts := oras.DefaultCopyOptions
	copyOpts.PreCopy = func(_ context.Context, desc ocispec.Descriptor) error {
		if desc.Size > maxArchiveSize {
			return keramoserr.NewErrorf(keramoserr.ErrRegistry,
				"OCI blob %s (%d bytes) exceeds the %d-byte limit", desc.Digest, desc.Size, maxArchiveSize)
		}
		return nil
	}
	manifest, err := oras.Copy(ctx, repo, tag, fs, tag, copyOpts)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to pull from registry", err)
	}

	logger.Debug("pulled %s (digest: %s)", ref, manifest.Digest.String())

	// Find the keramos.tgz file in dest
	entries, err := os.ReadDir(absDestDir)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRegistry, "failed to read destination directory", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) == ".tgz" || filepath.Ext(name) == ".gz" {
			return filepath.Join(absDestDir, name), nil
		}
	}

	return "", keramoserr.NewError(keramoserr.ErrRegistry, "pulled artifact does not contain a keramos archive")
}

// ociCredential is kept for backward-compatible migration from oci-credentials.json.
type ociCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func credentialFuncFromStore(store *CredentialStore) auth.CredentialFunc {
	return func(ctx context.Context, hostport string) (auth.Credential, error) {
		c, ok := store.GetForHost(hostport)
		if !ok {
			return auth.EmptyCredential, nil
		}
		return auth.Credential{
			Username: c.Username,
			Password: c.Password,
		}, nil
	}
}
