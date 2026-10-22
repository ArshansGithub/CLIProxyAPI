// Package signaturedrops counts the thinking blocks the Claude executor drops
// because their CAIS signature carries a generation identifier the fork does
// not recognize (see knownClaudeCAISChannelIDs in internal/signature).
//
// Such a drop strips a request's entire thinking history before it reaches
// Anthropic. The response still succeeds, so the only symptom was one warning
// line per request in main.log; channel id 18 fired that line 93,667 times
// over ten days (2026-09-19 to 2026-09-29) before anyone read it. This counter
// feeds GET /v0/management/signature-drops and the cache watch page's banner
// so the next identifier bump is visible on the page the operator already
// watches.
package signaturedrops

import (
	"sort"
	"sync"
	"time"
)

// Reason is one distinct drop reason with its count and the models it hit.
type Reason struct {
	Reason string `json:"reason"`
	// Count is dropped requests, not dropped blocks: one request that lost
	// forty thinking blocks to the same reason counts once, because the
	// question the page answers is "how much traffic is affected".
	Count     int64     `json:"count"`
	Blocks    int64     `json:"blocks"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	LastModel string    `json:"last_model"`
	Models    []string  `json:"models"`
}

// Snapshot is the payload of GET /v0/management/signature-drops.
type Snapshot struct {
	// Requests is the number of requests that lost at least one thinking block
	// to an unknown generation since the proxy started.
	Requests int64 `json:"requests"`
	Blocks   int64 `json:"blocks"`
	// RecentRequests counts the same within Window of now.
	RecentRequests int64         `json:"recent_requests"`
	Window         time.Duration `json:"-"`
	WindowSeconds  int64         `json:"window_seconds"`
	FirstSeen      time.Time     `json:"first_seen"`
	LastSeen       time.Time     `json:"last_seen"`
	Reasons        []Reason      `json:"reasons"`
}

type reasonState struct {
	count, blocks int64
	first, last   time.Time
	lastModel     string
	models        map[string]struct{}
}

// Store is a process-wide counter. It keeps no per-request history, only a
// short ring of timestamps for the recent-window count.
type Store struct {
	mu       sync.Mutex
	now      func() time.Time
	requests int64
	blocks   int64
	first    time.Time
	last     time.Time
	reasons  map[string]*reasonState
	recent   []time.Time
	window   time.Duration
}

// DefaultWindow is how far back "recent" reaches on the page: long enough that
// one refresh after a quiet stretch still shows what happened, short enough
// that a fixed allowlist clears the banner the same day.
const DefaultWindow = 6 * time.Hour

const maxRecent = 4096

// NewStore returns an empty counter.
func NewStore() *Store {
	return &Store{now: time.Now, reasons: map[string]*reasonState{}, window: DefaultWindow}
}

var (
	defaultMu    sync.RWMutex
	defaultStore = NewStore()
)

// Default is the counter the executor writes and the management API reads.
func Default() *Store {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultStore
}

// SetDefault swaps the process counter; tests use it, and nil restores a fresh one.
func SetDefault(s *Store) {
	if s == nil {
		s = NewStore()
	}
	defaultMu.Lock()
	defaultStore = s
	defaultMu.Unlock()
}

// Record notes one request that lost blocks thinking blocks to reason while
// targeting model. reason is the normalized drop reason the executor already
// classifies ("invalid Claude model-free CAIS signature: unknown channel_id 18").
func (s *Store) Record(reason, model string, blocks int64) {
	if s == nil || reason == "" {
		return
	}
	if blocks < 1 {
		blocks = 1
	}
	at := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.blocks += blocks
	if s.first.IsZero() {
		s.first = at
	}
	s.last = at
	r := s.reasons[reason]
	if r == nil {
		r = &reasonState{first: at, models: map[string]struct{}{}}
		s.reasons[reason] = r
	}
	r.count++
	r.blocks += blocks
	r.last = at
	r.lastModel = model
	if model != "" {
		r.models[model] = struct{}{}
	}
	s.recent = append(s.recent, at)
	if len(s.recent) > maxRecent {
		s.recent = s.recent[len(s.recent)-maxRecent:]
	}
}

// Snapshot summarizes the counter. Reasons are ordered by most recent first.
func (s *Store) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{Reasons: []Reason{}}
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{
		Requests:      s.requests,
		Blocks:        s.blocks,
		Window:        s.window,
		WindowSeconds: int64(s.window / time.Second),
		FirstSeen:     s.first,
		LastSeen:      s.last,
		Reasons:       make([]Reason, 0, len(s.reasons)),
	}
	cutoff := now.Add(-s.window)
	for i := len(s.recent) - 1; i >= 0; i-- {
		if s.recent[i].Before(cutoff) {
			break
		}
		out.RecentRequests++
	}
	// The ring holds at most maxRecent stamps; past that the recent count is a
	// floor, which is fine for a banner that only asks "is it still happening".
	for text, r := range s.reasons {
		models := make([]string, 0, len(r.models))
		for m := range r.models {
			models = append(models, m)
		}
		sort.Strings(models)
		out.Reasons = append(out.Reasons, Reason{
			Reason: text, Count: r.count, Blocks: r.blocks,
			FirstSeen: r.first, LastSeen: r.last, LastModel: r.lastModel, Models: models,
		})
	}
	sort.Slice(out.Reasons, func(i, j int) bool {
		if !out.Reasons[i].LastSeen.Equal(out.Reasons[j].LastSeen) {
			return out.Reasons[i].LastSeen.After(out.Reasons[j].LastSeen)
		}
		return out.Reasons[i].Reason < out.Reasons[j].Reason
	})
	return out
}

// Reset clears the counter and returns how many requests it had counted.
func (s *Store) Reset() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.requests
	s.requests, s.blocks = 0, 0
	s.first, s.last = time.Time{}, time.Time{}
	s.reasons = map[string]*reasonState{}
	s.recent = nil
	return n
}
