package egress

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// WrapProxyFunc returns a Transport.Proxy-compatible function that applies the
// egress policy to every request before delegating to next (which may be nil,
// meaning "no proxy"). Transport.Proxy is consulted on every request, which is
// what makes it the right hook: it sees the destination URL even when a
// forward proxy is configured, and it keeps *http.Transport as the concrete
// type so upstream code that type-asserts DefaultTransport keeps working.
func WrapProxyFunc(next func(*http.Request) (*url.URL, error), site string) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if err := CheckURL(req.URL, site); err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		return next(req)
	}
}

// GuardTransport installs the policy on t's Proxy function and returns t.
func GuardTransport(t *http.Transport) *http.Transport {
	if t == nil {
		return nil
	}
	t.Proxy = WrapProxyFunc(t.Proxy, "http.Transport")
	return t
}

// GuardWebsocketDialer installs the policy on d's Proxy function and returns d.
func GuardWebsocketDialer(d *websocket.Dialer, site string) *websocket.Dialer {
	if d == nil {
		return nil
	}
	d.Proxy = WrapProxyFunc(d.Proxy, site)
	return d
}

type guardedRoundTripper struct {
	next http.RoundTripper
	site string
}

func (g *guardedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := CheckURL(req.URL, g.site); err != nil {
		return nil, err
	}
	return g.next.RoundTrip(req)
}

// Unwrap returns the wrapped round tripper. The guard still runs on every
// request; this only lets callers inspect the concrete type underneath.
func (g *guardedRoundTripper) Unwrap() http.RoundTripper { return g.next }

// CloseIdleConnections forwards to the wrapped round tripper when it supports
// it. Without this the wrapper would shadow the method, and callers that evict
// cached round trippers by calling it would silently stop closing pools.
func (g *guardedRoundTripper) CloseIdleConnections() {
	if closer, ok := g.next.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// RoundTripper wraps a non-*http.Transport round tripper (the uTLS clients).
func RoundTripper(rt http.RoundTripper, site string) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &guardedRoundTripper{next: rt, site: site}
}

func init() {
	// Cover every client that relies on the default transport.
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		GuardTransport(t)
	}
}
