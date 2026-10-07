package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/tidwall/gjson"
)

const nsSessionID = "11111111-2222-4333-8444-555555555555"
const nsUserID = `{"device_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","account_uuid":"","session_id":"11111111-2222-4333-8444-555555555555"}`

func nsNativeHeaders(betas string) http.Header {
	return http.Header{
		"User-Agent":                  {"claude-cli/2.1.293 (external, cli)"},
		"X-App":                       {"cli"},
		"Anthropic-Beta":              {betas},
		"X-Claude-Code-Session-Id":    {nsSessionID},
		"X-Stainless-Package-Version": {"0.112.1"},
		"X-Stainless-Runtime-Version": {"v26.3.0"},
		"X-Stainless-Os":              {"MacOS"},
		"X-Stainless-Arch":            {"arm64"},
	}
}

func nsCaptureLogs(t *testing.T) *logtest.Hook {
	t.Helper()
	logger := log.StandardLogger()
	prevLevel := logger.GetLevel()
	prevHooks := logger.ReplaceHooks(make(log.LevelHooks))
	hook := logtest.NewLocal(logger)
	logger.SetLevel(log.DebugLevel)
	t.Cleanup(func() { logger.ReplaceHooks(prevHooks); logger.SetLevel(prevLevel) })
	return hook
}

func nsRun(t *testing.T, cfg *config.Config, payload string, headers http.Header) ([]byte, http.Header) {
	t.Helper()
	var seenBody []byte
	var seenHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenBody, _ = io.ReadAll(r.Body)
		seenHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"claude-opus-5-5","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()
	executor := NewClaudeExecutor(cfg)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{"api_key": "key-test", "base_url": server.URL}}
	p := []byte(payload)
	model := gjson.GetBytes(p, "model").String()
	_, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{Model: model, Payload: p}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FormatClaude, OriginalRequest: p, Headers: headers,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return seenBody, seenHeaders
}

func nsNativePayload(fallbacks string) string {
	return `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"fallbacks":` + fallbacks + `,` +
		`"messages":[{"role":"user","content":"do the task"}],"metadata":{"user_id":` + fmt.Sprintf("%q", nsUserID) + `}}`
}

// nsCloakCfg turns the cloak on for the test credential, as the existing
// applyCloaking tests do, so the dormant injection sites run.
func nsCloakCfg() *config.Config {
	return &config.Config{ClaudeKey: []config.ClaudeKey{{APIKey: "key-test", Cloak: &config.CloakConfig{}}}}
}

func nsCountLogs(hook *logtest.Hook, level log.Level, fragment string) int {
	n := 0
	for _, e := range hook.AllEntries() {
		if e.Level == level && strings.Contains(e.Message, fragment) {
			n++
		}
	}
	return n
}

// Native client, array fallbacks: body key gone, header untouched, one Warn
// for the session, Debug for the next request of the same session.
func TestStripFallbacks_NativeArrayStrippedHeaderKept(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	hook := nsCaptureLogs(t)
	betas := "claude-code-20250219,effort-2025-11-24,server-side-fallback-2026-06-01,fallback-credit-2026-06-01"
	body, headers := nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders(betas))
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("fallbacks reached the wire: %s", body)
	}
	if got := headers.Get("Anthropic-Beta"); got != betas {
		t.Fatalf("native header reshaped: %q", got)
	}
	if n := nsCountLogs(hook, log.WarnLevel, "fallbacks stripped"); n != 1 {
		t.Fatalf("want one Warn, got %d: %v", n, hook.AllEntries())
	}
	hook.Reset()
	nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders(betas))
	if n := nsCountLogs(hook, log.WarnLevel, "fallbacks stripped"); n != 0 {
		t.Fatalf("second request of the same session must not Warn again: %v", hook.AllEntries())
	}
	if n := nsCountLogs(hook, log.DebugLevel, "fallbacks stripped"); n != 1 {
		t.Fatalf("second request should log at Debug: %v", hook.AllEntries())
	}
}

// The 2.1.293 category form is a string; it is stripped the same way.
func TestStripFallbacks_NativeStringDefaultStripped(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	betas := "claude-code-20250219,server-side-fallback-2026-07-01"
	body, headers := nsRun(t, &config.Config{}, nsNativePayload(`"default"`), nsNativeHeaders(betas))
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("fallbacks reached the wire: %s", body)
	}
	if got := headers.Get("Anthropic-Beta"); got != betas {
		t.Fatalf("native header reshaped: %q", got)
	}
}

// Switch off: the native field passes through unchanged (today's behaviour).
func TestStripFallbacks_OffPassesThrough(t *testing.T) {
	SetStripFallbacks(false)
	body, _ := nsRun(t, &config.Config{}, nsNativePayload(`[{"model":"claude-opus-4-8"}]`), nsNativeHeaders("claude-code-20250219,server-side-fallback-2026-06-01"))
	if got := gjson.GetBytes(body, "fallbacks.0.model").String(); got != "claude-opus-4-8" {
		t.Fatalf("switch off must pass fallbacks through, got %s", body)
	}
}

