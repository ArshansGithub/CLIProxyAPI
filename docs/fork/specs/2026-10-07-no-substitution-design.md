# Retire the effort slugs; make substitution loud

Date 2026-10-07. Decided with Arshan in the proxy-upgrade session after Claude Code
2.1.293 was installed through the Homebrew cask. Revised the same day after an Opus 5.5
review (findings 1-18 of that review are folded in below).

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
   model swap, the legacy-id remap, the Explore agent's cap to Opus in a Fable session, and
   the auto-mode classifier using Claude Sonnet 5 in place of a Sonnet 5.5 pin.
2. Claude Code attaches `fallbacks` (an array such as Opus 4.8 behind Opus 5.5, or the
   string `"default"` under `server-side-fallback-2026-07-01`) so Anthropic's server
   answers on another model after a safety refusal; the proxy's cloaked path injects the
   array when the client did not send it (`claude_executor_cloaking.go` 998, 1040, 1460)
   and re-injects it in `reconcileClaudeCodeFableModelAfterPayload` whenever the key is
   missing and no payload rule touched it. Live, cloaking is off
   (`disable-claude-cloak-mode: true`), so only the native field reaches the wire today.
3. The proxy rewrites an effort the model does not offer: the cross-family clamp
   (`validate.go:135`), the remap (`apply.go:383`), and the configuration_update drop
   (`configuration_update.go:54-78`), each at Warn and each continuing with the rewritten
   value. All 94 main logs hold one clamp line (Sol `ultra` to max, 2026-09-29) and no
   remap or drop line, so the blast radius is small.

An unregistered model is already refused with `model_not_found`; nothing changes there.

## Decisions

- Retire slug usage; keep the fork code. Every agent page, `settings.json`, `cca` and the
  offline tools go back to bare ids with `effort:` frontmatter or `--effort`; the proxy's
  slug parser and pin stay in the stack, dormant, with `effort-pin.enabled: false`.
- `fallbacks` are stripped from the body at the proxy and never injected; a refusal
  surfaces as a refusal. Claude Code's own client-side substitution is switched off with
  its env switches. Refusing a request that carries `fallbacks` was rejected: every stock
  Claude Code request on Opus 5.5 and Fable would fail.
- The Anthropic-Beta header of a confirmed native client is never reshaped (patch
  4f5cbbae). The body strip is the one named exception to that patch and is recorded in
  its PATCHES row; the header, including a latched `server-side-fallback-*` beta, is
  forwarded as sent. The beta alone starts no fallback: the server needs the body field.
- An explicit effort the model does not offer is a 400, not a clamp. Conversions that are
  not substitutions stay: a budget converted to a level, `auto` converted to the middle
  level, `none` converted to the lowest level, budget clamps, the no-thinking-support
  strip, the forced-tool-choice disable, and the Kimi family clamp. The spec names them so
  a later reader knows they were kept on purpose.
- Haiku tier moves to bare `claude-haiku-5-5` (released 2026-10-07, the 2.1.293 default),
  deliberately without the `scoped/` prefix: helper calls may use any Claude account. This
  moves helper load from the Codex pool to the Claude accounts. The `gpt-6-luna: xhigh`
  modelSettings row stays for interactive Luna sessions.
- The Ceeya agent pages are not edited from this seat. A sitting note records the change
  and each stream's continuation gets one pointer line.

## Part 1: Claude Code side (this machine; live on the next `cca` start, no proxy restart)

Order matters. Agent pages are read at spawn time, so a live session picks up a bare id
the moment the file changes, and the pin would refuse it.

Step 1, `~/.cli-proxy-api/config.yaml`: `effort-pin.enabled: false`. The config watcher
applies it without a restart (`applyEffortPinConfig` runs on reload; it logs nothing when
disabled, so confirm with `GET /v0/management/effort-pin`). The block and its comment stay
so the pin can be turned back on.

