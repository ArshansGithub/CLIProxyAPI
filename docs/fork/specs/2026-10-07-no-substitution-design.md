# Retire the effort slugs; make substitution loud

Date 2026-10-07. Decided with Arshan in the proxy-upgrade session after Claude Code
2.1.293 was installed through the Homebrew cask.

## Why

The effort-suffixed model slugs (`claude-opus-5-5-medium`, fork commit cc0da4a2) and the
effort pin (54e71549) existed because Claude Code 2.1.284 gave an agent-team teammate the
session's effort instead of its definition's (anthropics/claude-code#80569). That is fixed
in 2.1.293: the in-process teammate builder now copies `effort` and `disallowedTools` from
the definition (2.1.284 copied only `model`), and a probe on 2026-10-07 proved it. A
headless session at effort `low` spawned the same definition (`effort: high`, model
`claude-sonnet-5-5-medium`) named and unnamed; both subagent transcripts record
`effort=high` and the proxy log shows both requests arriving with `original_value=high`
(session `401a6b59-da0a-44c8-b8e1-d58a35117bb4`, `main.log` 13:15:22-23).

His concern now is the other direction: silent substitution. When an agent asks for a model
or an effort and something else is served, he wants a loud error, never a quiet fallback.
Substitution happens in three layers today:

1. Claude Code substitutes models on its own: a refusal fallback to another model, a
   "previous model of the same tier" retry when the API refuses a model, the compaction
   model swap, and the auto-mode classifier using Claude Sonnet 5 in place of a Sonnet 5.5
   pin.
2. Claude Code attaches `fallbacks` (Opus 4.8 behind Opus 5.5, Opus 5 behind Fable 5.1) so
   Anthropic's server answers on another model after a safety refusal; the proxy's cloaked
   path injects the same field when the client did not send it
   (`claude_executor_cloaking.go` around lines 998, 1040 and 1460).
3. The proxy rewrites an effort the model does not offer: clamp (`validate.go:287`), remap
   (`apply.go:383`), or drop of a configuration_update item (`configuration_update.go:77`),
   each at Warn and each continuing with the rewritten value.

An unregistered model is already refused with `model_not_found`; nothing changes there.

## Decisions

- Retire slug usage; keep the fork code. Every agent page, `settings.json`, `cca` and the
  offline tools go back to bare ids with `effort:` frontmatter or `--effort`; the proxy's
  slug parser and pin stay in the stack, dormant, with `effort-pin.enabled: false`.
- `fallbacks` are stripped at the proxy and never injected; a refusal surfaces as a
  refusal. Claude Code's own client-side substitution is switched off with its env
  switches. Refusing a request that carries `fallbacks` was rejected: every stock Claude
  Code request on Opus 5.5 and Fable would fail.
- An effort the model does not offer is a 400, not a clamp.
- Haiku tier moves to `claude-haiku-5-5` (released 2026-10-07, the 2.1.293 default). The
  `gpt-6-luna: xhigh` modelSettings row stays for interactive Luna sessions.
- The Ceeya agent pages are not edited from this seat. A sitting note records the change
  and each stream's continuation gets one pointer line.

## Part 1: Claude Code side (this machine; live on the next `cca` start, no proxy restart)

`~/bin/cca`:
- `ANTHROPIC_DEFAULT_OPUS_MODEL=scoped/claude-opus-5-5[1m]`,
  `ANTHROPIC_DEFAULT_SONNET_MODEL=scoped/claude-sonnet-5-5[1m]`,
  `ANTHROPIC_DEFAULT_HAIKU_MODEL=claude-haiku-5-5` (no `[1m]`: the comment about haiku
  being 200k is stale for 5.5, but the helper calls do not need the pin).
- The comment that explains the slugs is replaced by one naming this spec.

`~/.claude/settings.json`:
- `env.ANTHROPIC_DEFAULT_HAIKU_MODEL` = `claude-haiku-5-5`.
- `env` gains `CLAUDE_CODE_NO_MODEL_FALLBACK=1`, `CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK=1`,
  `CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK=1`. These are present in the 2.1.293 binary
  (`CLAUDE_CODE_NO_MODEL_FALLBACK is set: model substitution is disabled`; the other two
  gate the refusal retry and the model-access ladder). They are undocumented; the probe in
  Part 4 is what proves them.
- `modelSettings` keeps its rows; the bare `claude-opus-5-5` and `claude-sonnet-5-5` rows
  (medium) rule for the prefixed tiers (his R749(5)). A `claude-haiku-5-5` row is not
  added: the model's own default is medium.
- `switchModelsOnFlag` stays `false`.

`~/.claude/agents/`: `luna.md` -> `model: gpt-6-luna`, `effort: high`; `sol.md` ->
`model: gpt-6.1-sol`, `effort: high`; `codebase-refactor-architect.md` ->
`model: claude-opus-5-5[1m]`, `effort: high`. The Fable-subagent hook in `settings.json`
keys on agent names (`fable-medium`, `fable-high`), not slugs, and stays.

`~/.cli-proxy-api/config.yaml`: `effort-pin.enabled: false`. The config watcher applies it
without a restart (`applyEffortPinConfig` runs on reload). The block and its comment stay
so the pin can be turned back on.

Sessions started before these edits keep their old environment until restarted.

## Part 2: proxy `no-substitution` (fork feature; `make verify`, PATCHES.yaml row, install on his word)

Config, a fork root section beside `effort-pin` (`forkRootSections` in
`internal/config/config_v8_fork.go`):

```yaml
no-substitution:
  strip-fallbacks: true            # drop `fallbacks` from every Claude request; never inject
  refuse-unsupported-effort: true  # 400 instead of clamp/remap/drop
```

Both default to false in code so an upstream-shaped config is unchanged; his config sets
both true.

`strip-fallbacks` (Claude executor):
- Delete `fallbacks` from the body before cloaking and before the native passthrough, so
  neither the three injection sites nor the client's own field reach the wire. The
  reconcile step that re-inserts a cloak-injected fallback (`reconcileClaudeCodeFableModel-
  AfterPayload`) sees no injected state and does nothing.
- Do not add `server-side-fallback-2026-06-01` or `fallback-credit-2026-06-01` on the
  proxy's own beta list when the body carries no `fallbacks`; a client-requested
  `server-side-fallback-*` beta is removed from the forwarded header too, so the upstream
  never sees a fallback contract. `fallback_credit_token` is left alone (it is a credit, not
  a substitution).
- Probes and helpers already carry no fallbacks; unchanged.
- One Info line per stripped request at most once per session key, not per request, so the
  log says the feature is on without flooding: `no-substitution: fallbacks stripped |
  model= session=`.

`refuse-unsupported-effort` (`internal/thinking`):
- `validate.go` clamp, `apply.go` remap and `configuration_update.go` drop each return an
  error of a new type `*UnsupportedEffortError{Provider, Model, Requested, Supported}` when
  the switch is on; `ApplyThinking` returns it unchanged.
- Today every executor returns the thinking error bare (`claude_executor_execute.go`,
  `codex_executor_execute.go:50,237`, `codex_websockets_stream.go:782`,
  `meta_executor_execute.go:49`, `antigravity_executor_tokens.go:55`), which the handler
  reports as an upstream failure. Each call site wraps an `*UnsupportedEffortError` in the
  executor's `statusErr{code: 400}` so the caller gets
  `{"type":"invalid_request_error","code":"effort_unsupported","message":"model gpt-6-luna
  does not offer effort \"ultra\"; it offers low, medium, high, xhigh, max"}` on its own
  wire format.
- The log line is at Error: `no-substitution: effort refused | provider= model=
  requested= supported=`. The existing Warn lines stay for the off state.
- The slug rewrite (`rewritten by model suffix`) is not a substitution: the caller chose
  the slug. It stays as it is.
- Codex `ultra` becomes a 400 under the switch (today it clamps to max). models.json keeps
  low..max.

Not in scope: a `/cache.html` counter or banner; the keepalive probe (it carries no
effort); non-Claude `fallbacks` (only the Claude wire format has the field).

Tests: `internal/thinking/no_substitution_test.go` (the three refusals on and off, the
error text, Codex ultra); `internal/runtime/executor/claude_executor_no_substitution_test.go`
(native and cloaked bodies with and without `fallbacks`, the beta list, probe untouched);
the side rig (`.ceeya/sittings/2026-09-29-proxy-effort-slugs/` fake upstream on :8398)
captures one Opus 5.5 request from a 2.1.293 `cca -p` session and shows no `fallbacks` key
and no `server-side-fallback` beta on the wire. Then `make verify`, `make verify-tagless`,
the PATCHES.yaml row (`feat`), `make patches`. Install as v8.0.3-locked.5 only on his word;
until then Part 1 is live and Part 2 is on the branch.

## Part 3: Ceeya handoff (written from this seat; pages untouched)

`.ceeya/sittings/2026-10-07-effort-slugs-retired/README.md` in the NewCeeya tree:
- The why, with the probe evidence above and the commands that reproduce it.
- The edit per agent page: 16 pages name a slug (`astra-high`, `fable-high`, `fable-medium`,
  `glm-flash`, `luna-low`, `luna-medium`, `luna-web`, `nutritionist`, `opus`, `reader-opus`,
  `reader-sonnet`, `seed-conductor`, `sol-low`, `sol-medium`, `sol`, `sonnet`, `ui-ux`);
  each becomes the bare id (account prefix and `[1m]` kept) plus an `effort:` line carrying
  the level the slug named. The conductor page stays bare Fable.
- The offline tools that were taught the slugs (`tools/sources/read.ts`, `author.ts`,
  `discover.ts`, `classify-corpus.ts`, the bench rows, `kind-run.sh`, `author-top.ts`)
  can name bare ids again with `--effort`; the slugs keep working, so the change is
  optional and can ride each tool's next edit.
- `decision.md` row 27 and row 28 need a dated addendum: the §2663b slug decision is
  superseded on 2.1.293; the named-spawn rule in row 27 no longer holds.
- The verify line: spawn one named agent from a page whose effort differs from the session
  and read `effort` in its transcript under `subagents/`; grep `main.log` for
  `rewritten by model suffix` (must be absent once a page is bare) and `effort refused`.
- The open items: the twelve worktree copies of the agent pages; the openers under
  `.ceeya/record/openers/` that carry the standing effort-pin paragraph.

One line appended to `.ceeya/record/{brands,ingredient,ketosis,scan}/continuation.md`
under its open items, pointing at the sitting and saying the pin is off and bare ids are
accepted again.

## Part 4: proof

1. A `cca -p` session at `--settings '{"effortLevel":"low"}'` spawns a bare-id page with
   `effort: high` named and unnamed: transcripts `effort=high`, no `rewritten by model
   suffix` line, no `effort-pin` refusal.
2. The same session on Opus 5.5 through the side rig: the captured body has no
   `fallbacks`; the beta header has no `server-side-fallback`.
3. A request with `output_config.effort: ultra` on `gpt-6-luna` returns 400
   `effort_unsupported` and one Error line.
4. `brew`'d 2.1.293 with the three env switches: a forced refusal on Opus 5.5 (the rig's
   fake upstream answering `stop_reason: refusal`) ends the turn with the refusal shown,
   no second request on another model in the rig's captures.

## Rollback

Part 1 is file edits with the previous values named here. Part 2 is a config block: both
keys false restores today's behaviour without a reinstall; the pin can be re-enabled the
same way.
