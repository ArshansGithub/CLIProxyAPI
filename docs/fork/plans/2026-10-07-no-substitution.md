# No-substitution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retire the effort-in-model-slug system on the Claude Code side and make every model or effort substitution a loud error at the proxy.

**Architecture:** Three independent deliverables. (A) The proxy fork gains a `no-substitution` config block with two switches held in package-level atomics: the Claude executor strips the `fallbacks` body key once on the finished body, and the thinking package refuses an explicit effort it would otherwise clamp, remap or drop. (B) The Claude Code side goes back to bare ids plus `effort:` frontmatter with Claude Code's own substitution switches set, pin off first. (C) A Ceeya sitting note and four continuation lines carry the change to the next Ceeya session.

**Tech Stack:** Go 1.26 (fork of router-for-me/CLIProxyAPI v8, `-tags locked`), logrus, gjson/sjson, httptest; bash/python3 for config edits; the fork's Makefile (`make verify`, `make verify-tagless`, `make patches`).

**Spec:** `docs/fork/specs/2026-10-07-no-substitution-design.md` (fork commit 9df6b669)

## Global Constraints

- Build and test with `-tags locked` (`make verify`) and also run `make verify-tagless` before every commit of Go changes (CLAUDE.md).
- Prefer fork-owned files; an upstream file gets one hook call, an early return, or a build-tag line (CLAUDE.md). New logic goes in `internal/thinking/no_substitution.go`, `internal/runtime/executor/claude_executor_no_substitution.go`, `internal/config/no_substitution.go`, `sdk/cliproxy/service_no_substitution.go`.
- Every carried change gets a `docs/fork/PATCHES.yaml` entry; run `make patches` after adding one; unannotated rows must stay 0.
- Commit subjects: `feat:` for the proxy feature, `lock:` for docs/register/tooling. Never push to `upstream`.
- Both switches default to false in code; his `~/.cli-proxy-api/config.yaml` sets both true.
- A confirmed native client's Anthropic-Beta header is never reshaped (patch 4f5cbbae). Only the body key is stripped; this is recorded as the one named exception in the PATCHES row.
- `make install` restarts his proxy: run it only on his explicit word. Everything before Task 12 runs on the branch, the test suite, or the side instance on :8319.
- Order on his machine: pin off (Task 7) before any agent page or settings edit (Tasks 8-9).
- Do not commit inside the NewCeeya repo; write the files and leave them for the stream's wrap to land.

## Review Focus

1. A request whose `fallbacks` is the string `"default"` (2.1.293 category mode) must be stripped like the array form; test added in Task 5.
2. A payload rule that sets `fallbacks` after cloaking must still be stripped (the strip runs last); test added in Task 5.
3. A `configuration_update` whose effort equals the top-level baseline must not be refused (nothing is substituted); test added in Task 4.
4. A cross-family request whose level the model does offer must pass with no error and no log line when the switch is on; test added in Task 3.
5. With the switch off, every behaviour is byte-identical to today (same body, same Warn lines); the on/off pairs in Tasks 3-5 pin this.

---

## File map

Proxy fork (`/Users/arshan/Desktop/NewShi/cpa-cache-watch`, branch `locked`):
- Create `internal/config/no_substitution.go`: `NoSubstitutionConfig` type.
- Modify `internal/config/config.go:108` (field after `EffortPin`), `internal/config/config_v8_fork.go:8` (root section).
- Create `internal/thinking/no_substitution.go`: the atomic switch and `refuseEffort`.
- Modify `internal/thinking/validate.go:133-136`, `internal/thinking/apply.go:247,371-383`, `internal/thinking/configuration_update.go:55-78`: one hook each.
- Create `internal/thinking/no_substitution_test.go` (external package) and `internal/thinking/no_substitution_internal_test.go` (package thinking).
- Create `internal/runtime/executor/claude_executor_no_substitution.go`: the atomic switch, the once-per-session set, `stripFallbacksForUpstream`.
- Modify `internal/runtime/executor/claude_executor_execute.go:205` and `claude_executor_stream.go:208` (one call each, right after `ensureModelMaxTokens`), `claude_executor_request.go:52,235` (new beta constant, no-add rule).
- Create `internal/runtime/executor/claude_executor_no_substitution_test.go`.
- Create `sdk/cliproxy/service_no_substitution.go`; modify `sdk/cliproxy/service_config.go:205`, `sdk/cliproxy/service_lifecycle.go:139` (one call each).
- Modify `config.example.yaml` (document the block), `docs/fork/PATCHES.yaml` (one `feat` row), `docs/fork/PATCHES.md` (generated).

His machine:
- Modify `~/.cli-proxy-api/config.yaml` (`effort-pin.enabled`, new `no-substitution` block), `~/.claude/agents/{luna,sol,codebase-refactor-architect}.md`, `~/.claude/settings.json` (env), `~/bin/cca`.

Ceeya (`/Users/arshan/Desktop/BeginningOfTheEnd/New/NewCeeya`):
- Create `.ceeya/sittings/2026-10-07-effort-slugs-retired/README.md`; append one line to `.ceeya/record/{brands,ingredient,ketosis,scan}/continuation.md`.

---

### Task 1: Config block `no-substitution`

**Files:**
- Create: `internal/config/no_substitution.go`
- Modify: `internal/config/config.go:108` (after the `EffortPin` field), `internal/config/config_v8_fork.go:8`
- Test: `internal/config/no_substitution_test.go`

**Interfaces:**
- Produces: `type NoSubstitutionConfig struct { StripFallbacks bool; RefuseUnsupportedEffort bool }` with yaml keys `strip-fallbacks`, `refuse-unsupported-effort`; `Config.NoSubstitution NoSubstitutionConfig` (yaml `no-substitution`). Later tasks read `cfg.NoSubstitution.StripFallbacks` and `cfg.NoSubstitution.RefuseUnsupportedEffort`.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/no_substitution_test.go
package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNoSubstitutionConfigDecodesFromYAML(t *testing.T) {
	var cfg Config
	src := []byte("no-substitution:\n  strip-fallbacks: true\n  refuse-unsupported-effort: true\n")
	if err := yaml.Unmarshal(src, &cfg); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if !cfg.NoSubstitution.StripFallbacks || !cfg.NoSubstitution.RefuseUnsupportedEffort {
		t.Fatalf("decoded %+v, want both true", cfg.NoSubstitution)
	}
}

func TestNoSubstitutionConfigDefaultsOff(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("api-keys: []\n"), &cfg); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if cfg.NoSubstitution.StripFallbacks || cfg.NoSubstitution.RefuseUnsupportedEffort {
		t.Fatalf("defaults %+v, want both false", cfg.NoSubstitution)
	}
}

