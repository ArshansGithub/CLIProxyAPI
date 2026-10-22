package thinking_test

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	_ "github.com/router-for-me/CLIProxyAPI/v8/internal/thinking/provider/claude"
	_ "github.com/router-for-me/CLIProxyAPI/v8/internal/thinking/provider/codex"
	"github.com/tidwall/gjson"
)

// Effort-suffixed slugs: "claude-opus-5-5-medium" is "claude-opus-5-5(medium)"
// spelled the way an agent definition can carry it. The hyphen form splits
// only when the whole id is not itself a catalog model and the remainder is.

func TestParseSuffixHyphenLevelOnCatalogModel(t *testing.T) {
	for _, tc := range []struct{ in, base, raw string }{
		{"claude-opus-5-5-medium", "claude-opus-5-5", "medium"},
		{"claude-fable-5-1-high", "claude-fable-5-1", "high"},
		{"gpt-6.1-sol-xhigh", "gpt-6.1-sol", "xhigh"},
		{"gpt-6-luna-low", "gpt-6-luna", "low"},
		{"claude-sonnet-5-5-MAX", "claude-sonnet-5-5", "MAX"},
	} {
		got := thinking.ParseSuffix(tc.in)
		if !got.HasSuffix || got.ModelName != tc.base || got.RawSuffix != tc.raw {
			t.Fatalf("%s: got %+v, want base=%s raw=%s", tc.in, got, tc.base, tc.raw)
		}
		if level, ok := thinking.ParseLevelSuffix(got.RawSuffix); !ok || level == "" {
			t.Fatalf("%s: raw suffix %q is not a level", tc.in, got.RawSuffix)
		}
	}
}

func TestParseSuffixHyphenKeepsRealCatalogIDs(t *testing.T) {
	for _, id := range []string{"gemini-3.1-pro-low", "gpt-oss-120b-medium"} {
		if registry.LookupStaticModelInfo(id) == nil {
			t.Fatalf("fixture %s is no longer a catalog model; pick another real id ending in a level word", id)
		}
		got := thinking.ParseSuffix(id)
		if got.HasSuffix || got.ModelName != id {
			t.Fatalf("%s is a real model id and must not be split, got %+v", id, got)
		}
	}
}

func TestParseSuffixHyphenUnknownRemainderPassesThrough(t *testing.T) {
	for _, id := range []string{"no-such-model-high", "-high", "scoped/-low", "claude-opus-5-5-ultra", "claude-opus-5-5-none"} {
		got := thinking.ParseSuffix(id)
		if got.HasSuffix || got.ModelName != id {
			t.Fatalf("%s: must pass through untouched, got %+v", id, got)
		}
	}
}

func TestParseSuffixHyphenOnPrefixedClone(t *testing.T) {
	reg := registry.GetGlobalRegistry()
	clone := registry.LookupStaticModelInfo("claude-opus-5-5")
	if clone == nil {
		t.Fatal("static catalog has no claude-opus-5-5")
	}
	clone.ID = "scoped/claude-opus-5-5"
	reg.RegisterClient("test-scoped-account", "claude", []*registry.ModelInfo{clone})
	t.Cleanup(func() { reg.UnregisterClient("test-scoped-account") })

	got := thinking.ParseSuffix("scoped/claude-opus-5-5-medium")
	if !got.HasSuffix || got.ModelName != "scoped/claude-opus-5-5" || got.RawSuffix != "medium" {
		t.Fatalf("prefixed slug: got %+v", got)
	}
	if got := thinking.ParseSuffix("side/claude-opus-5-5-medium"); got.HasSuffix {
		t.Fatalf("side/ is not registered here and must not split: %+v", got)
	}
}

func TestParseSuffixParenFormStillWins(t *testing.T) {
	got := thinking.ParseSuffix("claude-opus-5-5-medium(high)")
	if !got.HasSuffix || got.RawSuffix != "high" || got.ModelName != "claude-opus-5-5-medium" {
		t.Fatalf("paren form must be parsed first, got %+v", got)
	}
}

// A Claude Code caller sending output_config.effort=high to the -medium slug
// reaches Anthropic at medium, and the rewrite is a Warn line.
func TestClaudeSlugForcesEffortAndWarns(t *testing.T) {
	hook := captureWarnings(t)
	info := registry.LookupStaticModelInfo("claude-opus-5-5")
	body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "claude-opus-5-5-medium", "claude", "claude", "claude", info)
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "output_config.effort").String(); got != "medium" {
		t.Fatalf("output_config.effort = %q, want medium", got)
	}
	warnings := warningsMentioning(hook, "rewritten by model suffix")
	if len(warnings) != 1 {
		t.Fatalf("want exactly one rewrite warning, got %d: %v", len(warnings), hook.AllEntries())
	}
	if warnings[0].Data["original_value"] != "high" || warnings[0].Data["rewritten_to"] != "medium" || warnings[0].Data["model"] != "claude-opus-5-5-medium" {
		t.Fatalf("warning fields: %v", warnings[0].Data)
	}
}

// The same on the Codex path: gpt-6.1-sol-high sent with effort low goes up
// at high, with the Warn line.
func TestCodexSlugForcesEffortAndWarns(t *testing.T) {
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol-high","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"low"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"low"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol-high", "claude", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want high", got)
	}
	warnings := warningsMentioning(hook, "rewritten by model suffix")
	if len(warnings) != 1 {
		t.Fatalf("want exactly one rewrite warning, got %d: %v", len(warnings), hook.AllEntries())
	}
	if warnings[0].Data["original_value"] != "low" || warnings[0].Data["rewritten_to"] != "high" {
		t.Fatalf("warning fields: %v", warnings[0].Data)
	}
}

// A slug request that carries no effort of its own is forced without noise.
func TestSlugWithoutCallerEffortWarnsNothing(t *testing.T) {
	hook := captureWarnings(t)
	info := registry.LookupStaticModelInfo("claude-opus-5-5")
	body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"}]}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "claude-opus-5-5-medium", "claude", "claude", "claude", info)
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "output_config.effort").String(); got != "medium" {
		t.Fatalf("output_config.effort = %q, want medium", got)
	}
	if warnings := warningsMentioning(hook, "thinking:"); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

// The same effort in body and slug is not a rewrite.
func TestSlugMatchingCallerEffortWarnsNothing(t *testing.T) {
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"high"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.1-sol-high", "codex", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want high", got)
	}
	if warnings := warningsMentioning(hook, "thinking:"); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

// Bare slugs keep working as they do now: the caller's effort passes through.
func TestBareSlugStillHonoursCallerEffort(t *testing.T) {
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"low"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.1-sol", "codex", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "low" {
		t.Fatalf("reasoning.effort = %q, want low", got)
	}
	if warnings := warningsMentioning(hook, "thinking:"); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestParseSuffixHyphenMarksHyphenated(t *testing.T) {
	if got := thinking.ParseSuffix("gpt-6.1-sol-high"); !got.Hyphenated {
		t.Fatalf("hyphen slug must be marked Hyphenated: %+v", got)
	}
	if got := thinking.ParseSuffix("gpt-6.1-sol(high)"); got.Hyphenated {
		t.Fatalf("paren form must not be marked Hyphenated: %+v", got)
	}
}
