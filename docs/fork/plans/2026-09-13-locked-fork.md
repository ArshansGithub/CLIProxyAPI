# Locked Fork Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the `locked` branch of this CLIProxyAPI fork into a privacy-locked build that cannot download or execute unreviewed code, can only reach allowlisted hosts, and can adopt upstream tags through an audited, scripted workflow.

**Architecture:** Three independent mechanisms: (1) `internal/egress`, a host policy enforced by hooking the `Proxy` function of every `*http.Transport` and websocket dialer plus a RoundTripper wrapper for custom transports; (2) a `locked` build tag whose sibling `_locked.go` files stub the panel updater, plugin loader, plugin store, and remote model refresh; (3) embedded, pinned assets for the panel and model catalogs. A Makefile and `scripts/` carry the adoption workflow, and README, CLAUDE.md, and a skill make the fork self-describing.

**Tech Stack:** Go 1.26, gin, gorilla/websocket, Make, bash, `gh` CLI, bun (panel build only).

**Spec:** `docs/fork/specs/2026-09-13-locked-fork-design.md`

## Global Constraints

- Module path `github.com/router-for-me/CLIProxyAPI/v7`; Go `1.26.0` per `go.mod`. No new Go module dependencies.
- Every build of the fork uses `-tags locked`. A tagless build must still compile and behave as upstream.
- Edits to upstream-owned files must be the minimum that works: a build-tag header line, an early return, or a one-call hook. New behaviour goes in new files.
- Commit subject prefixes: `lock:` for everything in this plan. Every commit ends with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.
- Never push to the `upstream` remote. Never run `git push` in this plan except where Task 14 says so.
- `docs/*` is gitignored upstream; `docs/fork/` is un-ignored and is the only docs path this fork tracks.
- Run `go build ./... && go vet ./... && go test ./...` (tagless) and `go build -tags locked ./... && go test -tags locked ./...` before every commit that touches Go code.
- Working directory for every command: `/Users/arshan/Desktop/NewShi/cpa-cache-watch`, branch `locked`.

### Deviations from the spec, decided during planning

1. **Spec §4.3 (DefaultTransport replacement).** `internal/runtime/executor/antigravity_executor.go:229` and `internal/pluginhost/http_bridge.go:265` type-assert `http.DefaultTransport.(*http.Transport)` and build a raw transport if that fails. Replacing the default transport with a wrapper type would therefore create ungated clients. Instead, the gate hooks `Transport.Proxy` (consulted on every request, keeps the concrete type) and `websocket.Dialer.Proxy`. The RoundTripper wrapper is used only for the two uTLS round trippers, which are not `*http.Transport`.
2. **Spec §4.1 (registration next to constants).** Provider hosts are registered centrally in `internal/egress/providers.go`. The literal-extraction test in Task 4 enforces that every host literal in executor/auth code is registered, which gives the same guarantee with fewer upstream-file edits.
3. **Spec §6.2 (model snapshot directory).** Upstream already embeds `internal/registry/models/models.json` and `codex_client_models.json` and loads them in `init()`. The locked build keeps those files as the snapshot, adds `MODELS_VERSION` beside them, and only removes the remote refresh. No new snapshot directory.
4. **Spec §5 (plugin store file tagging).** `plugin_store.go` defines helpers used by other files, so tagging the whole file out breaks the build. The two handlers get a three-line early return keyed on `lockedbuild.Enabled` instead.

---

## File map

| Path | Responsibility |
|---|---|
| `internal/lockedbuild/enabled.go`, `enabled_locked.go` | `const Enabled` true only under the tag; the one place code asks "am I locked". |
| `internal/config/egress.go` | `EgressConfig` (mode, extra-allow), defaults, validation. |
| `internal/egress/policy.go` | `Policy` type: host set, mode, `Allowed`, `Check`. |
| `internal/egress/providers.go` | Built-in provider host table and `Register`. |
| `internal/egress/current.go` | Process-wide current policy: `SetConfig`, `Current`, `CheckURL`. |
| `internal/egress/transport.go` | `GuardTransport`, `WrapProxyFunc`, `GuardWebsocketDialer`, `RoundTripper`. |
| `internal/egress/coverage_test.go` | Fails on raw transport construction outside the allowlist. |
| `internal/egress/registration_test.go` | Fails on an unregistered `https://` / `wss://` host literal. |
| `internal/egress/known-unregistered.txt` | Literals exempt from registration, each with a reason. |
| `internal/managementasset/updater.go` | Upstream; gains `//go:build !locked`. |
| `internal/managementasset/updater_locked.go` | Stubs for the updater's exported API. |
| `internal/managementasset/embedded.go`, `embedded_locked.go` | `EmbeddedPanel()`; the locked one embeds `panel/management.html`. |
| `internal/managementasset/panel/` | `management.html`, `PANEL_VERSION`. |
| `internal/pluginhost/loader_locked.go` | Loader whose `Open` always errors. |
| `internal/registry/model_sources.go`, `model_sources_locked.go` | Remote catalog URLs; empty under the tag. |
| `internal/registry/models/MODELS_VERSION` | Pin for the embedded catalogs. |
| `internal/lockedbuild/strings_test.go` | Builds `-tags locked` and scans the binary for forbidden hosts. |
| `Makefile` | build, verify, install, rollback, upstream-fetch, upstream-audit, rebase, patches, panel-*, refresh-models. |
| `scripts/upstream-audit.sh`, `scripts/patches.sh`, `scripts/panel.sh`, `scripts/refresh-models.sh`, `scripts/install.sh` | The workflow, one script per concern. |
| `docs/fork/PATCHES.yaml`, `docs/fork/PATCHES.md` | Carried-patch register (hand-annotated YAML, generated Markdown). |
| `docs/fork/audits/` | One report per audited upstream tag. |
| `README.md`, `docs/UPSTREAM-README.md`, `CLAUDE.md`, `.claude/skills/fork-maintenance/SKILL.md` | Fork documentation and agent support. |

---

## Phase 0: repository hygiene

### Task 1: remotes, archive, Makefile skeleton

**Files:**
- Create: `Makefile`
- Create: `scripts/install.sh`
- Modify: git remotes (no tracked file)

**Interfaces:**
- Produces: `make build` (writes `dist/cliproxyapi`), `make verify`, `make install`, `make rollback`, `make upstream-fetch`. Later tasks add targets to this Makefile.

- [ ] **Step 1: Rename remotes so `origin` is the fork and `upstream` is router-for-me, fetch-only**

```bash
cd /Users/arshan/Desktop/NewShi/cpa-cache-watch
git remote rename origin upstream
git remote rename fork origin
git remote set-url --push upstream no_push
git remote -v
```
Expected: `origin` → `ArshansGithub/CLIProxyAPI` (fetch+push), `upstream` → `router-for-me/CLIProxyAPI` (fetch), push URL `no_push`.

- [ ] **Step 2: Archive the second checkout**

```bash
mkdir -p /Users/arshan/Desktop/NewShi/_archive
mv /Users/arshan/Desktop/NewShi/cliproxyapi-patched /Users/arshan/Desktop/NewShi/_archive/cliproxyapi-patched-2026-09-13
```

- [ ] **Step 3: Write `scripts/install.sh`**

```bash
#!/usr/bin/env bash
# Install dist/cliproxyapi over the Homebrew binary, restart the service, check health.
set -euo pipefail
BIN=/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi
SRC="${1:-dist/cliproxyapi}"
[ -x "$SRC" ] || { echo "missing $SRC (run make build)"; exit 1; }
cp "$BIN" "$BIN.prev"
cp "$SRC" "$BIN.new" && mv "$BIN.new" "$BIN"
brew services restart cliproxyapi >/dev/null
for _ in $(seq 1 30); do
  if curl -fsS -m 2 http://127.0.0.1:8317/healthz >/dev/null 2>&1; then
    echo "healthy; running: $("$BIN" -v 2>&1 | head -1)"; exit 0
  fi
  sleep 1
done
echo "proxy did not become healthy in 30s; run: make rollback"; exit 1
```
Then `chmod +x scripts/install.sh`.

- [ ] **Step 4: Write the Makefile**

```makefile
SHELL := /bin/bash
TAG_UPSTREAM ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null)
COMMIT       := $(shell git rev-parse --short HEAD)
VERSION      ?= $(TAG_UPSTREAM)-locked.dev
LDFLAGS      := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) \
                -X main.DefaultConfigPath=/opt/homebrew/etc/cliproxyapi.conf
GOFLAGS_LOCKED := -tags locked

.PHONY: build verify verify-tagless install rollback upstream-fetch

build:
	@mkdir -p dist
	go build $(GOFLAGS_LOCKED) -ldflags "$(LDFLAGS)" -o dist/cliproxyapi ./cmd/server
	@echo "built dist/cliproxyapi ($(VERSION) $(COMMIT))"

verify:
	go build $(GOFLAGS_LOCKED) ./...
	go vet $(GOFLAGS_LOCKED) ./...
	go test $(GOFLAGS_LOCKED) ./...

verify-tagless:
	go build ./... && go vet ./... && go test ./...

install: build
	./scripts/install.sh dist/cliproxyapi

rollback:
	@BIN=/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi; \
	[ -f $$BIN.prev ] || { echo "no .prev binary"; exit 1; }; \
	cp $$BIN.prev $$BIN && brew services restart cliproxyapi && echo "rolled back"

upstream-fetch:
	git fetch upstream --tags --no-write-fetch-head 'refs/tags/*:refs/tags/*'
	@echo "latest upstream tag: $$(git tag --sort=-v:refname | head -1)"
```

- [ ] **Step 5: Verify the skeleton works**

Run: `make verify-tagless && make build && ls -la dist/cliproxyapi && ./dist/cliproxyapi -v`
Expected: build and tests pass; version line prints `v7.2.154-locked.dev`.

- [ ] **Step 6: Add `dist/` to `.gitignore` and commit**

```bash
printf 'dist/\n' >> .gitignore
git add Makefile scripts/install.sh .gitignore
git commit -m "lock: Makefile, install script, fetch-only upstream remote

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 1: egress gate

### Task 2: `lockedbuild` package and `egress` config block

**Files:**
- Create: `internal/lockedbuild/enabled.go`, `internal/lockedbuild/enabled_locked.go`, `internal/lockedbuild/enabled_test.go`
- Create: `internal/config/egress.go`, `internal/config/egress_test.go`
- Modify: `internal/config/sdk_config.go:55` (add field after `UsageCacheStats`)
- Modify: `internal/config/config_load.go:96-99` and `internal/config/parse.go:50-53` (defaults + validate, mirroring `UsageCacheStats`)

**Interfaces:**
- Produces: `lockedbuild.Enabled bool` (const); `config.EgressConfig{Mode string; ExtraAllow []string}` with `WithDefaults()` and `Validate()`; `config.EgressModeEnforce = "enforce"`, `config.EgressModeAudit = "audit"`; field `SDKConfig.Egress`.

- [ ] **Step 1: Write the lockedbuild files**

`internal/lockedbuild/enabled.go`:
```go
//go:build !locked

