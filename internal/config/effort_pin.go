package config

import "strings"

// EffortPinConfig refuses a bare model id for the models whose effort the
// operator pins through the effort-suffixed slug ("claude-opus-5-5-medium").
//
// Claude Code 2.1.284 with agent teams sends a subagent the higher of the
// session's effort and its definition's, so an effort that was meant to be
// pinned can silently drift with the session dial. With the pin in the slug,
// a bare id is the one spelling under which drift is still possible; refusing
// it turns a silent regression into a request that fails with a message
// naming the slugs to use. The refusal can be lifted at runtime from the
// cache watch page (PUT /v0/management/effort-pin) without a restart.
type EffortPinConfig struct {
	// Enabled turns the refusal on.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Models are the base ids that must carry an effort suffix. An account
	// prefix ("scoped/") and a "[1m]" tag on the request are ignored when
	// matching. Any other model id is untouched.
	Models []string `yaml:"models" json:"models"`
	// AllowBare lifts the refusal at startup; the watch page switch changes
	// the running value only.
	AllowBare bool `yaml:"allow-bare,omitempty" json:"allow-bare,omitempty"`
}

// NormalizedModels returns the pinned base ids lowercased and trimmed, empty
// entries dropped.
func (c EffortPinConfig) NormalizedModels() []string {
	out := make([]string, 0, len(c.Models))
	for _, m := range c.Models {
		m = strings.ToLower(strings.TrimSpace(m))
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}
