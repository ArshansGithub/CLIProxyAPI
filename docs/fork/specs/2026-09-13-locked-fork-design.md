# CLIProxyAPI Locked Fork — Design

Date: 2026-09-13
Status: approved in discussion, pending written review
Base: feat/cache-watch @ 491d1d43 (upstream v7.2.154 + 60 local patches)

## 1. Goal

Produce and maintain a privacy-conscious build of CLIProxyAPI that:

- never downloads or executes anything at runtime that was not reviewed at build time;
- can only open outbound connections to hosts it has a reason to talk to;
- keeps every locally carried change legible, so adopting a new upstream tag is a
  bounded, auditable task rather than an open-ended merge;
- is described once, in-repo, so humans and agents do not re-derive what the fork is.

Non-goals: hardening the OS, protecting against a compromised local user, replacing
upstream's feature work, or forking the management panel's source.

## 2. Threat model

The proxy holds OAuth access and refresh tokens for Claude, Codex, and other
accounts, plus the management key that can read every credential file. Upstream is a
high-volume, single-maintainer-dominated project with no security policy, a CI that
only builds, and a panel repo whose releases are produced from unsigned tags by one
account. The attacker of concern is anyone who can influence what upstream ships:
a compromised maintainer account, a malicious contributor whose PR is merged, or a
compromised host that upstream's binary trusts at runtime.

Surfaces found in the current binary, ranked by what a compromise yields:

| Surface | Yield | Treatment |
|---|---|---|
| Panel auto-updater (3h) + unverified fallback host | JS with the management key: full token theft | Compiled out; panel embedded |
| Plugin store + dlopen loader | Native code in the token-holding process | Loader stubbed; install returns 403 |
| Model registry JSON refresh (3h) | Silent remote control of model list and alias routing | Embedded snapshot; reviewed refresh |
| Management api-call tool with `$TOKEN$` substitution | Live token sent to caller-chosen URL | Subject to egress policy |
| Antigravity version manifest (3h) | A version string; hard-coded fallback exists | Nothing in code; gate blocks it |
| `GITHUB_TOKEN` environment pickup | Token sent with updater requests | Removed with the updater |
| Home (Redis) mode | Forces cooldowns off, disables API keys | Off by config; not compiled out |

Rule for future surfaces: if the egress gate alone makes it inert and the code fails
gracefully, no code change. A stub is added only where the gate is insufficient
(local file inputs, or blocking would leave the proxy degraded with no clean update
path).

## 3. Architecture

Three mechanisms, each independently testable:

1. **Egress gate** (`internal/egress`): a policy that decides, per destination host,
   whether an outbound connection is permitted. Enforced at the chokepoints all
   HTTP and websocket traffic passes through.
2. **`locked` build tag**: upstream files implementing a runtime-download surface
   carry `//go:build !locked`; sibling `*_locked.go` files provide stubs with the
   same exported API. The Makefile always sets the tag; a tagless build reproduces
   upstream behaviour for comparison.
3. **Embedded assets**: the management panel and the model registry snapshots are
   committed artifacts compiled in with `go:embed`, each with a pin file recording
   source tag, commit, and sha256.

Around them: a repo layout and Makefile workflow for adopting upstream tags, an
audit tool that produces a committed report per tag, and in-repo documentation
(README, CLAUDE.md, a maintenance skill) so the fork explains itself.

## 4. Egress gate

### 4.1 Policy

`egress.Policy` is built once at startup from config and rebuilt on config reload.
Allowed hosts are the union of:

- **Registered provider hosts.** Each executor and auth package registers the hosts
  it uses next to its URL constants, e.g. `egress.Register("claude",
  "api.anthropic.com")`. Registration happens in package `init`. A provider's hosts
  are admitted whenever the provider is compiled in, regardless of whether a
  credential exists yet, because an OAuth login for a new provider must reach the
  provider's auth host before any credential file is written. Only code paths for
  configured providers ever dial these hosts, so admitting them costs nothing.
- **Configured base URLs.** The hostname of every `base-url` under
  `openai-compatibility`, `codex-api-key`, `claude-api-key`, and similar blocks.
- **Loopback.** `127.0.0.1`, `::1`, `localhost`.
- **`egress.extra-allow`** (config, list of hostnames): escape hatch for hosts that
  are neither provider constants nor base URLs.

