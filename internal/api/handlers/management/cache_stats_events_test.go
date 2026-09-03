package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/cachestats"
)

func getCacheStatsEvents(t *testing.T, query string) (int, cacheStatsEventsResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v0/management/cache-stats/events"+query, nil)
	(&Handler{}).GetCacheStatsEvents(c)

	var body cacheStatsEventsResponse
	if recorder.Code == http.StatusOK {
		if errDecode := json.Unmarshal(recorder.Body.Bytes(), &body); errDecode != nil {
			t.Fatalf("response is not valid JSON: %v (%s)", errDecode, recorder.Body.String())
		}
	}
	return recorder.Code, body
}

func TestGetCacheStatsEventsPagesByAfter(t *testing.T) {
	store := cachestats.NewStore(cachestats.Config{Enabled: true})
	cachestats.SetDefault(store)
	t.Cleanup(func() { cachestats.SetDefault(nil) })

	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	record := func(read int64, at time.Time) {
		store.Record(cachestats.Observation{
			SessionID: "11111111-2222-3333-4444-555555555555", KeyedBy: cachestats.KeyedBySession,
			Provider: "claude", Model: "claude-sonnet-5", AuthID: "auth-1", At: at,
			Signal: cachestats.SignalFull, PromptTokens: read + 10, CacheReadTokens: read,
		})
	}
	record(0, base)
	record(5000, base.Add(time.Second))
	record(1000, base.Add(2*time.Second))
	record(900, base.Add(3*time.Second))

	status, body := getCacheStatsEvents(t, "")
	if status != http.StatusOK || !body.Enabled || body.LatestSeq != 2 || len(body.Events) != 2 {
		t.Fatalf("status=%d body=%+v", status, body)
	}
	if body.Events[0].Kind != cachestats.EventPartialMiss || body.Events[0].ShortID != "11111111" {
		t.Fatalf("first event = %+v", body.Events[0])
	}

	status, body = getCacheStatsEvents(t, "?after=1")
	if status != http.StatusOK || len(body.Events) != 1 || body.Events[0].Seq != 2 {
		t.Fatalf("after=1 -> status=%d body=%+v", status, body)
	}

	status, body = getCacheStatsEvents(t, "?limit=1")
	if status != http.StatusOK || len(body.Events) != 1 || body.Events[0].Seq != 2 {
		t.Fatalf("limit=1 -> status=%d body=%+v", status, body)
	}
}

func TestGetCacheStatsEventsRejectsBadQuery(t *testing.T) {
	cachestats.SetDefault(nil)
	if status, _ := getCacheStatsEvents(t, "?after=x"); status != http.StatusBadRequest {
		t.Fatalf("after=x status = %d, want 400", status)
	}
	if status, _ := getCacheStatsEvents(t, "?limit=-1"); status != http.StatusBadRequest {
		t.Fatalf("limit=-1 status = %d, want 400", status)
	}
	status, body := getCacheStatsEvents(t, "")
	if status != http.StatusOK || body.Enabled || body.Events == nil {
		t.Fatalf("disabled store -> status=%d body=%+v", status, body)
	}
}
