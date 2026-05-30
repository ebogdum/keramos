package netguard

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIsBlockedMetadataPolicy(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"169.254.169.254", true},        // cloud metadata (link-local)
		{"::ffff:169.254.169.254", true}, // IPv4-mapped metadata
		{"fd00:ec2::254", true},          // AWS IMDSv6
		{"fe80::1", true},                // link-local v6
		{"0.0.0.0", true},                // unspecified
		{"224.0.0.1", true},              // multicast
		{"127.0.0.1", false},             // loopback ALLOWED (local registry)
		{"::1", false},                   // loopback v6 ALLOWED
		{"10.0.0.5", false},              // RFC1918 ALLOWED (private registry)
		{"192.168.1.10", false},          // RFC1918 ALLOWED
		{"100.64.0.1", false},            // CGNAT ALLOWED
		{"8.8.8.8", false},               // public ALLOWED
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if nil == ip {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := IsBlocked(ip, BlockMetadata); got != c.blocked {
			t.Errorf("BlockMetadata IsBlocked(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

func TestIsBlockedAllInternalPolicy(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"169.254.169.254", true},
		{"127.0.0.1", true}, // loopback BLOCKED at render time
		{"::1", true},       // loopback v6 BLOCKED
		{"10.0.0.5", true},  // RFC1918 BLOCKED
		{"192.168.1.10", true},
		{"100.64.0.1", true}, // CGNAT BLOCKED
		{"fc00::1", true},    // IPv6 ULA BLOCKED
		{"8.8.8.8", false},   // public still allowed
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := IsBlocked(ip, BlockAllInternal); got != c.blocked {
			t.Errorf("BlockAllInternal IsBlocked(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

func TestDialContextRefusesLiteralMetadataIP(t *testing.T) {
	dial := DialContext(BlockMetadata, "", 2*time.Second)
	_, err := dial(context.Background(), "tcp", "169.254.169.254:80")
	if nil == err {
		t.Fatal("expected dial to the metadata IP to be refused")
	}
	if !strings.Contains(err.Error(), "refusing to dial internal address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDialContextAllowEnvBypass(t *testing.T) {
	t.Setenv("KERAMOS_TEST_ALLOW", "1")
	dial := DialContext(BlockMetadata, "KERAMOS_TEST_ALLOW", 100*time.Millisecond)
	// With the bypass on, the guard must not reject before dialing; the dial
	// itself will fail fast (nothing listening) but NOT with our refusal error.
	_, err := dial(context.Background(), "tcp", "169.254.169.254:9")
	if nil != err && strings.Contains(err.Error(), "refusing to dial internal address") {
		t.Fatalf("bypass env should have skipped the guard, got: %v", err)
	}
}
