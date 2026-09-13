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
- update: `gh pr view PR --repo router-for-me/CLIProxyAPI --json state,mergedAt`; if merged, go to drop. To see what's currently carried, look at the PRs listed with layer `pick` in `docs/fork/PATCHES.yaml`.
- drop: `git rebase -i` is not available; use `git rebase --onto <parent-of-patch> <patch> locked` after moving `locked-prev`, remove the YAML entry, `make patches`, `make verify`.
Always finish with `make verify` and a one-paragraph report.