Matching is exact hostname, case-insensitive, port ignored. No wildcards; a host is
either listed or not.

### 4.2 Modes

`egress.mode`: `enforce` (default) or `audit`. In audit mode a disallowed host is
logged at warn level with the same detail but the request proceeds. Audit mode
exists for the first run after adopting a tag, to observe the real host set.

### 4.3 Enforcement points

- `egress.RoundTripper(base http.RoundTripper)`: checks `req.URL.Hostname()` before
  delegating. Installed in:
  - `helps.buildProxyTransport` (executors)
  - `proxyutil.BuildHTTPTransport` (used by `util.SetProxy`, auth flows, TUI)
  - `helps.newUtlsRoundTripper` (Claude uTLS path)
  - `http.DefaultTransport`, replaced in `cmd/server/main.go` before any client is
    built, so sites using `http.DefaultClient` or a zero `http.Client` are covered.
- `egress.CheckURL(u *url.URL) error`: called before dialing in
  `newProxyAwareWebsocketDialer` (Codex) and the xAI websocket dial.
- Management `api-call` tool handler (`internal/api/handlers/management/api_tools.go`)
  and the plugin quota probe, if present after a merge: call `egress.CheckURL`
  before issuing the request.

A blocked request returns `egress: host "x" not permitted (call site: pkg.func)` as
the error; the executor surfaces it as a 502 with that message and logs one line at
error level containing host, call site, provider, and mode.

### 4.4 Coverage tests

- `internal/egress/coverage_test.go` walks non-test Go files and fails if
  `http.Transport{`, `&http.Transport{`, `http.Client{` with a non-nil Transport, or
  `websocket.Dialer{` appears outside an allowlist of files
  (`proxy_helpers.go`, `proxyutil/proxy.go`, `utls_client.go`,
  `utls_transport.go`, `codex_websockets_connection.go`,
  `xai_websockets_executor.go`, `egress/*`). Adding a file to the allowlist is a
  deliberate commit.
- `internal/egress/registration_test.go` extracts every `https://` and `wss://`
  literal from non-test files under `internal/runtime/executor`, `internal/auth`,
  `sdk/auth`, and `internal/registry`, and fails if the hostname is not registered
  or explicitly listed in a `known-unregistered.txt` file with a reason (test
  fixtures, documentation URLs).

## 5. `locked` build tag

Files gaining `//go:build !locked` and their stubs:

| Upstream file | Stub | Stub behaviour |
|---|---|---|
| `internal/managementasset/updater.go` | `updater_locked.go` | `StartAutoUpdater`, `EnsureLatestManagementHTML` are no-ops returning true when the embedded asset exists; `SetCurrentConfig` retained. No release URL, no fallback URL, no GitHub token lookup exist in the locked binary. |
| `internal/pluginhost/loader_unix.go` (tag becomes `cgo && (linux \|\| darwin \|\| freebsd) && !locked`) | `loader_locked.go` | `openPlugin` returns `errors.New("native plugin loading is disabled in the locked build")`. Existing `!cgo` stub covers the rest. |
| `internal/api/handlers/management/plugin_store.go` | `plugin_store_locked.go` | `InstallPluginFromStore` responds 403 `{"error":"plugin store is disabled in the locked build"}`; `ListPluginStore` returns an empty list with the same message field. |
| `internal/registry/model_updater.go`, `codex_client_models_updater.go` | `model_updater_locked.go` | Startup and periodic refresh read from `internal/registry/snapshot/models.json` and `codex_client_models.json` via `go:embed`. No network. |

Panel serving under `locked`: `internal/managementasset/embedded_locked.go` exposes
`EmbeddedPanel() []byte`; the `/management.html` route handler uses it instead of
reading the static directory. `disable-control-panel: true` still removes the route.

Everything else is untouched. Home mode remains available but off; Antigravity's
manifest fetch remains and is blocked by the gate.

`internal/lockedbuild/strings_test.go` builds the binary with `-tags locked` into a
temp dir and asserts its string table contains none of:
`api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center`,
`cpamc.router-for.me`, `models.router-for.me`,
`raw.githubusercontent.com/router-for-me/models`.

