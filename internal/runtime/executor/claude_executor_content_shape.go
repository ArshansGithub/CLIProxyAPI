package executor

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// normalizeClaudeMessageContentShape rewrites every user/assistant
// messages[].content that is a bare JSON string into the equivalent single text
// block [{"type":"text","text":<string>}].
//
// Anthropic accepts both shapes and treats them identically for inference, but
// prompt caching is a byte-prefix match, so the two shapes are different cache
// keys. The rolling breakpoint exploits that difference by accident: it is
// attached by wrapping the final message's string content into a text block
// ({"content":[{"type":"text","text":…,"cache_control":…}]}) and, on the next
// request, once that message is no longer last, the same message is emitted
// again as the original bare string. Every message that arrives as a string —
// injected teammate/agent messages, task notifications, hook system turns, and
// plain typed prompts — therefore flips shape exactly once, invalidating the
// cached prefix from that message to the end of the conversation. In sessions
// that run many subagents this is the dominant source of cache_creation tokens.
//
// Both sides of the breakpoint regime produce the flip: Claude Code does it
// itself, and for a cloaked caller injectMessagesCacheControl does it on the
// caller's behalf at lastEligibleIndex. cpaOwnsCacheControl covers the second
// case; countCacheControls covers the first.
//
// The rewrite is gated on a breakpoint actually being in play. A request with
// no cache_control anywhere that CPA is not about to annotate has no cached
// prefix to stabilize, so normalizing it would buy nothing and cost wire
// fidelity — native Claude Code helper calls are markerless and must reach
// Anthropic byte-for-byte as the client sent them.
//
// Normalizing to block form before breakpoint placement makes the two requests
// byte-identical up to the breakpoint itself, which sits after the prefix and
// costs nothing. The rewrite is shape-only: text, role, order, and any
// cache_control the caller placed are preserved, and non-string content is
// never touched. Roles other than user/assistant are skipped so that a
// mid-conversation system message stays exactly as it arrived for
// rebuildMidSystemMessage to consume.
func normalizeClaudeMessageContentShape(payload []byte, cpaOwnsCacheControl bool) []byte {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload
	}
	if !cpaOwnsCacheControl && countCacheControls(payload) == 0 {
		return payload
	}
	messages := gjson.GetBytes(payload, "messages")
	if !messages.IsArray() {
		return payload
	}
	var updated []byte
	messages.ForEach(func(idx, message gjson.Result) bool {
		if role := message.Get("role").String(); role != "user" && role != "assistant" {
			return true
		}
		content := message.Get("content")
		if content.Type != gjson.String {
			return true
		}
		// Re-emit the string with the same escaping the caller used for its own
		// block-form messages (no HTML escaping), so the normalized bytes match
		// what Claude Code sends when it wraps the same text itself.
		block := `[{"type":"text","text":` + marshalJSONStringWithoutHTMLEscape(content.String()) + `}]`
		if updated == nil {
			updated = payload
		}
		next, errSet := sjson.SetRawBytes(updated, "messages."+idx.String()+".content", []byte(block))
		if errSet != nil {
			return true
		}
		updated = next
		return true
	})
	if updated == nil {
		return payload
	}
	return updated
}
