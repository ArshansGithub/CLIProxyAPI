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
	client := &http.Client{Transport: GuardTransport(&http.Transport{})}
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
	GuardTransport(tr)
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
