package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/signaturedrops"
)

func TestSignatureDropsEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := signaturedrops.NewStore()
	signaturedrops.SetDefault(store)
	t.Cleanup(func() { signaturedrops.SetDefault(nil) })
	const reason = "invalid Claude model-free CAIS signature: unknown channel_id 18"
	store.Record(reason, "claude-opus-5-5", 44)
	store.Record(reason, "claude-fable-5-1", 2)

	handler := &Handler{}
	engine := gin.New()
	engine.GET("/v0/management/signature-drops", handler.GetSignatureDrops)
	engine.DELETE("/v0/management/signature-drops", handler.DeleteSignatureDrops)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v0/management/signature-drops", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", rec.Code, rec.Body.String())
	}
	var snap struct {
		Requests       int64 `json:"requests"`
		Blocks         int64 `json:"blocks"`
		RecentRequests int64 `json:"recent_requests"`
		WindowSeconds  int64 `json:"window_seconds"`
		Reasons        []struct {
			Reason string   `json:"reason"`
			Count  int64    `json:"count"`
			Models []string `json:"models"`
		} `json:"reasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.Requests != 2 || snap.Blocks != 46 || snap.RecentRequests != 2 || snap.WindowSeconds == 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if len(snap.Reasons) != 1 || snap.Reasons[0].Reason != reason || len(snap.Reasons[0].Models) != 2 {
		t.Fatalf("reasons = %+v", snap.Reasons)
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v0/management/signature-drops", nil))
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("DELETE status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := store.Snapshot().Requests; got != 0 {
		t.Fatalf("after DELETE requests = %d, want 0", got)
	}
}
