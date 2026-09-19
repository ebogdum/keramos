package engine

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/netguard"
)

// validateOutboundURL parses and string-validates the URL: scheme must be
// http/https. Host-level enforcement happens at dial time (see safeDialer)
// to close the DNS-rebinding window between resolve-and-dial.
func validateOutboundURL(raw string) error {
	u, err := url.Parse(raw)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse URL", err)
	}
	if "http" != u.Scheme && "https" != u.Scheme {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"render-time URL must be http or https, got %q", u.Scheme)
	}
	if "" == u.Hostname() {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "URL has no host")
	}
	return nil
}

// isBlockedIP returns true for any address class keramos refuses to dial at
// render time when KERAMOS_RENDER_INTERNAL is unset. It delegates to the shared
// netguard classifier (block-all-internal policy) so render-time and
// artifact-fetch paths stay consistent.
func isBlockedIP(ip net.IP) bool {
	return netguard.IsBlocked(ip, netguard.BlockAllInternal)
}

// safeDialer enforces the SSRF blocklist at TCP dial time, after the OS
// resolver has produced a literal IP. This closes the DNS-rebinding window
// where validateOutboundURL would resolve a benign IP and the transport
// would later resolve and dial a metadata-service IP.
func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	if "1" == os.Getenv("KERAMOS_RENDER_INTERNAL") {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
	}
	host, port, err := net.SplitHostPort(addr)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "split host/port", err)
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "resolve %s", host)
	}
	if 0 == len(ips) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "no addresses for %s", host)
	}
	// Validate every resolved IP up front; one blocked address poisons the
	// whole hostname so an attacker can't pad metadata-service IPs after
	// public ones. Then iterate in resolver order, dialling each literal IP
	// (eliminating the resolve/dial race) until one succeeds — preserving
	// Happy-Eyeballs-style failover for legitimate dual-stack hosts.
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"refusing to dial internal address %s for %s (set KERAMOS_RENDER_INTERNAL=1 to allow)",
				ip, host)
		}
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, dErr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if nil == dErr {
			return conn, nil
		}
		lastErr = dErr
	}
	return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, lastErr, "dial %s (all %d addresses failed)", host, len(ips))
}

var safeHTTPTransport = &http.Transport{DialContext: safeDial}

var safeHTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: safeHTTPTransport,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if 10 <= len(via) {
			return keramoserr.NewError(keramoserr.ErrInternal, "too many redirects")
		}
		return validateOutboundURL(req.URL.String())
	},
}

// truncateError clamps response bodies before they appear in errors so that
// internal-API responses don't leak verbatim into stderr/logs.
func truncateError(body []byte) string {
	const max = 256
	if max < len(body) {
		return string(body[:max]) + "…(truncated)"
	}
	return string(body)
}

// registerExternalFuncs adds render-time external-API functions:
//
//   ${http "https://example/x" [headers]}        GET → response body string
//   ${httpJSON "https://example/x" [headers]}    GET → parsed JSON value
//   ${vault "secret/data/db" "password"}         HashiCorp Vault KV-v2 lookup
//
// Network egress at render time is opt-in via the KERAMOS_RENDER_NETWORK env var
// to avoid surprising hermetic-build assumptions. Without it the functions
// return a structured error.
func registerExternalFuncs(r *FuncRegistry) {
	r.Register("http", fnHTTP)
	r.Register("httpJSON", fnHTTPJSON)
	r.Register("vault", fnVault)
}

const renderNetworkEnv = "KERAMOS_RENDER_NETWORK"

func networkAllowed() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(renderNetworkEnv)))
	return "1" == v || "true" == v || "yes" == v
}

func fnHTTP(value any, args ...any) (any, error) {
	url := coerceString(value)
	if "" == url && 0 < len(args) {
		url = coerceString(args[0])
		args = args[1:]
	}
	if "" == url {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "http requires a URL")
	}
	if !networkAllowed() {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"render-time network calls disabled — set %s=1 to enable", renderNetworkEnv)
	}
	body, err := httpGet(url, args)
	if nil != err {
		return nil, err
	}
	return string(body), nil
}

func fnHTTPJSON(value any, args ...any) (any, error) {
	url := coerceString(value)
	if "" == url && 0 < len(args) {
		url = coerceString(args[0])
		args = args[1:]
	}
	if !networkAllowed() {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"render-time network calls disabled — set %s=1 to enable", renderNetworkEnv)
	}
	body, err := httpGet(url, args)
	if nil != err {
		return nil, err
	}
	var out any
	if jErr := json.Unmarshal(body, &out); nil != jErr {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "parse JSON response", jErr)
	}
	return out, nil
}

// httpGet performs a GET with a 10s timeout and optional `headers map[string]string`
// argument. The URL is validated against the SSRF policy before dialling, and
// every redirect is re-validated by the safeHTTPClient. Non-2xx responses
// produce an error with the body truncated to bound information leakage.
func httpGet(url string, args []any) ([]byte, error) {
	if vErr := validateOutboundURL(url); nil != vErr {
		return nil, vErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "build request", err)
	}
	if 0 < len(args) {
		if hm, ok := args[0].(map[string]any); ok {
			for k, v := range hm {
				req.Header.Set(k, coerceString(v))
			}
		}
	}
	resp, err := safeHTTPClient.Do(req)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "GET %s", url)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if 200 > resp.StatusCode || 300 <= resp.StatusCode {
		return nil, keramoserr.NewErrorf(keramoserr.ErrInternal,
			"GET %s returned %d: %s", url, resp.StatusCode, truncateError(body))
	}
	return body, nil
}

// fnVault reads a Vault KV-v2 secret. Auth is via the VAULT_ADDR + VAULT_TOKEN
// env vars (matching the official vault CLI). Path is the secret path; the
// optional second arg names a single field within the data map.
func fnVault(value any, args ...any) (any, error) {
	path := coerceString(value)
	if "" == path && 0 < len(args) {
		path = coerceString(args[0])
		args = args[1:]
	}
	if "" == path {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "vault requires a secret path")
	}
	if !networkAllowed() {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"render-time network calls disabled — set %s=1 to enable", renderNetworkEnv)
	}
	addr := strings.TrimRight(os.Getenv("VAULT_ADDR"), "/")
	token := os.Getenv("VAULT_TOKEN")
	if "" == addr || "" == token {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "VAULT_ADDR and VAULT_TOKEN must be set")
	}
	url := addr + "/v1/" + strings.TrimLeft(path, "/")
	if vErr := validateOutboundURL(url); nil != vErr {
		return nil, vErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if nil != reqErr {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, reqErr, "build vault request for %s", url)
	}
	req.Header.Set("X-Vault-Token", token)
	resp, err := safeHTTPClient.Do(req)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "vault GET %s", url)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if 200 != resp.StatusCode {
		return nil, keramoserr.NewErrorf(keramoserr.ErrInternal,
			"vault returned %d: %s", resp.StatusCode, truncateError(body))
	}
	var doc struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if jErr := json.Unmarshal(body, &doc); nil != jErr {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "parse vault response", jErr)
	}
	if 0 < len(args) {
		key := coerceString(args[0])
		v, ok := doc.Data.Data[key]
		if !ok {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"vault path %q has no key %q", path, key)
		}
		return v, nil
	}
	return doc.Data.Data, nil
}
