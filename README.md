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

`make panel-verify` showed the v1.22.18 release asset is byte-identical to a from-source build
(2026-09-13).

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
`make rebase` refuses to run until every item on the audit checklist in
`docs/fork/audits/<TAG>.md` is ticked, except the item that can only be confirmed after
install (that one stays open until the post-install step). Carried patches are listed in
`docs/fork/PATCHES.md`. Design: `docs/fork/specs/`.

## Upstream

Upstream's README is preserved at `docs/UPSTREAM-README.md`. This fork never pushes to upstream;
fixes that belong there are opened as PRs from this repo.
