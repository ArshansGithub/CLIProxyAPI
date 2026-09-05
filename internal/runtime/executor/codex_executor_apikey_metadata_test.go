package executor

import (
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/tidwall/gjson"
)

// The Codex CLI attaches internal_chat_message_metadata_passthrough to input
// items that carry attachments. Only the ChatGPT backend understands it; the
// public Responses API (Azure OpenAI, api.openai.com) rejects the request with
// "Unknown parameter: 'input[1].internal_chat_message_metadata_passthrough.content_item_kinds'".
const codexAttachmentBody = `{"model":"gpt-5.6-terra","input":[` +
	`{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},` +
	`{"type":"message","role":"user","content":[{"type":"input_text","text":"see"},{"type":"input_image","image_url":"data:image/png;base64,AA=="}],` +
	`"internal_chat_message_metadata_passthrough":{"content_item_kinds":["text","image"]}}` +
	`]}`

func TestStripCodexInternalMetadataForAPIKeyUpstream(t *testing.T) {
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"api_key":  "test",
		"base_url": "https://example.services.ai.azure.com/openai/v1",
	}}
	got := stripCodexInternalMetadataForAPIKey([]byte(codexAttachmentBody), auth, "https://example.services.ai.azure.com/openai/v1")
	if gjson.GetBytes(got, "input.1.internal_chat_message_metadata_passthrough").Exists() {
		t.Fatalf("an emptied passthrough object should be removed for api-key upstream, got %s", got)
	}
	if gjson.GetBytes(got, "input.1.content.1.type").String() != "input_image" {
		t.Fatalf("attachment content must survive stripping, got %s", got)
	}
	if gjson.GetBytes(got, "input.0.content.0.text").String() != "hi" {
		t.Fatalf("unrelated items must be untouched, got %s", got)
	}
}

func TestStripCodexInternalMetadataKeepsOAuthChatGPT(t *testing.T) {
	auth := &cliproxyauth.Auth{Metadata: map[string]any{"access_token": "tok"}}
	got := stripCodexInternalMetadataForAPIKey([]byte(codexAttachmentBody), auth, "https://chatgpt.com/backend-api/codex")
	if !gjson.GetBytes(got, "input.1.internal_chat_message_metadata_passthrough.content_item_kinds").Exists() {
		t.Fatalf("passthrough metadata must be preserved for the ChatGPT backend, got %s", got)
	}
}

func TestStripCodexInternalMetadataKeepsAPIKeyAgainstChatGPT(t *testing.T) {
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"api_key":  "test",
		"base_url": "https://chatgpt.com/backend-api/codex",
	}}
	got := stripCodexInternalMetadataForAPIKey([]byte(codexAttachmentBody), auth, "https://chatgpt.com/backend-api/codex")
	if !gjson.GetBytes(got, "input.1.internal_chat_message_metadata_passthrough").Exists() {
		t.Fatalf("passthrough metadata must be preserved when the api key targets chatgpt.com, got %s", got)
	}
}

func TestStripCodexInternalMetadataKeepsTurnIDForAPIKeyUpstream(t *testing.T) {
	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "test", "base_url": "https://api.openai.com/v1"}}
	in := []byte(`{"model":"m","input":[` +
		`{"type":"agent_message","id":"amsg_1","author":"/root","recipient":"/root/worker","content":[{"type":"input_text","text":"go"}],"internal_chat_message_metadata_passthrough":{"turn_id":"turn_1"}},` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"see"}],"internal_chat_message_metadata_passthrough":{"turn_id":"turn_2","content_item_kinds":["text","image"]}}` +
		`]}`)
	got := stripCodexInternalMetadataForAPIKey(in, auth, "https://api.openai.com/v1")
	if gjson.GetBytes(got, "input.0.internal_chat_message_metadata_passthrough.turn_id").String() != "turn_1" {
		t.Fatalf("agent message turn_id must be preserved, got %s", got)
	}
	if gjson.GetBytes(got, "input.1.internal_chat_message_metadata_passthrough.turn_id").String() != "turn_2" {
		t.Fatalf("turn_id next to content_item_kinds must be preserved, got %s", got)
	}
	if gjson.GetBytes(got, "input.1.internal_chat_message_metadata_passthrough.content_item_kinds").Exists() {
		t.Fatalf("content_item_kinds must be removed, got %s", got)
	}
}

func TestStripCodexInternalMetadataNoopWithoutField(t *testing.T) {
	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "test", "base_url": "https://api.openai.com/v1"}}
	in := []byte(`{"model":"m","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	got := stripCodexInternalMetadataForAPIKey(in, auth, "https://api.openai.com/v1")
	if string(got) != string(in) {
		t.Fatalf("body without the field must be returned unchanged\n got=%s\nwant=%s", got, in)
	}
}
