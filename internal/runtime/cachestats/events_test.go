package cachestats

import (
	"strings"
	"testing"
	"time"
)

func TestEventsRecordColdReadExpiry(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	first := observation("s1", 0, 40000, base)
	first.CacheCreation5mTokens = 40000
	store.Record(first)
	store.Record(observation("s1", 40000, 100, base.Add(time.Minute)))
	// Seven minutes idle on a 5m-pool session, then a cold read.
	store.Record(observation("s1", 0, 40100, base.Add(8*time.Minute)))

	page := store.Events(0, 0)
	if len(page.Events) != 1 {
		t.Fatalf("events = %d, want 1: %+v", len(page.Events), page.Events)
	}
	event := page.Events[0]
	if event.Kind != EventColdRead || event.T0Cause != T0CauseExpiry {
		t.Fatalf("event = %+v, want cold-read/expiry", event)
	}
	if event.Seq != 1 || page.LatestSeq != 1 {
		t.Fatalf("seq = %d latest = %d, want 1/1", event.Seq, page.LatestSeq)
	}
	if event.GapSeconds != 420 {
		t.Fatalf("gap = %v, want 420s", event.GapSeconds)
	}
	if event.LostTokens != 40000 || event.MaxReadTokens != 40000 || event.PrevReadTokens != 40000 {
		t.Fatalf("token fields = %+v", event)
	}
	if event.Regime != Regime5m {
		t.Fatalf("regime = %q, want 5m", event.Regime)
	}
	if !strings.Contains(event.Cause, "7m0s idle") || !strings.Contains(event.Cause, "expired") {
		t.Fatalf("cause = %q", event.Cause)
	}
	if event.ShortID != "s1" && event.SessionID != "s1" {
		t.Fatalf("session identity = %+v", event)
	}
}

func TestEventsRecordColdReadInsideTTLBlamesPrefix(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	first := observation("s1", 0, 40000, base)
	first.CacheCreation5mTokens = 40000
	store.Record(first)
	cold := observation("s1", 0, 40000, base.Add(30*time.Second))
	cold.CacheMissReason = "messages_changed"
	cold.CacheMissedTokens = 40000
	store.Record(cold)

	events := store.Events(0, 0).Events
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	cause := events[0].Cause
	if !strings.Contains(cause, "inside the 5m pool TTL") || !strings.Contains(cause, "no longer matched") {
		t.Fatalf("cause = %q", cause)
	}
	if !strings.Contains(cause, "cache_miss_reason=messages_changed (40,000 tokens missed)") {
		t.Fatalf("cause lacks upstream diagnosis: %q", cause)
	}
}

func TestEventsRecordRebind(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	store.Record(observation("s1", 0, 1000, base))
	moved := observation("s1", 0, 1000, base.Add(time.Second))
	moved.AuthID = "auth-2"
	store.Record(moved)

	events := store.Events(0, 0).Events
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	if event.T0Cause != T0CauseRebind || event.AuthID != "auth-2" || event.PrevAuthID != "auth-1" {
		t.Fatalf("event = %+v", event)
	}
	if !strings.Contains(event.Cause, "served by auth-2 after auth-1") {
		t.Fatalf("cause = %q", event.Cause)
	}
}

func TestEventsRecordPartialMiss(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	store.Record(observation("s1", 0, 1000, base))
	store.Record(observation("s1", 5000, 0, base.Add(time.Second)))
	store.Record(observation("s1", 1200, 4000, base.Add(2*time.Second)))

	events := store.Events(0, 0).Events
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != EventPartialMiss || event.LostTokens != 3800 || event.PrevReadTokens != 5000 || event.CacheReadTokens != 1200 {
		t.Fatalf("event = %+v", event)
	}
	if !strings.Contains(event.Cause, "read 1,200, down from 5,000") || !strings.Contains(event.Cause, "3,800 did not") {
		t.Fatalf("cause = %q", event.Cause)
	}
}

