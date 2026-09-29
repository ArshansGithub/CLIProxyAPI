package thinking_test

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	_ "github.com/router-for-me/CLIProxyAPI/v8/internal/thinking/provider/claude"
	_ "github.com/router-for-me/CLIProxyAPI/v8/internal/thinking/provider/codex"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/tidwall/gjson"
)

// captureWarnings attaches a hook to the standard logger at Info level, the
// proxy's default, so a test sees exactly what an operator would see.
func captureWarnings(t *testing.T) *logtest.Hook {
	t.Helper()
	logger := log.StandardLogger()
	previousLevel := logger.GetLevel()
	previousHooks := logger.ReplaceHooks(make(log.LevelHooks))
	hook := logtest.NewLocal(logger)
	logger.SetLevel(log.InfoLevel)
	t.Cleanup(func() {
		logger.ReplaceHooks(previousHooks)
		logger.SetLevel(previousLevel)
	})
	return hook
}

func warningsMentioning(hook *logtest.Hook, fragment string) []*log.Entry {
	var out []*log.Entry
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.WarnLevel && strings.Contains(entry.Message, fragment) {
			out = append(out, entry)
		}
	}
	return out
}

func codexModel(id string, levels []string, supportsUpdates bool) *registry.ModelInfo {
	return &registry.ModelInfo{
		ID:                         id,
		Type:                       "openai",
		Thinking:                   &registry.ThinkingSupport{Levels: levels},
		SupportConfigurationUpdate: supportsUpdates,
	}
}

var lunaLevels = []string{"low", "medium", "high", "xhigh", "max"}

func embeddedCodexPlusModel(t *testing.T, id string) *registry.ModelInfo {
	t.Helper()
	for _, m := range registry.GetCodexPlusModels() {
		if m != nil && m.ID == id {
			return m
		}
	}
	t.Fatalf("embedded codex-plus catalog has no %s", id)
	return nil
}

// Claude Code asking for "ultra" on gpt-6.1-sol: the Codex client catalog
// lists ultra, but the backend rejects it on the wire, so the embedded row
// stops at max and the request must reach Codex as "max", with a warning.
func TestClaudeToCodexUltraOnGPT61SolClampsToMax(t *testing.T) {
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"ultra"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"ultra"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "max" {
		t.Fatalf("reasoning.effort = %q, want max", got)
	}
	if warnings := warningsMentioning(hook, "level clamped"); len(warnings) != 1 {
		t.Fatalf("want exactly one clamp warning, got %d: %v", len(warnings), hook.AllEntries())
	}
}

// Every level the backend accepts for gpt-6.1-sol passes through untouched
// and without a warning.
func TestClaudeToCodexSupportedLevelsOnGPT61SolPassThrough(t *testing.T) {
	hook := captureWarnings(t)
	info := embeddedCodexPlusModel(t, "gpt-6.1-sol")
	for _, level := range []string{"low", "medium", "high", "xhigh", "max"} {
		hook.Reset()
		source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"` + level + `"}}`)
		body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"` + level + `"}}`)
		out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", info)
		if err != nil {
			t.Fatalf("%s: ApplyThinking returned error: %v", level, err)
		}
		if got := gjson.GetBytes(out, "reasoning.effort").String(); got != level {
			t.Fatalf("%s: reasoning.effort = %q", level, got)
		}
		if warnings := warningsMentioning(hook, "thinking:"); len(warnings) != 0 {
			t.Fatalf("%s: unexpected warnings for a supported level: %v", level, warnings)
		}
	}
}

// "ultra" on a model without it clamps to the nearest supported level, "max",
// and says so at Warn so the rewrite is visible at the default log level.
func TestClaudeToCodexUltraClampsToMaxWithWarning(t *testing.T) {
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6-luna","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"ultra"}}`)
	body := []byte(`{"model":"gpt-6-luna","input":[],"reasoning":{"effort":"ultra"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6-luna", "claude", "codex", "codex", codexModel("gpt-6-luna", lunaLevels, true))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "max" {
		t.Fatalf("reasoning.effort = %q, want max", got)
	}
	warnings := warningsMentioning(hook, "level clamped")
	if len(warnings) != 1 {
		t.Fatalf("want exactly one clamp warning, got %d: %v", len(warnings), hook.AllEntries())
	}
	fields := warnings[0].Data
	if fields["model"] != "gpt-6-luna" || fields["original_value"] != "ultra" || fields["clamped_to"] != "max" {
		t.Fatalf("clamp warning fields = %v", fields)
	}
}

// "max" on a model whose top level is "xhigh" is remapped by the high-intent
// rule before validation. That remap must also warn.
func TestClaudeToCodexMaxRemappedToXHighWithWarning(t *testing.T) {
	hook := captureWarnings(t)
	levels := []string{"low", "medium", "high", "xhigh"}
	source := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"max"}}`)
	body := []byte(`{"model":"gpt-5.5","input":[],"reasoning":{"effort":"max"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-5.5", "claude", "codex", "codex", codexModel("gpt-5.5", levels, true))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "xhigh" {
		t.Fatalf("reasoning.effort = %q, want xhigh", got)
	}
	warnings := warningsMentioning(hook, "level remapped")
	if len(warnings) != 1 {
		t.Fatalf("want exactly one remap warning, got %d: %v", len(warnings), hook.AllEntries())
	}
	fields := warnings[0].Data
	if fields["model"] != "gpt-5.5" || fields["original_value"] != "max" || fields["remapped_to"] != "xhigh" {
		t.Fatalf("remap warning fields = %v", fields)
	}
}

// A Codex client that changes effort mid-turn sends configuration_update
// items. When the selected model does not support them they are removed, and
// the removal must be visible rather than silent.
func TestCodexConfigurationUpdatesStrippedWithWarning(t *testing.T) {
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"medium"},"input":[{"type":"configuration_update","reasoning":{"effort":"xhigh"}},{"type":"message","role":"user","content":"hi"}]}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", codexModel("gpt-6.0-sol", lunaLevels, false))
	if err != nil {
		t.Fatalf("ApplyThinking returned error: %v", err)
	}
	if got := gjson.GetBytes(out, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1 (update stripped)", got)
	}
	warnings := warningsMentioning(hook, "configuration_update")
	if len(warnings) != 1 {
		t.Fatalf("want exactly one strip warning, got %d: %v", len(warnings), hook.AllEntries())
	}
	fields := warnings[0].Data
	if fields["model"] != "gpt-6.0-sol" || fields["dropped_effort"] != "xhigh" {
		t.Fatalf("strip warning fields = %v", fields)
	}
}

// The clamp table must know "ultra" sits above "max", so an unsupported "max"
// never clamps upward into "ultra" on a tie and "ultra" clamps down to "max".
func TestValidateConfigOrdersUltraAboveMax(t *testing.T) {
	info := codexModel("gpt-6-luna", lunaLevels, true)
	validated, err := thinking.ValidateConfig(thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelUltra}, info, "claude", "codex", false)
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if validated.Level != thinking.LevelMax {
		t.Fatalf("ultra on a max-topped model clamped to %q, want max", validated.Level)
	}
	onlyUltra := codexModel("hypothetical", []string{"low", "ultra"}, true)
	validated, err = thinking.ValidateConfig(thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMax}, onlyUltra, "claude", "codex", false)
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if validated.Level != thinking.LevelUltra {
		t.Fatalf("max on a [low, ultra] model clamped to %q, want ultra (nearest)", validated.Level)
	}
}
