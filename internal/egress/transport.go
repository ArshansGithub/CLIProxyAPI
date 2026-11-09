package egress

import (
	"context"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

// WrapProxyFunc returns a Transport.Proxy-compatible function that applies the
// egress policy to every request before delegating to next (which may be nil,
// meaning "no proxy"). Transport.Proxy is consulted on every request, which is
// what makes it the right hook: it sees the destination URL even when a
// forward proxy is configured, and it keeps *http.Transport as the concrete
// type so upstream code that type-asserts DefaultTransport keeps working.
//
// A non-nil proxy URL is where net/http actually opens the socket, so the
// proxy host is checked too: an allowed destination must not become a way to
// reach an unlisted proxy. Refusals name the site with a " proxy" suffix.
func WrapProxyFunc(next func(*http.Request) (*url.URL, error), site string) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if err := CheckURL(req.URL, site); err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		proxyURL, err := next(req)
		if err != nil || proxyURL == nil {
			return proxyURL, err
		}
		if err := CheckURL(proxyURL, site+" proxy"); err != nil {
			return nil, err
		}
		return proxyURL, nil
	}
}

// CheckHostPort applies the current policy to a "host:port" or bare host
// string, as handed to a dialer.
func CheckHostPort(addr, site string) error {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	return Current().Check(host, site)
}

// GuardDialContext wraps a DialContext-shaped function so the address it is
// about to connect to is checked first. Use it where a socket is opened to a
// forward proxy: Transport.Proxy never sees SOCKS proxies or hand-rolled
// CONNECT dials, so the check has to sit at the dial.
func GuardDialContext(dial func(ctx context.Context, network, addr string) (net.Conn, error), site string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if err := CheckHostPort(addr, site); err != nil {
			return nil, err
		}
		return dial(ctx, network, addr)
	}
}

// GuardDialer wraps a proxy.Dialer the same way. Pass it as the forward
// dialer of a SOCKS or CONNECT dialer: the forward dialer is what connects to
// the proxy host, which is exactly the address that needs checking.
func GuardDialer(d proxy.Dialer, site string) proxy.Dialer {
	if d == nil {
		d = proxy.Direct
	}
	return &guardedDialer{next: d, site: site}
}

type guardedDialer struct {
	next proxy.Dialer
	site string
}

func (g *guardedDialer) Dial(network, addr string) (net.Conn, error) {
	if err := CheckHostPort(addr, g.site); err != nil {
		return nil, err
	}
	return g.next.Dial(network, addr)
}

func (g *guardedDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := CheckHostPort(addr, g.site); err != nil {
		return nil, err
	}
	if cd, ok := g.next.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return g.next.Dial(network, addr)
}

// GuardTransport installs the policy on t's Proxy function and returns t.
// site names the construction point and is what a refusal log identifies, so
// give each call site its own label rather than a shared "http.Transport".
func GuardTransport(t *http.Transport, site string) *http.Transport {
	if t == nil {
		return nil
	}
	t.Proxy = WrapProxyFunc(t.Proxy, site)
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
		GuardTransport(t, "http.DefaultTransport")
	}
}
