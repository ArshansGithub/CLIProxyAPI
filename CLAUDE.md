# CLAUDE.md — locked fork of CLIProxyAPI

This repository is a fork of router-for-me/CLIProxyAPI, maintained as a privacy-locked build.
Read `README.md` for what that means and `docs/fork/specs/2026-09-13-locked-fork-design.md` for why.
Upstream's own agent notes are in `AGENTS.md`; they describe the codebase layout and still apply.

## Rules that override everything else
- Never push to the `upstream` remote. Its push URL is `no_push`; do not change that.
- Always build and test with `-tags locked` (`make verify`). Also run `make verify-tagless` before committing Go changes.
- Prefer a `_locked.go` sibling or the egress gate over editing an upstream file. When an upstream edit is unavoidable, keep it to a build-tag line, an early return, or one hook call.
- Every carried change gets a `docs/fork/PATCHES.yaml` entry; run `make patches` after adding one.
- Commit subjects use a layer prefix: `lock:` (lockdown/tooling/docs), `feat:` (fork features), `fix:` (mirrors an open upstream PR, e.g. #5444; see `docs/fork/PATCHES.yaml` for the current list), `pick:` (cherry-picked upstream PR).
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
