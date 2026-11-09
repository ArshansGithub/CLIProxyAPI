// Package effortpin refuses a bare model id for the models whose effort the
// operator pins through the effort-suffixed slug ("claude-opus-5-5-medium",
// "gpt-6.1-sol-high"; see internal/thinking/suffix.go).
//
// A pinned effort that reaches the proxy as a bare id plus a body effort is the
// one spelling under which the effort can still drift with a client's session
// dial (Claude Code 2.1.284 with agent teams, anthropics/claude-code #80569).
// Refusing the bare id turns that silent regression into a 400 whose message
// names the slugs to use. The operator can lift the refusal for the running
// process from the cache watch page; the config value returns on restart.
package effortpin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
)

// Snapshot is the payload of GET /v0/management/effort-pin.
type Snapshot struct {
	Enabled   bool `json:"enabled"`
	AllowBare bool `json:"allow_bare"`
	// Active is true when a bare id is refused right now.
	Active bool     `json:"active"`
	Models []string `json:"models"`
	// Refusals counts refused requests since the proxy started.
	Refusals int64 `json:"refusals"`
	// AllowedBare counts bare-id requests that passed because the switch was on.
	AllowedBare      int64     `json:"allowed_bare"`
	LastRefusedModel string    `json:"last_refused_model,omitempty"`
	LastRefusedAt    time.Time `json:"last_refused_at,omitempty"`
	LastAllowedModel string    `json:"last_allowed_model,omitempty"`
	LastAllowedAt    time.Time `json:"last_allowed_at,omitempty"`
}

// Store is the process-wide pin state.
type Store struct {
	mu          sync.Mutex
	now         func() time.Time
	enabled     bool
	allowBare   bool
	models      map[string]struct{}
	refusals    int64
	allowed     int64
	lastRefused string
	lastRefAt   time.Time
	lastAllowed string
	lastAllowAt time.Time
}

// NewStore returns an empty, disabled store.
func NewStore() *Store {
	return &Store{now: time.Now, models: map[string]struct{}{}}
}

var (
	defaultMu    sync.RWMutex
	defaultStore = NewStore()
)

// Default is the store the auth manager consults and the management API serves.
func Default() *Store {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultStore
}

// SetDefault swaps the process store; tests use it, and nil restores a fresh one.
func SetDefault(s *Store) {
	if s == nil {
		s = NewStore()
	}
	defaultMu.Lock()
	defaultStore = s
	defaultMu.Unlock()
}

// Configure applies the config block. It resets the runtime switch to the
// configured value, so a config reload is the way to make a switch permanent.
func (s *Store) Configure(enabled bool, models []string, allowBare bool) {
	if s == nil {
		return
	}
	set := make(map[string]struct{}, len(models))
	for _, m := range models {
		m = strings.ToLower(strings.TrimSpace(m))
		if m != "" {
			set[m] = struct{}{}
		}
	}
	s.mu.Lock()
	s.enabled = enabled
	s.models = set
	s.allowBare = allowBare
	s.mu.Unlock()
}

// SetAllowBare flips the runtime switch and returns the resulting snapshot.
func (s *Store) SetAllowBare(allow bool) Snapshot {
	if s == nil {
		return Snapshot{Models: []string{}}
	}
	s.mu.Lock()
	s.allowBare = allow
	s.mu.Unlock()
	return s.Snapshot()
}

// baseID strips an account prefix ("scoped/"), a trailing "[1m]" tag and
// surrounding space, and lowercases. The effort suffix is judged separately.
func baseID(model string) string {
	m := strings.TrimSpace(model)
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	m = strings.TrimSuffix(m, "[1m]")
	return strings.ToLower(strings.TrimSpace(m))
}

// Refuse reports whether model must be refused, with the message to return.
// A model outside the pinned set, a slug carrying an effort suffix, a disabled
// store or a lifted switch all pass. A bare pinned id counts either as a
// refusal or, with the switch on, as an allowed bare request.
func (s *Store) Refuse(model string) (bool, string) {
	if s == nil {
		return false, ""
	}
	parsed := thinking.ParseSuffix(strings.TrimSuffix(strings.TrimSpace(model), "[1m]"))
	if parsed.HasSuffix {
		return false, ""
	}
	base := baseID(model)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return false, ""
	}
	if _, pinned := s.models[base]; !pinned {
		return false, ""
	}
	at := s.now()
	if s.allowBare {
		s.allowed++
		s.lastAllowed = strings.TrimSpace(model)
		s.lastAllowAt = at
		return false, ""
	}
	s.refusals++
	s.lastRefused = strings.TrimSpace(model)
	s.lastRefAt = at
	return true, Message(model)
}

// Message is the refusal text for a bare pinned id.
func Message(model string) string {
	m := strings.TrimSuffix(strings.TrimSpace(model), "[1m]")
	return fmt.Sprintf("model %q names no effort: the effort pin is on, so send an effort-suffixed slug (%s-low, %s-medium, %s-high, %s-xhigh or %s-max) whose effort is forced on the upstream call; a bare id would run at whatever effort reached the proxy. To allow bare ids for this process, switch the pin off on /cache.html or PUT /v0/management/effort-pin {\"allow_bare\": true}.", m, m, m, m, m, m)
}

// Snapshot summarizes the state.
func (s *Store) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{Models: []string{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	models := make([]string, 0, len(s.models))
	for m := range s.models {
		models = append(models, m)
	}
	sort.Strings(models)
	return Snapshot{
		Enabled: s.enabled, AllowBare: s.allowBare, Active: s.enabled && !s.allowBare, Models: models,
		Refusals: s.refusals, AllowedBare: s.allowed,
		LastRefusedModel: s.lastRefused, LastRefusedAt: s.lastRefAt,
		LastAllowedModel: s.lastAllowed, LastAllowedAt: s.lastAllowAt,
	}
}
