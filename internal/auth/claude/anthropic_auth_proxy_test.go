package claude

import (
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"golang.org/x/net/proxy"
)

// utlsTransportOf returns the utlsRoundTripper underneath the egress guard that
// NewAnthropicHttpClient installs around it.
func utlsTransportOf(t *testing.T, rt http.RoundTripper) *utlsRoundTripper {
	t.Helper()
	if guard, ok := rt.(interface{ Unwrap() http.RoundTripper }); ok {
		rt = guard.Unwrap()
	}
	transport, ok := rt.(*utlsRoundTripper)
	if !ok || transport == nil {
		t.Fatalf("expected utlsRoundTripper, got %T", rt)
	}
	return transport
}

func TestNewClaudeAuthWithProxyURL_OverrideDirectTakesPrecedence(t *testing.T) {
	cfg := &config.Config{SDKConfig: config.SDKConfig{ProxyURL: "socks5://proxy.example.com:1080"}}
	auth := NewClaudeAuthWithProxyURL(cfg, "direct")

	transport := utlsTransportOf(t, auth.httpClient.Transport)
	if transport.dialer != proxy.Direct {
		t.Fatalf("expected proxy.Direct, got %T", transport.dialer)
	}
}

func TestNewClaudeAuthWithProxyURL_OverrideProxyAppliedWithoutConfig(t *testing.T) {
	auth := NewClaudeAuthWithProxyURL(nil, "socks5://proxy.example.com:1080")

	transport := utlsTransportOf(t, auth.httpClient.Transport)
	if transport.dialer == proxy.Direct {
		t.Fatalf("expected proxy dialer, got %T", transport.dialer)
	}
}
