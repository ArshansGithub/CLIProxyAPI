package thinking

import (
	"strings"

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
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ThinkingConfig{}
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return ThinkingConfig{}
	}

	var effort string
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "configuration_update" {
			value := item.Get("reasoning.effort")
			if value.Type == gjson.String {
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

// stripConfigurationUpdatesLogged strips unsupported configuration_update
// items and, when any carried a reasoning effort, warns with the model and the
// effort that was dropped. A Codex client that changed effort mid-turn would
// otherwise keep running at the top-level baseline with nothing in the log.
func stripConfigurationUpdatesLogged(body []byte, provider, model string, modelInfo *registry.ModelInfo) []byte {
	dropped := extractConfigurationUpdateConfig(body)
	stripped := stripConfigurationUpdates(body)
	if !hasThinkingConfig(dropped) || len(stripped) == len(body) {
		return stripped
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
	baseline := extractCodexConfig(stripped)
	log.WithFields(log.Fields{
		"provider":       provider,
		"model":          model,
		"dropped_effort": droppedEffort,
		"baseline":       string(baseline.Level),
	}).Warn("thinking: configuration_update effort dropped (model does not support in-turn updates) |")
	return stripped
}

// stripConfigurationUpdates removes unsupported Responses input items without
// modifying other input items or introducing an input field.
func stripConfigurationUpdates(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body
	}

	var kept []string
	removed := false
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "configuration_update" {
			removed = true
		} else {
			kept = append(kept, item.Raw)
		}
		return true
	})
	if !removed {
		return body
	}
	updated, errSet := sjson.SetRawBytes(body, "input", []byte("["+strings.Join(kept, ",")+"]"))
	if errSet != nil {
		return body
	}
	return updated
}

// stripResponsesEffort leaves summary and unrelated reasoning fields intact.
func stripResponsesEffort(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) || !gjson.GetBytes(body, "reasoning.effort").Exists() {
		return body
	}
	result, errDelete := sjson.DeleteBytes(body, "reasoning.effort")
	if errDelete != nil {
		return body
	}
	if reasoning := gjson.GetBytes(result, "reasoning"); reasoning.IsObject() && len(reasoning.Map()) == 0 {
		result, _ = sjson.DeleteBytes(result, "reasoning")
	}
	return result
}