func TestNoSubstitutionIsAForkRootSection(t *testing.T) {
	for _, s := range forkRootSections {
		if s == "no-substitution" {
			return
		}
	}
	t.Fatalf("forkRootSections = %v, missing no-substitution", forkRootSections)
}
```

(If the package already imports a different yaml module, use the same one: `grep -rn 'gopkg.in/yaml' internal/config/*.go | head -1`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags locked ./internal/config/ -run 'TestNoSubstitution' -v`
Expected: FAIL, `cfg.NoSubstitution undefined`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/config/no_substitution.go
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
```

In `internal/config/config.go`, after the `EffortPin` field (line 108):

```go
	// NoSubstitution makes model and effort substitution loud (fork-only; see
	// NoSubstitutionConfig).
	NoSubstitution NoSubstitutionConfig `yaml:"no-substitution" json:"no-substitution"`
```

In `internal/config/config_v8_fork.go:8`:

```go
var forkRootSections = []string{"egress", "usage-cache-stats", "overload-retry", "effort-pin", "no-substitution"}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags locked ./internal/config/ -v -run 'TestNoSubstitution|V8|Fork'`
Expected: PASS (the existing v8 layout tests must still pass with the new root section).

- [ ] **Step 5: Commit**

```bash
git add internal/config/no_substitution.go internal/config/no_substitution_test.go internal/config/config.go internal/config/config_v8_fork.go
git commit -m "feat(no-substitution): config block with strip-fallbacks and refuse-unsupported-effort

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 2: The two switches and the reload hook

**Files:**
- Create: `internal/thinking/no_substitution.go`, `internal/runtime/executor/claude_executor_no_substitution.go` (switch only; the strip itself comes in Task 5), `sdk/cliproxy/service_no_substitution.go`
- Modify: `sdk/cliproxy/service_config.go:205`, `sdk/cliproxy/service_lifecycle.go:139`
- Test: `internal/thinking/no_substitution_test.go`, `sdk/cliproxy/service_no_substitution_test.go`

**Interfaces:**
- Produces: `thinking.SetRefuseUnsupportedEffort(bool)`, `thinking.RefuseUnsupportedEffort() bool`; `executor.SetStripFallbacks(bool)`, `executor.StripFallbacks() bool`; `(s *Service) applyNoSubstitutionConfig(cfg *config.Config)`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/thinking/no_substitution_test.go
package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
)

func TestRefuseUnsupportedEffortSwitch(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	if thinking.RefuseUnsupportedEffort() {
		t.Fatal("default must be off")
	}
	thinking.SetRefuseUnsupportedEffort(true)
	if !thinking.RefuseUnsupportedEffort() {
		t.Fatal("switch did not turn on")
	}
}
```

```go
// sdk/cliproxy/service_no_substitution_test.go
package cliproxy

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
)

func TestApplyNoSubstitutionConfigSetsBothSwitches(t *testing.T) {
	t.Cleanup(func() {
		thinking.SetRefuseUnsupportedEffort(false)
		executor.SetStripFallbacks(false)
	})
	s := &Service{}
	s.applyNoSubstitutionConfig(&config.Config{NoSubstitution: config.NoSubstitutionConfig{StripFallbacks: true, RefuseUnsupportedEffort: true}})
	if !thinking.RefuseUnsupportedEffort() || !executor.StripFallbacks() {
		t.Fatalf("switches = refuse %v strip %v, want both true", thinking.RefuseUnsupportedEffort(), executor.StripFallbacks())
	}
	s.applyNoSubstitutionConfig(&config.Config{})
	if thinking.RefuseUnsupportedEffort() || executor.StripFallbacks() {
		t.Fatal("a reload with the block absent must turn both off")
	}
	s.applyNoSubstitutionConfig(nil) // must not panic
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags locked ./internal/thinking/ ./sdk/cliproxy/ -run 'TestRefuseUnsupportedEffortSwitch|TestApplyNoSubstitutionConfig' -v`
Expected: FAIL, undefined `SetRefuseUnsupportedEffort`, `applyNoSubstitutionConfig`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/thinking/no_substitution.go
package thinking

import (
	"fmt"
	"strings"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// refuseUnsupportedEffort is the no-substitution.refuse-unsupported-effort
// switch (fork-only). It is set from the config at startup and on reload.
var refuseUnsupportedEffort atomic.Bool

// SetRefuseUnsupportedEffort turns the refusal on or off for the process.
func SetRefuseUnsupportedEffort(on bool) { refuseUnsupportedEffort.Store(on) }

// RefuseUnsupportedEffort reports whether an explicit effort the model does
// not offer is refused with a 400 instead of being clamped, remapped or dropped.
func RefuseUnsupportedEffort() bool { return refuseUnsupportedEffort.Load() }

// refuseEffort builds the 400 for a refused effort and logs it at Error so
// the refusal is never only a warning an operator has to go looking for.
func refuseEffort(provider, model, requested string, supported []string) *ThinkingError {
	offered := strings.Join(supported, ", ")
	log.WithFields(log.Fields{
		"provider":  provider,
		"model":     model,
		"requested": requested,
		"supported": offered,
	}).Error("no-substitution: effort refused |")
	msg := fmt.Sprintf("effort_unsupported: model %s does not offer effort %q; it offers %s", model, requested, offered)
	return NewThinkingErrorWithModel(ErrLevelNotSupported, msg, model)
}
```

```go
// internal/runtime/executor/claude_executor_no_substitution.go
package executor

import "sync/atomic"

// stripFallbacks is the no-substitution.strip-fallbacks switch (fork-only).
var stripFallbacks atomic.Bool

// SetStripFallbacks turns the Claude `fallbacks` strip on or off for the process.
func SetStripFallbacks(on bool) { stripFallbacks.Store(on) }

// StripFallbacks reports whether the Claude executor deletes the `fallbacks`
// body key and never injects its own.
func StripFallbacks() bool { return stripFallbacks.Load() }
```

```go
// sdk/cliproxy/service_no_substitution.go
package cliproxy

import (
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	log "github.com/sirupsen/logrus"
)

// applyNoSubstitutionConfig installs the no-substitution switches at startup
// and on every config reload, beside applyEffortPinConfig. A reload with the
// block absent turns both off, which is the rollback path.
func (s *Service) applyNoSubstitutionConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	executor.SetStripFallbacks(cfg.NoSubstitution.StripFallbacks)
	thinking.SetRefuseUnsupportedEffort(cfg.NoSubstitution.RefuseUnsupportedEffort)
	log.WithFields(log.Fields{
		"strip_fallbacks":           cfg.NoSubstitution.StripFallbacks,
		"refuse_unsupported_effort": cfg.NoSubstitution.RefuseUnsupportedEffort,
	}).Info("no-substitution: switches applied |")
}
```

Add `s.applyNoSubstitutionConfig(cfg)` directly after `s.applyEffortPinConfig(cfg)` at `sdk/cliproxy/service_config.go:205`, and `s.applyNoSubstitutionConfig(s.cfg)` after `s.applyEffortPinConfig(s.cfg)` at `sdk/cliproxy/service_lifecycle.go:139`.

The fields `strip_fallbacks` and `refuse_unsupported_effort` need adding to `logFieldOrder` in `internal/logging/global_logger.go` (the formatter prints only allowlisted names); add them next to `rewritten_to`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags locked ./internal/thinking/ ./sdk/cliproxy/ -run 'TestRefuseUnsupportedEffortSwitch|TestApplyNoSubstitutionConfig' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/thinking/no_substitution.go internal/thinking/no_substitution_test.go internal/runtime/executor/claude_executor_no_substitution.go sdk/cliproxy/service_no_substitution.go sdk/cliproxy/service_no_substitution_test.go sdk/cliproxy/service_config.go sdk/cliproxy/service_lifecycle.go internal/logging/global_logger.go
git commit -m "feat(no-substitution): process switches installed from config at startup and reload

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 3: Refuse the cross-family clamp and the remap

**Files:**
- Modify: `internal/thinking/validate.go:133-136`, `internal/thinking/apply.go:371-383`
- Test: `internal/thinking/no_substitution_test.go` (append)

**Interfaces:**
- Consumes: `thinking.RefuseUnsupportedEffort()`, `refuseEffort(...)` from Task 2.

- [ ] **Step 1: Write the failing tests** (append to `internal/thinking/no_substitution_test.go`; `captureWarnings`, `codexModel`, `embeddedCodexPlusModel` and `lunaLevels` already exist in `effort_visibility_test.go` in the same package)

```go
func errorsMentioning(hook *logtest.Hook, fragment string) []*log.Entry {
	var out []*log.Entry
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.ErrorLevel && strings.Contains(entry.Message, fragment) {
			out = append(out, entry)
		}
	}
	return out
}

// The cross-family clamp (Claude wire asking "ultra" of gpt-6.1-sol) becomes
// a 400 under the switch, and stays a clamp with the old Warn without it.
func TestClampRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"ultra"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"ultra"}}`)
	info := embeddedCodexPlusModel(t, "gpt-6.1-sol")

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", info)
	if err == nil {
		t.Fatal("want an error with the switch on")
	}
	var te *thinking.ThinkingError
	if !errors.As(err, &te) || te.StatusCode() != 400 {
		t.Fatalf("error = %T %v, want *ThinkingError with 400", err, err)
	}
	if !strings.HasPrefix(err.Error(), "effort_unsupported: model gpt-6.1-sol does not offer effort \"ultra\"; it offers ") {
		t.Fatalf("message = %q", err.Error())
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %d: %v", len(got), hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "max" {
		t.Fatalf("switch off: reasoning.effort = %q, want max", got)
	}
	if got := warningsMentioning(hook, "level clamped"); len(got) != 1 {
		t.Fatalf("switch off: want the old clamp Warn, got %v", hook.AllEntries())
	}
}

// A supported level crosses families untouched and silently with the switch on.
func TestSupportedLevelPassesWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"high"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q", got)
	}
	if len(hook.AllEntries()) != 0 {
		t.Fatalf("unexpected log lines: %v", hook.AllEntries())
	}
}

// The "as high as this model goes" remap (max on a model that stops at xhigh)
// becomes a 400 under the switch. Mirrors the remap case in effort_visibility_test.go.
func TestRemapRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	info := codexModel("gpt-5.5", []string{"low", "medium", "high", "xhigh"}, false)
	source := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"max"}}`)
	body := []byte(`{"model":"gpt-5.5","input":[],"reasoning":{"effort":"max"}}`)

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-5.5", "claude", "codex", "codex", info)
	if err == nil || !strings.Contains(err.Error(), `does not offer effort "max"`) {
		t.Fatalf("want the refusal, got %v", err)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %v", hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-5.5", "claude", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "xhigh" {
		t.Fatalf("switch off: reasoning.effort = %q, want xhigh", got)
	}
	if got := warningsMentioning(hook, "level remapped"); len(got) != 1 {
		t.Fatalf("switch off: want the old remap Warn, got %v", hook.AllEntries())
	}
}
```

Add `"errors"` to the test file's imports and the logrus/logtest imports used above (copy from `effort_visibility_test.go`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags locked ./internal/thinking/ -run 'TestClampRefused|TestSupportedLevelPasses|TestRemapRefused' -v`
Expected: `TestClampRefusedWhenSwitchOn` and `TestRemapRefusedWhenSwitchOn` FAIL ("want an error"); `TestSupportedLevelPasses` PASS.

- [ ] **Step 3: Write minimal implementation**

`internal/thinking/validate.go` lines 133-136 become:

```go
		if !isLevelSupported(string(config.Level), support.Levels) {
			if allowClampUnsupported && RefuseUnsupportedEffort() {
				return nil, refuseEffort(toFormat, modelInfo.ID, strings.ToLower(string(config.Level)), normalizeLevels(support.Levels))
			}
			if allowClampUnsupported {
				config.Level = clampLevel(config.Level, modelInfo, toFormat)
			}
```

`internal/thinking/apply.go` lines 371-383: inside the `if modelInfoResolved && ...` block, after computing `config.Level = mapConfiguredHighIntent(...)`, replace the `if !strings.EqualFold(...)` body's opening with:

```go
		if !strings.EqualFold(string(requested), string(config.Level)) {
			if RefuseUnsupportedEffort() {
				return body, refuseEffort(providerFormat, modelInfo.ID, strings.ToLower(string(requested)), normalizeLevels(modelInfo.Thinking.Levels))
			}
			// ...existing Warn block unchanged...
```

(`normalizeLevels` already exists in validate.go.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags locked ./internal/thinking/ -v`
Expected: PASS, including every pre-existing test in the package.

- [ ] **Step 5: Commit**

```bash
git add internal/thinking/validate.go internal/thinking/apply.go internal/thinking/no_substitution_test.go
git commit -m "feat(no-substitution): refuse an explicit effort the model does not offer instead of clamping or remapping

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 4: Refuse a dropped configuration_update effort

**Files:**
- Modify: `internal/thinking/configuration_update.go:55-78`, `internal/thinking/apply.go:247`
- Test: `internal/thinking/no_substitution_test.go` (append)

**Interfaces:**
- Changes `stripConfigurationUpdatesLogged(body []byte, provider, model string, modelInfo *registry.ModelInfo) []byte` to return `([]byte, error)`.

- [ ] **Step 1: Write the failing tests** (append)

```go
// A Codex client changing effort mid-turn on a model without in-turn updates
// is refused under the switch when the dropped effort differs from the
// top-level baseline, and still stripped with the old Warn without it.
func TestConfigurationUpdateDropRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"medium"},"input":[{"type":"configuration_update","reasoning":{"effort":"xhigh"}},{"type":"message","role":"user","content":"hi"}]}`)
	info := codexModel("gpt-6.0-sol", lunaLevels, false)

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", info)
	if err == nil || !strings.Contains(err.Error(), `does not offer effort "xhigh"; it offers medium`) {
		t.Fatalf("want the refusal naming the baseline as the offer, got %v", err)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %v", hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "input.#").Int(); got != 1 {
		t.Fatalf("switch off: input length = %d, want 1", got)
	}
	if got := warningsMentioning(hook, "configuration_update"); len(got) != 1 {
		t.Fatalf("switch off: want the old drop Warn, got %v", hook.AllEntries())
	}
}

// An update whose effort equals the baseline substitutes nothing: stripped
// silently even with the switch on.
func TestConfigurationUpdateEqualToBaselineNotRefused(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"medium"},"input":[{"type":"configuration_update","reasoning":{"effort":"medium"}},{"type":"message","role":"user","content":"hi"}]}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", codexModel("gpt-6.0-sol", lunaLevels, false))
	if err != nil {
		t.Fatalf("ApplyThinking: %v", err)
	}
	if got := gjson.GetBytes(out, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1 (update stripped)", got)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 0 {
		t.Fatalf("unexpected refusal: %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags locked ./internal/thinking/ -run 'TestConfigurationUpdate' -v`
Expected: `TestConfigurationUpdateDropRefusedWhenSwitchOn` FAIL ("want the refusal"); the equal-to-baseline test PASS.

- [ ] **Step 3: Write minimal implementation**

In `internal/thinking/configuration_update.go`, change the signature and the tail of `stripConfigurationUpdatesLogged`:

```go
func stripConfigurationUpdatesLogged(body []byte, provider, model string, modelInfo *registry.ModelInfo) ([]byte, error) {
	dropped := extractConfigurationUpdateConfig(body)
	stripped := stripConfigurationUpdates(body)
	if !hasThinkingConfig(dropped) || len(stripped) == len(body) {
		return stripped, nil
	}
	if modelInfo != nil && modelInfo.ID != "" {
		model = modelInfo.ID
	}
	droppedEffort := string(dropped.Level)
	switch dropped.Mode {
	case ModeNone:
		droppedEffort = string(LevelNone)
	case ModeAuto:
		droppedEffort = string(LevelAuto)
	}
	baseline := extractCodexConfig(stripped)
	if RefuseUnsupportedEffort() && !strings.EqualFold(droppedEffort, string(baseline.Level)) {
		// The model cannot change effort mid-turn, so the only effort it can
		// honour is the top-level baseline; that is what it "offers" here.
		return stripped, refuseEffort(provider, model, droppedEffort, []string{string(baseline.Level)})
	}
	log.WithFields(log.Fields{
		"provider":       provider,
		"model":          model,
		"dropped_effort": droppedEffort,
		"baseline":       string(baseline.Level),
	}).Warn("thinking: configuration_update effort dropped (model does not support in-turn updates) |")
	return stripped, nil
}
```

In `internal/thinking/apply.go:247`:

```go
		var errStrip error
		body, errStrip = stripConfigurationUpdatesLogged(body, providerFormat, baseModel, modelInfo)
		if errStrip != nil {
			return body, errStrip
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags locked ./internal/thinking/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/thinking/configuration_update.go internal/thinking/apply.go internal/thinking/no_substitution_test.go
git commit -m "feat(no-substitution): refuse a mid-turn effort change the model cannot honour

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 5: Strip `fallbacks` on the Claude wire

**Files:**
- Modify: `internal/runtime/executor/claude_executor_no_substitution.go` (add the strip), `internal/runtime/executor/claude_executor_execute.go:205`, `internal/runtime/executor/claude_executor_stream.go:208`, `internal/runtime/executor/claude_executor_request.go:52,84-116,235`
- Test: `internal/runtime/executor/claude_executor_no_substitution_test.go`

**Interfaces:**
- Consumes: `StripFallbacks()` from Task 2.
- Produces: `stripFallbacksForUpstream(body []byte, confirmedNative bool, sessionID string) []byte`; `claudeServerSideFallbackCategoryBeta = "server-side-fallback-2026-07-01"`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/runtime/executor/claude_executor_no_substitution_test.go
package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/tidwall/gjson"
)

const nsSessionID = "11111111-2222-4333-8444-555555555555"
const nsUserID = `{"device_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","account_uuid":"","session_id":"11111111-2222-4333-8444-555555555555"}`

func nsNativeHeaders(betas string) http.Header {
	return http.Header{
		"User-Agent":                  {"claude-cli/2.1.293 (external, cli)"},
		"X-App":                       {"cli"},
		"Anthropic-Beta":              {betas},
		"X-Claude-Code-Session-Id":    {nsSessionID},
		"X-Stainless-Package-Version": {"0.112.1"},
		"X-Stainless-Runtime-Version": {"v26.3.0"},
		"X-Stainless-Os":              {"MacOS"},
		"X-Stainless-Arch":            {"arm64"},
	}
}

func nsCaptureLogs(t *testing.T) *logtest.Hook {
	t.Helper()
	logger := log.StandardLogger()
	prevLevel := logger.GetLevel()
	prevHooks := logger.ReplaceHooks(make(log.LevelHooks))
	hook := logtest.NewLocal(logger)
	logger.SetLevel(log.DebugLevel)
	t.Cleanup(func() { logger.ReplaceHooks(prevHooks); logger.SetLevel(prevLevel) })
	return hook
}

func nsRun(t *testing.T, cfg *config.Config, payload string, headers http.Header) ([]byte, http.Header) {
	t.Helper()
	var seenBody []byte
	var seenHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		seenHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"claude-opus-5-5","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()
	executor := NewClaudeExecutor(cfg)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "key-test", "base_url": server.URL}}
	p := []byte(payload)
	_, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{Model: "claude-opus-5-5", Payload: p}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FormatClaude, OriginalRequest: p, Headers: headers,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return seenBody, seenHeaders
}

func nsNativePayload(fallbacks string) string {
	return `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"fallbacks":` + fallbacks + `,` +
		`"messages":[{"role":"user","content":"do the task"}],"metadata":{"user_id":` + fmt.Sprintf("%q", nsUserID) + `}}`
}

func nsCountLogs(hook *logtest.Hook, level log.Level, fragment string) int {
	n := 0
	for _, e := range hook.AllEntries() {
		if e.Level == level && strings.Contains(e.Message, fragment) {
			n++
		}
	}
	return n
}

// Native client, array fallbacks: body key gone, header untouched, one Warn
// for the session, Debug for the next request of the same session.
func TestStripFallbacks_NativeArrayStrippedHeaderKept(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	hook := nsCaptureLogs(t)
	betas := "claude-code-20250219,effort-2025-11-24,server-side-fallback-2026-06-01,fallback-credit-2026-06-01"
	body, headers := nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders(betas))
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("fallbacks reached the wire: %s", body)
	}
	if got := headers.Get("Anthropic-Beta"); got != betas {
		t.Fatalf("native header reshaped: %q", got)
	}
	if n := nsCountLogs(hook, log.WarnLevel, "fallbacks stripped"); n != 1 {
		t.Fatalf("want one Warn, got %d: %v", n, hook.AllEntries())
	}
	hook.Reset()
	nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders(betas))
	if n := nsCountLogs(hook, log.WarnLevel, "fallbacks stripped"); n != 0 {
		t.Fatalf("second request of the same session must not Warn again: %v", hook.AllEntries())
	}
	if n := nsCountLogs(hook, log.DebugLevel, "fallbacks stripped"); n != 1 {
		t.Fatalf("second request should log at Debug: %v", hook.AllEntries())
	}
}

// The 2.1.293 category form is a string; it is stripped the same way.
func TestStripFallbacks_NativeStringDefaultStripped(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	betas := "claude-code-20250219,server-side-fallback-2026-07-01"
	body, headers := nsRun(t, &config.Config{}, nsNativePayload(`"default"`), nsNativeHeaders(betas))
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("fallbacks reached the wire: %s", body)
	}
	if got := headers.Get("Anthropic-Beta"); got != betas {
		t.Fatalf("native header reshaped: %q", got)
	}
}

// Switch off: the native field passes through unchanged (today's behaviour).
func TestStripFallbacks_OffPassesThrough(t *testing.T) {
	SetStripFallbacks(false)
	body, _ := nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders("claude-code-20250219,server-side-fallback-2026-06-01"))
	if got := gjson.GetBytes(body, "fallbacks.0.model").String(); got != "claude-opus-4-8" {
		t.Fatalf("switch off must pass fallbacks through, got %s", body)
	}
}

// Cloaked caller on Opus 5.5: the cloak's own injection and the reconcile
// re-injection are both undone, and the proxy's beta list carries neither
// fallback beta. Cloaking is forced on through the config so the dormant
// injection sites run.
func TestStripFallbacks_CloakedInjectionUndoneAndBetasOmitted(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	cfg := &config.Config{}
	// Cloak mode defaults to on when the caller is not a confirmed native client.
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, headers := nsRun(t, cfg, payload, http.Header{"User-Agent": {"curl/8.0"}})
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("cloak injected fallbacks survived the strip: %s", body)
	}
	for _, beta := range []string{"server-side-fallback-2026-06-01", "server-side-fallback-2026-07-01"} {
		if strings.Contains(headers.Get("Anthropic-Beta"), beta) {
			t.Fatalf("proxy added %s with no fallbacks in the body: %q", beta, headers.Get("Anthropic-Beta"))
		}
	}
}

// A payload rule that sets fallbacks after cloaking is stripped too: the
// strip runs on the finished body.
func TestStripFallbacks_PayloadRuleSettingFallbacksIsStripped(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	cfg := &config.Config{}
	cfg.Payload.Override = []config.PayloadRule{{
		Models: []config.PayloadModelRule{{Name: "claude-opus-5-5"}},
		Params: map[string]any{"fallbacks": []any{map[string]any{"model": "claude-opus-5"}}},
	}}
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, _ := nsRun(t, cfg, payload, http.Header{"User-Agent": {"curl/8.0"}})
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("payload-rule fallbacks reached the wire: %s", body)
	}
}

// fallback_credit_token keeps its beta: a credit is not a substitution.
func TestStripFallbacks_CreditTokenKeepsCreditBeta(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"fallback_credit_token":"tok_1","messages":[{"role":"user","content":"do the task"}]}`
	_, headers := nsRun(t, &config.Config{}, payload, http.Header{"User-Agent": {"curl/8.0"}})
	if !strings.Contains(headers.Get("Anthropic-Beta"), "fallback-credit-2026-06-01") {
		t.Fatalf("credit beta missing: %q", headers.Get("Anthropic-Beta"))
	}
}
```

(The exact field names of `config.PayloadRule`/`PayloadModelRule`/`Params` and `cfg.Payload.Default`: copy from the existing test at `claude_executor_test.go:6677`, which builds a payload rule for a model; adjust the literal to match.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags locked ./internal/runtime/executor/ -run 'TestStripFallbacks' -v`
Expected: FAIL, undefined `resetStripFallbacksSeen`; after adding a stub, the strip tests fail with "fallbacks reached the wire".

- [ ] **Step 3: Write minimal implementation**

Replace `internal/runtime/executor/claude_executor_no_substitution.go` with:

```go
package executor

import (
	"container/list"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// stripFallbacks is the no-substitution.strip-fallbacks switch (fork-only;
// docs/fork/specs/2026-10-07-no-substitution-design.md).
var stripFallbacks atomic.Bool

// SetStripFallbacks turns the Claude `fallbacks` strip on or off for the process.
func SetStripFallbacks(on bool) { stripFallbacks.Store(on) }

// StripFallbacks reports whether the Claude executor deletes the `fallbacks`
// body key and never injects its own.
func StripFallbacks() bool { return stripFallbacks.Load() }

// stripFallbacksSeen bounds the once-per-session Warn: sessions started
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

// stripFallbacksForUpstream deletes the `fallbacks` key, whatever its JSON
// type, from the finished body. It runs after the cloak reconcile and the
// payload rules and before the CCH billing signature, so nothing re-adds the
// key. A confirmed native client carrying the key means the client-side
// switch is not set, which is worth one Warn per session; cloaked and
// translated callers log at Debug.
func stripFallbacksForUpstream(body []byte, confirmedNative bool, sessionID string) []byte {
	if !StripFallbacks() || len(body) == 0 {
		return body
	}
	field := gjson.GetBytes(body, "fallbacks")
	if !field.Exists() {
		return body
	}
	shape := "array"
	if field.Type == gjson.String {
		shape = field.String()
	}
	out, err := sjson.DeleteBytes(body, "fallbacks")
	if err != nil {
		return body
	}
	fields := log.Fields{"model": gjson.GetBytes(body, "model").String(), "session": sessionID, "shape": shape}
	if confirmedNative && firstStripForSession(sessionID) {
		log.WithFields(fields).Warn("no-substitution: fallbacks stripped |")
	} else {
		log.WithFields(fields).Debug("no-substitution: fallbacks stripped |")
	}
	return out
}
```

Hook calls. In `internal/runtime/executor/claude_executor_execute.go`, directly after `body = ensureModelMaxTokens(body, baseModel)` (line 205):

```go
	body = stripFallbacksForUpstream(body, confirmedClaudeCode, claudeSessionID)
```

The same line in `internal/runtime/executor/claude_executor_stream.go` after its `ensureModelMaxTokens` (line 208); the stream path's session variable is also named `claudeSessionID` (see line 271).

In `internal/runtime/executor/claude_executor_request.go`:
- Add the constant after line 52: `claudeServerSideFallbackCategoryBeta = "server-side-fallback-2026-07-01"`.
- Add `claudeServerSideFallbackCategoryBeta` to `claudeManagedBetaSet` (the `func() map[string]bool` at line 84; add it wherever `claudeServerSideFallbackBeta` is listed) so a cloaked caller's copy of it is managed like the 06-01 form.
- Line 235 becomes:

```go
	hasFallbacks := gjson.GetBytes(body, "fallbacks").Exists()
	if !isProbeOrHelper && (hasFallbacks || ((requested[claudeServerSideFallbackBeta] || requested[claudeServerSideFallbackCategoryBeta]) && !StripFallbacks())) {
		betas = append(betas, claudeServerSideFallbackBeta)
	}
```

Lines 238-243 (`shouldIncludeFallbackCredit`) stay as they are: the credit beta still follows `fallback_credit_token` or an explicit request. Add `shape` and `session` to `logFieldOrder` in `internal/logging/global_logger.go` if not already present (`grep -n '"session"' internal/logging/global_logger.go`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags locked ./internal/runtime/executor/ -run 'TestStripFallbacks|TestClaudeExecutor_ConfirmedNative|TestClaudeExecutor_Unconfirmed' -v`
Expected: PASS, including the pre-existing passthrough tests (the probe-shaped case there sends the fallback beta with no body key and must still keep its header).

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/executor/claude_executor_no_substitution.go internal/runtime/executor/claude_executor_no_substitution_test.go internal/runtime/executor/claude_executor_execute.go internal/runtime/executor/claude_executor_stream.go internal/runtime/executor/claude_executor_request.go internal/logging/global_logger.go
git commit -m "feat(no-substitution): strip the Claude fallbacks key on the finished body and never inject one

The body strip is the one named exception to 4f5cbbae: a confirmed native
client's Anthropic-Beta header is still forwarded as sent.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 6: Register the patch, document the block, verify, tag the branch state

**Files:**
- Modify: `config.example.yaml` (after the `effort-pin` block), `docs/fork/PATCHES.yaml` (before the `# --- locked build ---` marker), `docs/fork/PATCHES.md` (generated)

- [ ] **Step 1: Document the block in `config.example.yaml`** (after the effort-pin example)

```yaml
# No-substitution (fork-only, 2026-10-07): make a model or effort substitution loud.
# strip-fallbacks deletes the Claude `fallbacks` body key from every request and
# never injects one, so a safety refusal comes back as a refusal, not as an answer
# from another model; a confirmed native client's beta header is left as sent.
# refuse-unsupported-effort answers 400 ("effort_unsupported: ...") when a caller
# names an effort the model does not offer, instead of clamping, remapping or
# dropping it with a warning. Both default to false.
no-substitution:
  strip-fallbacks: false
  refuse-unsupported-effort: false
```

- [ ] **Step 2: Add the PATCHES.yaml row** (insert before the `# ----------------------------------------------------------- locked build ---` line)

```yaml
- subjects:
    - "feat(no-substitution): config block with strip-fallbacks and refuse-unsupported-effort"
    - "feat(no-substitution): process switches installed from config at startup and reload"
    - "feat(no-substitution): refuse an explicit effort the model does not offer instead of clamping or remapping"
    - "feat(no-substitution): refuse a mid-turn effort change the model cannot honour"
    - "feat(no-substitution): strip the Claude fallbacks key on the finished body and never inject one"
  layer: feat
  upstream_pr: null
  drop: "never; fork policy (docs/fork/specs/2026-10-07-no-substitution-design.md)"
  note: >-
    2026-10-07. After Claude Code 2.1.293 fixed teammate effort (#80569) the
    effort slugs were retired on the client side; this is the loud-error half.
    strip-fallbacks runs once on the finished Claude body, after the cloak
    reconcile and payload rules and before CCH signing, and is the one named
    exception to the 4f5cbbae passthrough promise: only the body key goes, a
    confirmed native client's Anthropic-Beta header is forwarded as sent, with
    one Warn per session. refuse-unsupported-effort turns the cross-family
    clamp, the high-intent remap and the configuration_update drop into a 400
    whose message begins effort_unsupported:, logged at Error; conversions
    that substitute nothing (budget to level, auto to middle, none to lowest,
    budget clamps) are kept on purpose.
```

- [ ] **Step 3: Run the full verification**

Run: `make verify && make verify-tagless && make patches`
Expected: both suites PASS; `make patches` prints `unannotated rows: 0`.

- [ ] **Step 4: Commit**

```bash
git add config.example.yaml docs/fork/PATCHES.yaml docs/fork/PATCHES.md
git commit -m "lock: register the no-substitution patches and document the config block

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 7: Pin off on his machine (first client-side step)

**Files:**
- Modify: `~/.cli-proxy-api/config.yaml` (`effort-pin.enabled`, lines 213-220)

- [ ] **Step 1: Back up and flip the pin**

```bash
cp ~/.cli-proxy-api/config.yaml ~/.cli-proxy-api/config.yaml.bak-2026-10-07-pin-off
python3 - <<'EOF'
import re
p='/Users/arshan/.cli-proxy-api/config.yaml'
s=open(p).read()
new,n=re.subn(r'(^effort-pin:\n  enabled: )true', r'\1false', s, flags=re.M)
assert n==1, n
open(p,'w').write(new)
EOF
```

- [ ] **Step 2: Confirm the running proxy took it**

```bash
KEY=$(ruby -ryaml -e 'print (YAML.load_file(ARGV[0])["api-keys"]||[]).last' ~/.cli-proxy-api/config.yaml)
sleep 3
curl -s http://127.0.0.1:8317/v0/management/effort-pin -H "Authorization: Bearer $KEY"
```

Expected: JSON with `"enabled":false,"active":false`. If `enabled` is still true after 10 s, the watcher did not fire; check `tail -20 ~/.cli-proxy-api/logs/main.log` for a config reload line before going on. Do not proceed to Task 8 until `active` is false.

- [ ] **Step 3: Prove a bare id is accepted**

```bash
curl -s http://127.0.0.1:8317/v1/messages -H "Authorization: Bearer $KEY" -H 'content-type: application/json' \
  -d '{"model":"gpt-6-luna","max_tokens":8,"messages":[{"role":"user","content":"Reply with ok"}]}' | head -c 300
```

Expected: a message response, not a 400 naming the slugs.

---

### Task 8: Global agent pages back to bare ids with `effort:`

**Files:**
- Modify: `~/.claude/agents/luna.md`, `~/.claude/agents/sol.md`, `~/.claude/agents/codebase-refactor-architect.md`

- [ ] **Step 1: Edit the three frontmatter blocks**

```bash
cd ~/.claude/agents
cp luna.md luna.md.bak-2026-10-07; cp sol.md sol.md.bak-2026-10-07; cp codebase-refactor-architect.md codebase-refactor-architect.md.bak-2026-10-07
sed -i '' 's/^model: gpt-6-luna-high$/model: gpt-6-luna\neffort: high/' luna.md
sed -i '' 's/^model: gpt-6.1-sol-high$/model: gpt-6.1-sol\neffort: high/' sol.md
sed -i '' 's/^model: claude-opus-5-5-high\[1m\]$/model: claude-opus-5-5[1m]\neffort: high/' codebase-refactor-architect.md
grep -n '^model:\|^effort:' luna.md sol.md codebase-refactor-architect.md
```

Expected: each file shows a bare `model:` line followed by `effort: high`.

- [ ] **Step 2: Verify the spawn path resolves effort (proof 1, part one)**

```bash
D=/tmp/effort-probe-bare; rm -rf $D; mkdir -p $D/.claude/agents
cat > $D/.claude/agents/effort-probe.md <<'EOF'
---
name: effort-probe
description: Probe agent that replies with one word.
model: claude-sonnet-5-5
effort: high
maxTurns: 2
---
Reply with exactly the single word: ok
EOF
cd $D && env -u CLAUDE_CODE_EFFORT_LEVEL ~/bin/cca -p --output-format json --max-turns 8 --model sonnet \
  --settings '{"effortLevel":"low"}' --permission-mode bypassPermissions \
  'Call the Agent tool twice in parallel: (1) subagent_type "effort-probe", name "probe", prompt "Reply with the single word ok"; (2) subagent_type "effort-probe" with no name, same prompt. Then answer done.' \
  > /tmp/effort-probe-bare.json 2>/tmp/effort-probe-bare.err
S=$(python3 -c "import json;print([e for e in json.load(open('/tmp/effort-probe-bare.json')) if e.get('type')=='result'][0]['session_id'])")
for f in ~/.claude/projects/-private-tmp-effort-probe-bare/$S/subagents/*.jsonl; do echo "$f: $(grep -o '"effort":"[a-z]*"' "$f" | sort -u | tr '\n' ' ')"; done
grep -c 'rewritten by model suffix' ~/.cli-proxy-api/logs/main.log; grep 'effort-pin: request refused' ~/.cli-proxy-api/logs/main.log | tail -1
```

Expected: both subagent transcripts show `"effort":"high"`; the `rewritten by model suffix` count did not grow during the run; no new `effort-pin: request refused` line.

- [ ] **Step 3: Proof 1, part two (the no-effort page takes the modelSettings row)**

```bash
cat > $D/.claude/agents/effort-plain.md <<'EOF'
---
name: effort-plain
description: Probe agent with no effort line.
model: claude-sonnet-5-5
maxTurns: 2
---
Reply with exactly the single word: ok
EOF
cd $D && env -u CLAUDE_CODE_EFFORT_LEVEL ~/bin/cca -p --output-format json --max-turns 6 --model sonnet --permission-mode bypassPermissions \
  'Call the Agent tool once: subagent_type "effort-plain", name "plain", prompt "Reply with the single word ok". Then answer done.' > /tmp/effort-plain.json 2>/dev/null
S=$(python3 -c "import json;print([e for e in json.load(open('/tmp/effort-plain.json')) if e.get('type')=='result'][0]['session_id'])")
grep -o '"effort":"[a-z]*"' ~/.claude/projects/-private-tmp-effort-probe-bare/$S/subagents/*.jsonl | sort -u
```

Expected: `"effort":"medium"` (the bare `claude-sonnet-5-5` modelSettings row).

---

### Task 9: settings.json env switches and cca tier defaults

**Files:**
- Modify: `~/.claude/settings.json` (`env`), `~/bin/cca` (lines 63-70)

- [ ] **Step 1: settings.json**

```bash
cp ~/.claude/settings.json ~/.claude/settings.json.bak-2026-10-07-no-substitution
python3 - <<'EOF'
import json
p='/Users/arshan/.claude/settings.json'
d=json.load(open(p))
env=d.setdefault('env',{})
env['ANTHROPIC_DEFAULT_HAIKU_MODEL']='claude-haiku-5-5'
env['CLAUDE_CODE_NO_MODEL_FALLBACK']='1'
env['CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK']='1'
env['CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK']='1'
env['CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP']='1'
assert 'CLAUDE_CODE_EFFORT_LEVEL' not in env
assert d.get('switchModelsOnFlag') is False
json.dump(d,open(p,'w'),indent=2); open(p,'a').write('\n')
EOF
python3 -c "import json;print(json.load(open('/Users/arshan/.claude/settings.json'))['env'])"
```

Expected: the printed env carries the five keys with the values above and no `CLAUDE_CODE_EFFORT_LEVEL`.

- [ ] **Step 2: cca**

```bash
cp ~/bin/cca ~/bin/cca.bak-2026-10-07-no-substitution
python3 - <<'EOF'
p='/Users/arshan/bin/cca'
s=open(p).read()
old_comment='''  # The [1m] suffix is Claude Code's per-selection 1M pin; the gateway passes it through.
  # NOTE: haiku is truly 200k — never give it [1m].
'''
new_comment='''  # The [1m] suffix is Claude Code's per-selection 1M pin; the gateway passes it through.
  # Haiku 5.5 has 1M too, but the helper calls do not need the pin.
'''
old_block='''  # The harness's opus/sonnet/haiku tiers (the picker, `--model opus`, plugin agents that say
  # `model: sonnet`, the small-model helper calls) resolve to effort slugs, so the proxy's
  # effort pin (his 2668) never refuses a model Claude Code chose on its own.
  export ANTHROPIC_DEFAULT_OPUS_MODEL="${ANTHROPIC_DEFAULT_OPUS_MODEL:-scoped/claude-opus-5-5-medium[1m]}"
  export ANTHROPIC_DEFAULT_SONNET_MODEL="${ANTHROPIC_DEFAULT_SONNET_MODEL:-scoped/claude-sonnet-5-5-medium[1m]}"
  export ANTHROPIC_DEFAULT_HAIKU_MODEL="${ANTHROPIC_DEFAULT_HAIKU_MODEL:-gpt-6-luna-low}"
'''
new_block='''  # The harness's opus/sonnet/haiku tiers (the picker, `--model opus`, plugin agents that say
  # `model: sonnet`, the small-model helper calls) resolve to bare ids; effort comes from the
  # agent page or the modelSettings row. The effort slugs were retired 2026-10-07
  # (cpa-cache-watch docs/fork/specs/2026-10-07-no-substitution-design.md).
  export ANTHROPIC_DEFAULT_OPUS_MODEL="${ANTHROPIC_DEFAULT_OPUS_MODEL:-scoped/claude-opus-5-5[1m]}"
  export ANTHROPIC_DEFAULT_SONNET_MODEL="${ANTHROPIC_DEFAULT_SONNET_MODEL:-scoped/claude-sonnet-5-5[1m]}"
  export ANTHROPIC_DEFAULT_HAIKU_MODEL="${ANTHROPIC_DEFAULT_HAIKU_MODEL:-claude-haiku-5-5}"
'''
assert old_comment in s and old_block in s
s=s.replace(old_comment,new_comment).replace(old_block,new_block)
open(p,'w').write(s)
EOF
bash -n ~/bin/cca && grep -n 'ANTHROPIC_DEFAULT_' ~/bin/cca
```

Expected: `bash -n` silent; the three exports show the bare ids.

- [ ] **Step 3: Smoke test a fresh session and the Haiku tier**

```bash
cd /tmp && ~/bin/cca -p --max-turns 2 --model haiku 'Reply with the single word ok' 2>&1 | tail -2
grep -E 'model=claude-haiku-5-5' ~/.cli-proxy-api/logs/main.log | tail -1
```

Expected: `ok`; the proxy log shows a `claude-haiku-5-5` request (routed to a Claude account), no `effort-pin` refusal, no `model_not_found`.

---

### Task 10: Side-instance proofs 2, 3 and 4 with the new binary

**Files:**
- Uses: `.ceeya/sittings/2026-09-29-proxy-effort-slugs/` in the NewCeeya tree (its `README.md` documents the rig: `side-config.yaml` on :8319, `fake-upstream.py` on :8398, `captures/`), `dist/cliproxyapi` from `make build`.

- [ ] **Step 1: Build and start the side instance with the new block**

```bash
cd /Users/arshan/Desktop/NewShi/cpa-cache-watch && make build
RIG=/Users/arshan/Desktop/BeginningOfTheEnd/New/NewCeeya/.ceeya/sittings/2026-09-29-proxy-effort-slugs
cat $RIG/README.md | sed -n '/## The rig/,/## Measurement 1/p'
cp $RIG/side-config.yaml $RIG/side-config-no-substitution.yaml
printf '\nno-substitution:\n  strip-fallbacks: true\n  refuse-unsupported-effort: true\n' >> $RIG/side-config-no-substitution.yaml
```

Start `fake-upstream.py` and the side proxy with `dist/cliproxyapi -config $RIG/side-config-no-substitution.yaml` the way the README says (the README's commands are the source of truth for ports, pid files and the capture directory); confirm `GET http://127.0.0.1:8319/v1/models` answers.

- [ ] **Step 2: Proof 3 (effort refusal) against :8319**

```bash
SKEY=$(ruby -ryaml -e 'print (YAML.load_file(ARGV[0])["api-keys"]||[]).last' $RIG/side-config-no-substitution.yaml)
curl -s -o /tmp/p3.json -w '%{http_code}\n' http://127.0.0.1:8319/v1/messages -H "Authorization: Bearer $SKEY" -H 'content-type: application/json' \
  -d '{"model":"gpt-6.1-sol","max_tokens":8,"thinking":{"type":"adaptive"},"output_config":{"effort":"ultra"},"messages":[{"role":"user","content":"hi"}]}'
cat /tmp/p3.json; grep -c 'no-substitution: effort refused' $RIG/side-proxy.stdout
```

Expected: `400`, a body whose `error.message` begins `effort_unsupported: model gpt-6.1-sol does not offer effort "ultra"`, and one Error line. Control: edit the side config to `refuse-unsupported-effort: false`, wait for the reload line, repeat the curl: `200` with the fake upstream's capture showing `reasoning.effort = max` and the old `level clamped` Warn.

- [ ] **Step 3: Proof 2 (fallback strip) against :8319**

Run A, switches off, strip on: in a shell with the four `CLAUDE_CODE_*` switches unset, point `cca` at the side instance and run one Opus turn:

```bash
cd /tmp && env -u CLAUDE_CODE_NO_MODEL_FALLBACK -u CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK -u CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK -u CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP \
  CCA_PROXY_URL=http://127.0.0.1:8319 CCA_PROXY_KEY=$SKEY ~/bin/cca -p --max-turns 2 --model opus 'Reply with the single word ok' >/dev/null 2>&1
ls -t $RIG/captures | head -3
```

(`settings.json` env would re-set the switches; for run A, run with `--settings '{"env":{}}'` is not enough because user settings still apply, so temporarily move the four keys out with the python one-liner from Task 9 reversed, and restore them after run C.)

Read the newest capture: its body must have no `fallbacks` key; `side-proxy.stdout` must show one `no-substitution: fallbacks stripped` Warn naming the session, and the request must have been classed native (the log's client-version line names `claude-cli/2.1.293`). If the ingress body carried no `fallbacks` at all (no Warn, no key), replay the previous session's captured native request with its original headers via curl and record whether it was classed native.

Run B, strip off: set `strip-fallbacks: false` in the side config, wait for the reload, repeat run A: the capture carries `fallbacks`.

Run C, strip on, switches on (settings restored): repeat; the capture has no `fallbacks` and no Warn fires.

- [ ] **Step 4: Proof 4 (client refusal fallback) against :8319**

Make `fake-upstream.py` answer `stop_reason: "refusal"` with `stop_details: {"type":"refusal","category":"cyber"}` for one Opus 5.5 turn (the README's `spawn-agent` switch shows how the fake chooses an answer; add a `refusal` switch beside it). Control run with the four switches unset: count captures for that session; if there are two with different `model` values, the client fallback armed and the control is valid; if only one, mark proof 4 "not reproducible through the rig" in the sitting note and do not claim it. Run with the switches set: exactly one capture, and the session's JSON result reports the refusal.

- [ ] **Step 5: Stop the side instance and restore**

Stop the side proxy and the fake upstream (their pid files are in the rig), restore `~/.claude/settings.json` from the Task 9 state if it was changed for run A, and write the capture names and log lines into `$RIG/no-substitution-proofs.md`.

---

### Task 11: Ceeya sitting note and continuation lines

**Files:**
- Create: `/Users/arshan/Desktop/BeginningOfTheEnd/New/NewCeeya/.ceeya/sittings/2026-10-07-effort-slugs-retired/README.md`
- Modify (append one line each): `.ceeya/record/brands/continuation.md`, `.ceeya/record/ingredient/continuation.md`, `.ceeya/record/ketosis/continuation.md`, `.ceeya/record/scan/continuation.md`

- [ ] **Step 1: Write the sitting note**

```markdown
# Effort slugs retired; substitution made loud (2026-10-07, the proxy seat)

His words in the proxy-upgrade session (2026-10-07): "i think we shoudl get rid of the effort slug
system", and on silent regressions, "is ther a way to make errors loud, and fallbacks not occur?
both for effort and models". Design: cpa-cache-watch `docs/fork/specs/2026-10-07-no-substitution-design.md`
(fork commit 9df6b669), reviewed by an Opus 5.5 lane and approved by him.

## Why the slugs can go

Claude Code 2.1.293 copies `effort` from the agent definition into an in-process teammate
(2.1.284 copied only `model`; issue #80569 is still open but the binary is fixed). Measured
2026-10-07 13:15 through the live proxy: a headless session at `low` spawned
`effort-probe` (`effort: high`, `model: claude-sonnet-5-5-medium`) named and unnamed; both
transcripts under `~/.claude/projects/-private-tmp-effort-probe/401a6b59-da0a-44c8-b8e1-d58a35117bb4/subagents/`
record `"effort":"high"`, and `main.log` 13:15:22-23 shows both requests arriving
`original_value=high`. Repeat it: `grep -o '"effort":"[a-z]*"' <transcript> | sort -u`.

## What changed outside this tree (done)

- `~/.cli-proxy-api/config.yaml`: `effort-pin.enabled: false`. Bare ids are accepted again;
  the slugs still resolve (the fork code is dormant, not removed), so nothing breaks while
  pages are migrated.
- `~/bin/cca`: the opus/sonnet tiers are `scoped/claude-opus-5-5[1m]` and
  `scoped/claude-sonnet-5-5[1m]`; the haiku tier is `claude-haiku-5-5` (Claude accounts, not
  the Codex pool).
- `~/.claude/settings.json` env: `CLAUDE_CODE_NO_MODEL_FALLBACK=1`,
  `CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK=1`, `CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK=1`,
  `CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP=1`. A refusal is now shown, not retried on another
  model. Known cost: a compaction that would need a model swap fails with "Compaction
  unavailable" instead; if a Fable session hits it, drop `CLAUDE_CODE_NO_MODEL_FALLBACK` alone.
- `~/.claude/agents/{luna,sol,codebase-refactor-architect}.md`: bare ids with `effort: high`.
- The proxy branch carries `no-substitution` (strip `fallbacks`; 400 `effort_unsupported:` on
  an effort the model lacks). Installed as v8.0.3-locked.5 on his word; until then the pin-off
  and the client switches are what is live.

## What the next session does in this tree

Each of these 17 pages names a slug; make the `model:` bare (keep the account prefix and
`[1m]`) and add an `effort:` line with the level the slug named:

| page | model today | model after | effort |
|---|---|---|---|
| astra-high | gpt-6-astra-high | gpt-6-astra | high |
| fable-high | claude-fable-5-1-high[1m] | claude-fable-5-1[1m] | high |
| fable-medium | claude-fable-5-1-medium[1m] | claude-fable-5-1[1m] | medium |
| glm-flash | cf-glm53-flash-high | cf-glm53-flash | high |
| luna-low | gpt-6-luna-low | gpt-6-luna | low |
| luna-medium | gpt-6-luna-medium | gpt-6-luna | medium |
| luna-web | gpt-6-luna-low | gpt-6-luna | low |
| nutritionist | scoped/claude-opus-5-5-medium[1m] | scoped/claude-opus-5-5[1m] | medium |
| opus | scoped/claude-opus-5-5-medium[1m] | scoped/claude-opus-5-5[1m] | medium |
| reader-opus | scoped/claude-opus-5-5-medium[1m] | scoped/claude-opus-5-5[1m] | medium |
| reader-sonnet | scoped/claude-sonnet-5-5-medium[1m] | scoped/claude-sonnet-5-5[1m] | medium |
| seed-conductor | scoped/claude-opus-5-5-medium[1m] | scoped/claude-opus-5-5[1m] | medium |
| sol-low | gpt-6.1-sol-low | gpt-6.1-sol | low |
| sol-medium | gpt-6.1-sol-medium | gpt-6.1-sol | medium |
| sol | gpt-6.1-sol-high | gpt-6.1-sol | high |
| sonnet | scoped/claude-sonnet-5-5-medium[1m] | scoped/claude-sonnet-5-5[1m] | medium |
| ui-ux | scoped/claude-opus-5-5-medium[1m] | scoped/claude-opus-5-5[1m] | medium |

The conductor page stays bare Fable. The twelve worktree copies of `.claude/agents/` take
the same edit when their lanes next land. The openers under `.ceeya/record/openers/` carry the
standing effort-pin paragraph; retire it. `decision.md` rows 27 and 28 take a dated addendum:
§2663b superseded on 2.1.293; the named-spawn rule in row 27 no longer holds. The offline
tools (`tools/sources/read.ts`, `author.ts`, `discover.ts`, `classify-corpus.ts`, the bench
rows, `kind-run.sh`, `author-top.ts`) may name bare ids with `--effort` again; the slugs
keep working, so each changes on its next edit.

## Verify line

Spawn one named agent from a page whose effort differs from the session; its transcript
under `subagents/` shows the page's effort. `grep -c 'rewritten by model suffix'
~/.cli-proxy-api/logs/main.log` must not grow once a page is bare; `grep 'effort refused'`
and `grep 'fallbacks stripped'` show the loud path once v8.0.3-locked.5 is installed.
```

- [ ] **Step 2: Append one line to each stream continuation**

```bash
N=/Users/arshan/Desktop/BeginningOfTheEnd/New/NewCeeya
for s in brands ingredient ketosis scan; do
  printf '\n## From the proxy seat (2026-10-07), one line\nThe effort slugs are retired on his word: the proxy effort pin is off and bare ids are accepted again; this stream edits its agent pages to bare `model:` plus `effort:` on its next session, table and verify line in `.ceeya/sittings/2026-10-07-effort-slugs-retired/README.md`.\n' >> $N/.ceeya/record/$s/continuation.md
done
tail -3 $N/.ceeya/record/brands/continuation.md
```

Expected: each file ends with the new heading and line. Do not commit in the NewCeeya repo; the next wrap lands it.

---

### Task 12: Install on his word, then the live proofs

Only after he says so.

- [ ] **Step 1: Add the block to the live config and install**

```bash
python3 - <<'EOF'
p='/Users/arshan/.cli-proxy-api/config.yaml'
s=open(p).read()
assert 'no-substitution:' not in s
s=s.rstrip('\n')+'\n# No-substitution (his word 2026-10-07): fallbacks stripped, unsupported effort refused.\nno-substitution:\n  strip-fallbacks: true\n  refuse-unsupported-effort: true\n'
open(p,'w').write(s)
EOF
cd /Users/arshan/Desktop/NewShi/cpa-cache-watch && git tag v8.0.3-locked.5 && make install VERSION=v8.0.3-locked.5
/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi --version
grep 'no-substitution: switches applied' ~/.cli-proxy-api/logs/main.log | tail -1
```

Expected: version `v8.0.3-locked.5`; the switches line shows both true. (Add the block only after the new binary runs: the old binary's v8 decoder refuses unknown root sections.)

- [ ] **Step 2: Live proof 3 and proof 5**

Repeat Task 10 step 2's curl against :8317 (expect 400 `effort_unsupported:`). Then, in an interactive `cca` session on Fable 5.1, run `/compact`: it must complete; if it reports "Compaction unavailable", remove `CLAUDE_CODE_NO_MODEL_FALLBACK` from `settings.json` env and record it in the sitting note.

- [ ] **Step 3: Tick the audit's post-install item and record the outcome**

Tick `- [ ] After install:` in `docs/fork/audits/v8.0.20.md` only if the egress audit session in that item was actually run; otherwise leave it. Append the proof outcomes (capture names, log lines) to `docs/fork/specs/2026-10-07-no-substitution-design.md` under a `## Outcome` heading and commit as `lock: no-substitution proofs and install outcome`.
