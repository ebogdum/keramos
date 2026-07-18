package repo

import (
	"net/http"
	"net/url"
	"testing"
)

func reqWith(rawURL string, hdr map[string]string) *http.Request {
	u, _ := url.Parse(rawURL)
	r := &http.Request{URL: u, Header: http.Header{}}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	return r
}

// TestRedirectStripsCredsOnPlaintextDowngrade proves creds are removed on a
// same-host https->http redirect (the stdlib keeps them; we must strip).
func TestRedirectStripsCredsOnPlaintextDowngrade(t *testing.T) {
	t.Setenv("KERAMOS_ALLOW_PLAINTEXT_AUTH", "")
	policy := redirectPolicy(ClientConfig{})
	orig := reqWith("https://repo.example/a", map[string]string{"Authorization": "Bearer s", "X-API-Key": "k"})
	next := reqWith("http://repo.example/a", map[string]string{"Authorization": "Bearer s", "X-API-Key": "k"})
	if err := policy(next, []*http.Request{orig}); nil != err {
		t.Fatalf("same-host redirect should be allowed: %v", err)
	}
	if "" != next.Header.Get("Authorization") || "" != next.Header.Get("X-API-Key") {
		t.Fatalf("credentials must be stripped on plaintext downgrade: %v", next.Header)
	}
}

// TestRedirectStripsAPIKeyCrossHostWithoutOptIn proves X-API-Key is governed by
// the policy (stdlib does not strip non-standard headers).
func TestRedirectStripsAPIKeyCrossHost(t *testing.T) {
	policy := redirectPolicy(ClientConfig{}) // no pass-credentials
	orig := reqWith("https://good.example/a", map[string]string{"X-API-Key": "k"})
	// cross-host without pass-credentials is blocked outright
	next := reqWith("https://evil.example/a", map[string]string{"X-API-Key": "k"})
	if err := policy(next, []*http.Request{orig}); nil == err {
		t.Fatal("cross-host redirect must be blocked by default")
	}
}

// TestRedirectForwardsAPIKeyWhenAuthorized proves X-API-Key IS forwarded on an
// authorized https cross-host hop with --pass-credentials-all.
func TestRedirectForwardsAPIKeyWhenAuthorized(t *testing.T) {
	policy := redirectPolicy(ClientConfig{PassCredentialsAll: true})
	orig := reqWith("https://good.example/a", map[string]string{"X-API-Key": "k"})
	next := reqWith("https://cdn.example/a", nil)
	if err := policy(next, []*http.Request{orig}); nil != err {
		t.Fatalf("authorized hop: %v", err)
	}
	if "k" != next.Header.Get("X-API-Key") {
		t.Fatal("X-API-Key should be forwarded on an authorized hop")
	}
}

// TestURLUnderRepo proves the boundary-aware match rejects look-alike hosts.
func TestURLUnderRepo(t *testing.T) {
	cases := []struct {
		raw, repo string
		want      bool
	}{
		{"https://good.com/charts/x", "https://good.com", true},
		{"https://good.com/charts", "https://good.com/charts", true},
		{"https://good.com.attacker.net/x", "https://good.com", false},
		{"https://good.commercial/x", "https://good.com", false},
		{"http://good.com/x", "https://good.com", false}, // scheme differs
		{"https://good.com/chartsX", "https://good.com/charts", false},
	}
	for _, c := range cases {
		if got := urlUnderRepo(c.raw, c.repo); got != c.want {
			t.Errorf("urlUnderRepo(%q,%q)=%v want %v", c.raw, c.repo, got, c.want)
		}
	}
}
