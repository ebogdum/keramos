package repo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/netguard"
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

// NewAuthenticatedClient creates an HTTP client that injects auth headers from the given store.
func NewAuthenticatedClient(store *CredentialStore) (*AuthenticatedClient, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	caFile := os.Getenv("KERAMOS_CA_FILE")
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

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: netguard.DialContext(netguard.BlockMetadata, "KERAMOS_ALLOW_INTERNAL_FETCH", defaultConnectTimeout),
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultOverallTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if 0 == len(via) {
				return nil
			}
			originalHost := via[0].URL.Host
			if req.URL.Host != originalHost {
				return fmt.Errorf("redirect to different host %q blocked (original: %q)", req.URL.Host, originalHost)
			}
			return nil
		},
	}

	return &AuthenticatedClient{
		inner:   client,
		store:   store,
		version: "dev",
	}, nil
}

// NewClientWithTLS returns an AuthenticatedClient with explicit TLS material
// (CA bundle, client cert, client key). Empty file paths are ignored. Falls
// back to the default client when all three are empty.
func NewClientWithTLS(caFile, certFile, keyFile string) (*AuthenticatedClient, error) {
	if "" == caFile && "" == certFile && "" == keyFile {
		return DefaultClient()
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
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
	if "" != certFile && "" != keyFile {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrAuth, err, "failed to load client cert/key from %s, %s", certFile, keyFile)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
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
		Transport: transport,
		Timeout:   defaultOverallTimeout,
		// Block cross-host redirects so credentials/mTLS material cannot leak.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if 0 == len(via) {
				return nil
			}
			if req.URL.Host != via[0].URL.Host {
				return fmt.Errorf("redirect to different host %q blocked (original: %q)", req.URL.Host, via[0].URL.Host)
			}
			return nil
		},
	}
	return &AuthenticatedClient{inner: client, version: "dev"}, nil
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
