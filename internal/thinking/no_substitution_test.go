package thinking_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/tidwall/gjson"
)

func TestRefuseUnsupportedEffortSwitch(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	if thinking.RefuseUnsupportedEffort() {
		t.Fatal("default must be off")
	}
	thinking.SetRefuseUnsupportedEffort(true)
	if !thinking.RefuseUnsupportedEffort() {
		t.Fatal("switch did not turn on")
	}
}

func errorsMentioning(hook *logtest.Hook, fragment string) []*log.Entry {
	var out []*log.Entry
	for _, entry := range hook.AllEntries() {
		if entry.Level == log.ErrorLevel && strings.Contains(entry.Message, fragment) {
			out = append(out, entry)
		}
	}
	return out
}

// The cross-family clamp (Claude wire asking "ultra" of gpt-6.1-sol) becomes
// a 400 under the switch, and stays a clamp with the old Warn without it.
func TestClampRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"ultra"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"ultra"}}`)
	info := embeddedCodexPlusModel(t, "gpt-6.1-sol")

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", info)
	if err == nil {
		t.Fatal("want an error with the switch on")
	}
	var te *thinking.ThinkingError
	if !errors.As(err, &te) || te.StatusCode() != 400 {
		t.Fatalf("error = %T %v, want *ThinkingError with 400", err, err)
	}
	if !strings.HasPrefix(err.Error(), "effort_unsupported: model gpt-6.1-sol does not offer effort \"ultra\"; it offers ") {
		t.Fatalf("message = %q", err.Error())
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %d: %v", len(got), hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "max" {
		t.Fatalf("switch off: reasoning.effort = %q, want max", got)
	}
	if got := warningsMentioning(hook, "level clamped"); len(got) != 1 {
		t.Fatalf("switch off: want the old clamp Warn, got %v", hook.AllEntries())
	}
}

// A supported level crosses families untouched and silently with the switch on.
func TestSupportedLevelPassesWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	hook := captureWarnings(t)
	source := []byte(`{"model":"gpt-6.1-sol","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`)
	body := []byte(`{"model":"gpt-6.1-sol","input":[],"reasoning":{"effort":"high"}}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-6.1-sol", "claude", "codex", "codex", embeddedCodexPlusModel(t, "gpt-6.1-sol"))
	if err != nil {
		t.Fatalf("ApplyThinking: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q", got)
	}
	if len(hook.AllEntries()) != 0 {
		t.Fatalf("unexpected log lines: %v", hook.AllEntries())
	}
}

// The "as high as this model goes" remap (max on a model that stops at xhigh)
// becomes a 400 under the switch. Mirrors the remap case in effort_visibility_test.go.
func TestRemapRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	info := codexModel("gpt-5.5", []string{"low", "medium", "high", "xhigh"}, false)
	source := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"max"}}`)
	body := []byte(`{"model":"gpt-5.5","input":[],"reasoning":{"effort":"max"}}`)

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-5.5", "claude", "codex", "codex", info)
	if err == nil || !strings.Contains(err.Error(), `does not offer effort "max"`) {
		t.Fatalf("want the refusal, got %v", err)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %v", hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, source, "gpt-5.5", "claude", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "reasoning.effort").String(); got != "xhigh" {
		t.Fatalf("switch off: reasoning.effort = %q, want xhigh", got)
	}
	if got := warningsMentioning(hook, "level remapped"); len(got) != 1 {
		t.Fatalf("switch off: want the old remap Warn, got %v", hook.AllEntries())
	}
}