func TestEventsFirstRequestAndHitsProduceNothing(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	store.Record(observation("s1", 0, 1000, base))
	store.Record(observation("s1", 1000, 10, base.Add(time.Second)))
	store.Record(observation("s1", 1010, 10, base.Add(2*time.Second)))

	if page := store.Events(0, 0); len(page.Events) != 0 || page.LatestSeq != 0 {
		t.Fatalf("expected no events, got %+v", page)
	}
}

func TestEventsAlertCrossingIsRecordedOnce(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := NewStore(Config{
		Enabled: true, MaxSessions: 10, PerSessionRequests: 10, IdleTTL: time.Hour,
		Alert: AlertConfig{Enabled: true, LostTokensPerHour: 5000},
	})
	store.Record(observation("s1", 0, 1000, base))
	store.Record(observation("s1", 10000, 0, base.Add(time.Second)))
	store.Record(observation("s1", 2000, 0, base.Add(2*time.Second))) // lost 8000: crosses
	store.Record(observation("s1", 1000, 0, base.Add(3*time.Second))) // lost 9000: still alerting, no new alert

	var alerts, misses int
	for _, event := range store.Events(0, 0).Events {
		switch event.Kind {
		case EventAlert:
			alerts++
		case EventPartialMiss:
			misses++
		}
	}
	if alerts != 1 || misses != 2 {
		t.Fatalf("alerts = %d misses = %d, want 1/2", alerts, misses)
	}
}

func TestEventsPagingAndBound(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := NewStore(Config{Enabled: true, MaxSessions: 10, PerSessionRequests: 10, IdleTTL: time.Hour, MaxEvents: 3})

	store.Record(observation("s1", 0, 1000, base))
	store.Record(observation("s1", 9000, 0, base.Add(time.Second)))
	for i := 0; i < 5; i++ {
		// Each request reads less than the high-water mark: five misses.
		store.Record(observation("s1", int64(8000-i*100), 0, base.Add(time.Duration(2+i)*time.Second)))
	}

	page := store.Events(0, 0)
	if page.LatestSeq != 5 {
		t.Fatalf("latest = %d, want 5", page.LatestSeq)
	}
	if len(page.Events) != 3 || page.Events[0].Seq != 3 {
		t.Fatalf("ring = %+v, want seqs 3..5", page.Events)
	}
	if after := store.Events(4, 0); len(after.Events) != 1 || after.Events[0].Seq != 5 {
		t.Fatalf("after=4 -> %+v, want seq 5 only", after.Events)
	}
	if limited := store.Events(0, 2); len(limited.Events) != 2 || limited.Events[0].Seq != 4 {
		t.Fatalf("limit=2 -> %+v, want seqs 4..5", limited.Events)
	}
	// Reset drops the ring but keeps the sequence monotonic, so a client that
	// polls with `after` never sees stale sequence numbers reused.
	if store.Reset(); len(store.Events(0, 0).Events) != 0 || store.Events(0, 0).LatestSeq != 5 {
		t.Fatalf("reset -> %+v, want empty ring with latest 5", store.Events(0, 0))
	}
	var nilStore *Store
	if page := nilStore.Events(0, 0); page.Events == nil {
		t.Fatal("nil store must return an empty, non-nil page")
	}
}

func TestSummaryCarriesLastRequestState(t *testing.T) {
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, 10, 10, time.Hour)

	store.Record(observation("s1", 0, 1000, base))
	store.Record(observation("s1", 5000, 0, base.Add(time.Second)))
	miss := observation("s1", 1200, 4000, base.Add(31*time.Second))
	miss.CacheMissReason = "messages_changed"
	store.Record(miss)

	detail, ok := store.Session("s1")
	if !ok {
		t.Fatal("Session(s1) not found")
	}
	summary := detail.Summary
	if summary.LastCacheReadTokens != 1200 || summary.MaxCacheReadTokens != 5000 {
		t.Fatalf("read fields = %+v", summary)
	}
	if summary.LastTier != TierMiss || summary.LastMissReason != "messages_changed" || summary.LastGapSeconds != 30 {
		t.Fatalf("state fields = %+v", summary)
	}
}
