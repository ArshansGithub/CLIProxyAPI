package proxyutil

import (
	"context"
	"errors"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/egress"
)

// 203.0.113.9 (TEST-NET-3) is not in the test policy, so the proxy host is
// refused without any socket being opened.
func TestBuildHTTPTransportSOCKS5RefusesUnlistedProxyHost(t *testing.T) {
	t.Parallel()
	allowProxyTestHost(t)
	transport, _, err := BuildHTTPTransport("socks5://203.0.113.9:1080")
	if err != nil {
		t.Fatalf("BuildHTTPTransport: %v", err)
	}
	_, errDial := transport.DialContext(context.Background(), "tcp", "api.anthropic.com:443")
	if !errors.Is(errDial, egress.ErrNotPermitted) {
		t.Fatalf("dial through unlisted SOCKS proxy must be refused by egress, got %v", errDial)
	}
}

func TestBuildDialerRefusesUnlistedProxyHost(t *testing.T) {
	t.Parallel()
	allowProxyTestHost(t)
	for _, raw := range []string{"socks5://203.0.113.9:1080", "http://203.0.113.9:3128"} {
		dialer, _, err := BuildDialer(raw)
		if err != nil {
			t.Fatalf("BuildDialer(%s): %v", raw, err)
		}
		if _, errDial := dialer.Dial("tcp", "api.anthropic.com:443"); !errors.Is(errDial, egress.ErrNotPermitted) {
			t.Fatalf("%s: dial through unlisted proxy must be refused by egress, got %v", raw, errDial)
		}
	}
}