// Package lockedbuild reports whether the binary was compiled with the locked tag.
package lockedbuild

// Enabled is true only in builds compiled with `-tags locked`.
const Enabled = false
```
`internal/lockedbuild/enabled_locked.go`:
```go
//go:build locked

package lockedbuild

// Enabled is true only in builds compiled with `-tags locked`.
const Enabled = true
```
`internal/lockedbuild/enabled_test.go`:
```go
package lockedbuild

import "testing"

func TestEnabledIsAConstant(t *testing.T) {
	// The value depends on the build tag; the test only asserts the symbol exists
	// and is usable as a bool in both builds.
	if Enabled && !Enabled {
		t.Fatal("unreachable")
	}
}
```

- [ ] **Step 2: Write the failing config test**

`internal/config/egress_test.go`:
```go
package config

import "testing"

func TestEgressConfigDefaults(t *testing.T) {
	cfg := EgressConfig{}.WithDefaults()
	if cfg.Mode != EgressModeEnforce {
		t.Fatalf("default mode = %q, want %q", cfg.Mode, EgressModeEnforce)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestEgressConfigValidateRejectsUnknownMode(t *testing.T) {
	cfg := EgressConfig{Mode: "yolo"}.WithDefaults()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestEgressConfigNormalizesExtraAllow(t *testing.T) {
	cfg := EgressConfig{ExtraAllow: []string{" Example.COM ", "", "api.example.com:443"}}.WithDefaults()
	want := []string{"example.com", "api.example.com"}
	if len(cfg.ExtraAllow) != len(want) {
		t.Fatalf("ExtraAllow = %v, want %v", cfg.ExtraAllow, want)
	}
	for i := range want {
		if cfg.ExtraAllow[i] != want[i] {
			t.Fatalf("ExtraAllow[%d] = %q, want %q", i, cfg.ExtraAllow[i], want[i])
		}
	}
}
```

- [ ] **Step 3: Run it to confirm failure**

Run: `go test ./internal/config/ -run TestEgressConfig -v`
Expected: FAIL, `undefined: EgressConfig`.

- [ ] **Step 4: Implement `internal/config/egress.go`**

```go
package config

import (
	"fmt"
	"net"
	"strings"
)

const (
	// EgressModeEnforce refuses connections to hosts outside the policy.
	EgressModeEnforce = "enforce"
	// EgressModeAudit logs would-be refusals and lets the connection proceed.
	EgressModeAudit = "audit"
)

// EgressConfig controls the outbound host policy. See internal/egress.
type EgressConfig struct {
	// Mode is "enforce" (default) or "audit".
	Mode string `yaml:"mode" json:"mode"`
	// ExtraAllow lists hostnames admitted in addition to provider hosts and
	// configured base URLs. Ports are ignored; matching is case-insensitive.
	ExtraAllow []string `yaml:"extra-allow" json:"extra-allow"`
}

// WithDefaults fills the mode and normalizes ExtraAllow to lowercase hostnames.
func (c EgressConfig) WithDefaults() EgressConfig {
	out := c
	if strings.TrimSpace(out.Mode) == "" {
		out.Mode = EgressModeEnforce
	}
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	normalized := make([]string, 0, len(c.ExtraAllow))
	for _, raw := range c.ExtraAllow {
		host := NormalizeEgressHost(raw)
		if host != "" {
			normalized = append(normalized, host)
		}
	}
	out.ExtraAllow = normalized
	return out
}

// Validate rejects unknown modes.
func (c EgressConfig) Validate() error {
	switch c.Mode {
	case EgressModeEnforce, EgressModeAudit:
		return nil
	}
	return fmt.Errorf("egress.mode must be %q or %q, got %q", EgressModeEnforce, EgressModeAudit, c.Mode)
}

// NormalizeEgressHost lowercases, trims, and strips a port. Returns "" for empty input.
func NormalizeEgressHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}
```

- [ ] **Step 5: Wire the field and defaults**

In `internal/config/sdk_config.go` directly after the `UsageCacheStats` field (line 55):
```go
	// Egress configures the outbound host allowlist enforced by internal/egress.
	Egress EgressConfig `yaml:"egress" json:"egress"`
```
In `internal/config/config_load.go` directly after the `UsageCacheStats` validate block (around line 99), and identically in `internal/config/parse.go` after line 53:
```go
	cfg.Egress = cfg.Egress.WithDefaults()
	if errValidate := cfg.Egress.Validate(); errValidate != nil {
		return nil, errValidate
	}
```
(Match the surrounding function's return shape exactly; if the neighbouring block returns `err` differently, copy that form.)

- [ ] **Step 6: Run tests**

Run: `go test ./internal/config/ ./internal/lockedbuild/ && go test -tags locked ./internal/lockedbuild/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/lockedbuild internal/config/egress.go internal/config/egress_test.go internal/config/sdk_config.go internal/config/config_load.go internal/config/parse.go
git commit -m "lock: lockedbuild.Enabled and the egress config block

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 3: egress policy

**Files:**
- Create: `internal/egress/policy.go`, `internal/egress/policy_test.go`, `internal/egress/current.go`

**Interfaces:**
- Produces:
  - `type Policy struct` with `func (p *Policy) Allowed(host string) bool`, `func (p *Policy) Check(host, site string) error`, `func (p *Policy) Mode() string`.
  - `func NewPolicy(cfg *config.Config, builtin []string) *Policy` — `builtin` comes from Task 4.
  - `func SetConfig(cfg *config.Config)` rebuilds and installs the process-wide policy; `func Current() *Policy`; `func CheckURL(u *url.URL, site string) error`; `func CheckURLString(raw, site string) error`.
  - `var ErrNotPermitted` sentinel; errors wrap it and read `egress: host "x" not permitted (site=pkg.func, mode=enforce)`.
- Consumes: `config.EgressConfig`, `config.NormalizeEgressHost`.

- [ ] **Step 1: Write the failing tests**

`internal/egress/policy_test.go`:
```go
package egress

import (
	"errors"
	"net/url"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func testConfig(mode string, extra ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Egress = config.EgressConfig{Mode: mode, ExtraAllow: extra}.WithDefaults()
	cfg.OpenAICompatibility = []config.OpenAICompatibility{{Name: "or", BaseURL: "https://openrouter.ai/api/v1"}}
	return cfg
}

func TestPolicyAllowsBuiltinConfiguredLoopbackAndExtra(t *testing.T) {
	p := NewPolicy(testConfig("enforce", "Extra.Example.com"), []string{"api.anthropic.com"})
	for _, host := range []string{"api.anthropic.com", "API.ANTHROPIC.COM", "openrouter.ai", "127.0.0.1", "::1", "localhost", "extra.example.com"} {
		if !p.Allowed(host) {
			t.Errorf("%q should be allowed", host)
		}
	}
	if p.Allowed("evil.example.net") {
		t.Error("unlisted host should be refused")
	}
}

func TestPolicyCheckEnforceReturnsErrNotPermitted(t *testing.T) {
	p := NewPolicy(testConfig("enforce"), nil)
	err := p.Check("evil.example.net", "test.site")
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
	want := `egress: host "evil.example.net" not permitted (site=test.site, mode=enforce)`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestPolicyCheckAuditAllowsButRecords(t *testing.T) {
	p := NewPolicy(testConfig("audit"), nil)
	if err := p.Check("evil.example.net", "test.site"); err != nil {
		t.Fatalf("audit mode must not refuse: %v", err)
	}
	if got := p.AuditRefusals(); got != 1 {
		t.Fatalf("audit refusals = %d, want 1", got)
	}
}

func TestCheckURLUsesHostnameOnly(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), []string{"api.anthropic.com"})
	u, _ := url.Parse("https://api.anthropic.com:443/v1/messages?x=1")
	if err := CheckURL(u, "t"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := CheckURLString("wss://chatgpt.com/backend-api/codex/responses", "t"); err == nil {
		t.Fatal("chatgpt.com is not registered in this test and must be refused")
	}
}

func TestNoPolicyInstalledRefusesInEnforce(t *testing.T) {
	resetForTest()
	if err := CheckURLString("https://api.anthropic.com/", "t"); err == nil {
		t.Fatal("with no policy installed, enforce must refuse")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/egress/ -v`
Expected: FAIL, package does not exist / undefined symbols.

- [ ] **Step 3: Implement `policy.go`**

```go
// Package egress decides which outbound hosts the proxy may connect to.
//
// The policy is the union of built-in provider hosts (providers.go), the
// hostname of every configured base URL, loopback, and egress.extra-allow.
// Enforcement points live in transport.go.
package egress

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

// ErrNotPermitted is wrapped by every refusal.
var ErrNotPermitted = errors.New("egress: host not permitted")

// Policy is an immutable host set plus a mode.
type Policy struct {
	hosts         map[string]struct{}
	mode          string
	auditRefusals atomic.Int64
}

// NewPolicy builds a policy from config and the built-in provider host list.
func NewPolicy(cfg *config.Config, builtin []string) *Policy {
	p := &Policy{hosts: map[string]struct{}{}, mode: config.EgressModeEnforce}
	add := func(raw string) {
		if h := config.NormalizeEgressHost(raw); h != "" {
			p.hosts[h] = struct{}{}
		}
	}
	for _, h := range []string{"127.0.0.1", "::1", "localhost"} {
		add(h)
	}
	for _, h := range builtin {
		add(h)
	}
	if cfg != nil {
		p.mode = cfg.Egress.WithDefaults().Mode
		for _, h := range cfg.Egress.ExtraAllow {
			add(h)
		}
		for _, u := range configuredBaseURLs(cfg) {
			if parsed, err := url.Parse(strings.TrimSpace(u)); err == nil {
				add(parsed.Hostname())
			}
		}
	}
	return p
}

// Mode returns "enforce" or "audit".
func (p *Policy) Mode() string { return p.mode }

// Allowed reports whether host (with or without port, any case) is in the set.
func (p *Policy) Allowed(host string) bool {
	_, ok := p.hosts[config.NormalizeEgressHost(host)]
	return ok
}

// Check returns nil when host is allowed. In enforce mode a refusal is an
// error wrapping ErrNotPermitted; in audit mode it is logged and counted.
func (p *Policy) Check(host, site string) error {
	if p.Allowed(host) {
		return nil
	}
	if p.mode == config.EgressModeAudit {
		p.auditRefusals.Add(1)
		log.Warnf("egress audit: host %q would be refused (site=%s)", config.NormalizeEgressHost(host), site)
		return nil
	}
	err := fmt.Errorf("egress: host %q not permitted (site=%s, mode=%s)", config.NormalizeEgressHost(host), site, p.mode)
	log.Errorf("%v", err)
	return errors.Join(ErrNotPermitted, err) // errors.Is works; Error() below keeps the message clean
}

// AuditRefusals returns how many audit-mode refusals this policy has logged.
func (p *Policy) AuditRefusals() int64 { return p.auditRefusals.Load() }

// configuredBaseURLs collects every base-url field a user can set in config.
// Extend this list when upstream adds a provider block with a base URL.
func configuredBaseURLs(cfg *config.Config) []string {
	var out []string
	for _, e := range cfg.OpenAICompatibility {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.CodexKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.ClaudeKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.GeminiKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.VertexCompatAPIKey {
		out = append(out, e.BaseURL)
	}
	return out
}
```

`errors.Join` changes `Error()` to two lines. To keep the exact message the test expects, replace the last line of `Check` with a typed error:

```go
	return &refusal{msg: err.Error()}
}

type refusal struct{ msg string }

func (r *refusal) Error() string   { return r.msg }
func (r *refusal) Is(target error) bool { return target == ErrNotPermitted }
```
and drop the `errors.Join` line. Keep the `err := fmt.Errorf(...)` and `log.Errorf` lines.

The four slice field names in `configuredBaseURLs` (`CodexKey`, `ClaudeKey`, `GeminiKey`, `VertexCompatAPIKey`) must match `internal/config/config_types.go`. Verify with:
```bash
grep -n 'BaseURL string' -B 40 internal/config/config_types.go | grep -n '^.*type \|yaml:"base-url'
awk '/^type Config struct/,/^}/' internal/config/config_types.go | grep -n '\[\]'
```
Use the exact field names that appear; only include blocks whose element type has a `BaseURL` field.

- [ ] **Step 4: Implement `current.go`**

```go
package egress

import (
	"net/url"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

var current atomic.Pointer[Policy]

// SetConfig rebuilds the process-wide policy from cfg and the built-in hosts.
// Call it after config load and on every config reload.
func SetConfig(cfg *config.Config) { SetConfigWithBuiltin(cfg, BuiltinHosts()) }

// SetConfigWithBuiltin is SetConfig with an explicit built-in list (tests).
func SetConfigWithBuiltin(cfg *config.Config, builtin []string) {
	current.Store(NewPolicy(cfg, builtin))
}

// Current returns the installed policy. Before SetConfig runs it returns a
// loopback-only enforce policy, so nothing leaks during startup.
func Current() *Policy {
	if p := current.Load(); p != nil {
		return p
	}
	return NewPolicy(nil, nil)
}

// CheckURL applies the current policy to u's hostname.
func CheckURL(u *url.URL, site string) error {
	if u == nil {
		return nil
	}
	return Current().Check(u.Hostname(), site)
}

// CheckURLString parses raw and applies the current policy.
func CheckURLString(raw, site string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	return CheckURL(u, site)
}

func resetForTest() { current.Store(nil) }
```
`BuiltinHosts()` is defined in Task 4; for this task create a temporary `internal/egress/providers.go` containing only:
```go
package egress

// BuiltinHosts returns the provider hosts compiled into this binary. Filled in Task 4.
func BuiltinHosts() []string { return nil }
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/egress/ -v`
Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/egress
git commit -m "lock: egress policy with enforce and audit modes

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 4: provider host registry and the registration test

**Files:**
- Modify: `internal/egress/providers.go` (replace the stub)
- Create: `internal/egress/registration_test.go`, `internal/egress/known-unregistered.txt`

**Interfaces:**
- Produces: `func BuiltinHosts() []string`; `func Register(provider string, hosts ...string)` for use from `init()` anywhere.

- [ ] **Step 1: Write the failing registration test**

`internal/egress/registration_test.go`:
```go
package egress

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// scanRoots are the packages whose URL literals name upstream provider hosts.
var scanRoots = []string{"internal/runtime/executor", "internal/auth", "sdk/auth", "internal/registry", "internal/client"}

var urlLiteral = regexp.MustCompile(`"(?:https|wss)://([A-Za-z0-9.-]+)`)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}

func loadKnownUnregistered(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	f, err := os.Open(filepath.Join(root, "internal/egress/known-unregistered.txt"))
	if err != nil {
		t.Fatalf("open known-unregistered.txt: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		host := strings.Fields(line)[0] // "host  # reason"
		out[strings.ToLower(host)] = true
	}
	return out
}

func TestEveryProviderHostLiteralIsRegistered(t *testing.T) {
	root := repoRoot(t)
	known := loadKnownUnregistered(t, root)
	registered := map[string]bool{}
	for _, h := range BuiltinHosts() {
		registered[strings.ToLower(h)] = true
	}
	missing := map[string][]string{}
	for _, rel := range scanRoots {
		_ = filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, _ := os.ReadFile(path)
			for _, m := range urlLiteral.FindAllStringSubmatch(string(data), -1) {
				host := strings.ToLower(m[1])
				if registered[host] || known[host] || strings.Contains(host, "example") || host == "localhost" {
					continue
				}
				r, _ := filepath.Rel(root, path)
				missing[host] = append(missing[host], r)
			}
			return nil
		})
	}
	if len(missing) > 0 {
		hosts := make([]string, 0, len(missing))
		for h := range missing {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		for _, h := range hosts {
			t.Errorf("unregistered host %q in %v — add to providers.go or known-unregistered.txt with a reason", h, missing[h])
		}
	}
}
```

- [ ] **Step 2: Run it to see the real list of hosts**

Run: `go test ./internal/egress/ -run TestEveryProviderHostLiteralIsRegistered -v 2>&1 | grep unregistered | sort`
Expected: FAIL listing every host literal in the scanned packages (roughly the 30 hosts found during planning: api.anthropic.com, chatgpt.com, api.openai.com, auth.openai.com, platform.openai.com, console.anthropic.com, api.x.ai, auth.x.ai, accounts.x.ai, vidgen.x.ai, x.ai, cli-chat-proxy.grok.com, api.kimi.com, cloudcode-pa.googleapis.com, daily-cloudcode-pa.googleapis.com, daily-cloudcode-pa.sandbox.googleapis.com, www.googleapis.com, oauth2.googleapis.com, aiplatform.googleapis.com, us-east5-aiplatform.googleapis.com, vertex.googleapis.com, generativelanguage.googleapis.com, vertexaisearch.cloud.google.com, github.com, api.github.com, raw.githubusercontent.com, models.router-for.me, cpamc.router-for.me, plus any others). Copy the printed list; the next step sorts it.

- [ ] **Step 3: Write `providers.go`**

Every host from step 2 goes in exactly one of two places. Provider API and OAuth hosts go here, grouped by provider. Hosts that the locked build must never contact (GitHub release feeds, `models.router-for.me`, `cpamc.router-for.me`, `raw.githubusercontent.com`) go in `known-unregistered.txt` instead, so they are not admitted.

```go
package egress

import (
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]map[string]struct{}{}
)

// Register records hosts a provider's code may contact. Safe from init().
func Register(provider string, hosts ...string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	set, ok := registry[provider]
	if !ok {
		set = map[string]struct{}{}
		registry[provider] = set
	}
	for _, h := range hosts {
		set[h] = struct{}{}
	}
}

// BuiltinHosts returns every registered host, sorted, for NewPolicy.
func BuiltinHosts() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var out []string
	for _, set := range registry {
		for h := range set {
			out = append(out, h)
		}
	}
	sort.Strings(out)
	return out
}

