# CLIProxyAPI — locked build

A privacy-locked fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI).
Same proxy, same accounts, same clients: Claude Code, Codex and friends still route through
one local endpoint with pooled OAuth logins, cooldowns and model aliases. The difference is
what the binary is *allowed* to do once it is running:

- **It cannot download or execute anything it did not ship with.** No auto-updating web
  panel, no plugin installs, no remote model catalog. What you reviewed at build time is
  what runs.
- **It can only talk to hosts it has a reason to talk to.** Every outbound connection,
  HTTP or websocket, passes an allowlist before DNS resolution. Anything else is refused
  and logged.
- **Every carried change is registered and every upstream tag is audited before adoption.**
  Updating is a checklist, not a merge.

You can tell at a glance which build you are on: the management panel carries a
`🔒 PRIVACY-LOCKED BUILD` badge with the live egress mode, `cliproxyapi -v` prints a
`-locked` version, and the startup log says `egress: mode=enforce — N hosts allowed`.

## The problem with the stock binary

The proxy is the most sensitive process on the machine. It holds the OAuth access and
refresh tokens for every connected Claude, Codex, Gemini and other account, plus a
management key that can read all of them. That is exactly the process that should not be
taking instructions from the internet. The stock build does, in several places, on a timer:

| What it does on its own | How often | What a compromise yields |
|---|---|---|
| Downloads a new web panel from the GitHub releases feed, with an unverified fallback host (`cpamc.router-for.me`) when GitHub is unavailable | every 3 hours | JavaScript running with the management key in your browser: every token, readable and exfiltratable |
| Installs native plugins (`.dylib` / `.so`) from GitHub and loads them with `dlopen` | on request via the management API | Arbitrary native code inside the token-holding process |
| Refreshes its model catalog from a maintainer-controlled URL (`models.router-for.me`) | every 3 hours | Silent remote control of which models exist and where aliases route |
| Fetches a client version manifest from a Google Cloud Run host | every 3 hours | Low on its own, but it is one more host the binary trusts unconditionally |
| Picks up `GITHUB_TOKEN` from the environment and sends it with updater requests | with every update check | A credential you never gave it, leaving the machine |
| Offers a management "api-call" tool that substitutes a live token into a caller-chosen URL | on request | The token itself, sent wherever the caller points |

None of these fetches is verified beyond a same-origin checksum. Upstream is a high-volume
project dominated by a single maintainer, with no security policy, a CI that only builds,
and a panel repository whose releases are produced from unsigned tags by one account. The
attacker of concern is not upstream's intent. It is anyone who can influence what upstream
ships: a compromised maintainer account, a malicious contribution that gets merged, or a
compromised host that the binary already trusts at runtime. With the stock build, any one of
those becomes your tokens within three hours, with nothing to notice.

## What this fork does about it

Three mechanisms, each independently tested, all switched on by the `locked` build tag:

1. **Egress gate** (`internal/egress`). A host allowlist made of the registered provider
   hosts, the hostnames of your configured `base-url`s, loopback, and anything you add under
   `egress.extra-allow`. It hooks the proxy function of every HTTP transport and websocket
   dialer, so it runs on every request, including redirects, before DNS. Two repository
   tests keep it honest: one fails if a new `https://` or `wss://` host literal appears
   in the scanned source trees without being registered, and one fails if any code constructs a raw
   transport or reassigns a proxy function outside a short, commented allowlist.
2. **Compiled-out surfaces.** The panel auto-updater and its fallback host, the native plugin
   loader and store, and the model catalog refresh all carry `//go:build !locked`. A test
   builds the locked binary and asserts that none of those hostnames survive in it.
3. **Embedded, pinned assets.** The web panel is a reviewed build compiled into the binary,
   with its source tag, commit and sha256 recorded in `PANEL_VERSION`; `make panel-verify`
   rebuilds that tag from source and confirms the GitHub release asset is byte-identical
   (it was, for v1.22.18, on 2026-09-13). The model catalog is an embedded snapshot behind a
   hash-checked pin; `make refresh-models` shows you the diff before anything changes.

| Area | Upstream | This fork (`-tags locked`) |
|---|---|---|
| Web panel | downloaded at runtime, auto-updated | reviewed build embedded; stamped with a locked badge at serve time |
| Plugins | native code loaded with `dlopen`, installed from GitHub | loader refuses; store endpoints return 403 |
| Model catalog | fetched from `models.router-for.me` every 3h | embedded snapshot; reviewed refresh |
| Outbound hosts | anything the code asks for | allowlist: provider hosts + configured `base-url`s + loopback + `egress.extra-allow` |
| Startup | trusts whatever is on disk and online | fail-closed: loopback-only until the policy loads; install refuses a non-`-locked` binary |
| Home mode, Postgres / object / git token stores | connect through their own client libraries, outside any gate | refused at startup; file store only |
| Updates | pull a tag and hope | audit report with a checklist gates the rebase; every carried patch registered |
| Claude Code cache | — | keepalive probing, per-session cache stats, `/cache.html` watch page, TUI Cache tab |

Everything else (executors, auth flows, management API, TUI) is upstream code at the pinned tag.

## Egress policy

```yaml
egress:
  mode: enforce        # or "audit" to log instead of refuse (use after adopting a new tag)
  extra-allow:         # hostnames beyond provider hosts and configured base-urls
    - my-oauth-callback.example
```

The gate checks the host a socket actually goes to, not only the URL a request names: a
forward proxy (`proxy-url` in config, a per-credential `proxy_url`, or the management
api-call tool's `proxy_url` field) is checked at the proxy hook and at every SOCKS and
CONNECT dial. Hosts from `proxy-url` fields in config are admitted automatically; a
per-credential proxy host must be listed under `extra-allow`.

A refused request fails with `egress: host "x" not permitted (site=..., mode=enforce)` and one
error log line. In audit mode the same event is a warning, `egress audit: host "x" would be
refused`, and the panel badge turns red so you do not forget to switch back. Provider hosts
are listed in `internal/egress/providers.go`; do not add one to make a test pass without
understanding why the code contacts it.

## Build and install

```
make build      # go build -tags locked → dist/cliproxyapi
make verify     # build, vet, tests, coverage tests
make install    # swap into /opt/homebrew/opt/cliproxyapi/bin, restart, health-check
make rollback   # restore the previous binary
```

`make install` keeps the previous binary as `.prev` and refuses to finish unless the running
proxy reports a `-locked` version.

## Adopting an upstream tag

```
make upstream-fetch
make upstream-audit TAG=v7.2.170     # writes docs/fork/audits/v7.2.170.md — read it
make rebase TAG=v7.2.170             # refuses without a completed audit
make verify && make install
git tag v7.2.170-locked.1 && git push origin locked && git push origin v7.2.170-locked.1
```

The audit report lists every upstream commit since the last adoption, flags the ones that
touch watched paths (transports, dialers, updaters, plugin and registry code), diffs
dependencies, and ends with a checklist. `make rebase` refuses to run until every item is
ticked, except the one that can only be confirmed after install: run one Claude Code and one
Codex session in audit mode and confirm nothing unexpected was logged. Carried patches are
listed in `docs/fork/PATCHES.md`; design and plan are under `docs/fork/specs/` and
`docs/fork/plans/`. The `/fork-maintenance` skill in `.claude/skills/` walks an agent through
all three workflows.

## Upstream

Upstream's README is preserved at `docs/UPSTREAM-README.md`. This fork never pushes to upstream;
fixes that belong there are opened as PRs from this repo.
