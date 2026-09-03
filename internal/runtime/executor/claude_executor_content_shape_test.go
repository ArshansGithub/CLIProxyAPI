package executor

import (
	"bytes"
	"testing"

	"strconv"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// The two request bodies below are the shape Claude Code 2.1.252 sends for the
// same injected teammate message on consecutive requests: block-wrapped with the
// breakpoint while it is the final message, then a bare string once it is not.
// Captured from a live session; only the text is shortened.
const contentShapeReqLast = `{"model":"claude-fable-5-1","messages":[
{"role":"user","content":[{"type":"text","text":"spawn two haiku agents"}]},
{"role":"assistant","content":[{"type":"text","text":"Spawning both now."}]},
{"role":"user","content":[{"type":"text","text":"Another Claude session sent a message:\n<teammate-message teammate_id=\"sleeper-1\">done</teammate-message>","cache_control":{"type":"ephemeral"}}]}
]}`

const contentShapeReqNext = `{"model":"claude-fable-5-1","messages":[
{"role":"user","content":[{"type":"text","text":"spawn two haiku agents"}]},
{"role":"assistant","content":[{"type":"text","text":"Spawning both now."}]},
{"role":"user","content":"Another Claude session sent a message:\n<teammate-message teammate_id=\"sleeper-1\">done</teammate-message>"},
{"role":"assistant","content":[{"type":"text","text":"Agent 1 reported in."}]},
{"role":"user","content":[{"type":"text","text":"Another Claude session sent a message:\n<teammate-message teammate_id=\"sleeper-2\">done</teammate-message>","cache_control":{"type":"ephemeral"}}]}
]}`

func stripCacheControl(t *testing.T, payload []byte, path string) []byte {
	t.Helper()
	out, err := sjson.DeleteBytes(payload, path)
	if err != nil {
		t.Fatalf("delete %s: %v", path, err)
	}
	return out
}

func TestNormalizeClaudeMessageContentShape_StringBecomesTextBlock(t *testing.T) {
	out := normalizeClaudeMessageContentShape([]byte(contentShapeReqNext), false)
	got := gjson.GetBytes(out, "messages.2.content")
	if !got.IsArray() || len(got.Array()) != 1 {
		t.Fatalf("messages.2.content should be a single-block array, got %s", got.Raw)
	}
	if got.Get("0.type").String() != "text" {
		t.Fatalf("block type = %q, want text", got.Get("0.type").String())
	}
	want := gjson.Get(contentShapeReqNext, "messages.2.content").String()
	if got.Get("0.text").String() != want {
		t.Fatalf("text mismatch:\n got %q\nwant %q", got.Get("0.text").String(), want)
	}
	// Block-form messages are untouched, byte for byte.
	for _, i := range []string{"0", "1", "3", "4"} {
		if gjson.GetBytes(out, "messages."+i).Raw != gjson.Get(contentShapeReqNext, "messages."+i).Raw {
			t.Fatalf("messages.%s was modified", i)
		}
	}
}

// The property the fix exists for: after normalization, the shared prefix of
// two consecutive requests is byte-identical once the moving breakpoint is
// removed — so the earlier request's cache entry is a prefix match for the
// later one.
func TestNormalizeClaudeMessageContentShape_ConsecutiveRequestsSharePrefix(t *testing.T) {
	last := normalizeClaudeMessageContentShape([]byte(contentShapeReqLast), false)
	next := normalizeClaudeMessageContentShape([]byte(contentShapeReqNext), false)

	last = stripCacheControl(t, last, "messages.2.content.0.cache_control")
	next = stripCacheControl(t, next, "messages.4.content.0.cache_control")

	for i := 0; i < 3; i++ {
		a := gjson.GetBytes(last, "messages."+strconv.Itoa(i)).Raw
		b := gjson.GetBytes(next, "messages."+strconv.Itoa(i)).Raw
		if a != b {
			t.Fatalf("messages[%d] differs between consecutive requests after normalization:\n last=%s\n next=%s", i, a, b)
		}
	}

	// And without the fix the third message really does differ (the regression the test guards).
	rawLast := stripCacheControl(t, []byte(contentShapeReqLast), "messages.2.content.0.cache_control")
	if gjson.GetBytes(rawLast, "messages.2").Raw == gjson.Get(contentShapeReqNext, "messages.2").Raw {
		t.Fatalf("fixture no longer reproduces the shape flip")
	}
}

// A markerless request has no cached prefix to protect, and native Claude Code
// helper calls must reach Anthropic exactly as the client sent them, so the
// rewrite must not fire when no breakpoint is in play.
func TestNormalizeClaudeMessageContentShape_SkipsMarkerlessRequest(t *testing.T) {
	in := []byte(`{"model":"claude-haiku-4-5","max_tokens":1,"messages":[{"role":"user","content":"helper probe"}]}`)
	if got := normalizeClaudeMessageContentShape(in, false); !bytes.Equal(got, in) {
		t.Fatalf("markerless request was rewritten:\n got %s\nwant %s", got, in)
	}
	// The same body is normalized once CPA is about to place the breakpoint
	// itself, which is the cloaked path that produces the identical flip.
	got := normalizeClaudeMessageContentShape(in, true)
	if !gjson.GetBytes(got, "messages.0.content").IsArray() {
		t.Fatalf("cpa-owned request should be normalized, got %s", got)
	}
}

// rebuildMidSystemMessage consumes mid-conversation system messages in the shape
// they arrived, and Anthropic only accepts user/assistant on the wire anyway.
func TestNormalizeClaudeMessageContentShape_SkipsNonUserAssistantRoles(t *testing.T) {
	in := []byte(`{"system":[{"type":"text","text":"Top rule","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]},{"role":"system","content":"Mid rule"},{"role":"user","content":"continue"}]}`)
	out := normalizeClaudeMessageContentShape(in, false)
	if got := gjson.GetBytes(out, `messages.#(role=="system").content`); got.Type != gjson.String || got.String() != "Mid rule" {
		t.Fatalf("mid system message = %s, want the original string preserved", got.Raw)
	}
	if !gjson.GetBytes(out, "messages.2.content").IsArray() {
		t.Fatalf("user message should still be normalized, got %s", out)
	}
}

func TestNormalizeClaudeMessageContentShape_NoopWhenNothingToDo(t *testing.T) {
	in := []byte(contentShapeReqLast)
	out := normalizeClaudeMessageContentShape(in, false)
	if !bytes.Equal(in, out) {
		t.Fatalf("payload with no string content should be returned unchanged")
	}
	for _, bad := range [][]byte{nil, []byte(""), []byte("{not json"), []byte(`{"messages":"nope"}`)} {
		if got := normalizeClaudeMessageContentShape(bad, true); !bytes.Equal(got, bad) {
			t.Fatalf("malformed input %q should pass through unchanged", bad)
		}
	}
}
