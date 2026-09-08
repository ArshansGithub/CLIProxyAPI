package executor

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/tidwall/gjson"
)

// Native Codex clients (Originator header present) are passed through unchanged:
// neither the constant-union schema simplification nor the empty-incomplete
// failure classification may alter what a real Codex client sent or receives.
func TestNormalizeCodexToolSchemasSkippedForNativeCodexCaller(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","input":[],"tools":[{"type":"function","name":"t1","parameters":{"type":"object","properties":{"action":{"type":"string","oneOf":[
		{"const":"a","description":"A"},{"const":"b","description":"B"},{"const":"c","description":"C"},{"const":"d","description":"D"},
		{"const":"e","description":"E"},{"const":"f","description":"F"},{"const":"g","description":"G"},{"const":"h","description":"H"},
		{"const":"i","description":"I"},{"const":"j","description":"J"},{"const":"k","description":"K"},{"const":"l","description":"L"},{"const":"m","description":"M"}]}}}}]}`)

	native := http.Header{"Originator": []string{"Codex Desktop"}, "User-Agent": []string{"Codex Desktop/26.901.51231 codex-cli/0.153.4"}}
	if got := normalizeCodexToolSchemasForCaller(body, native); !bytes.Equal(got, body) {
		t.Fatalf("native Codex request was reshaped:\n got: %s\nwant: %s", got, body)
	}

	other := http.Header{"User-Agent": []string{"opencode/1.0"}}
	got := normalizeCodexToolSchemasForCaller(body, other)
	if gjson.GetBytes(got, "tools.0.parameters.properties.action.oneOf").Exists() {
		t.Fatalf("non-native request kept the complex oneOf: %s", got)
	}
}

func TestCodexTerminalEmptyIncompleteSkippedForNativeCodexCaller(t *testing.T) {
	payload := []byte(`{"type":"response.incomplete","response":{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":10,"output_tokens":0,"total_tokens":10}}}`)
	native := http.Header{"Originator": []string{"codex_exec"}}
	if codexTerminalEmptyIncompleteForCaller(native, payload, 0, false) {
		t.Fatalf("native Codex caller must receive response.incomplete unchanged")
	}
	other := http.Header{"User-Agent": []string{"opencode/1.0"}}
	if !codexTerminalEmptyIncompleteForCaller(other, payload, 0, false) {
		t.Fatalf("non-native caller should still get the empty-incomplete failure classification")
	}
}