// Built-in provider hosts. Each line corresponds to URL literals in the
// package named in the comment; registration_test.go fails if a literal in
// executor/auth code is missing here and not excused in known-unregistered.txt.
func init() {
	Register("claude", // internal/runtime/executor/claude_*, internal/auth/claude
		"api.anthropic.com", "console.anthropic.com")
	Register("codex", // internal/runtime/executor/codex_*, internal/auth/codex, sdk/auth/codex_device.go
		"chatgpt.com", "api.openai.com", "auth.openai.com", "platform.openai.com")
	Register("xai", // internal/auth/xai, internal/runtime/executor/xai_*
		"api.x.ai", "auth.x.ai", "accounts.x.ai", "vidgen.x.ai", "x.ai", "cli-chat-proxy.grok.com")
	Register("kimi", // internal/auth/kimi
		"api.kimi.com")
	Register("gemini", // internal/auth/gemini*, internal/runtime/executor/gemini_*, antigravity
		"cloudcode-pa.googleapis.com", "daily-cloudcode-pa.googleapis.com",
		"daily-cloudcode-pa.sandbox.googleapis.com", "www.googleapis.com", "oauth2.googleapis.com",
		"aiplatform.googleapis.com", "us-east5-aiplatform.googleapis.com", "vertex.googleapis.com",
		"generativelanguage.googleapis.com", "vertexaisearch.cloud.google.com")
}
```
Add or remove entries so the set matches step 2's output exactly, minus the hosts moved to `known-unregistered.txt`.

- [ ] **Step 4: Write `known-unregistered.txt`**

```
# Hosts that appear as literals in scanned packages but are deliberately NOT
# admitted by the egress policy. Format: host  # reason
github.com                     # plugin store / release feeds; locked build never installs plugins
api.github.com                 # panel and plugin release lookups; compiled out under locked
raw.githubusercontent.com      # remote model catalog; locked build uses the embedded snapshot
models.router-for.me           # remote model catalog mirror; same
cpamc.router-for.me            # unverified panel fallback host; compiled out under locked
antigravity-hub-auto-updater-974169037036.us-central1.run.app  # version manifest; code has a built-in fallback
```
Append any other host from step 2 that is a documentation link or fixture, each with a reason.

- [ ] **Step 5: Run the test until it passes**

Run: `go test ./internal/egress/ -v`
Expected: PASS. If a host is still reported, decide: provider host → `providers.go`; anything else → `known-unregistered.txt` with a reason.

- [ ] **Step 6: Commit**

```bash
git add internal/egress
git commit -m "lock: register provider hosts and enforce registration by test

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 5: enforcement hooks

**Files:**
- Create: `internal/egress/transport.go`, `internal/egress/transport_test.go`
- Modify: `sdk/proxyutil/proxy.go:76-91` (`cloneDefaultTransport`, `NewDirectTransport`, `BuildHTTPTransport` return sites)
- Modify: `internal/runtime/executor/antigravity_executor.go:229-232`
- Modify: `internal/runtime/executor/helps/utls_client.go:285-289` (`cachedClaudeCodeRoundTripper`) and `:305`
- Modify: `internal/auth/claude/utls_transport.go:194` and `:253`
- Modify: `internal/api/handlers/management/api_tools.go:117` (after `url.Parse`) and `:521`
- Modify: `internal/runtime/executor/codex_websockets_connection.go:160-167`
- Modify: `internal/api/server.go:199` (next to `managementasset.SetCurrentConfig(cfg)`)
- Modify: `cmd/server/main.go:583` (next to `managementasset.SetCurrentConfig(cfg)`)

**Interfaces:**
- Produces: `egress.GuardTransport(t *http.Transport) *http.Transport`, `egress.WrapProxyFunc(next func(*http.Request) (*url.URL, error), site string)`, `egress.GuardWebsocketDialer(d *websocket.Dialer, site string) *websocket.Dialer`, `egress.RoundTripper(rt http.RoundTripper, site string) http.RoundTripper`.
- Consumes: `CheckURL` from Task 3.

