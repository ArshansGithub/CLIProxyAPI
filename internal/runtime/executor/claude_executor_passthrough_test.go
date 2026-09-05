package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

// A confirmed native Claude Code client is a passthrough client: the proxy must
// not reshape its cache_control ttl or its Anthropic-Beta header to match the
// proxy's own model of what a subagent or probe "should" send. Only cloaked and
// translated callers are normalised to that wire.
func TestClaudeExecutor_ConfirmedNativeSubagentAndProbeKeepCallerTTLAndBetas(t *testing.T) {
	const sessionID = "11111111-2222-4333-8444-555555555555"
	const userID = `{"device_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","account_uuid":"","session_id":"11111111-2222-4333-8444-555555555555"}`
	confirmedHeaders := func(betas string) http.Header {
		return http.Header{
			"User-Agent":                  {"claude-cli/2.1.258 (external, cli)"},
			"X-App":                       {"cli"},
			"Anthropic-Beta":              {betas},
			"X-Claude-Code-Session-Id":    {sessionID},
			"X-Stainless-Package-Version": {"0.112.1"},
			"X-Stainless-Runtime-Version": {"v26.3.0"},
			"X-Stainless-Os":              {"MacOS"},
			"X-Stainless-Arch":            {"arm64"},
		}
	}

	tests := []struct {
		name    string
		payload string
		headers http.Header
	}{
		{
			name: "subagent with thinking disabled keeps 1h ttl, extended-cache-ttl and effort",
			payload: `{"model":"claude-opus-4-6","thinking":{"type":"disabled"},` +
				`"system":[{"type":"text","text":"subagent-system","cache_control":{"type":"ephemeral","ttl":"1h"}}],` +
				`"messages":[{"role":"user","content":"do the task"}],"metadata":{"user_id":` + fmt.Sprintf("%q", userID) + `}}`,
			headers: func() http.Header {
				h := confirmedHeaders("claude-code-20250219,effort-2025-11-24,thinking-display-updates-2026-08-18,extended-cache-ttl-2025-04-11")
				h.Set("X-Claude-Code-Agent-Id", "agent-sub-1")
				return h
			}(),
		},
		{
			name: "probe-shaped turn keeps 1h ttl and fallback betas",
			payload: `{"model":"claude-opus-4-6","max_tokens":1,` +
				`"system":[{"type":"text","text":"probe-system","cache_control":{"type":"ephemeral","ttl":"1h"}}],` +
				`"messages":[{"role":"user","content":"quota"}],"metadata":{"user_id":` + fmt.Sprintf("%q", userID) + `}}`,
			headers: confirmedHeaders("claude-code-20250219,server-side-fallback-2026-06-01,thinking-display-updates-2026-08-18,extended-cache-ttl-2025-04-11"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seenBody []byte
			var seenHeaders http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenBody, _ = io.ReadAll(r.Body)
				seenHeaders = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"claude-opus-4-6","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
			}))
			defer server.Close()

			executor := NewClaudeExecutor(&config.Config{})
			auth := &cliproxyauth.Auth{Attributes: map[string]string{
				"api_key":  "key-confirmed-client",
				"base_url": server.URL,
			}}
			payload := []byte(tt.payload)
			_, errExecute := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{
				Model:   "claude-opus-4-6",
				Payload: payload,
			}, cliproxyexecutor.Options{
				SourceFormat:    sdktranslator.FormatClaude,
				OriginalRequest: payload,
				Headers:         tt.headers,
			})
			if errExecute != nil {
				t.Fatalf("Execute() error = %v", errExecute)
			}
			if got := gjson.GetBytes(seenBody, "system.0.cache_control.ttl").String(); got != "1h" {
				t.Fatalf("system.0.cache_control.ttl = %q, want caller's 1h preserved; body=%s", got, seenBody)
			}
			if got, want := seenHeaders.Get("Anthropic-Beta"), tt.headers.Get("Anthropic-Beta"); got != want {
				t.Fatalf("Anthropic-Beta = %q, want caller's header preserved %q", got, want)
			}
		})
	}
}

// The same shapes from a caller that is not confirmed native Claude Code are
// still normalised, so cloaked callers keep matching the measured native wire.
func TestClaudeExecutor_UnconfirmedSubagentStillStrippedToNativeWire(t *testing.T) {
	var seenBody []byte
	var seenHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		seenHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"claude-opus-4-6","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	executor := NewClaudeExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{
		ID: "unconfirmed-subagent",
		Attributes: map[string]string{
			"api_key":    "sk-ant-oat-unconfirmed",
			"base_url":   server.URL,
			"cloak_mode": "always",
		},
		Metadata: claudeOAuthTestMetadata(),
	}
	payload := []byte(`{"model":"claude-opus-4-6","system":[{"type":"text","text":"subagent-system","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":[{"type":"text","text":"do the task"}]}]}`)
	headers := http.Header{
		"X-Claude-Code-Agent-Id": {"agent-sub-2"},
		"Anthropic-Beta":         {"extended-cache-ttl-2025-04-11"},
	}
	_, errExecute := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{Model: "claude-opus-4-6", Payload: payload}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FormatClaude,
		Headers:      headers,
	})
	if errExecute != nil {
		t.Fatalf("Execute() error = %v", errExecute)
	}
	if gjson.GetBytes(seenBody, "system.#(text==\"subagent-system\").cache_control.ttl").Exists() {
		t.Fatalf("unconfirmed subagent must have ttl stripped, body=%s", seenBody)
	}
	if betas := seenHeaders.Get("Anthropic-Beta"); containsBeta(betas, "extended-cache-ttl-2025-04-11") {
		t.Fatalf("unconfirmed subagent must not carry extended-cache-ttl, got %q", betas)
	}
}

func containsBeta(betas, want string) bool {
	for _, beta := range splitBetas(betas) {
		if beta == want {
			return true
		}
	}
	return false
}

func splitBetas(betas string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(betas); i++ {
		if i == len(betas) || betas[i] == ',' {
			if part := betas[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}
