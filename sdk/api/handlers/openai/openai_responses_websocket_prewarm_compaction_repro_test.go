package openai

import (
	"testing"

	"github.com/tidwall/gjson"
)

// Reproduces a Codex Desktop subagent spawned after the parent thread compacted.
// Frame 1 (prewarm, generate:false) carries the tools as an additional_tools item.
// Frame 2 references the local prewarm id and carries the forked parent history,
// which includes a compaction item but no additional_tools item.
// With a Codex HTTP upstream the proxy must forward the tools it was given.
func TestPrewarmToolsSurviveCompactionReplayBypass(t *testing.T) {
	prewarm := []byte(`{"type":"response.create","model":"gpt-6-astra","generate":false,"input":[
		{"type":"additional_tools","id":"at_1","role":"developer","tools":[{"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec","description":"run js"}]}]},
		{"type":"message","role":"developer","id":"dev_1","content":[{"type":"input_text","text":"base instructions"}]}
	],"stream":true,"store":false}`)
	_, lastRequest, errMsg := normalizeResponseCreateRequest(prewarm)
	if errMsg != nil {
		t.Fatalf("prewarm normalize: %v", errMsg.Error)
	}

	continuation := []byte(`{"type":"response.create","model":"gpt-6-astra","previous_response_id":"resp_prewarm_abc","input":[
		{"type":"compaction","id":"cmp_1","encrypted_content":"opaque"},
		{"type":"message","role":"assistant","id":"asst_1","content":[{"type":"output_text","text":"earlier answer"}]},
		{"type":"agent_message","id":"amsg_1","content":[{"type":"input_text","text":"run ls -la"}]}
	],"stream":true,"store":false}`)

	// v7.2.159 (bd03aabc) owns this path: a followup that references the pending
	// synthetic prewarm id is merged with the warm-up request rather than routed
	// through the incremental-state normalizer, which is what the carried #5566
	// pick used to do. The regression this test pins is unchanged: the tools the
	// prewarm frame carried must still reach the upstream request.
	normalized, _, errMsg := normalizeResponsesWebsocketPrewarmFollowup(continuation, lastRequest)
	if errMsg != nil {
		t.Fatalf("continuation normalize: %v", errMsg.Error)
	}
	input := gjson.GetBytes(normalized, "input").Array()
	hasTools := false
	for _, item := range input {
		if item.Get("type").String() == "additional_tools" {
			hasTools = true
		}
	}
	if !hasTools {
		types := make([]string, 0, len(input))
		for _, item := range input {
			types = append(types, item.Get("type").String())
		}
		t.Fatalf("upstream request lost the prewarm additional_tools item; input types = %v", types)
	}
}
