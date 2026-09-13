package egress

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
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
