package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCacheWatchPageIsServedAndGated(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/cache.html", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	for _, want := range []string{"<title>Cache watch</title>", "/v0/management/cache-stats/events", "/v0/management/cache-keepalive"} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("page missing %q", want)
		}
	}

	cfg := *server.cfg
	cfg.RemoteManagement.DisableControlPanel = true
	server.UpdateClients(&cfg)
	rr = httptest.NewRecorder()
	server.engine.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cache.html", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("with the control panel disabled status = %d, want 404", rr.Code)
	}
}