- [ ] **Step 1: Write the failing transport tests**

`internal/egress/transport_test.go`:
```go
package egress

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

func TestGuardedTransportRefusesUnlistedHost(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	client := &http.Client{Transport: GuardTransport(&http.Transport{})}
	// 127.0.0.1 is loopback and allowed.
	if _, err := client.Get(srv.URL); err != nil {
		t.Fatalf("loopback should pass: %v", err)
	}
	// Force a non-loopback hostname that resolves nowhere; the gate must refuse before DNS.
	_, err := client.Get("http://evil.example.net/")
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}

func TestGuardTransportPreservesExistingProxyFunc(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce", "api.example.com"), nil)
	called := false
	tr := &http.Transport{Proxy: func(r *http.Request) (*url.URL, error) { called = true; return nil, nil }}
	GuardTransport(tr)
	req, _ := http.NewRequest("GET", "https://api.example.com/", nil)
	if _, err := tr.Proxy(req); err != nil || !called {
		t.Fatalf("existing proxy func must run after the check (err=%v called=%v)", err, called)
	}
}

func TestGuardWebsocketDialerRefuses(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	d := GuardWebsocketDialer(&websocket.Dialer{}, "test")
	req, _ := http.NewRequest("GET", "https://evil.example.net/ws", nil)
	if _, err := d.Proxy(req); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}

func TestRoundTripperWrapperRefuses(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), nil)
	rt := RoundTripper(http.DefaultTransport, "test")
	req, _ := http.NewRequest("GET", "https://evil.example.net/", nil)
	if _, err := rt.RoundTrip(req); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/egress/ -run 'Guard|RoundTripper' -v`
Expected: FAIL, undefined functions.

- [ ] **Step 3: Implement `transport.go`**

```go
package egress

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// WrapProxyFunc returns a Transport.Proxy-compatible function that applies the
// egress policy to every request before delegating to next (which may be nil,
// meaning "no proxy"). Transport.Proxy is consulted on every request, which is
// what makes it the right hook: it sees the destination URL even when a
// forward proxy is configured, and it keeps *http.Transport as the concrete
// type so upstream code that type-asserts DefaultTransport keeps working.
func WrapProxyFunc(next func(*http.Request) (*url.URL, error), site string) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if err := CheckURL(req.URL, site); err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		return next(req)
	}
}

// GuardTransport installs the policy on t's Proxy function and returns t.
func GuardTransport(t *http.Transport) *http.Transport {
	if t == nil {
		return nil
	}
	t.Proxy = WrapProxyFunc(t.Proxy, "http.Transport")
	return t
}

// GuardWebsocketDialer installs the policy on d's Proxy function and returns d.
func GuardWebsocketDialer(d *websocket.Dialer, site string) *websocket.Dialer {
	if d == nil {
		return nil
	}
	d.Proxy = WrapProxyFunc(d.Proxy, site)
	return d
}

type guardedRoundTripper struct {
	next http.RoundTripper
	site string
}

func (g *guardedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := CheckURL(req.URL, g.site); err != nil {
		return nil, err
	}
	return g.next.RoundTrip(req)
}

// RoundTripper wraps a non-*http.Transport round tripper (the uTLS clients).
func RoundTripper(rt http.RoundTripper, site string) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &guardedRoundTripper{next: rt, site: site}
}

func init() {
	// Cover every client that relies on the default transport.
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		GuardTransport(t)
	}
}
```

- [ ] **Step 4: Run the transport tests**

Run: `go test ./internal/egress/ -v`
Expected: PASS.

- [ ] **Step 5: Install the hooks in upstream files**

Each edit is one line or one call. Import `"github.com/router-for-me/CLIProxyAPI/v7/internal/egress"` in every touched file.

`sdk/proxyutil/proxy.go` — wrap the two constructors so every transport the package hands out is guarded:
```go
func cloneDefaultTransport() *http.Transport {
	return egress.GuardTransport(&http.Transport{})
}

func NewDirectTransport() *http.Transport {
	// keep the existing body; wrap the returned value:
	return egress.GuardTransport(t)   // where t is the transport the body builds
}
```
Read lines 76–134 first; `BuildHTTPTransport` builds on these two, so guarding them covers `util.SetProxy` and `helps.buildProxyTransport` without touching either.

`internal/runtime/executor/antigravity_executor.go:232`:
```go
	return egress.GuardTransport(&http.Transport{})
```
(Line 229's clone of DefaultTransport is already guarded by the package `init`.)

`internal/runtime/executor/helps/utls_client.go:285`:
```go
func cachedClaudeCodeRoundTripper(proxyURL string) http.RoundTripper {
	return egress.RoundTripper(claudeCodeRoundTripperCache.GetOrAdd(proxyURL, func() http.RoundTripper {
		return newClaudeCodeRoundTripper(proxyURL)
	}), "helps.claudeCodeRoundTripper")
}
```
and at `:305` wrap the literal: `transport := egress.GuardTransport(&http.Transport{ ... })` — keep the existing fields.

`internal/auth/claude/utls_transport.go:194`: `roundTripper.transport = egress.GuardTransport(&http.Transport{ ... })` keeping fields; `:253`: `return &http.Client{Transport: egress.RoundTripper(newUtlsRoundTripper(cfg), "claude.oauthUTLS")}`.

`internal/api/handlers/management/api_tools.go` — after line 117's `parsedURL, errParseURL := url.Parse(urlStr)` and its error check:
```go
	if errEgress := egress.CheckURL(parsedURL, "management.APICall"); errEgress != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": errEgress.Error()})
		return
	}
```
and at `:521`: `return egress.GuardTransport(&http.Transport{Proxy: nil})`.

`internal/runtime/executor/codex_websockets_connection.go:160` — change the function's first statement so the dialer literal is wrapped:
```go
	dialer := egress.GuardWebsocketDialer(&websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  codexResponsesWebsocketHandshakeTO,
		EnableCompression: true,
		NetDialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}, "codex.websocket")
```
Read the rest of that function: if it later assigns `dialer.Proxy = ...` for a configured proxy, wrap that assignment too: `dialer.Proxy = egress.WrapProxyFunc(http.ProxyURL(setting.URL), "codex.websocket")` (use whatever expression the code assigns). The xAI executor reuses this function, so it is covered.

`internal/api/server.go:199` and `cmd/server/main.go:583` — directly after `managementasset.SetCurrentConfig(cfg)` add:
```go
	egress.SetConfig(cfg)
```

- [ ] **Step 6: Build both ways and run the whole suite**

Run: `make verify-tagless && make verify`
Expected: PASS. If a test in the executor packages now fails with `egress: host ... not permitted`, that test hits a real hostname with no policy installed; fix by adding at the top of that test `egress.SetConfigWithBuiltin(nil, []string{"<that host>"})` or by pointing it at `httptest` (loopback is always allowed). Do not weaken the gate.

- [ ] **Step 7: Live check against the running proxy's hosts**

Run: `go run -tags locked ./cmd/server -config /Users/arshan/.cli-proxy-api/config.yaml -v` (prints version and exits; confirms the binary links). Then build and run a throwaway instance on another port with audit mode to see the host set:
```bash
sed 's/^port: 8317/port: 8399/' /Users/arshan/.cli-proxy-api/config.yaml > /tmp/cpa-audit.yaml
printf 'egress:\n  mode: audit\n' >> /tmp/cpa-audit.yaml
timeout 40 ./dist/cliproxyapi -config /tmp/cpa-audit.yaml 2>&1 | grep -i "egress" | head
```
Expected: no `egress audit` lines during startup other than, at most, the panel/model refresh hosts (which Phase 2 removes). Any provider host in those lines must be added to `providers.go` before continuing.

- [ ] **Step 8: Commit**

```bash
git add internal/egress sdk/proxyutil/proxy.go internal/runtime/executor/antigravity_executor.go internal/runtime/executor/helps/utls_client.go internal/auth/claude/utls_transport.go internal/api/handlers/management/api_tools.go internal/runtime/executor/codex_websockets_connection.go internal/api/server.go cmd/server/main.go
git commit -m "lock: enforce the egress policy at every transport and dialer

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 6: raw-transport coverage test

**Files:**
- Create: `internal/egress/coverage_test.go`, `internal/egress/coverage_allowlist.txt`

**Interfaces:**
- Produces: nothing exported. `scripts/upstream-audit.sh` (Task 11) re-uses `coverage_allowlist.txt`.

- [ ] **Step 1: Write the allowlist**

`internal/egress/coverage_allowlist.txt`:
```
# Files permitted to construct http.Transport or websocket.Dialer literals.
# Each must guard the value (egress.GuardTransport / GuardWebsocketDialer).
sdk/proxyutil/proxy.go
internal/runtime/executor/antigravity_executor.go
internal/runtime/executor/helps/utls_client.go
internal/auth/claude/utls_transport.go
internal/api/handlers/management/api_tools.go
internal/runtime/executor/codex_websockets_connection.go
internal/pluginhost/http_bridge.go
internal/egress/transport.go
```

- [ ] **Step 2: Write the test**

`internal/egress/coverage_test.go`:
```go
package egress

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var rawTransport = regexp.MustCompile(`&?http\.Transport\{|&?websocket\.Dialer\{`)

func TestNoUnguardedTransportConstruction(t *testing.T) {
	root := repoRoot(t)
	allowed := map[string]bool{}
	f, err := os.Open(filepath.Join(root, "internal/egress/coverage_allowlist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			allowed[line] = true
		}
	}
	f.Close()
	for _, rel := range []string{"internal", "sdk", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, _ := os.ReadFile(path)
			if !rawTransport.Match(data) {
				return nil
			}
			r, _ := filepath.Rel(root, path)
			if !allowed[r] {
				t.Errorf("%s constructs a raw transport/dialer; guard it with egress and add it to coverage_allowlist.txt", r)
			}
			return nil
		})
	}
}
```

- [ ] **Step 3: Run it**

Run: `go test ./internal/egress/ -run TestNoUnguardedTransportConstruction -v`
Expected: PASS. If it reports a file, open that file: guard the construction as in Task 5 step 5 and add the path to the allowlist. `internal/pluginhost/http_bridge.go` is allowed without a guard because plugins cannot load under `locked` (Task 8); note that in the allowlist comment.

- [ ] **Step 4: Commit**

```bash
git add internal/egress/coverage_test.go internal/egress/coverage_allowlist.txt
git commit -m "lock: fail the build on unguarded transport construction

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 2: `locked` build tag

### Task 7: embedded panel and updater stub

**Files:**
- Modify: `internal/managementasset/updater.go:1` (add build tag header) and `updater_test.go:1` (same tag, it tests the updater)
- Create: `internal/managementasset/updater_locked.go`, `embedded.go`, `embedded_locked.go`, `embedded_locked_test.go`
- Create: `internal/managementasset/panel/management.html`, `internal/managementasset/panel/PANEL_VERSION`
- Modify: `internal/api/server_management.go:304-310` (`serveManagementControlPanel`)

