package repo

import (
	"net/http"
	"net/url"
	"testing"
)

func mkReq(rawURL string) *http.Request {
	u, _ := url.Parse(rawURL)
	return &http.Request{URL: u, Header: http.Header{}}
}

// TestRedirectDefaultBlocksCrossHost verifies the safe default: a redirect that
// changes host is refused, so credentials cannot leak to an attacker's host.
func TestRedirectDefaultBlocksCrossHost(t *testing.T) {
	policy := redirectPolicy(ClientConfig{})
	orig := mkReq("https://good.example/index.yaml")
	orig.Header.Set("Authorization", "Bearer secret")
	next := mkReq("https://evil.example/index.yaml")
	if err := policy(next, []*http.Request{orig}); nil == err {
		t.Fatal("expected cross-host redirect to be blocked by default")
	}
	if "" != next.Header.Get("Authorization") {
		t.Fatal("credentials must NOT be forwarded when redirect is blocked")
	}
}

// TestRedirectSameHostKeepsAuth verifies same-host redirects are allowed.
func TestRedirectSameHostKeepsAuth(t *testing.T) {
	policy := redirectPolicy(ClientConfig{})
	orig := mkReq("https://good.example/a")
	orig.Header.Set("Authorization", "Bearer secret")
	next := mkReq("https://good.example/b")
	if err := policy(next, []*http.Request{orig}); nil != err {
		t.Fatalf("same-host redirect should be allowed: %v", err)
	}
}

// TestRedirectPassCredentialsForwardsFirstHopOnly verifies --pass-credentials
// allows the first cross-host hop and forwards the Authorization header to it,
// but does NOT forward on a subsequent hop.
func TestRedirectPassCredentialsForwardsFirstHopOnly(t *testing.T) {
	policy := redirectPolicy(ClientConfig{PassCredentials: true})
	orig := mkReq("https://good.example/a")
	orig.Header.Set("Authorization", "Bearer secret")

	first := mkReq("https://cdn.example/a")
	if err := policy(first, []*http.Request{orig}); nil != err {
		t.Fatalf("first cross-host hop should be allowed: %v", err)
	}
	if "Bearer secret" != first.Header.Get("Authorization") {
		t.Fatal("credentials should be forwarded on the first hop with --pass-credentials")
	}

	// Second hop: still allowed, but creds NOT forwarded (first-hop-only).
	second := mkReq("https://cdn2.example/a")
	if err := policy(second, []*http.Request{orig, first}); nil != err {
		t.Fatalf("second hop should be allowed under pass-credentials: %v", err)
	}
	if "" != second.Header.Get("Authorization") {
		t.Fatal("--pass-credentials must forward on the FIRST hop only")
	}
}

// TestRedirectPassAllForwardsEveryHop verifies --pass-credentials-all forwards
// on every redirect hop.
func TestRedirectPassAllForwardsEveryHop(t *testing.T) {
	policy := redirectPolicy(ClientConfig{PassCredentialsAll: true})
	orig := mkReq("https://good.example/a")
	orig.Header.Set("Authorization", "Bearer secret")
	first := mkReq("https://cdn.example/a")
	_ = policy(first, []*http.Request{orig})
	second := mkReq("https://cdn2.example/a")
	if err := policy(second, []*http.Request{orig, first}); nil != err {
		t.Fatalf("pass-all should allow every hop: %v", err)
	}
	if "Bearer secret" != second.Header.Get("Authorization") {
		t.Fatal("--pass-credentials-all must forward on every hop")
	}
}

// TestRedirectNeverForwardsOverPlaintext verifies the plaintext guard holds
// even when forwarding is enabled: creds are not sent to an http:// target.
func TestRedirectNeverForwardsOverPlaintext(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "")
	policy := redirectPolicy(ClientConfig{PassCredentialsAll: true})
	orig := mkReq("https://good.example/a")
	orig.Header.Set("Authorization", "Bearer secret")
	next := mkReq("http://cdn.example/a") // plaintext
	if err := policy(next, []*http.Request{orig}); nil != err {
		t.Fatalf("hop allowed: %v", err)
	}
	if "" != next.Header.Get("Authorization") {
		t.Fatal("credentials must not be forwarded over plaintext http")
	}
}

// TestBuildTLSConfigInsecure verifies the insecure flag disables verification.
func TestBuildTLSConfigInsecure(t *testing.T) {
	cfg, err := buildTLSConfig(ClientConfig{InsecureSkipTLSVerify: true})
	if nil != err {
		t.Fatalf("buildTLSConfig: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipTLSVerify must set tls.Config.InsecureSkipVerify")
	}
	secure, _ := buildTLSConfig(ClientConfig{})
	if secure.InsecureSkipVerify {
		t.Fatal("default config must verify TLS")
	}
}
