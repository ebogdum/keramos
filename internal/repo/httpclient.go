package repo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/netguard"
)

const (
	defaultConnectTimeout = 30 * time.Second
	defaultOverallTimeout = 5 * time.Minute
	maxRetries            = 3
	baseBackoff           = 1 * time.Second
)

// AuthenticatedClient wraps http.Client with credential injection, retries, and User-Agent.
type AuthenticatedClient struct {
	inner   *http.Client
	store   *CredentialStore
	version string
}

// ClientConfig captures the per-repository transport policy: TLS material,
// whether to skip certificate verification, and whether to forward credentials
// across host-changing redirects. The zero value is the safe default —
// verified TLS and cross-host redirects blocked.
type ClientConfig struct {
	CAFile                string
	CertFile              string
	KeyFile               string
	InsecureSkipTLSVerify bool
	PassCredentials       bool // forward credentials to a redirect target on the first hop
	PassCredentialsAll    bool // forward credentials on every redirect hop
}

// buildTLSConfig assembles a *tls.Config from explicit material plus the
// KERAMOS_CA_FILE env fallback, honoring an opt-in verification bypass.
func buildTLSConfig(cfg ClientConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	caFile := cfg.CAFile
	if "" == caFile {
		caFile = os.Getenv("KERAMOS_CA_FILE")
	}
	if "" != caFile {
		caCert, err := os.ReadFile(caFile)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrAuth, err, "failed to read CA file: %s", caFile)
		}
		pool, err := x509.SystemCertPool()
		if nil != err {
			pool = x509.NewCertPool()
		}
		pool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = pool
	}
	if "" != cfg.CertFile && "" != cfg.KeyFile {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrAuth, err, "failed to load client cert/key from %s, %s", cfg.CertFile, cfg.KeyFile)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if cfg.InsecureSkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // explicit opt-in via --insecure-skip-tls-verify
	}
	return tlsConfig, nil
}

// redirectPolicy returns the CheckRedirect func for a client config. By default
// cross-host redirects are blocked. --pass-credentials allows the first
// cross-host hop and forwards the Authorization header to it;
// --pass-credentials-all forwards on every hop. The plaintext guard still
// applies — credentials are never forwarded over http:// unless
// KERAMOS_ALLOW_PLAINTEXT_AUTH=1.
// sensitiveAuthHeaders are the credential-bearing headers keramos may set
// (see injectHeaders). The redirect policy manages ALL of them explicitly
// rather than trusting net/http, which ignores scheme downgrades and does not
// know about non-standard headers like X-API-Key.
var sensitiveAuthHeaders = []string{"Authorization", "X-API-Key"}

func redirectPolicy(cfg ClientConfig) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if 0 == len(via) {
			return nil
		}
		orig := via[0].URL.Host
		crossHost := req.URL.Host != orig
		if crossHost && !cfg.PassCredentials && !cfg.PassCredentialsAll {
			return fmt.Errorf("redirect to different host %q blocked (original: %q); add --pass-credentials to the repo to allow", req.URL.Host, orig)
		}
		// Is forwarding credentials to THIS hop authorized?
		forward := false
		switch {
		case cfg.PassCredentialsAll:
			forward = true
		case cfg.PassCredentials && 1 == len(via):
			forward = true // first cross-host hop only
		case !crossHost:
			forward = true // same host keeps its own auth
		}
		// Never send credentials over plaintext http:// unless explicitly opted
		// in — this covers a same-host https→http downgrade, which the stdlib
		// treats as same-host and would otherwise carry the header through.
		if "https" != req.URL.Scheme && "1" != os.Getenv("KERAMOS_ALLOW_PLAINTEXT_AUTH") {
			forward = false
		}
		if !forward {
			// Strip every credential header the client set; do not rely on the
			// stdlib to do it (it misses scheme downgrades and X-API-Key).
			for _, h := range sensitiveAuthHeaders {
				req.Header.Del(h)
			}
			return nil
		}
		// Forwarding authorized: re-attach the original request's credential
		// headers (the stdlib strips Authorization on cross-host redirects).
		for _, h := range sensitiveAuthHeaders {
			if v := via[0].Header.Get(h); "" != v {
				req.Header.Set(h, v)
			}
		}
		return nil
	}
}

// newClient builds an AuthenticatedClient from a config and (optional) store.
func newClient(cfg ClientConfig, store *CredentialStore) (*AuthenticatedClient, error) {
	tlsConfig, err := buildTLSConfig(cfg)
	if nil != err {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         netguard.DialContext(netguard.BlockMetadata, "KERAMOS_ALLOW_INTERNAL_FETCH", defaultConnectTimeout),
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport:     transport,
		Timeout:       defaultOverallTimeout,
		CheckRedirect: redirectPolicy(cfg),
	}
	return &AuthenticatedClient{inner: client, store: store, version: "dev"}, nil
}

// NewAuthenticatedClient creates an HTTP client that injects auth headers from the given store.
func NewAuthenticatedClient(store *CredentialStore) (*AuthenticatedClient, error) {
	return newClient(ClientConfig{}, store)
}

// NewClientWithTLS returns an AuthenticatedClient with explicit TLS material
// (CA bundle, client cert, client key). Empty file paths are ignored. Falls
// back to the default client when all three are empty.
func NewClientWithTLS(caFile, certFile, keyFile string) (*AuthenticatedClient, error) {
	if "" == caFile && "" == certFile && "" == keyFile {
		return DefaultClient()
	}
	return newClient(ClientConfig{CAFile: caFile, CertFile: certFile, KeyFile: keyFile}, nil)
}