**Interfaces:**
- Produces: `managementasset.EmbeddedPanel() ([]byte, bool)`; `managementasset.PanelVersion() string` (locked only meaningfully).
- Consumes: the reviewed v1.22.18 bundle at `/opt/homebrew/etc/static/management.html`, sha256 `f11e7f970ed474d262049b236c54f35ef59f4b9a2da5b9fe1946af14a8641553`.

- [ ] **Step 1: Copy the audited bundle in and write the pin**

```bash
mkdir -p internal/managementasset/panel
cp /opt/homebrew/etc/static/management.html internal/managementasset/panel/management.html
shasum -a 256 internal/managementasset/panel/management.html
```
Expected hash: `f11e7f970ed474d262049b236c54f35ef59f4b9a2da5b9fe1946af14a8641553`. If it differs, the file on disk changed since the audit: stop and re-audit before continuing.

`internal/managementasset/panel/PANEL_VERSION`:
```
repo=router-for-me/Cli-Proxy-API-Management-Center
tag=v1.22.18
sha256=f11e7f970ed474d262049b236c54f35ef59f4b9a2da5b9fe1946af14a8641553
source=github-release-asset
reviewed=2026-09-13
```

- [ ] **Step 2: Write the failing embed test**

`internal/managementasset/embedded_locked_test.go`:
```go
//go:build locked

package managementasset

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEmbeddedPanelMatchesPin(t *testing.T) {
	data, ok := EmbeddedPanel()
	if !ok || len(data) == 0 {
		t.Fatal("locked build must embed the panel")
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	want := ""
	for _, line := range strings.Split(panelVersionText, "\n") {
		if strings.HasPrefix(line, "sha256=") {
			want = strings.TrimPrefix(line, "sha256=")
		}
	}
	if want == "" || got != want {
		t.Fatalf("embedded panel sha256 %s, PANEL_VERSION says %q", got, want)
	}
}
```

- [ ] **Step 3: Run to confirm failure**

Run: `go test -tags locked ./internal/managementasset/ -run TestEmbeddedPanel -v`
Expected: FAIL, undefined `EmbeddedPanel` / `panelVersionText`.

- [ ] **Step 4: Write the embed files**

`internal/managementasset/embedded.go`:
```go
//go:build !locked

package managementasset

// EmbeddedPanel reports no embedded panel in the default build; the runtime
// updater in updater.go supplies the asset instead.
func EmbeddedPanel() ([]byte, bool) { return nil, false }

// PanelVersion is empty in the default build.
func PanelVersion() string { return "" }
```
`internal/managementasset/embedded_locked.go`:
```go
//go:build locked

package managementasset

import _ "embed"

//go:embed panel/management.html
var embeddedPanel []byte

//go:embed panel/PANEL_VERSION
var panelVersionText string

// EmbeddedPanel returns the reviewed panel bundle compiled into this binary.
func EmbeddedPanel() ([]byte, bool) { return embeddedPanel, len(embeddedPanel) > 0 }

// PanelVersion returns the PANEL_VERSION pin text.
func PanelVersion() string { return panelVersionText }
```

- [ ] **Step 5: Tag the updater out and write its stub**

Add as the first line of `internal/managementasset/updater.go` and `updater_test.go` (followed by a blank line):
```go
//go:build !locked
```
`internal/managementasset/updater_locked.go`:
```go
//go:build locked

package managementasset

import (
	"context"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// ManagementFileName exposes the control panel asset filename.
const ManagementFileName = "management.html"

// SetCurrentConfig is retained for call-site compatibility; the locked build
// never syncs the panel so the config is not needed.
func SetCurrentConfig(_ *config.Config) {}

// StartAutoUpdater is a no-op: the locked build embeds a reviewed panel.
func StartAutoUpdater(_ context.Context, _ string) {}

// StaticDir mirrors the default build's path derivation so callers that only
// display the path keep working. Nothing is written there.
func StaticDir(configFilePath string) string { return staticDirFor(configFilePath) }

// FilePath returns where the default build would keep the asset.
func FilePath(configFilePath string) string { return staticDirFor(configFilePath) + "/" + ManagementFileName }

// EnsureLatestManagementHTML never downloads; it reports whether the embedded
// panel exists.
func EnsureLatestManagementHTML(_ context.Context, _ string, _ string, _ string) bool {
	_, ok := EmbeddedPanel()
	return ok
}
```
`staticDirFor` must reproduce the default build's `StaticDir` logic. Read `updater.go:143-172`, copy that function body into a new file `internal/managementasset/static_dir.go` (no build tag) as `func staticDirFor(configFilePath string) string`, and make the tagless `StaticDir` call it too, so the logic exists once.

- [ ] **Step 6: Serve the embedded panel**

In `internal/api/server_management.go`, after the `DisableControlPanel` check (line 309) and before `filePath := ...`:
```go
	if data, ok := managementasset.EmbeddedPanel(); ok {
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		return
	}
```

- [ ] **Step 7: Verify both builds**

Run: `make verify-tagless && make verify`
Expected: PASS, including `TestEmbeddedPanelMatchesPin` under the tag.

- [ ] **Step 8: Commit**

```bash
git add internal/managementasset internal/api/server_management.go
git commit -m "lock: embed the reviewed panel; no runtime updater under the tag

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 8: plugin loader and store lockout

**Files:**
- Modify: `internal/pluginhost/loader_unix.go:1` and `loader_windows.go:1` (extend build tags with `&& !locked`)
- Create: `internal/pluginhost/loader_locked.go`, `internal/pluginhost/loader_locked_test.go`
- Modify: `internal/api/handlers/management/plugin_store.go:132` and `:224` (early returns)
- Create: `internal/api/handlers/management/plugin_store_locked_test.go`

**Interfaces:**
- Consumes: `lockedbuild.Enabled`; `pluginLoader` interface, `pluginFile`, `pluginClient`, `Host` from `internal/pluginhost` (read `loader_unsupported.go` for the shape of a minimal loader).

- [ ] **Step 1: Write the failing loader test**

`internal/pluginhost/loader_locked_test.go`:
```go
//go:build locked

package pluginhost

import (
	"strings"
	"testing"
)

