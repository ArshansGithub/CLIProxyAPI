package executor

import (
	"container/list"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/MoeForge/moego/ojson"
	log "github.com/sirupsen/logrus"
)

// stripFallbacks is the no-substitution.strip-fallbacks switch (fork-only;
// docs/fork/specs/2026-10-07-no-substitution-design.md).
var stripFallbacks atomic.Bool

// SetStripFallbacks turns the Claude `fallbacks` strip on or off for the process.
func SetStripFallbacks(on bool) { stripFallbacks.Store(on) }

// StripFallbacks reports whether the Claude executor deletes the `fallbacks`
// body key and never injects its own.
func StripFallbacks() bool { return stripFallbacks.Load() }

// stripFallbacksSeenMax bounds the once-per-session Warn: sessions started
// before the client-side switches were set keep sending `fallbacks` on every
// request until they restart, and one Warn per session says enough.
const stripFallbacksSeenMax = 1024

var stripFallbacksSeen = struct {
	mu    sync.Mutex
	order *list.List
	keys  map[string]*list.Element
}{order: list.New(), keys: map[string]*list.Element{}}

func resetStripFallbacksSeen() {
	stripFallbacksSeen.mu.Lock()
	defer stripFallbacksSeen.mu.Unlock()
	stripFallbacksSeen.order.Init()
	stripFallbacksSeen.keys = map[string]*list.Element{}
}

// firstStripForSession reports whether this session key has not warned yet,
// recording it. An empty key is never remembered, so it always warns.
func firstStripForSession(key string) bool {
	if key == "" {
		return true
	}
	stripFallbacksSeen.mu.Lock()
	defer stripFallbacksSeen.mu.Unlock()
	if el, ok := stripFallbacksSeen.keys[key]; ok {
		stripFallbacksSeen.order.MoveToFront(el)
		return false
	}
	stripFallbacksSeen.keys[key] = stripFallbacksSeen.order.PushFront(key)
	for stripFallbacksSeen.order.Len() > stripFallbacksSeenMax {
		oldest := stripFallbacksSeen.order.Back()
		delete(stripFallbacksSeen.keys, oldest.Value.(string))
		stripFallbacksSeen.order.Remove(oldest)
	}
	return true
}

// stripSessionKey picks the key for the once-per-session Warn: the resolved
// Claude Code session id when the fingerprint profile computed one, else the
// raw session header, so the bound holds whatever the profile setting.
func stripSessionKey(sessionID string, headers http.Header) string {
	if sessionID != "" {
		return sessionID
	}
	if headers == nil {
		return ""
	}
	return strings.TrimSpace(headers.Get("X-Claude-Code-Session-Id"))
}

// stripFallbacksForUpstream deletes the `fallbacks` key, whatever its JSON
// type, from the finished body. It runs after the cloak reconcile and the
// payload rules and before the CCH billing signature, so nothing re-adds the
// key. A confirmed native client carrying the key means the client-side
// switch is not set, which is worth one Warn per session; cloaked and
// translated callers log at Debug.
func stripFallbacksForUpstreamDoc(doc *ojson.Doc, confirmedNative bool, sessionID string) {
	if !StripFallbacks() || doc == nil || doc.Len() == 0 {
		return
	}
	field := doc.Get("fallbacks")
	if !field.Exists() {
		return
	}
	shape := "array"
	if field.Type == ojson.String {
		shape = field.String()
	}
	model := doc.Get("model").String()
	if err := doc.Delete("fallbacks"); err != nil {
		return
	}
	fields := log.Fields{"model": model, "session": sessionID, "shape": shape}
	if confirmedNative && firstStripForSession(sessionID) {
		log.WithFields(fields).Warn("no-substitution: fallbacks stripped |")
	} else {
		log.WithFields(fields).Debug("no-substitution: fallbacks stripped |")
	}
}