## 6. Embedded assets

### 6.1 Panel

```
internal/managementasset/panel/management.html
internal/managementasset/panel/PANEL_VERSION   # tag, commit, sha256, built-at, bun version
```

Initial pin: `Cli-Proxy-API-Management-Center` v1.22.18, the bundle audited on
2026-09-13 (sha256
`f11e7f970ed474d262049b236c54f35ef59f4b9a2da5b9fe1946af14a8641553`).

Make targets:

- `panel-build`: clone panel repo at the pinned tag to a temp dir, `bun install
  --frozen-lockfile && bun run build`, copy `dist/index.html` to the panel path,
  rewrite `PANEL_VERSION`.
- `panel-verify`: `panel-build` into a temp path, compare sha256 with the GitHub
  release asset for the same tag; print both and PASS/MISMATCH. Never fails the
  build; a mismatch is a finding.
- `panel-bump TAG=`: print `git log --format='%h %an %s' <pinned>..<TAG>`, `git
  diff --stat`, and grep the diff's added lines for `eval(`, `new Function`,
  `fetch(`, `XMLHttpRequest`, `WebSocket`, `sendBeacon`, `postMessage`,
  `localStorage`, `document.cookie`, `import(`, and new `https://` literals. Waits
  for an explicit `y` before building and updating the pin.

Known side effect: the panel's plugin-store tab errors, because its GitHub lookups
route through the proxy's api-call tool and the gate refuses them. Not worked
around.

### 6.2 Model registry

```
internal/registry/snapshot/models.json
internal/registry/snapshot/codex_client_models.json
internal/registry/snapshot/MODELS_VERSION   # source URL, fetched-at, sha256 each
```

`make refresh-models`: fetch both files from the upstream models repo (this target
is the one place the build tooling, not the binary, talks to that host), show a
unified diff against the snapshot, wait for `y`, then overwrite and update the pin.

## 7. Repository layout and workflow

### 7.1 Identity

- GitHub: existing fork `ArshansGithub/CLIProxyAPI`; default branch becomes `locked`.
- Local: `~/Desktop/NewShi/cpa-cache-watch` is the single checkout.
  `~/Desktop/NewShi/cliproxyapi-patched` is archived (moved, not deleted, outside
  the working set).
- Remotes: `origin` = the fork (push), `upstream` = router-for-me (fetch only; the
  Makefile refuses to push to it).

### 7.2 Branches and tags

- `locked`: a linear stack of fork commits on top of exactly one upstream tag.
- `locked-prev`: moved to the previous tip before each rebase.
- Fork releases: `v<upstream>-locked.<n>`, e.g. `v7.2.159-locked.1`.
- Upstream tags are fetched; upstream `main`/`dev` are not tracked by any local
  branch.

Commit subject prefixes:

| Prefix | Meaning | Drop condition |
|---|---|---|
| `lock:` | lockdown mechanisms, embedded assets, tooling, docs | never |
| `feat:` | fork features (keepalive, cachestats, cache.html, TUI cache tab, overload-retry) | upstream ships an equivalent |
| `fix:` | Claude-path fixes mirroring open upstream PRs #5443–#5446 | PR merged |
| `pick:` | cherry-picked open upstream PRs #5338, #5420, #5422, #5566 | PR merged |

`docs/fork/PATCHES.md` lists every carried commit: hash, prefix, one line, upstream
PR link if any, drop condition. Regenerated by `make patches` from the commit range
and hand-annotated columns kept in a sidecar `PATCHES.yaml`.

### 7.3 Adopting an upstream tag

```
make upstream-fetch                 # git fetch upstream --tags (no branches)
make upstream-audit TAG=v7.2.170    # section 8; writes docs/fork/audits/v7.2.170.md
make rebase TAG=v7.2.170            # moves locked-prev, rebase --onto with rerere
make verify                         # build -tags locked, vet, go test ./..., coverage tests
make install                        # section 7.4
git tag v7.2.170-locked.1 && git push origin locked --tags
```

`make rebase` refuses to run if the audit report for that tag does not exist or has
unticked checklist items.

### 7.4 Build and install

