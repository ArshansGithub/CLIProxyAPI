package cachestats

import (
	"fmt"
	"strings"
	"time"
)

// EventKind names one kind of prompt-cache loss the store noticed.
type EventKind string

const (
	// EventColdRead is a request after the session's first that read nothing
	// from cache: the prefix expired, or the session moved to another account.
	EventColdRead EventKind = "cold-read"
	// EventPartialMiss is a request that read less than the session's
	// high-water mark: part of the prefix stopped matching.
	EventPartialMiss EventKind = "partial-miss"
	// EventAlert marks the sustained-loss alert crossing its threshold.
	EventAlert EventKind = "alert"
)

// DefaultMaxEvents bounds the process-wide event ring when the config leaves
// it unset.
const DefaultMaxEvents = 1000

// Event is one retained cache-loss event with the evidence behind it.
type Event struct {
	// Seq is a store-wide monotonically increasing sequence number, so a
	// client can poll for everything after the last event it saw.
	Seq        int64     `json:"seq"`
	At         time.Time `json:"at"`
	Kind       EventKind `json:"kind"`
	SessionID  string    `json:"session_id"`
	ShortID    string    `json:"short_id"`
	KeyedBy    KeyedBy   `json:"keyed_by"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	AuthID     string    `json:"auth_id"`
	PrevAuthID string    `json:"prev_auth_id,omitempty"`
	RequestSeq int       `json:"request_seq"`
	// CacheReadTokens is what the triggering request read; PrevReadTokens is
	// the previous request's read and MaxReadTokens the session high-water mark.
	CacheReadTokens int64 `json:"cache_read_tokens"`
	PrevReadTokens  int64 `json:"prev_read_tokens"`
	MaxReadTokens   int64 `json:"max_read_tokens"`
	LostTokens      int64 `json:"lost_tokens"`
	// MissReason and MissedTokens are the upstream diagnostics when present.
	MissReason   string `json:"miss_reason,omitempty"`
	MissedTokens int64  `json:"missed_tokens,omitempty"`
	// GapSeconds is the idle time between this request and the previous one
	// in the same session. Zero for the first request.
	GapSeconds float64 `json:"gap_seconds"`
	Regime     Regime  `json:"regime"`
	T0Cause    T0Cause `json:"t0_cause,omitempty"`
	IsProbe    bool    `json:"is_probe"`
	// Cause is a plain-language explanation assembled from the fields above.
	Cause string `json:"cause"`
}

// EventsPage is one read of the event ring.
type EventsPage struct {
	// LatestSeq is the newest sequence number the store has issued, whether or
	// not it is inside this page. A client passes it back as `after`.
	LatestSeq int64   `json:"latest_seq"`
	Events    []Event `json:"events"`
}

// Events returns retained events with a sequence number greater than after,
// oldest first, capped at limit. A limit of zero or less means the whole ring.
func (s *Store) Events(after int64, limit int) EventsPage {
	page := EventsPage{Events: []Event{}}
	if s == nil {
		return page
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	page.LatestSeq = s.eventSeq
	start := 0
	for start < len(s.events) && s.events[start].Seq <= after {
		start++
	}
	selected := s.events[start:]
	if limit > 0 && len(selected) > limit {
		selected = selected[len(selected)-limit:]
	}
	page.Events = make([]Event, len(selected))
	copy(page.Events, selected)
	return page
}

// appendEventLocked assigns the next sequence number and retains the event.
func (s *Store) appendEventLocked(event Event) {
	s.eventSeq++
	event.Seq = s.eventSeq
	s.events = append(s.events, event)
	limit := s.cfg.MaxEvents
	if limit <= 0 {
		limit = DefaultMaxEvents
	}
	if len(s.events) > limit {
		s.events = append(s.events[:0], s.events[len(s.events)-limit:]...)
	}
}

// regimeTTL is the cache TTL the session's dominant pool promises.
func regimeTTL(regime Regime) time.Duration {
	switch regime {
	case Regime1h:
		return time.Hour
	case Regime5m, RegimeMixed:
		return 5 * time.Minute
	default:
		return 0
	}
}

// describeCause writes the explanation shown next to an event. It reasons only
// from what the proxy can see: the gap since the previous request, the
// account that served each request, the token counts, and the upstream
// miss diagnosis when one was reported.
func describeCause(event Event) string {
	var parts []string
	gap := time.Duration(event.GapSeconds * float64(time.Second)).Round(time.Second)
	ttl := regimeTTL(event.Regime)
	who := "request"
	if event.IsProbe {
		who = "keepalive probe"
	}

	switch event.Kind {
	case EventAlert:
		parts = append(parts, fmt.Sprintf("sustained loss: %s cached tokens lost inside the last hour", formatTokens(event.LostTokens)))
	case EventColdRead:
		switch event.T0Cause {
		case T0CauseRebind:
			parts = append(parts, fmt.Sprintf("%s served by %s after %s: the cached prefix lives on the previous account, not this one (session affinity rebind: cooldown, quota, or pool rotation)",
				who, labelOr(event.AuthID, "an unknown account"), labelOr(event.PrevAuthID, "another account")))
		default:
			switch {
			case ttl > 0 && gap > ttl:
				parts = append(parts, fmt.Sprintf("%s read nothing after %s idle on the same account, past the %s pool TTL: the entry expired", who, gap, ttlLabel(ttl)))
			case ttl > 0:
				parts = append(parts, fmt.Sprintf("%s read nothing after only %s idle on the same account, inside the %s pool TTL: the prefix no longer matched (system prompt, tools, model, or headers changed)", who, gap, ttlLabel(ttl)))
			default:
				parts = append(parts, fmt.Sprintf("%s read nothing after %s idle on the same account", who, gap))
			}
		}
	case EventPartialMiss:
		parts = append(parts, fmt.Sprintf("%s read %s, down from %s (session high-water %s): the first %s tokens still matched, %s did not",
			who, formatTokens(event.CacheReadTokens), formatTokens(event.PrevReadTokens), formatTokens(event.MaxReadTokens),
			formatTokens(event.CacheReadTokens), formatTokens(event.LostTokens)))
		switch {
		case event.PrevAuthID != "" && event.PrevAuthID != event.AuthID:
			parts = append(parts, fmt.Sprintf("account changed from %s to %s", event.PrevAuthID, event.AuthID))
		case ttl > 0 && gap > ttl:
			parts = append(parts, fmt.Sprintf("%s idle exceeds the %s pool TTL, so later breakpoints expired while an earlier one stayed warm", gap, ttlLabel(ttl)))
		default:
			parts = append(parts, fmt.Sprintf("%s idle on the same account: content after the matched breakpoint changed", gap))
		}
	}

	if reason := strings.TrimSpace(event.MissReason); reason != "" {
		if event.MissedTokens > 0 {
			parts = append(parts, fmt.Sprintf("upstream reports cache_miss_reason=%s (%s tokens missed)", reason, formatTokens(event.MissedTokens)))
		} else {
			parts = append(parts, fmt.Sprintf("upstream reports cache_miss_reason=%s", reason))
		}
	}
	return strings.Join(parts, "; ")
}

func ttlLabel(ttl time.Duration) string {
	if ttl >= time.Hour {
		return "1h"
	}
	return "5m"
}

func labelOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// formatTokens renders a token count with thousands separators.
func formatTokens(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)
	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(r)
	}
	if negative {
		return "-" + out.String()
	}
	return out.String()
}