func TestLockedLoaderRefusesToOpen(t *testing.T) {
	loader := defaultPluginLoader()
	_, err := loader.Open(pluginFile{Path: "/tmp/x.dylib"}, nil)
	if err == nil || !strings.Contains(err.Error(), "locked build") {
		t.Fatalf("want locked-build error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test -tags locked ./internal/pluginhost/ -run TestLockedLoader -v`
Expected: FAIL (either compile error from two `defaultPluginLoader` definitions, or the unix loader opening the file). Both mean the tag isn't wired yet.

- [ ] **Step 3: Adjust tags and write the locked loader**

`loader_unix.go` line 1 becomes `//go:build cgo && (linux || darwin || freebsd) && !locked`.
`loader_windows.go` line 1 becomes `//go:build windows && !locked`.
Check `loader_unsupported.go` (`!cgo && !windows`): it must not also compile under `locked` on a cgo build; it won't, because `cgo` is true on your macOS builds. Leave it.

`internal/pluginhost/loader_locked.go`:
```go
//go:build locked

package pluginhost

import "errors"

// lockedLoader refuses to load any native plugin.
type lockedLoader struct{}

func defaultPluginLoader() pluginLoader { return lockedLoader{} }

func (lockedLoader) Open(file pluginFile, _ *Host) (pluginClient, error) {
	return nil, errors.New("pluginhost: native plugin loading is disabled in the locked build: " + file.Path)
}
```
If `pluginLoader` has methods beyond `Open`, copy their signatures from `loader_unsupported.go` and return the same error.

- [ ] **Step 4: Gate the store handlers**

In `internal/api/handlers/management/plugin_store.go`, first lines inside `ListPluginStore` (132) and `InstallPluginFromStore` (224):
```go
	if lockedbuild.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "plugin store is disabled in the locked build"})
		return
	}
```
Import `"github.com/router-for-me/CLIProxyAPI/v7/internal/lockedbuild"`.

`internal/api/handlers/management/plugin_store_locked_test.go`:
```go
//go:build locked

package management

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPluginStoreRefusedUnderLocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	for _, fn := range []func(*gin.Context){h.ListPluginStore, h.InstallPluginFromStore} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		fn(c)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	}
}
```
If `Handler` cannot be zero-constructed (nil-pointer panics before the guard), the guard is not first in the function; move it.

- [ ] **Step 5: Verify both builds**

Run: `make verify-tagless && make verify`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pluginhost internal/api/handlers/management/plugin_store.go internal/api/handlers/management/plugin_store_locked_test.go
git commit -m "lock: refuse native plugin loading and store installs under the tag

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 9: model catalog: no remote refresh, pinned snapshot

**Files:**
- Create: `internal/registry/model_sources.go`, `internal/registry/model_sources_locked.go`, `internal/registry/model_sources_locked_test.go`
- Modify: `internal/registry/model_updater.go:20-25` (remove the URL slice literal) and `:77-81` (`StartModelsUpdater` early return)
- Modify: `internal/registry/codex_client_models_updater.go:14-18` and `:25-29` (same two edits)
- Create: `internal/registry/models/MODELS_VERSION`
- Create: `scripts/refresh-models.sh`; Modify: `Makefile` (add `refresh-models`)

**Interfaces:**
- Produces: package-level `var modelsURLs []string`, `var codexClientModelsURLs []string`, `const remoteModelRefreshEnabled bool`, defined once per build variant. `make refresh-models`.

- [ ] **Step 1: Move the URL slices into a tagless-variant file**

Read `model_updater.go:20-25` and `codex_client_models_updater.go:14-18`; the exact slice names are whatever those files use (`modelsURLs` / `codexClientModelsURLs` or similar). Cut both literals out and put them, unchanged, in `internal/registry/model_sources.go`:
```go
//go:build !locked

package registry

// Remote catalog sources. Under the locked tag these are empty and refresh is off.
const remoteModelRefreshEnabled = true

var modelsURLs = []string{
	"https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/models.json",
	"https://models.router-for.me/models.json",
}

var codexClientModelsURLs = []string{
	"https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/codex_client_models.json",
	"https://models.router-for.me/codex_client_models.json",
}
```
`internal/registry/model_sources_locked.go`:
```go
//go:build locked

package registry

// The locked build never fetches catalogs; it serves the embedded snapshot
// pinned in models/MODELS_VERSION.
const remoteModelRefreshEnabled = false

var modelsURLs []string

var codexClientModelsURLs []string
```

- [ ] **Step 2: Early-return in both Start functions**

`StartModelsUpdater` and `StartCodexClientModelsUpdater` each get, as their first statement:
```go
	if !remoteModelRefreshEnabled {
		log.Info("locked build: remote model catalog refresh disabled; using embedded snapshot")
		return
	}
```

- [ ] **Step 3: Write the locked test**

`internal/registry/model_sources_locked_test.go`:
```go
//go:build locked

package registry

import (
	"context"
	"testing"
)

func TestLockedBuildHasNoRemoteModelSources(t *testing.T) {
	if remoteModelRefreshEnabled || len(modelsURLs) != 0 || len(codexClientModelsURLs) != 0 {
		t.Fatal("locked build must not have remote model sources")
	}
	// Must return immediately without starting a goroutine that fetches.
	StartModelsUpdater(context.Background())
	StartCodexClientModelsUpdater(context.Background())
}
```

- [ ] **Step 4: Write the pin and the refresh script**

`internal/registry/models/MODELS_VERSION` (compute the hashes):
```bash
M=$(shasum -a 256 internal/registry/models/models.json | cut -d' ' -f1)
C=$(shasum -a 256 internal/registry/models/codex_client_models.json | cut -d' ' -f1)
cat > internal/registry/models/MODELS_VERSION <<EOF
source=https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/
models.json.sha256=$M
codex_client_models.json.sha256=$C
snapshot-from=upstream v7.2.154 embedded catalogs
reviewed=2026-09-13
EOF
```

`scripts/refresh-models.sh`:
```bash
#!/usr/bin/env bash
# Fetch the upstream model catalogs, show the diff against the embedded
# snapshot, and only overwrite after an explicit yes.
set -euo pipefail
BASE=https://raw.githubusercontent.com/router-for-me/models/refs/heads/main
DIR=internal/registry/models
TMP=$(mktemp -d)
for f in models.json codex_client_models.json; do
  curl -fsSL "$BASE/$f" -o "$TMP/$f"
  python3 -m json.tool "$TMP/$f" >/dev/null || { echo "$f is not valid JSON; aborting"; exit 1; }
  echo "=== diff $f (embedded → remote) ==="
  diff -u "$DIR/$f" "$TMP/$f" || true
done
read -r -p "Apply these catalogs to the embedded snapshot? [y/N] " ans
[ "$ans" = "y" ] || { echo "aborted"; exit 1; }
cp "$TMP"/models.json "$TMP"/codex_client_models.json "$DIR/"
M=$(shasum -a 256 "$DIR/models.json" | cut -d' ' -f1)
C=$(shasum -a 256 "$DIR/codex_client_models.json" | cut -d' ' -f1)
{
  echo "source=$BASE/"
  echo "models.json.sha256=$M"
  echo "codex_client_models.json.sha256=$C"
  echo "snapshot-from=refresh-models $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "reviewed=$(date -u +%Y-%m-%d)"
} > "$DIR/MODELS_VERSION"
echo "updated; review with: git diff $DIR"
```
`chmod +x scripts/refresh-models.sh`. Makefile target:
```makefile
.PHONY: refresh-models
refresh-models:
	./scripts/refresh-models.sh
```

- [ ] **Step 5: Verify both builds**

Run: `make verify-tagless && make verify`
Expected: PASS. Tagless tests that exercise the remote fetch must still pass because the slices are unchanged there.

- [ ] **Step 6: Commit**

```bash
git add internal/registry scripts/refresh-models.sh Makefile
git commit -m "lock: serve the embedded model snapshot; reviewed refresh only

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 10: string-table test and startup wiring check

**Files:**
- Create: `internal/lockedbuild/strings_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build locked

package lockedbuild

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// forbidden are byte strings that must not survive into a locked binary.
var forbidden = []string{
	"api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center",
	"cpamc.router-for.me",
	"models.router-for.me",
	"raw.githubusercontent.com/router-for-me/models",
}

func TestLockedBinaryHasNoForbiddenHosts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the server binary")
	}
	root, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	out := filepath.Join(t.TempDir(), "cliproxyapi")
	cmd := exec.Command("go", "build", "-tags", "locked", "-o", out, "./cmd/server")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, b)
	}
	bin, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range forbidden {
		if bytes.Contains(bin, []byte(s)) {
			t.Errorf("locked binary still contains %q", s)
		}
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test -tags locked ./internal/lockedbuild/ -run TestLockedBinaryHasNoForbiddenHosts -v`
Expected: PASS. If a host is reported, `grep -rn "<host>" --include='*.go' internal sdk cmd | grep -v _test` finds the file that still compiles it in; move that literal behind the tag as in Tasks 7 and 9. `known-unregistered.txt` and the audit script are not Go and do not count.

- [ ] **Step 3: Commit**

```bash
git add internal/lockedbuild/strings_test.go
git commit -m "lock: assert the locked binary carries no update or catalog hosts

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 3: workflow, documentation, agent support

### Task 11: patch register and upstream audit script

**Files:**
- Create: `scripts/patches.sh`, `docs/fork/PATCHES.yaml`, `docs/fork/PATCHES.md`
- Create: `scripts/upstream-audit.sh`, `docs/fork/audits/.gitkeep`
- Modify: `Makefile` (add `patches`, `upstream-audit`, `rebase`)

**Interfaces:**
- Produces: `make patches`, `make upstream-audit TAG=`, `make rebase TAG=`; audit reports at `docs/fork/audits/<TAG>.md`.

- [ ] **Step 1: Write `docs/fork/PATCHES.yaml` by hand from the current stack**

List every commit in `git log --reverse --format='%h %s' $(git describe --tags --abbrev=0 --match 'v[0-9]*')..HEAD` that is not a merge commit. Entries:
```yaml
# Hand-maintained register of carried patches. `make patches` renders PATCHES.md.
# layer: lock | feat | fix | pick
# drop: the condition under which the patch is removed from the stack
- subject: "keepalive: add session-keyed prompt-cache probe scheduler"
  layer: feat
  upstream_pr: 5446
  drop: "upstream merges #5446 or ships equivalent keepalive"
- subject: "cache-stats: bounded per-session prompt-cache statistics store"
  layer: feat
  upstream_pr: 5445
  drop: "upstream merges #5445"
- subject: "fix(claude): treat the pinned Claude Code version as a floor, not a lock"
  layer: fix
  upstream_pr: 5444
  drop: "upstream merges #5444"
- subject: "fix(executor): recover truncated streamed tool calls"
  layer: pick
  upstream_pr: 5338
  drop: "upstream merges #5338"
```
Continue for every commit. Group the many `keepalive:` / `cache-stats:` commits under one entry each by listing `subjects:` as a list instead of `subject:` where they belong to one feature; `patches.sh` matches on subject prefix. The `lock:` commits from this plan get `upstream_pr: null`, `drop: never`.

- [ ] **Step 2: Write `scripts/patches.sh`**

```bash
#!/usr/bin/env bash
# Render docs/fork/PATCHES.md from the commit stack and PATCHES.yaml annotations.
set -euo pipefail
BASE=$(git describe --tags --abbrev=0 --match 'v[0-9]*')
OUT=docs/fork/PATCHES.md
{
  echo "# Carried patches"
  echo
  echo "Base upstream tag: \`$BASE\`. Generated by \`make patches\` on $(date -u +%Y-%m-%d). Annotations live in PATCHES.yaml."
  echo
  echo "| Commit | Layer | Subject | Upstream PR | Drop when |"
  echo "|---|---|---|---|---|"
  git log --reverse --no-merges --format='%h%x09%s' "$BASE"..HEAD | while IFS=$'\t' read -r hash subject; do
    layer=$(printf '%s' "$subject" | sed -E 's/^(lock|feat|fix|pick)[:(].*/\1/; t; s/.*/?/')
    pr=$(python3 - "$subject" <<'PY'
import sys, yaml
subject = sys.argv[1]
for e in yaml.safe_load(open('docs/fork/PATCHES.yaml')) or []:
    subs = e.get('subjects') or [e.get('subject')]
    if any(subject.startswith(s.split(':')[0]) and (s == subject or s.split(':')[0] in subject) for s in subs if s):
        print(f"{e.get('upstream_pr') or ''}\t{e.get('drop','')}"); break
else:
    print("\t(unannotated)")
PY
)
    prnum=${pr%%$'\t'*}; drop=${pr#*$'\t'}
    link=""; [ -n "$prnum" ] && link="[#$prnum](https://github.com/router-for-me/CLIProxyAPI/pull/$prnum)"
    echo "| $hash | $layer | $subject | $link | $drop |"
  done
} > "$OUT"
echo "wrote $OUT"; grep -c unannotated "$OUT" | sed 's/^/unannotated rows: /'
```
`chmod +x scripts/patches.sh`. Requires PyYAML: `python3 -c 'import yaml'`; if missing, `python3 -m pip install --user pyyaml`.

- [ ] **Step 3: Write `scripts/upstream-audit.sh`**

```bash
#!/usr/bin/env bash
# Audit an upstream tag before adopting it. Writes docs/fork/audits/<TAG>.md.
# Read-only: fetches tags and GitHub metadata, never pushes, never checks out.
set -euo pipefail
TAG="${1:?usage: upstream-audit.sh vX.Y.Z}"
BASE=$(git describe --tags --abbrev=0 --match 'v[0-9]*')
OUT="docs/fork/audits/$TAG.md"
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null || { echo "tag $TAG not found; run make upstream-fetch"; exit 1; }
WT=$(mktemp -d); git worktree add -q "$WT" "$TAG"; trap 'git worktree remove --force "$WT"' EXIT
TOP5=$(gh api 'repos/router-for-me/CLIProxyAPI/contributors?per_page=5' --jq '.[].login' 2>/dev/null | tr '\n' '|' | sed 's/|$//')

{
echo "# Upstream audit: $BASE → $TAG"
echo; echo "Generated $(date -u +%Y-%m-%dT%H:%M:%SZ). Base=\`$BASE\` Target=\`$TAG\`."
echo; echo "## 1. Range"
echo "Commits: $(git rev-list --count "$BASE".."$TAG")"; echo
git log --format='%an' "$BASE".."$TAG" | sort | uniq -c | sort -rn | sed 's/^/    /'
echo; echo "## 2. Dependencies"; echo '```diff'
git diff "$BASE" "$TAG" -- go.mod go.sum | head -200; echo '```'
echo; echo "govulncheck:"; echo '```'
(cd "$WT" && (govulncheck ./... 2>&1 || true) | tail -30); echo '```'
echo; echo "## 3. Commits (GPG status; [review] = author outside top-5 contributors)"; echo
git log --format='%h %G? %an %s' "$BASE".."$TAG" | while read -r h g a rest; do
  mark=""; [[ "$a" =~ ^($TOP5)$ ]] || mark=" [review]"
  echo "- \`$h\` $g $a$mark — $rest"
done
echo; echo "## 4. Added-line grep (non-test Go)"; echo '```'
git diff "$BASE" "$TAG" -- '*.go' ':!*_test.go' | grep -nE '^\+' | grep -E 'os/exec|syscall\.|dlopen|os\.WriteFile|os\.Create\(|func init\(\)|go:embed|base64\.|"(https|wss)://' | head -120; echo '```'
echo; echo "## 5. Watched paths (full diff)"
for p in internal/managementasset internal/pluginstore internal/pluginhost internal/registry internal/home; do
  echo; echo "### $p"; echo '```diff'; git diff "$BASE" "$TAG" -- "$p" | head -400; echo '```'
done
echo; echo '### files containing $TOKEN$'; echo '```'
(cd "$WT" && grep -rln '\$TOKEN\$' --include='*.go' internal sdk 2>/dev/null || true); echo '```'
echo; echo "## 6. Pins"
echo "- panel pin: $(grep '^tag=' internal/managementasset/panel/PANEL_VERSION)"
echo "- panel latest: $(gh release view --repo router-for-me/Cli-Proxy-API-Management-Center --json tagName --jq .tagName 2>/dev/null || echo unknown)"
echo "- models snapshot: $(grep '^reviewed=' internal/registry/models/MODELS_VERSION)"
echo; echo "## 7. New raw transport/dialer sites at $TAG"; echo '```'
(cd "$WT" && grep -rlE '&?http\.Transport\{|&?websocket\.Dialer\{' --include='*.go' internal sdk cmd | grep -v _test | sort) > "$WT/.sites"
grep -v '^#' internal/egress/coverage_allowlist.txt | sort > "$WT/.allowed"
comm -23 "$WT/.sites" "$WT/.allowed" || true; echo '```'
echo; echo "## 8. Checklist"
echo "- [ ] Read every [review] commit and every watched-path diff"
echo "- [ ] Dependency changes accepted (or none)"
echo "- [ ] Panel pin decision recorded (keep / bump via make panel-bump)"
echo "- [ ] Models snapshot decision recorded (keep / make refresh-models)"
echo "- [ ] New raw transport sites guarded and allowlisted (or none)"
echo "- [ ] After install: one Claude Code and one Codex session in egress audit mode showed no unexpected hosts"
} > "$OUT"
echo "wrote $OUT"
```
`chmod +x scripts/upstream-audit.sh`. Requires `govulncheck` (`go install golang.org/x/vuln/cmd/govulncheck@latest`) and `gh`.

- [ ] **Step 4: Add the Makefile targets**

```makefile
.PHONY: patches upstream-audit rebase
patches:
	./scripts/patches.sh