// Cloaked caller on Opus 5.5: the cloak's own injection and the reconcile
// re-injection are both undone, and the proxy's beta list carries neither
// fallback beta.
func TestStripFallbacks_CloakedInjectionUndoneAndBetasOmitted(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, headers := nsRun(t, nsCloakCfg(), payload, http.Header{"User-Agent": {"curl/8.0"}})
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("cloak injected fallbacks survived the strip: %s", body)
	}
	for _, beta := range []string{"server-side-fallback-2026-06-01", "server-side-fallback-2026-07-01"} {
		if strings.Contains(headers.Get("Anthropic-Beta"), beta) {
			t.Fatalf("proxy added %s with no fallbacks in the body: %q", beta, headers.Get("Anthropic-Beta"))
		}
	}
}

// Without the switch the cloak injects the native-shaped fallback: this pins
// the control so the test above cannot pass vacuously.
func TestStripFallbacks_OffCloakedInjectionStands(t *testing.T) {
	SetStripFallbacks(false)
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, _ := nsRun(t, nsCloakCfg(), payload, http.Header{"User-Agent": {"curl/8.0"}})
	if got := gjson.GetBytes(body, "fallbacks.0.model").String(); got != "claude-opus-4-8" {
		t.Fatalf("switch off: cloak should inject claude-opus-4-8, got %s", body)
	}
}

// A payload rule that sets fallbacks after cloaking is stripped too: the
// strip runs on the finished body.
func TestStripFallbacks_PayloadRuleSettingFallbacksIsStripped(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	cfg := nsCloakCfg()
	cfg.Payload.Override = []config.PayloadRule{{
		Models: []config.PayloadModelRule{{Name: "claude-opus-5-5"}},
		Params: map[string]any{"fallbacks": []any{map[string]any{"model": "claude-opus-5"}}},
	}}
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, _ := nsRun(t, cfg, payload, http.Header{"User-Agent": {"curl/8.0"}})
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("payload-rule fallbacks reached the wire: %s", body)
	}
}

// fallback_credit_token keeps its beta: a credit is not a substitution.
func TestStripFallbacks_CreditTokenKeepsCreditBeta(t *testing.T) {
	t.Cleanup(func() { SetStripFallbacks(false); resetStripFallbacksSeen() })
	SetStripFallbacks(true)
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"fallback_credit_token":"tok_1","messages":[{"role":"user","content":"do the task"}]}`
	_, headers := nsRun(t, nsCloakCfg(), payload, http.Header{"User-Agent": {"curl/8.0"}})
	if !strings.Contains(headers.Get("Anthropic-Beta"), "fallback-credit-2026-06-01") {
		t.Fatalf("credit beta missing: %q", headers.Get("Anthropic-Beta"))
	}
}

// Finding 1 of the branch review: with the switch OFF a cloaked caller's
// fallback betas go out exactly as before this feature: 06-01 when requested
// or when the body carries fallbacks, 07-01 when requested.
func TestStripFallbacks_OffKeepsCallerFallbackBetas(t *testing.T) {
	SetStripFallbacks(false)
	both := "claude-code-20250219,server-side-fallback-2026-06-01,server-side-fallback-2026-07-01"
	payload := `{"model":"claude-opus-5-5","max_tokens":64,"thinking":{"type":"adaptive"},"fallbacks":"default","messages":[{"role":"user","content":"do the task"}]}`
	_, headers := nsRun(t, nsCloakCfg(), payload, http.Header{"User-Agent": {"curl/8.0"}, "Anthropic-Beta": {both}})
	got := headers.Get("Anthropic-Beta")
	for _, beta := range []string{"server-side-fallback-2026-06-01", "server-side-fallback-2026-07-01"} {
		if !strings.Contains(got, beta) {
			t.Fatalf("switch off: %s missing from %q", beta, got)
		}
	}
	only07 := "claude-code-20250219,server-side-fallback-2026-07-01"
	noBody := `{"model":"claude-opus-5","max_tokens":64,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"do the task"}]}`
	body, headers := nsRun(t, nsCloakCfg(), noBody, http.Header{"User-Agent": {"curl/8.0"}, "Anthropic-Beta": {only07}})
	if gjson.GetBytes(body, "fallbacks").Exists() {
		t.Fatalf("control: claude-opus-5 must not receive an injected fallback: %s", body)
	}
	got = headers.Get("Anthropic-Beta")
	if !strings.Contains(got, "server-side-fallback-2026-07-01") || strings.Contains(got, "server-side-fallback-2026-06-01") {
		t.Fatalf("switch off, 07-01 requested and no body fallbacks: want 07-01 and not 06-01, got %q", got)
	}
}
