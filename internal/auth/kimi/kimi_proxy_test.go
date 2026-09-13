package kimi

import (
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/egress"
)

// allowKimiProxyTestHost permits the destination these proxy tests inspect the
// proxy function with. Nothing is dialled; the gate runs on every request, so
// the host still has to be permitted.
func allowKimiProxyTestHost(t *testing.T) {
	t.Helper()
	egress.SetConfigWithBuiltin(nil, []string{"example.com"})
}

func TestNewDeviceFlowClientWithDeviceIDAndProxyURL_OverrideDirectDisablesProxy(t *testing.T) {
	allowKimiProxyTestHost(t)
	cfg := &config.Config{SDKConfig: config.SDKConfig{ProxyURL: "http://proxy.example.com:8080"}}
	client := NewDeviceFlowClientWithDeviceIDAndProxyURL(cfg, "device-1", "direct")

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http.Transport, got %T", client.httpClient.Transport)
	}
	req, errReq := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errReq != nil {
		t.Fatalf("new request: %v", errReq)
	}
	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("proxy func: %v", errProxy)
	}
	if proxyURL != nil {
		t.Fatalf("expected direct transport to select no proxy, got %v", proxyURL)
	}
}

func TestNewDeviceFlowClientWithDeviceIDAndProxyURL_OverrideProxyTakesPrecedence(t *testing.T) {
	allowKimiProxyTestHost(t)
	cfg := &config.Config{SDKConfig: config.SDKConfig{ProxyURL: "http://global.example.com:8080"}}
	client := NewDeviceFlowClientWithDeviceIDAndProxyURL(cfg, "device-1", "http://override.example.com:8081")

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http.Transport, got %T", client.httpClient.Transport)
	}
	req, errReq := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errReq != nil {
		t.Fatalf("new request: %v", errReq)
	}
	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("proxy func: %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://override.example.com:8081" {
		t.Fatalf("proxy URL = %v, want http://override.example.com:8081", proxyURL)
	}
}