// NewClientWithConfig builds an AuthenticatedClient from a full per-repo config
// (TLS material, insecure-skip, and redirect credential policy).
func NewClientWithConfig(cfg ClientConfig, store *CredentialStore) (*AuthenticatedClient, error) {
	return newClient(cfg, store)
}

// ClientForURL builds a client honoring the stored configuration of whichever
// repository's URL is a prefix of rawURL — its TLS material, insecure-skip, and
// credential-forwarding policy — plus a per-host insecure flag from the
// credential store (`keramos login --insecure`). Falls back to the default client
// when no repo matches.
func ClientForURL(rawURL string) (*AuthenticatedClient, error) {
	store, storeErr := LoadCredentialStore()
	if nil != storeErr {
		store = &CredentialStore{Credentials: map[string]Credential{}}
	}
	cfg := ClientConfig{}
	if rf, err := LoadRepoFile(); nil == err {
		for _, r := range rf.Repositories {
			if "" != r.URL && urlUnderRepo(rawURL, r.URL) {
				cfg.CAFile, cfg.CertFile, cfg.KeyFile = r.CAFile, r.CertFile, r.KeyFile
				cfg.InsecureSkipTLSVerify = r.InsecureSkipTLSVerify
				cfg.PassCredentials = r.PassCredentials
				cfg.PassCredentialsAll = r.PassCredentialsAll
				break
			}
		}
	}
	// A host the operator marked insecure at login time also skips TLS verify.
	if host := hostOf(rawURL); "" != host {
		if cred, ok := store.GetForHost(host); ok && cred.Insecure {
			cfg.InsecureSkipTLSVerify = true
		}
	}
	return newClient(cfg, store)
}

// urlUnderRepo reports whether rawURL belongs to the repository at repoURL:
// identical scheme+host and a path at or beneath the repo's path. A raw prefix
// match would let a look-alike host (good.com.attacker.net) or sibling
// (good.commercial) inherit good.com's TLS-skip / pass-credentials policy.
func urlUnderRepo(rawURL, repoURL string) bool {
	a, err1 := url.Parse(rawURL)
	b, err2 := url.Parse(repoURL)
	if nil != err1 || nil != err2 {
		return false
	}
	if a.Scheme != b.Scheme || a.Host != b.Host {
		return false
	}
	ap := strings.TrimSuffix(a.Path, "/")
	bp := strings.TrimSuffix(b.Path, "/")
	return ap == bp || strings.HasPrefix(ap, bp+"/")
}

// hostOf extracts the host[:port] from a URL for credential lookup.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if nil != err {
		return ""
	}
	return u.Host
}

// SetVersion sets the version string used in the User-Agent header.
func (ac *AuthenticatedClient) SetVersion(v string) {
	ac.version = v
}

// Do executes an HTTP request with auth injection, User-Agent, and retry logic.
// Requests with a non-replayable body (no GetBody) are not retried.
func (ac *AuthenticatedClient) Do(req *http.Request) (*http.Response, error) {
	ac.injectHeaders(req)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if 0 < attempt {
			// Reset body for retry. If the request has a body but no GetBody,
			// the original body has already been consumed; bail out instead of
			// silently sending an empty body.
			if nil != req.Body {
				if nil == req.GetBody {
					return nil, lastErr
				}
				body, getErr := req.GetBody()
				if nil != getErr {
					return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to rewind request body for retry", getErr)
				}
				req.Body = body
			}

			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * baseBackoff
			logger.Debug("retrying request to %s (attempt %d, backoff %v)", req.URL, attempt+1, backoff)
			sleepFn(backoff)
		}

		resp, err := ac.inner.Do(req)
		if nil != err {
			lastErr = keramoserr.WrapError(keramoserr.ErrRepo, "HTTP request failed", err)
			continue
		}

		if resp.StatusCode < 500 {
			return resp, nil
		}

		// Server error — drain and close body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastErr = keramoserr.NewErrorf(keramoserr.ErrRepo, "server error: HTTP %d", resp.StatusCode)
	}

	return nil, lastErr
}

// Get is a convenience wrapper for HTTP GET requests.
func (ac *AuthenticatedClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to create request", err)
	}
	return ac.Do(req)
}

func (ac *AuthenticatedClient) injectHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "keramos/"+ac.version)

	if nil == ac.store {
		return
	}
	cred, ok := ac.store.GetForHost(req.URL.Host)
	if !ok {
		return
	}

	// Refuse to send credentials over plaintext HTTP. Basic/Bearer/API-key
	// material would otherwise travel in cleartext to any on-path observer.
	// Operators who genuinely need this (e.g. a trusted in-cluster registry on
	// a private link) must opt in via KERAMOS_ALLOW_PLAINTEXT_AUTH=1, matching the
	// OCI registry path (see oci.go).
	if "https" != req.URL.Scheme && "1" != os.Getenv("KERAMOS_ALLOW_PLAINTEXT_AUTH") {
		logger.Warn("refusing to send credentials to %s over %s (set KERAMOS_ALLOW_PLAINTEXT_AUTH=1 to override)",
			req.URL.Host, req.URL.Scheme)
		return
	}

	switch cred.Type {
	case AuthBasic:
		req.SetBasicAuth(cred.Username, cred.Password)
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+cred.Token)
	case AuthAPIKey:
		req.Header.Set("X-API-Key", cred.APIKey)
	}
}

// sleepFn is a package-level variable so tests can replace it.
var sleepFn = time.Sleep