upstream-audit:
	@[ -n "$(TAG)" ] || { echo "usage: make upstream-audit TAG=vX.Y.Z"; exit 1; }
	./scripts/upstream-audit.sh $(TAG)

rebase:
	@[ -n "$(TAG)" ] || { echo "usage: make rebase TAG=vX.Y.Z"; exit 1; }
	@[ -f docs/fork/audits/$(TAG).md ] || { echo "no audit report for $(TAG); run make upstream-audit TAG=$(TAG)"; exit 1; }
	@! grep -q '^- \[ \]' docs/fork/audits/$(TAG).md || { echo "audit checklist for $(TAG) has unticked items"; exit 1; }
	git config rerere.enabled true
	git branch -f locked-prev HEAD
	git rebase --onto $(TAG) $$(git describe --tags --abbrev=0 --match 'v[0-9]*') locked
	@echo "rebased onto $(TAG); now: make verify"
```
Note: the audit-mode checklist item can only be ticked after install, so for a real adoption tick it after step 6 of Task 14 and re-run nothing; the gate is meant to make skipping visible, not to block the rebase forever. Document this in the report header line.

- [ ] **Step 5: Run the generators once**

Run: `make patches && ./scripts/upstream-audit.sh v7.2.154 && head -30 docs/fork/audits/v7.2.154.md`
Expected: `PATCHES.md` renders with zero `(unannotated)` rows; the v7.2.154 audit is an empty-range smoke test (0 commits) proving the script runs end to end. Delete `docs/fork/audits/v7.2.154.md` afterwards; keep `.gitkeep`.

- [ ] **Step 6: Commit**

```bash
git add scripts/patches.sh scripts/upstream-audit.sh docs/fork/PATCHES.yaml docs/fork/PATCHES.md docs/fork/audits/.gitkeep Makefile
git commit -m "lock: patch register and upstream audit workflow

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 12: panel build, verify, and bump

**Files:**
- Create: `scripts/panel.sh`
- Modify: `Makefile` (add `panel-build`, `panel-verify`, `panel-bump`)

- [ ] **Step 1: Write `scripts/panel.sh`**

```bash
#!/usr/bin/env bash
# build   : build the pinned panel tag from source into internal/managementasset/panel
# verify  : build the pinned tag to a temp dir and compare with the GitHub release asset
# bump T  : show the source diff pinned→T, ask, then build T and update the pin
set -euo pipefail
REPO=router-for-me/Cli-Proxy-API-Management-Center
DIR=internal/managementasset/panel
PIN="$DIR/PANEL_VERSION"
pinned_tag() { grep '^tag=' "$PIN" | cut -d= -f2; }
build_tag() { # $1 tag, $2 out-dir
  local tag="$1" out="$2" src; src=$(mktemp -d)
  git clone -q --depth 1 --branch "$tag" "https://github.com/$REPO.git" "$src"
  (cd "$src" && bun install --frozen-lockfile >/dev/null && VERSION="$tag" bun run build >/dev/null)
  cp "$src/dist/index.html" "$out/management.html"
  (cd "$src" && git rev-parse HEAD) > "$out/.commit"
}
write_pin() { # $1 tag, $2 commit, $3 source
  local sum; sum=$(shasum -a 256 "$DIR/management.html" | cut -d' ' -f1)
  printf 'repo=%s\ntag=%s\ncommit=%s\nsha256=%s\nsource=%s\nreviewed=%s\n' "$REPO" "$1" "$2" "$sum" "$3" "$(date -u +%Y-%m-%d)" > "$PIN"
}
case "${1:-}" in
  build)
    tag=$(pinned_tag); tmp=$(mktemp -d); build_tag "$tag" "$tmp"
    cp "$tmp/management.html" "$DIR/management.html"; write_pin "$tag" "$(cat "$tmp/.commit")" "local-build"
    echo "built $tag → $DIR"; grep sha256 "$PIN" ;;
  verify)
    tag=$(pinned_tag); tmp=$(mktemp -d); build_tag "$tag" "$tmp"
    local_sum=$(shasum -a 256 "$tmp/management.html" | cut -d' ' -f1)
    gh release download "$tag" --repo "$REPO" --pattern management.html --dir "$tmp/rel" >/dev/null
    rel_sum=$(shasum -a 256 "$tmp/rel/management.html" | cut -d' ' -f1)
    echo "local build : $local_sum"; echo "release     : $rel_sum"
    [ "$local_sum" = "$rel_sum" ] && echo "PASS: release matches source build" || echo "MISMATCH: investigate before trusting release assets" ;;
  bump)
    new="${2:?usage: panel.sh bump vX.Y.Z}"; old=$(pinned_tag); src=$(mktemp -d)
    git clone -q "https://github.com/$REPO.git" "$src"
    echo "=== commits $old..$new ==="; (cd "$src" && git log --format='%h %an %s' "$old".."$new")
    echo; echo "=== files ==="; (cd "$src" && git diff --stat "$old".."$new" | tail -40)
    echo; echo "=== risky primitives in added lines ==="
    (cd "$src" && git diff "$old".."$new" -- 'src/*' | grep -E '^\+' | grep -nE 'eval\(|new Function|fetch\(|XMLHttpRequest|WebSocket|sendBeacon|postMessage|localStorage|document\.cookie|import\(|"https?://' || echo "(none)")
    read -r -p "Build $new and update the pin? [y/N] " ans; [ "$ans" = "y" ] || { echo aborted; exit 1; }
    tmp=$(mktemp -d); build_tag "$new" "$tmp"; cp "$tmp/management.html" "$DIR/management.html"; write_pin "$new" "$(cat "$tmp/.commit")" "local-build"
    echo "pinned $new"; grep sha256 "$PIN" ;;
  *) echo "usage: panel.sh build|verify|bump TAG"; exit 1 ;;
esac
```
`chmod +x scripts/panel.sh`. Makefile:
```makefile
.PHONY: panel-build panel-verify panel-bump
panel-build:
	./scripts/panel.sh build
panel-verify:
	./scripts/panel.sh verify
panel-bump:
	@[ -n "$(TAG)" ] || { echo "usage: make panel-bump TAG=vX.Y.Z"; exit 1; }
	./scripts/panel.sh bump $(TAG)
```

- [ ] **Step 2: Run verify against the current pin**

Run: `command -v bun || echo "install bun: brew install oven-sh/bun/bun"; make panel-verify`
Expected: prints both hashes. PASS means the v1.22.18 release is reproducible from source; record the outcome in the commit message. MISMATCH is a finding to report to the user, not a blocker for this task.

- [ ] **Step 3: Commit**

```bash
git add scripts/panel.sh Makefile
git commit -m "lock: panel build, verify, and bump workflow

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 13: README, CLAUDE.md, and the fork-maintenance skill

**Files:**
- Create: `docs/UPSTREAM-README.md` (verbatim copy of current `README.md`)
- Replace: `README.md`
- Replace: `CLAUDE.md` (currently `@AGENTS.md`); leave `AGENTS.md` as upstream's
- Create: `.claude/skills/fork-maintenance/SKILL.md`

- [ ] **Step 1: Preserve upstream's README and write ours**

```bash
git mv README.md docs/UPSTREAM-README.md
```
`README.md`:
```markdown
# CLIProxyAPI — locked build

A privacy-locked fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI).
It proxies Claude Code, Codex, and other CLIs through OAuth accounts the way upstream does,
but the binary cannot download or execute anything it did not ship with, and it can only
open outbound connections to hosts it has a reason to talk to.

## Why

The proxy holds OAuth tokens for every connected account and a management key that can read
them all. Upstream's binary phones home on its own: it replaces its web panel from a GitHub
release feed every three hours, installs native plugins from GitHub, and refreshes its model
catalog from a maintainer-controlled URL. None of that is verified beyond a same-origin
checksum. This fork removes those paths at build time and gates every remaining connection.

## What is different

| Area | Upstream | This fork (`-tags locked`) |
|---|---|---|
| Web panel | downloaded at runtime, auto-updated | reviewed build embedded in the binary; see `internal/managementasset/panel/PANEL_VERSION` |
| Plugins | native `.dylib`/`.so` loaded with `dlopen`, installed from GitHub | loader refuses; store endpoints return 403 |
| Model catalog | fetched from `models.router-for.me` every 3h | embedded snapshot; `make refresh-models` shows a diff before updating |
| Outbound hosts | anything the code asks for | allowlist: provider hosts + configured `base-url`s + loopback + `egress.extra-allow` |
| Claude Code cache | — | keepalive probing, per-session cache stats, `/cache.html` watch page, TUI Cache tab |

Everything else (executors, auth flows, management API, TUI) is upstream code at the pinned tag.

## Egress policy

