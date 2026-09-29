package signaturedrops

import (
	"testing"
	"time"
)

func TestStore_RecordAndSnapshot(t *testing.T) {
	base := time.Date(2026, 9, 29, 4, 30, 0, 0, time.UTC)
	clock := base
	s := NewStore()
	s.now = func() time.Time { return clock }

	const ch18 = "invalid Claude model-free CAIS signature: unknown channel_id 18"
	const env5 = "invalid Claude model-free CAIS signature: unknown envelope version 5"
	s.Record(ch18, "claude-opus-5-5", 44)
	clock = clock.Add(time.Minute)
	s.Record(ch18, "claude-fable-5-1", 3)
	clock = clock.Add(time.Minute)
	s.Record(env5, "claude-opus-5-5", 1)

	snap := s.Snapshot()
	if snap.Requests != 3 || snap.Blocks != 48 {
		t.Fatalf("requests=%d blocks=%d, want 3 and 48", snap.Requests, snap.Blocks)
	}
	if snap.RecentRequests != 3 {
		t.Fatalf("recent=%d, want 3 inside the window", snap.RecentRequests)
	}
	if !snap.FirstSeen.Equal(base) || !snap.LastSeen.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("first=%v last=%v", snap.FirstSeen, snap.LastSeen)
	}
	if len(snap.Reasons) != 2 || snap.Reasons[0].Reason != env5 {
		t.Fatalf("reasons ordered most recent first: %+v", snap.Reasons)
	}
	r := snap.Reasons[1]
	if r.Count != 2 || r.Blocks != 47 || r.LastModel != "claude-fable-5-1" {
		t.Fatalf("channel 18 reason: %+v", r)
	}
	if len(r.Models) != 2 || r.Models[0] != "claude-fable-5-1" || r.Models[1] != "claude-opus-5-5" {
		t.Fatalf("models sorted: %v", r.Models)
	}
}

func TestStore_RecentWindowExpires(t *testing.T) {
	base := time.Date(2026, 9, 29, 4, 30, 0, 0, time.UTC)
	clock := base
	s := NewStore()
	s.now = func() time.Time { return clock }
	s.Record("r", "m", 1)
	clock = base.Add(DefaultWindow + time.Second)
	s.Record("r", "m", 1)
	snap := s.Snapshot()
	if snap.Requests != 2 || snap.RecentRequests != 1 {
		t.Fatalf("requests=%d recent=%d, want 2 total and 1 recent", snap.Requests, snap.RecentRequests)
	}
}

func TestStore_ResetAndNilSafety(t *testing.T) {
	s := NewStore()
	s.Record("r", "m", 2)
	if n := s.Reset(); n != 1 {
		t.Fatalf("reset returned %d, want 1", n)
	}
	if snap := s.Snapshot(); snap.Requests != 0 || len(snap.Reasons) != 0 || snap.RecentRequests != 0 {
		t.Fatalf("after reset: %+v", snap)
	}
	var none *Store
	none.Record("r", "m", 1)
	if snap := none.Snapshot(); snap.Requests != 0 || snap.Reasons == nil {
		t.Fatalf("nil store snapshot: %+v", snap)
	}
	s.Record("", "m", 1)
	if snap := s.Snapshot(); snap.Requests != 0 {
		t.Fatal("empty reason must not count")
	}
}
