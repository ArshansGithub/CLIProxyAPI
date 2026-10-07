package thinking

import (
	"strings"

	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func isResponsesFormat(format string) bool {
	return format == "codex" || format == "openai-response"
}

// extractConfigurationUpdateConfig returns the last nonempty Responses effort update.
func extractConfigurationUpdateConfig(body []byte) ThinkingConfig {
	if len(body) == 0 || !ojson.ValidBytes(body) {
		return ThinkingConfig{}
	}
	return extractConfigurationUpdateConfigFromInput(ojson.GetBytesNoCopy(body, "input"))
}

func extractConfigurationUpdateConfigDoc(doc *ojson.Doc, docValid bool) ThinkingConfig {
	if doc == nil || !docValid {
		return ThinkingConfig{}
	}
	return extractConfigurationUpdateConfigFromInput(doc.Get("input"))
}

func extractConfigurationUpdateConfigFromInput(input ojson.Result) ThinkingConfig {
	if !input.IsArray() {
		return ThinkingConfig{}
	}

	var effort string
	input.ForEach(func(_, item ojson.Result) bool {
		if item.Get("type").String() == "configuration_update" {
			value := item.Get("reasoning.effort")
			if value.Type == ojson.String {
				if normalized := strings.ToLower(strings.TrimSpace(value.String())); normalized != "" {
					effort = normalized
				}
			}
		}
		return true
	})
	switch effort {
	case "":
		return ThinkingConfig{}
	case "none":
		return ThinkingConfig{Mode: ModeNone, Budget: 0}
	case "auto":
		return ThinkingConfig{Mode: ModeAuto, Budget: -1}
	default:
		return ThinkingConfig{Mode: ModeLevel, Level: ThinkingLevel(effort)}
	}
}

// CollapseConfigurationUpdates removes every configuration_update input item.
// Unless keepTopLevelEffort is true, the last nonempty update effort is also
// promoted into the top-level reasoning.effort. Targets that reject
// configuration_update items entirely (for example Sign in with ChatGPT
// subscriptions) use it to degrade to the classic request-level effort,
// accepting the resulting prompt-cache loss. Keep the top-level effort when
// an explicit model thinking suffix already resolved the operator's override
// into it. A body without array input or without any configuration_update
// item is returned unchanged, and the top-level effort is only touched when
// an update carries a usable effort.
func CollapseConfigurationUpdates(body []byte, keepTopLevelEffort ...bool) []byte {
	if len(body) == 0 || !ojson.ValidBytes(body) {
		return body
	}
	doc := ojson.NewDoc(body)
	if !collapseConfigurationUpdatesDoc(doc, true, keepTopLevelEffort...) {
		return body
	}
	return doc.Bytes()
}

// CollapseConfigurationUpdatesDoc removes every configuration_update input item
// on doc and optionally promotes the last nonempty update effort to reasoning.effort.
func CollapseConfigurationUpdatesDoc(doc *ojson.Doc, keepTopLevelEffort ...bool) {
	_ = collapseConfigurationUpdatesDoc(doc, true, keepTopLevelEffort...)
}

func collapseConfigurationUpdatesDoc(doc *ojson.Doc, docValid bool, keepTopLevelEffort ...bool) bool {
	if doc == nil || !docValid {
		return false
	}
	effort := ""
	if !(len(keepTopLevelEffort) > 0 && keepTopLevelEffort[0]) {
		effort = reasoningEffortFromConfig(extractConfigurationUpdateConfigDoc(doc, docValid))
	}
	modified := stripConfigurationUpdatesDoc(doc, docValid)
	if effort != "" {
		if doc.Set("reasoning.effort", effort) == nil {
			modified = true
		}
	}
	return modified
}

// StripConfigurationUpdates removes configuration_update input items without changing reasoning.
func StripConfigurationUpdates(body []byte) []byte {
	return stripConfigurationUpdates(body)
}

// StripConfigurationUpdatesDoc removes configuration_update input items on doc.
func StripConfigurationUpdatesDoc(doc *ojson.Doc) {
	_ = stripConfigurationUpdatesDoc(doc, true)
}

// stripConfigurationUpdatesLoggedDoc strips unsupported configuration_update
// items and, when any carried a reasoning effort, warns with the model and the
// effort that was dropped. A Codex client that changed effort mid-turn would
// otherwise keep running at the top-level baseline with nothing in the log.
func stripConfigurationUpdatesLoggedDoc(doc *ojson.Doc, docValid bool, provider, model string, modelInfo *registry.ModelInfo) (bool, error) {
	dropped := extractConfigurationUpdateConfigDoc(doc, docValid)
	if !stripConfigurationUpdatesDoc(doc, docValid) {
		return false, nil
	}
	if !hasThinkingConfig(dropped) {
		return true, nil
	}
	if modelInfo != nil && modelInfo.ID != "" {
		model = modelInfo.ID
	}
	droppedEffort := string(dropped.Level)
	switch dropped.Mode {
	case ModeNone:
		droppedEffort = string(LevelNone)
	case ModeAuto:
		droppedEffort = string(LevelAuto)
	}
	baseline := extractCodexConfigFromGetter(doc.Get)
	baselineEffort := string(baseline.Level)
	switch baseline.Mode {
	case ModeNone:
		baselineEffort = string(LevelNone)
	case ModeAuto:
		baselineEffort = string(LevelAuto)
	}
	if RefuseUnsupportedEffort() && !strings.EqualFold(droppedEffort, baselineEffort) {
		// The model cannot change effort mid-turn, so the only effort it can
		// honour is the top-level baseline; that is what it "offers" here.
		offers := []string{baselineEffort}
		if baselineEffort == "" {
			offers = []string{"only the top-level reasoning.effort, which is unset"}
		}
		return true, refuseEffort(provider, model, droppedEffort, offers)
	}
	log.WithFields(log.Fields{
		"provider":       provider,
		"model":          model,
		"dropped_effort": droppedEffort,
		"baseline":       string(baseline.Level),
	}).Warn("thinking: configuration_update effort dropped (model does not support in-turn updates) |")
	return true, nil
}

// stripConfigurationUpdates removes unsupported Responses input items without
// modifying other input items or introducing an input field.
func stripConfigurationUpdates(body []byte) []byte {
	if len(body) == 0 || !ojson.ValidBytes(body) {
		return body
	}
	doc := ojson.NewDoc(body)
	if !stripConfigurationUpdatesDoc(doc, true) {
		return body
	}
	return doc.Bytes()
}

func stripConfigurationUpdatesDoc(doc *ojson.Doc, docValid bool) bool {
	if doc == nil || !docValid {
		return false
	}
	input := doc.Get("input")
	if !input.IsArray() {
		return false
	}

	var kept []string
	removed := false
	input.ForEach(func(_, item ojson.Result) bool {
		if item.Get("type").String() == "configuration_update" {
			removed = true
		} else {
			kept = append(kept, item.Raw)
		}
		return true
	})
	if !removed {
		return false
	}
	return doc.SetRaw("input", "["+strings.Join(kept, ",")+"]") == nil
}

// stripResponsesEffort leaves summary and unrelated reasoning fields intact.
func stripResponsesEffort(body []byte) []byte {
	if len(body) == 0 || !ojson.ValidBytes(body) {
		return body
	}
	doc := ojson.NewDoc(body)
	if !stripResponsesEffortDoc(doc, true) {
		return body
	}
	return doc.Bytes()
}

func stripResponsesEffortDoc(doc *ojson.Doc, docValid bool) bool {
	if doc == nil || !docValid || !doc.Get("reasoning.effort").Exists() {
		return false
	}
	if errDelete := doc.Delete("reasoning.effort"); errDelete != nil {
		return false
	}
	if reasoning := doc.Get("reasoning"); reasoning.IsObject() && len(reasoning.Map()) == 0 {
		_ = doc.Delete("reasoning")
	}
	return true
}