Step 2, `~/.claude/agents/`: `luna.md` -> `model: gpt-6-luna`, `effort: high`; `sol.md` ->
`model: gpt-6.1-sol`, `effort: high`; `codebase-refactor-architect.md` ->
`model: claude-opus-5-5[1m]`, `effort: high`. The Fable-subagent hook in `settings.json`
keys on agent names (`fable-medium`, `fable-high`), not slugs, and stays.

Step 3, `~/.claude/settings.json`:
- `env.ANTHROPIC_DEFAULT_HAIKU_MODEL` = `claude-haiku-5-5` (this env row overrides the
  `cca` default; the `cca` edit below keeps the two in step).
- `env` gains, one decision per switch, all present in the 2.1.293 binary:
  - `CLAUDE_CODE_NO_MODEL_FALLBACK=1`: no model substitution at all. Known cost: a
    compaction that would need a model swap now throws "Compaction unavailable:
    CLAUDE_CODE_NO_MODEL_FALLBACK is set", which the binary reaches on the Fable
    usage-credit consent path. Proof 5 covers it; this is the one switch with its own
    rollback line.
  - `CLAUDE_CODE_DISABLE_REFUSAL_FALLBACK=1`: no client-side retry on another model after
    a refusal. (`CLAUDE_CODE_DISABLE_REFUSAL_RETRY` is a separate switch for retrying the
    same model; it stays unset, a same-model retry is not a substitution.)
  - `CLAUDE_CODE_DISABLE_MODEL_ACCESS_FALLBACK=1`: no "previous model of the same tier"
    ladder.
  - `CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP=1`: an old model id fails instead of being
    remapped.
  - Accepted, not switched: the Explore agent's cap to Opus in a Fable session
    (`CLAUDE_CODE_DISABLE_EXPLORE_INHERIT_CAP` stays unset; his rule is that no agent runs
    on Fable without his word, so the cap enforces his intent), and the auto-mode and
    background classifier models (`CLAUDE_CODE_AUTO_MODE_MODEL`,
    `CLAUDE_CODE_BG_CLASSIFIER_MODEL` stay unset; they are Claude Code's own safety
    classifier, not his agents' work).
- `modelSettings` keeps its rows; the bare `claude-opus-5-5` and `claude-sonnet-5-5` rows
  (medium) rule for the prefixed tiers (his R749(5)). A `claude-haiku-5-5` row is not
  added: the model's own default is medium. `CLAUDE_CODE_EFFORT_LEVEL` must stay unset: it
  beats a definition's `effort:` (precedence env, definition, settings).
- `switchModelsOnFlag` stays `false`.

Step 4, `~/bin/cca`: `ANTHROPIC_DEFAULT_OPUS_MODEL=scoped/claude-opus-5-5[1m]`,
`ANTHROPIC_DEFAULT_SONNET_MODEL=scoped/claude-sonnet-5-5[1m]`,
`ANTHROPIC_DEFAULT_HAIKU_MODEL=claude-haiku-5-5`; the comment that explains the slugs is
replaced by one naming this spec; the stale "haiku is truly 200k" note goes.

Sessions started before these edits keep their old environment until restarted.

## Part 2: proxy `no-substitution` (fork feature; `make verify`, PATCHES.yaml row, install on his word)

Config, a fork root section beside `effort-pin` (`forkRootSections` in
`internal/config/config_v8_fork.go`; a line in `internal/watcher/diff/config_diff.go` so a
reload names the change):

```yaml
no-substitution:
  strip-fallbacks: true            # drop the `fallbacks` body key from every Claude request; never inject
  refuse-unsupported-effort: true  # 400 instead of clamp/remap/drop
```

Both default to false in code so an upstream-shaped config is unchanged; his config sets
both true. The switches reach the code through a fork store
(`internal/runtime/nosubstitution`, package-level atomics with `SetStripFallbacks` and
`SetRefuseUnsupportedEffort`), installed by `applyNoSubstitutionConfig` beside
`applyEffortPinConfig` at `sdk/cliproxy/service_config.go:205` and
`service_lifecycle.go:139`, so a reload flips them live and the rollback line below holds.
Following CLAUDE.md, the logic lives in fork files
(`internal/thinking/no_substitution.go`, `internal/runtime/executor/claude_executor_no_substitution.go`);
each upstream site gets one hook call.

`strip-fallbacks` (Claude executor):
- Strip once, on the finished body, after `reconcileClaudeCodeFableModelAfterPayload` and
  before the CCH billing header and signing (`claude_executor_execute.go` near line 205
  and the twin in `claude_executor_stream.go` near 205), because CCH hashes the body it
  sends. The key is deleted whatever its JSON type (array or the string `"default"`), and
  a payload rule that set it is stripped too.
- For a confirmed native client the Anthropic-Beta header is forwarded unchanged (the
  4f5cbbae contract; 2.1.293 latches `server-side-fallback-*` and
  `fallback-credit-2026-06-01` for the session even when the body has no `fallbacks`).
  On the cloaked and translated paths the proxy's own beta list adds neither
  `server-side-fallback-2026-06-01` nor `-2026-07-01` when the body has no `fallbacks`;
  `fallback-credit-2026-06-01` is still added when the body carries
  `fallback_credit_token` or the caller requested it (a credit is not a substitution, and a
  token without its beta is rejected upstream).
- Keepalive probes are built from the observed client body (`keepalive.go` `ProbeBody`)
  and run on the same executor path, so they are stripped the same way; their header stays
  the one the warmed request sent.
- Log line: when the strip removes a key from a confirmed native client, Warn
  (`no-substitution: fallbacks stripped | model= session= shape=array|default`), because
  with Part 1 live a native request carrying `fallbacks` means the env switch failed. On a
  cloaked or translated request, Debug. No once-per-session set; the Warn is the signal.

`refuse-unsupported-effort` (`internal/thinking`):
- Refuse only where the caller named a level and the proxy would silently change it: the
  cross-family clamp of an explicit level (`validate.go:135`, `allowClampUnsupported`), the
  remap at `apply.go:383`, and the configuration_update drop, and the drop only when the
  dropped effort differs from the top-level baseline (`supported` is then the baseline).
  The budget-to-level (`validate.go:111`) and auto-to-middle (`:165`) conversions stay.
- Each refusal returns `NewThinkingError(ErrLevelNotSupported, msg)` with message
  `effort_unsupported: model gpt-6-luna does not offer effort "ultra"; it offers low,
  medium, high, xhigh, max`. `ThinkingError.StatusCode()` is already 400
  (`internal/thinking/errors.go:83`) and `clienterror.HTTPStatusFromError` honours it, so a
  400 is a request fault with no credential rotation or cooldown, and no executor is
  edited. On `/v1/messages` the handler emits only `type` and `message`, hence the
  `effort_unsupported:` prefix in the text; a `code` field appears only on OpenAI-shaped
  wires.
- Log line at Error: `no-substitution: effort refused | provider= model= requested=
  supported=`. The existing Warn lines stay for the off state; the generic
  `thinking: validation failed` Warn at `apply.go:394` will print beside it.
- The slug rewrite (`rewritten by model suffix`) is not a substitution: the caller chose
  the slug. It stays as it is.
- Codex `ultra`: `/v1/responses` and chat already answer 400 `LEVEL_NOT_SUPPORTED`
  (same-family entry has `allowClampUnsupported` false); only a cross-family entry such as
  `/v1/messages` to a Codex model clamps today, and that is the case the switch turns into
  a 400. Native Responses on `support_configuration_update` models is passed through
  unvalidated (`apply.go:268-289`), so the upstream decides there.

Not in scope: a `/cache.html` counter or banner; non-Claude `fallbacks` (only the Claude
wire format has the field).

Tests: `internal/thinking/no_substitution_test.go` (the three refusals on and off, the
message text, cross-family `ultra`, the drop equal to baseline not refused);
`internal/runtime/executor/claude_executor_no_substitution_test.go` (native body with array
and string `fallbacks`, cloaked body with the injection sites forced on, a payload rule that
sets the key, the proxy's beta list with and without `fallback_credit_token`, the native
header untouched, the keepalive probe body stripped and its header kept). Then `make
verify`, `make verify-tagless`, the PATCHES.yaml row (`feat`, noting the 4f5cbbae
exception), `make patches`. Install as v8.0.3-locked.5 only on his word; until then Part 1
is live and Part 2 is on the branch.

## Part 3: Ceeya handoff (written from this seat; pages untouched)

`.ceeya/sittings/2026-10-07-effort-slugs-retired/README.md` in the NewCeeya tree:
- The why, with the probe evidence above and the commands that reproduce it.
- The edit per agent page: 17 pages name a slug (`astra-high`, `fable-high`, `fable-medium`,
  `glm-flash`, `luna-low`, `luna-medium`, `luna-web`, `nutritionist`, `opus`, `reader-opus`,
  `reader-sonnet`, `seed-conductor`, `sol-low`, `sol-medium`, `sol`, `sonnet`, `ui-ux`);
  each becomes the bare id (account prefix and `[1m]` kept) plus an `effort:` line carrying
  the level the slug named. The conductor page stays bare Fable.
- The offline tools that were taught the slugs (`tools/sources/read.ts`, `author.ts`,
  `discover.ts`, `classify-corpus.ts`, the bench rows, `kind-run.sh`, `author-top.ts`)
  can name bare ids again with `--effort`; the slugs keep working, so the change is
  optional and can ride each tool's next edit.
- `decision.md` rows 27 and 28 need a dated addendum: the §2663b slug decision is
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

Each proof has a control so a vacuous pass is visible.

1. Effort resolution. A `cca -p` session at `--settings '{"effortLevel":"low"}'`,
   `CLAUDE_CODE_EFFORT_LEVEL` unset, spawns three pages named and unnamed: a bare-id page
   with `effort: high` (transcripts `effort=high`), a bare-id page with no `effort:` line
   (transcript carries the modelSettings row, medium), and the same with the pin confirmed
   off (no `effort-pin` refusal, no `rewritten by model suffix` line).
2. Fallback strip. Through the side rig's fake upstream (`.ceeya/sittings/2026-09-29-proxy-
   effort-slugs/`, :8398), on `/v1/messages` with a native Opus 5.5 body: run A with the
   Part 1 switches off and `strip-fallbacks` on (ingress carries `fallbacks`, the capture
   does not, the Warn line fires); run B with `strip-fallbacks` off (the capture carries
   it); run C with the switches on (ingress carries no `fallbacks`; no Warn). The cloaked
   injection sites are dormant live (`disable-claude-cloak-mode: true`) and are covered by
   the unit tests only.
3. Effort refusal. `/v1/messages`, `thinking.type: adaptive`, `output_config.effort:
   ultra` on `gpt-6-luna` returns 400 whose message begins `effort_unsupported:` and one
   Error line; the same request with the switch off is clamped to max with the old Warn.
4. Client refusal fallback. The rig's fake upstream answers `stop_reason: refusal` on
   Opus 5.5. Control run without the switches shows a second request on another model in
   the captures (if it does not, the catalog flag `refusal_fallback` never armed through
   the rig and the proof is marked not reproducible, not passed). Run with the switches
   shows the refusal in the turn's result and no second request.
5. Compaction. `/compact` on a `cca` Fable 5.1 session with the switches set completes.

## Rollback

Part 1 is file edits with the previous values named here; `CLAUDE_CODE_NO_MODEL_FALLBACK`
alone can be removed if proof 5 or a live compaction fails, the other switches stay. Part 2
is a config block: both keys false restores today's behaviour on the next reload without a
reinstall; the pin can be re-enabled the same way.