```yaml
egress:
  mode: enforce        # or "audit" to log instead of refuse (use after adopting a new tag)
  extra-allow:         # hostnames beyond provider hosts and configured base-urls
    - my-oauth-callback.example
```
A refused request fails with `egress: host "x" not permitted (site=..., mode=enforce)` and one
error log line. Provider hosts are listed in `internal/egress/providers.go`.

## Build and install

```
make build      # go build -tags locked → dist/cliproxyapi
make verify     # build, vet, tests, coverage tests
make install    # swap into /opt/homebrew/opt/cliproxyapi/bin, restart, health-check
make rollback   # restore the previous binary
```

## Adopting an upstream tag

```
make upstream-fetch
make upstream-audit TAG=v7.2.170     # writes docs/fork/audits/v7.2.170.md — read it
make rebase TAG=v7.2.170             # refuses without a completed audit
make verify && make install
git tag v7.2.170-locked.1 && git push origin locked --tags
```
Carried patches are listed in `docs/fork/PATCHES.md`. Design: `docs/fork/specs/`.

## Upstream

Upstream's README is preserved at `docs/UPSTREAM-README.md`. This fork never pushes to upstream;
fixes that belong there are opened as PRs from this repo.
```

- [ ] **Step 2: Write `CLAUDE.md`**

```markdown
# CLAUDE.md — locked fork of CLIProxyAPI

This repository is a fork of router-for-me/CLIProxyAPI, maintained as a privacy-locked build.
Read `README.md` for what that means and `docs/fork/specs/2026-09-13-locked-fork-design.md` for why.
Upstream's own agent notes are in `AGENTS.md`; they describe the codebase layout and still apply.

## Rules that override everything else
- Never push to the `upstream` remote. Its push URL is `no_push`; do not change that.
- Always build and test with `-tags locked` (`make verify`). Also run `make verify-tagless` before committing Go changes.
- Prefer a `_locked.go` sibling or the egress gate over editing an upstream file. When an upstream edit is unavoidable, keep it to a build-tag line, an early return, or one hook call.
- Every carried change gets a `docs/fork/PATCHES.yaml` entry; run `make patches` after adding one.
- Commit subjects use a layer prefix: `lock:` (lockdown/tooling/docs), `feat:` (fork features), `fix:` (mirrors an open upstream PR), `pick:` (cherry-picked upstream PR).
- Never adopt an upstream tag without `make upstream-audit TAG=` and a completed checklist in `docs/fork/audits/`.
- Do not add hosts to `internal/egress/providers.go` to make a test pass; understand why the code contacts that host first.

## Where things live
- Egress gate: `internal/egress` (policy, provider hosts, transport hooks, coverage tests)
- Locked stubs: any `*_locked.go`; `internal/lockedbuild.Enabled` is the runtime switch
- Embedded panel and pin: `internal/managementasset/panel/`
- Model snapshot and pin: `internal/registry/models/`
- Workflow: `Makefile`, `scripts/`
- Fork features: `internal/runtime/keepalive`, `internal/runtime/cachestats`, `internal/api/cachewatch`, `internal/tui/cache_tab.go`

## Workflows
Use the `fork-maintenance` skill (`/fork-maintenance audit-tag|adopt-tag|carry-patch`) for the recurring tasks.
```

- [ ] **Step 3: Write the skill**

`.claude/skills/fork-maintenance/SKILL.md`:
```markdown
---
name: fork-maintenance
description: Use for any work on this locked CLIProxyAPI fork — auditing or adopting an upstream tag, adding/updating/dropping a carried patch, bumping the panel or model pins. Encodes the fork's rules so they never need re-explaining.
---

# Fork maintenance

Read `CLAUDE.md` first. This fork = upstream tag + a linear stack of `lock:`/`feat:`/`fix:`/`pick:` commits on branch `locked`. The build is always `-tags locked`.

## audit-tag TAG
1. `make upstream-fetch` and confirm TAG exists: `git tag -l TAG`.
2. `make upstream-audit TAG=TAG`; open `docs/fork/audits/TAG.md`.
3. Read every `[review]` commit and every watched-path diff in full. For each, write one line in the report under a `## Notes` heading: what it does, whether it touches credentials, egress, or the panel/plugin/registry paths, and accept/reject.
4. Decide panel and models pins (sections 6): run `make panel-bump TAG=` or `make refresh-models` only after reading their diffs.
5. Tick the checklist items you completed. Leave the post-install item unticked until adopt-tag step 6.
6. Report to the user: commits flagged, dependency changes, pin decisions, anything rejected.

## adopt-tag TAG
1. Requires a completed audit (`make rebase` checks). `git status` must be clean.
2. `make rebase TAG=TAG`. Resolve conflicts one patch at a time; the conflicting patch's `PATCHES.yaml` entry says what it is for. Never resolve by dropping fork behaviour silently — if a patch is obsolete, drop it deliberately and update `PATCHES.yaml`.
3. `make verify-tagless && make verify`. Fix registration/coverage test failures by understanding the new host or client site, per CLAUDE.md.
4. `make patches`; commit the regenerated `PATCHES.md`.
5. `make install`; then set `egress.mode: audit` in `~/.cli-proxy-api/config.yaml`, run one Claude Code session and one Codex session, `grep 'egress audit' ~/.cli-proxy-api/logs/main.log`, and set the mode back to `enforce`.
6. Tick the last checklist item in the audit report, commit it, tag `TAG-locked.1`, `git push origin locked --tags`.
7. Report: what conflicted, what was dropped, audit-mode result, the new tag.

## carry-patch (add | update | drop) [PR]
- add: commit with the right prefix; add a `PATCHES.yaml` entry with `upstream_pr` and `drop`; if it fixes upstream behaviour, open the upstream PR from `origin` and record its number.
- update: `gh pr view PR --repo router-for-me/CLIProxyAPI --json state,mergedAt`; if merged, go to drop.
- drop: `git rebase -i` is not available; use `git rebase --onto <parent-of-patch> <patch> locked` after moving `locked-prev`, remove the YAML entry, `make patches`, `make verify`.
Always finish with `make verify` and a one-paragraph report.
```

- [ ] **Step 4: Verify and commit**

Run: `make verify` (docs don't affect it, but `git mv` of README must not break `go:embed` or tests referencing README; it does not).
```bash
git add README.md docs/UPSTREAM-README.md CLAUDE.md .claude/skills/fork-maintenance/SKILL.md
git commit -m "lock: fork README, CLAUDE.md, and the fork-maintenance skill

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

### Task 14: first adoption — v7.2.159 (or newest), install, audit-mode run, tag

**Files:** the whole tree; `docs/fork/audits/<TAG>.md`; `docs/fork/PATCHES.md`.

**Interfaces:** consumes every Make target above.

- [ ] **Step 1: Fetch and pick the tag**

Run: `make upstream-fetch` → note the newest `v7.2.*` tag (v7.2.159 at planning time; use the newest).

- [ ] **Step 2: Audit it**

Run: `make upstream-audit TAG=<TAG>`, then follow the skill's `audit-tag` steps 3–6. Known items from planning that will appear: `b192f655` (plugin quota probe with `$TOKEN$`, watched path), `fc96a87f` (credential filename migration), `1d5f7b2a` (quota deadline wall-clock fix, wanted), `456d4c37` (cooldown clearing on credential change, wanted), the selector rewrite. Record accept/reject notes for each.

- [ ] **Step 3: Rebase**

Run: `make rebase TAG=<TAG>`. Expected conflicts (from the dry run at planning): `claude_executor_execute.go`, `claude_executor_stream.go`, `codex_websockets_request.go`, `helps/usage_helpers.go`, `signature/claude_validation.go`, `signature/provider_compatibility.go`. Resolve each keeping both upstream's new behaviour and the fork's, guided by the commit's `PATCHES.yaml` purpose. If upstream's `6a73f396` (subagent 1h cache TTL) makes the fork's `claude: never reshape a confirmed native client's cache ttl or betas` redundant, drop that patch deliberately and note it.

- [ ] **Step 4: Verify**

Run: `make verify-tagless && make verify`. The new `plugin_quota.go` from `b192f655` constructs its own request: add `egress.CheckURL(parsed, "management.pluginQuotaProbe")` before it is issued (same pattern as Task 5 for `api_tools.go`) and add the file to `coverage_allowlist.txt` only if it constructs a transport. Re-run until green. `make patches`; commit.

- [ ] **Step 5: Install and run in audit mode**

```bash
make install
python3 - <<'PY'
import re,io
p='/Users/arshan/.cli-proxy-api/config.yaml'; s=open(p).read()
if 'egress:' not in s: s += '\negress:\n  mode: audit\n'
open(p,'w').write(s)
PY
```
The proxy reloads config on change. Use Claude Code and Codex normally for ~15 minutes, then:
```bash
grep 'egress audit' ~/.cli-proxy-api/logs/main.log | sed -E 's/.*host "([^"]+)".*site=([^)]+).*/\1 \2/' | sort | uniq -c
```
Expected: empty. Any line names a host to investigate: provider host → `providers.go` (with a `PATCHES.yaml`-style note in the commit), tooling host → confirm it is compiled out, anything else → report to the user before allowing it. Then set `mode: enforce` and confirm sessions still work.

- [ ] **Step 6: Tick, tag, push**

Tick the last checklist item in `docs/fork/audits/<TAG>.md`; commit it.
```bash
git tag <TAG>-locked.1
git push origin locked --tags
```
Before pushing, confirm `git remote get-url --push origin` is the ArshansGithub fork. Report the tag, the running version from `make install`, the conflicts resolved, any patch dropped, and the audit-mode result.

---

## Self-review notes

- Spec §4 (gate, modes, chokepoints, coverage, registration): Tasks 2–6. §5 (stubs, string test): Tasks 7–10. §6 (panel, models, make targets): Tasks 7, 9, 12. §7 (remotes, branches, prefixes, PATCHES, adoption steps, install): Tasks 1, 11, 14. §8 (audit report sections 1–8): Task 11 step 3. §9 (README, CLAUDE.md, skill): Task 13. §10 tests: each task. §11 order: Phases 0–3 and Task 14.
- The spec's `make rebase` gate on unticked items conflicts with the post-install checklist item; Task 11 step 4 documents that the last item is ticked after install and the gate is meant to make skipping visible. The skill's adopt-tag workflow reflects this.
- Names used across tasks: `egress.SetConfig`, `SetConfigWithBuiltin`, `Current`, `CheckURL`, `CheckURLString`, `GuardTransport`, `WrapProxyFunc`, `GuardWebsocketDialer`, `RoundTripper`, `BuiltinHosts`, `Register`, `ErrNotPermitted`; `lockedbuild.Enabled`; `managementasset.EmbeddedPanel`, `PanelVersion`, `staticDirFor`; `registry.remoteModelRefreshEnabled`, `modelsURLs`, `codexClientModelsURLs`; `config.EgressConfig`, `NormalizeEgressHost`, `EgressModeEnforce`, `EgressModeAudit`. Checked for consistency.