`make build` runs `go build -tags locked -ldflags "-s -w -X main.Version=<tag>
-X main.Commit=<sha> -X main.DefaultConfigPath=/opt/homebrew/etc/cliproxyapi.conf"`.
`make install` copies the binary over `/opt/homebrew/opt/cliproxyapi/bin/cliproxyapi`
(keeping a `.prev` copy), runs `brew services restart cliproxyapi`, waits for
`GET /healthz` on 127.0.0.1:8317, and prints the version the running binary reports.
`make rollback` restores `.prev`.

## 8. Upstream audit tool

`scripts/upstream-audit.sh <TAG>` writes `docs/fork/audits/<TAG>.md` containing:

1. Range summary: base tag, target tag, commit count, distinct authors with counts.
2. Dependencies: `git diff <base>..<TAG> -- go.mod go.sum`; `govulncheck ./...` on
   a temp worktree at `<TAG>`.
3. Commits: hash, author, GPG status, subject. Authors outside the top five
   upstream contributors are marked `[review]`.
4. Added-line grep over non-test Go files: `os/exec`, `syscall.`, `dlopen`,
   `os.WriteFile`, `os.Create(`, `func init()`, `go:embed`, `base64.`, and every
   new `https://` / `wss://` literal, each with file:line.
5. Watched paths: full `git diff` for `internal/managementasset`,
   `internal/pluginstore`, `internal/pluginhost`, `internal/registry`,
   `internal/home`, and any file containing `$TOKEN$`.
6. Pins: latest tag of the panel repo and the models repo versus the current pins.
7. Raw-client-site scan (the same file walk as `coverage_test.go`, run as a script
   rather than a test) against a temp worktree at `<TAG>`, reported as the set of
   files added since the current base. The registration test is not run here; it
   only makes sense on the rebased tree and runs under `make verify`.
8. Checklist (markdown checkboxes): flagged commits read; dependency changes
   accepted; panel pin decision; models pin decision; audit-mode run after install.

Only GitHub reads (`git fetch`, `gh api` for tags) are performed; nothing is pushed.

## 9. Documentation and agent support

- `README.md`: replaced. Sections: what this is, threat model, what is compiled out,
  the egress allowlist and `extra-allow`, build and install, adopting a tag, links to
  `PATCHES.md` and the audits directory. No sponsor or affiliate content.
  Upstream's README is kept verbatim at `docs/UPSTREAM-README.md`.
- `CLAUDE.md` (repo root): what the fork is in five lines; the layer prefixes; hard
  rules (never push to `upstream`; always build with `-tags locked`; never edit an
  upstream file when a `_locked.go` sibling or the gate can do the job; every carried
  change gets a `PATCHES.md` entry; run `make verify` before claiming done); pointers
  to the spec, `PATCHES.md`, and the skill.
- `.claude/skills/fork-maintenance/SKILL.md`: three workflows, each a checklist:
  **audit-tag**, **adopt-tag**, **carry-patch** (add, update, or drop a `feat:`,
  `fix:`, or `pick:` commit, including the `PATCHES.md` update and the upstream PR
  status check). Invoked as `/fork-maintenance <workflow> [TAG|PR]`.

## 10. Testing

- Unit: `internal/egress` policy (allow/deny, provider gating, extra-allow,
  audit mode), RoundTripper and CheckURL behaviour, config reload.
- Coverage tests (section 4.4) and string-table test (section 5) run under
  `make verify` and in the audit tool.
- Stubs: each `_locked.go` file has a test asserting the stub's behaviour and that
  the corresponding route or function returns the documented error.
- Embedded assets: a test asserts the sha256 in `PANEL_VERSION` and
  `MODELS_VERSION` matches the embedded bytes.
- End to end, manual, once per adoption: install, run in `audit` mode for one
  Claude Code session and one Codex session, confirm the log shows no unexpected
  hosts, switch to `enforce`.

## 11. Delivery order

1. Repo hygiene: `locked` branch, remotes, archive the second checkout, Makefile
   skeleton, `PATCHES.md` generator.
2. Egress gate with tests and coverage tests.
3. `locked` tag stubs, embedded panel, model snapshots, string-table test.
4. Audit tool and the adoption targets.
5. README, CLAUDE.md, skill.
6. First real adoption: audit and rebase onto the current upstream tag, install,
   audit-mode run, tag `-locked.1`.
