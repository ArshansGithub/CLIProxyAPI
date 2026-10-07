package thinking_test

import (
	"errors"
	"strings"
	"testing"

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
