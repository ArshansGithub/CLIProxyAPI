package helps

import (
	"strings"

	"github.com/MoeForge/moego/ojson"
)

// claudeCacheDiagnosticsFromRoot reads the cache_miss_reason diagnostics
// object. A non-streaming body carries it beside its usage node; a streaming
// response carries it once, on message_start, under message. The 5m/1h
// cache_creation split is upstream's (populateClaudeUsageStructuredFields).
func claudeCacheDiagnosticsFromRoot(root ojson.Result) (string, int64) {
	reason := root.Get("diagnostics.cache_miss_reason")
	if !reason.Exists() {
		reason = root.Get("message.diagnostics.cache_miss_reason")
	}
	if !reason.Exists() {
		return "", 0
	}
	return strings.TrimSpace(reason.Get("type").String()), reason.Get("cache_missed_input_tokens").Int()
}

// RequestMaxTokens reads the generation cap from the first payload that carries
// one, trying the Anthropic name and the two OpenAI-family spellings so a
// translated request still reports the cap the client asked for.
func RequestMaxTokens(payloads ...[]byte) int64 {
	for _, payload := range payloads {
		if len(payload) == 0 {
			continue
		}
		for _, field := range []string{"max_tokens", "max_output_tokens", "max_completion_tokens"} {
			if node := ojson.GetBytes(payload, field); node.Exists() {
				if value := node.Int(); value > 0 {
					return value
				}
			}
		}
	}
	return 0
}
