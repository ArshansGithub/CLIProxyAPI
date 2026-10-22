package registry

import (
	"encoding/json"
	"testing"
)

// TestEmbeddedCatalogsCarryGPT61Sol guards the fork's gpt-6.1-sol rows. The
// live Codex catalog (chatgpt.com/backend-api/codex/models, 2026-09-29) lists
// gpt-6.1-sol for team, plus and pro plans, not free, with in-turn effort
// updates. The Codex client catalog advertises reasoning levels low through
// ultra, but the backend rejects reasoning.effort=ultra on the wire (it stops
// at max), so models.json carries low through max and the proxy clamps ultra
// to max with a warning. A rebase that drops the row, or a refresh that adds
// "ultra" back, would make the proxy reject the model or forward a level the
// backend refuses.
func TestEmbeddedCatalogsCarryGPT61Sol(t *testing.T) {
	wantLevels := []string{"low", "medium", "high", "xhigh", "max"}
	wantClientLevels := []string{"low", "medium", "high", "xhigh", "max", "ultra"}
	for name, models := range map[string][]*ModelInfo{
		"codex-team": GetCodexTeamModels(),
		"codex-plus": GetCodexPlusModels(),
		"codex-pro":  GetCodexProModels(),
	} {
		var found *ModelInfo
		for _, m := range models {
			if m != nil && m.ID == "gpt-6.1-sol" {
				found = m
				break
			}
		}
		if found == nil {
			t.Errorf("%s: gpt-6.1-sol missing from embedded models.json", name)
			continue
		}
		if found.Version != "gpt-6.1" {
			t.Errorf("%s: gpt-6.1-sol version = %q, want gpt-6.1", name, found.Version)
		}
		if !found.SupportConfigurationUpdate {
			t.Errorf("%s: gpt-6.1-sol must support configuration updates (in-turn effort changes)", name)
		}
		if found.Thinking == nil {
			t.Errorf("%s: gpt-6.1-sol has no thinking support", name)
			continue
		}
		if got := found.Thinking.Levels; !equalStrings(got, wantLevels) {
			t.Errorf("%s: gpt-6.1-sol levels = %v, want %v", name, got, wantLevels)
		}
	}
	for _, m := range GetCodexFreeModels() {
		if m != nil && m.ID == "gpt-6.1-sol" {
			t.Errorf("codex-free: gpt-6.1-sol listed, but free-plan accounts do not receive it")
		}
	}

	var payload struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(GetCodexClientModelsJSON(), &payload); err != nil {
		t.Fatalf("decode embedded codex_client_models.json: %v", err)
	}
	var row map[string]any
	for _, m := range payload.Models {
		if m["slug"] == "gpt-6.1-sol" {
			row = m
			break
		}
	}
	if row == nil {
		t.Fatal("codex_client_models.json: gpt-6.1-sol missing")
	}
	rawLevels, _ := row["supported_reasoning_levels"].([]any)
	var levels []string
	for _, raw := range rawLevels {
		if level, ok := raw.(map[string]any); ok {
			if effort, ok := level["effort"].(string); ok {
				levels = append(levels, effort)
			}
		}
	}
	if !equalStrings(levels, wantClientLevels) {
		t.Errorf("codex_client_models.json: gpt-6.1-sol levels = %v, want %v", levels, wantClientLevels)
	}
	if row["supports_reasoning_effort_updates"] != true {
		t.Errorf("codex_client_models.json: gpt-6.1-sol supports_reasoning_effort_updates = %v, want true", row["supports_reasoning_effort_updates"])
	}
}

// TestEmbeddedCodexRowsNeverAdvertiseUltra pins the wire truth: the Codex
// backend rejects reasoning.effort=ultra ("Supported values are: none,
// minimal, low, medium, high, xhigh, and max", checked 2026-09-29), so no
// models.json row may list it. A row that did would make the proxy forward
// "ultra" instead of clamping it to max.
func TestEmbeddedCodexRowsNeverAdvertiseUltra(t *testing.T) {
	for name, models := range map[string][]*ModelInfo{
		"codex-free": GetCodexFreeModels(),
		"codex-team": GetCodexTeamModels(),
		"codex-plus": GetCodexPlusModels(),
		"codex-pro":  GetCodexProModels(),
	} {
		for _, m := range models {
			if m == nil || m.Thinking == nil {
				continue
			}
			for _, l := range m.Thinking.Levels {
				if l == "ultra" {
					t.Errorf("%s: %s advertises ultra, which the Codex backend rejects on the wire", name, m.ID)
				}
			}
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
