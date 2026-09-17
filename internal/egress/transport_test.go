package egress

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

func TestGuardedTransportRefusesUnlistedHost(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	client := &http.Client{Transport: GuardTransport(&http.Transport{}, "test.guarded")}
	// 127.0.0.1 is loopback and allowed.
	if _, err := client.Get(srv.URL); err != nil {
		t.Fatalf("loopback should pass: %v", err)
	}
	// Force a non-loopback hostname that resolves nowhere; the gate must refuse before DNS.
	_, err := client.Get("http://evil.example.net/")
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}

func TestGuardTransportPreservesExistingProxyFunc(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce", "api.example.com"), nil)
	called := false
	tr := &http.Transport{Proxy: func(r *http.Request) (*url.URL, error) { called = true; return nil, nil }}
	GuardTransport(tr, "test.preserve")
	req, _ := http.NewRequest("GET", "https://api.example.com/", nil)
	if _, err := tr.Proxy(req); err != nil || !called {
		t.Fatalf("existing proxy func must run after the check (err=%v called=%v)", err, called)
	}
}

func TestGuardWebsocketDialerRefuses(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	d := GuardWebsocketDialer(&websocket.Dialer{}, "test")
	req, _ := http.NewRequest("GET", "https://evil.example.net/ws", nil)
	if _, err := d.Proxy(req); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}

func TestRoundTripperWrapperRefuses(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	rt := RoundTripper(http.DefaultTransport, "test")
	req, _ := http.NewRequest("GET", "https://evil.example.net/", nil)
	if _, err := rt.RoundTrip(req); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}

type closeRecordingRoundTripper struct {
	closed bool
}

func (c *closeRecordingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not used")
}

func (c *closeRecordingRoundTripper) CloseIdleConnections() { c.closed = true }

func TestRoundTripperPassesThroughCloseIdleConnections(t *testing.T) {
	inner := &closeRecordingRoundTripper{}
	rt := RoundTripper(inner, "test")
	closer, ok := rt.(interface{ CloseIdleConnections() })
	if !ok {
		t.Fatal("guarded round tripper must expose CloseIdleConnections so pool eviction still closes pools")
	}
	closer.CloseIdleConnections()
	if !inner.closed {
		t.Fatal("CloseIdleConnections did not reach the wrapped round tripper")
	}
}

func TestRoundTripperCloseIdleConnectionsIgnoresRoundTrippersWithout(t *testing.T) {
	rt := RoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil }), "test")
	rt.(interface{ CloseIdleConnections() }).CloseIdleConnections()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestWrapProxyFuncRefusesUnlistedProxyHost(t *testing.T) {
	t.Cleanup(resetForTest)
	SetConfigWithBuiltin(testConfig("enforce"), []string{"api.anthropic.com"})
	next := func(*http.Request) (*url.URL, error) { return url.Parse("http://proxy.evil.example:8080") }
	req, _ := http.NewRequest(http.MethodGet, "https://api.anthropic.com/v1/messages", nil)
	_, err := WrapProxyFunc(next, "test.site")(req)
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("proxy host must be checked even when the destination is allowed; got %v", err)
	}
	if !strings.Contains(err.Error(), "proxy.evil.example") || !strings.Contains(err.Error(), "test.site proxy") {
		t.Fatalf("refusal should name the proxy host and the proxy site, got %q", err)
	}
}

func TestWrapProxyFuncAllowsListedProxyHost(t *testing.T) {
	t.Cleanup(resetForTest)
	SetConfigWithBuiltin(testConfig("enforce", "proxy.corp.example"), []string{"api.anthropic.com"})
	next := func(*http.Request) (*url.URL, error) { return url.Parse("http://proxy.corp.example:8080") }
	req, _ := http.NewRequest(http.MethodGet, "https://api.anthropic.com/v1/messages", nil)
	u, err := WrapProxyFunc(next, "test.site")(req)
	if err != nil || u == nil || u.Host != "proxy.corp.example:8080" {
		t.Fatalf("listed proxy host must pass through, got u=%v err=%v", u, err)
	}
}

func TestGuardDialerRefusesUnlistedHost(t *testing.T) {
	t.Cleanup(resetForTest)
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	d := GuardDialer(proxy.Direct, "test.dialer")
	if _, err := d.Dial("tcp", "proxy.evil.example:1080"); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("Dial to unlisted host must be refused before connecting, got %v", err)
	}
	if _, err := d.(proxy.ContextDialer).DialContext(context.Background(), "tcp", "proxy.evil.example:1080"); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("DialContext to unlisted host must be refused before connecting, got %v", err)
	}
}

func TestGuardDialContextRefusesUnlistedHost(t *testing.T) {
	t.Cleanup(resetForTest)
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	called := false
	fn := GuardDialContext(func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil }, "test.dialctx")
	if _, err := fn(context.Background(), "tcp", "proxy.evil.example:1080"); !errors.Is(err, ErrNotPermitted) || called {
		t.Fatalf("guarded dial func must refuse before calling through; err=%v called=%v", err, called)
	}
	if _, err := fn(context.Background(), "tcp", "127.0.0.1:1080"); err != nil || !called {
		t.Fatalf("guarded dial func must call through for loopback; err=%v called=%v", err, called)
	}
}

func TestCheckHostPortStripsPort(t *testing.T) {
	t.Cleanup(resetForTest)
	SetConfigWithBuiltin(testConfig("enforce", "proxy.corp.example"), nil)
	if err := CheckHostPort("proxy.corp.example:8080", "s"); err != nil {
		t.Fatalf("listed host:port should pass, got %v", err)
	}
	if err := CheckHostPort("[::1]:8080", "s"); err != nil {
		t.Fatalf("bracketed loopback should pass, got %v", err)
	}
	if err := CheckHostPort("proxy.evil.example", "s"); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("bare unlisted host should be refused, got %v", err)
	}
}
