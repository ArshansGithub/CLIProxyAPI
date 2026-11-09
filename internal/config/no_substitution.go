package config

// NoSubstitutionConfig makes a model or effort substitution loud instead of
// silent (fork-only; docs/fork/specs/2026-10-07-no-substitution-design.md).
//
// StripFallbacks deletes the Claude `fallbacks` body key from every request
// before it reaches the upstream and stops the proxy from injecting its own,
// so a safety refusal comes back as a refusal rather than an answer from a
// different model. The Anthropic-Beta header of a confirmed native client is
// left as sent.
//
// RefuseUnsupportedEffort answers 400 when a caller names an effort level the
// selected model does not offer, where the proxy would otherwise clamp, remap
// or drop it with only a warning.
type NoSubstitutionConfig struct {
	StripFallbacks          bool `yaml:"strip-fallbacks" json:"strip-fallbacks"`
	RefuseUnsupportedEffort bool `yaml:"refuse-unsupported-effort" json:"refuse-unsupported-effort"`
}
