package executor

import "sync/atomic"

// stripFallbacks is the no-substitution.strip-fallbacks switch (fork-only;
// docs/fork/specs/2026-10-07-no-substitution-design.md).
var stripFallbacks atomic.Bool

// SetStripFallbacks turns the Claude `fallbacks` strip on or off for the process.
func SetStripFallbacks(on bool) { stripFallbacks.Store(on) }

// StripFallbacks reports whether the Claude executor deletes the `fallbacks`
// body key and never injects its own.
func StripFallbacks() bool { return stripFallbacks.Load() }