// A Codex client changing effort mid-turn on a model without in-turn updates
// is refused under the switch when the dropped effort differs from the
// top-level baseline, and still stripped with the old Warn without it.
func TestConfigurationUpdateDropRefusedWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"medium"},"input":[{"type":"configuration_update","reasoning":{"effort":"xhigh"}},{"type":"message","role":"user","content":"hi"}]}`)
	info := codexModel("gpt-6.0-sol", lunaLevels, false)

	thinking.SetRefuseUnsupportedEffort(true)
	_, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", info)
	if err == nil || !strings.Contains(err.Error(), `does not offer effort "xhigh"; it offers medium`) {
		t.Fatalf("want the refusal naming the baseline as the offer, got %v", err)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 1 {
		t.Fatalf("want one Error line, got %v", hook.AllEntries())
	}

	hook.Reset()
	thinking.SetRefuseUnsupportedEffort(false)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", info)
	if err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if got := gjson.GetBytes(out, "input.#").Int(); got != 1 {
		t.Fatalf("switch off: input length = %d, want 1", got)
	}
	if got := warningsMentioning(hook, "configuration_update"); len(got) != 1 {
		t.Fatalf("switch off: want the old drop Warn, got %v", hook.AllEntries())
	}
}

// An update whose effort equals the baseline substitutes nothing: stripped
// silently even with the switch on.
func TestConfigurationUpdateEqualToBaselineNotRefused(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	hook := captureWarnings(t)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"medium"},"input":[{"type":"configuration_update","reasoning":{"effort":"medium"}},{"type":"message","role":"user","content":"hi"}]}`)
	out, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", codexModel("gpt-6.0-sol", lunaLevels, false))
	if err != nil {
		t.Fatalf("ApplyThinking: %v", err)
	}
	if got := gjson.GetBytes(out, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1 (update stripped)", got)
	}
	if got := errorsMentioning(hook, "effort refused"); len(got) != 0 {
		t.Fatalf("unexpected refusal: %v", got)
	}
}

// Finding 2 of the branch review: the Kimi family clamp (same wire format,
// different model family) is a kept conversion, not a refusal.
func TestKimiFamilyClampKeptWhenSwitchOn(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	models := registry.GetKimiModels()
	reg := registry.GetGlobalRegistry()
	clientID := "test-no-substitution-kimi"
	reg.RegisterClient(clientID, "kimi", models)
	t.Cleanup(func() { reg.UnregisterClient(clientID) })
	body := []byte(`{"model":"kimi-k2.5","messages":[{"role":"user","content":"hi"}],"thinking":{"type":"adaptive"},"output_config":{"effort":"max"}}`)
	out, err := thinking.ApplyThinking(body, "kimi-k2.5", "claude", "claude", "claude")
	if err != nil {
		t.Fatalf("Kimi max must still clamp, got error: %v", err)
	}
	if got := gjson.GetBytes(out, "output_config.effort").String(); got != "high" {
		t.Fatalf("output_config.effort = %q, want high", got)
	}
}

// Finding 3: an update of "none" against a baseline of "none" substitutes
// nothing and must not be refused.
func TestConfigurationUpdateNoneEqualToNoneBaselineNotRefused(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	body := []byte(`{"model":"gpt-6.0-sol","reasoning":{"effort":"none"},"input":[{"type":"configuration_update","reasoning":{"effort":"none"}},{"type":"message","role":"user","content":"hi"}]}`)
	if _, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", codexModel("gpt-6.0-sol", lunaLevels, false)); err != nil {
		t.Fatalf("none against none must not be refused: %v", err)
	}
}

// Finding 3: with no top-level effort at all, a dropped update is still a
// substitution; the message says what the model is left with.
func TestConfigurationUpdateWithoutBaselineRefusedWithClearMessage(t *testing.T) {
	t.Cleanup(func() { thinking.SetRefuseUnsupportedEffort(false) })
	thinking.SetRefuseUnsupportedEffort(true)
	body := []byte(`{"model":"gpt-6.0-sol","input":[{"type":"configuration_update","reasoning":{"effort":"xhigh"}},{"type":"message","role":"user","content":"hi"}]}`)
	_, err := thinking.ApplyThinkingWithModelInfo(body, body, "gpt-6.0-sol", "codex", "codex", "codex", codexModel("gpt-6.0-sol", lunaLevels, false))
	if err == nil || !strings.Contains(err.Error(), "only the top-level reasoning.effort, which is unset") {
		t.Fatalf("want the unset-baseline wording, got %v", err)
	}
}
