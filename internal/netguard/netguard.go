// Package netguard provides an SSRF-resistant dialer shared by every outbound
// HTTP path in keramos (artifact fetch, OCI, repo index, Artifact Hub search,
// marketplace, and render-time http/vault). It resolves the destination host
// itself and refuses to connect to internal address classes, closing the
// DNS-rebinding window between resolve and dial.
package netguard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// Policy selects how aggressively internal address classes are blocked.
type Policy int

const (
	// BlockMetadata blocks the cloud metadata service (169.254.169.254 and its
	// IPv6 form), all link-local and multicast addresses, and the unspecified
	// address. It deliberately ALLOWS loopback and RFC1918 / CGNAT so local
	// development registries (localhost:5000) and private/in-cluster
	// registries remain reachable. Use for artifact fetches, where the primary
	// SSRF threat is a hostile chart redirecting to the metadata endpoint to
	// steal cloud IAM credentials — not loopback, which a CLI user already
	// controls.
	BlockMetadata Policy = iota
	// BlockAllInternal additionally blocks loopback, RFC1918, CGNAT, and IPv6
	// ULA. Use for render-time fetches, where reaching ANY internal service is
	// unwanted (the template author is not necessarily the operator).
	BlockAllInternal
)

// cloudMetadataV6 is the AWS IMDSv6 ULA address, which IsPrivate() would
// otherwise permit under the BlockMetadata policy.
var cloudMetadataV6 = net.ParseIP("fd00:ec2::254")

// IsBlocked reports whether ip belongs to an address class disallowed by p.
func IsBlocked(ip net.IP, p Policy) bool {
	// Always blocked: link-local (incl. 169.254.169.254 metadata), multicast,
	// unspecified, and the explicit IPv6 metadata address.
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.Equal(cloudMetadataV6) {
		return true
	}
	if v4 := ip.To4(); nil != v4 {
		// 192.0.0.0/24 — IETF protocol assignments.
		if 192 == v4[0] && 0 == v4[1] && 0 == v4[2] {
			return true
		}
		// 198.18.0.0/15 — benchmarking.
		if 198 == v4[0] && (18 == v4[1] || 19 == v4[1]) {
			return true
		}
	}
	if BlockAllInternal == p {
		if ip.IsLoopback() || ip.IsPrivate() { // loopback + RFC1918 + IPv6 ULA
			return true
		}
		if v4 := ip.To4(); nil != v4 {
			// 127.0.0.0/8, 0.0.0.0/8 (mapped-v6 belt-and-braces), CGNAT.
			if 127 == v4[0] || 0 == v4[0] {
				return true
			}
			if 100 == v4[0] && 64 <= v4[1] && 127 >= v4[1] {
				return true
			}
		}
	}
	return false
}

// DialContext returns a dial function that enforces policy p. When allowEnv is
// non-empty and that environment variable equals "1", the guard is bypassed so
// operators can opt in to internal targets. connectTimeout bounds each dial.
func DialContext(p Policy, allowEnv string, connectTimeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if 0 == connectTimeout {
		connectTimeout = 30 * time.Second
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{Timeout: connectTimeout}
		if "" != allowEnv && "1" == os.Getenv(allowEnv) {
			return d.DialContext(ctx, network, addr)
		}
		host, port, err := net.SplitHostPort(addr)
		if nil != err {
			return nil, err
		}
		// A literal IP destination is checked directly without DNS.
		if literal := net.ParseIP(host); nil != literal {
			if IsBlocked(literal, p) {
				return nil, blockedError(literal.String(), host, allowEnv)
			}
			return d.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if nil != err {
			return nil, err
		}
		if 0 == len(ips) {
			return nil, fmt.Errorf("netguard: no addresses for %s", host)
		}
		// Validate every resolved IP first so an attacker cannot pad allowed
		// addresses ahead of a metadata IP in the resolver response.
		for _, ip := range ips {
			if IsBlocked(ip, p) {
				return nil, blockedError(ip.String(), host, allowEnv)
			}
		}
		// Dial each literal IP (not the hostname) to eliminate the
		// resolve/dial race, preserving dual-stack failover.
		var lastErr error
		for _, ip := range ips {
			conn, dErr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if nil == dErr {
				return conn, nil
			}
			lastErr = dErr
		}
		return nil, fmt.Errorf("netguard: dial %s failed: %w", host, lastErr)
	}
}

// HTTPClient returns an *http.Client whose transport enforces policy p (via an
// SSRF-guarded dialer), pins TLS 1.2 as the floor, and honours proxy env vars.
// Use for the simple outbound clients (Artifact Hub search, marketplace).
func HTTPClient(p Policy, allowEnv string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         DialContext(p, allowEnv, 30*time.Second),
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func blockedError(ip, host, allowEnv string) error {
	if "" != allowEnv {
		return fmt.Errorf("netguard: refusing to dial internal address %s for %s (set %s=1 to allow)", ip, host, allowEnv)
	}
	return fmt.Errorf("netguard: refusing to dial internal address %s for %s", ip, host)
}
