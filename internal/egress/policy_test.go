package egress

import (
	"errors"
	"net/url"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func testConfig(mode string, extra ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Egress = config.EgressConfig{Mode: mode, ExtraAllow: extra}.WithDefaults()
	cfg.OpenAICompatibility = []config.OpenAICompatibility{{Name: "or", BaseURL: "https://openrouter.ai/api/v1"}}
	return cfg
}

func TestPolicyAllowsBuiltinConfiguredLoopbackAndExtra(t *testing.T) {
	p := NewPolicy(testConfig("enforce", "Extra.Example.com"), []string{"api.anthropic.com"})
	for _, host := range []string{"api.anthropic.com", "API.ANTHROPIC.COM", "openrouter.ai", "127.0.0.1", "::1", "localhost", "extra.example.com"} {
		if !p.Allowed(host) {
			t.Errorf("%q should be allowed", host)
		}
	}
	if p.Allowed("evil.example.net") {
		t.Error("unlisted host should be refused")
	}
}

func TestPolicyCheckEnforceReturnsErrNotPermitted(t *testing.T) {
	p := NewPolicy(testConfig("enforce"), nil)
	err := p.Check("evil.example.net", "test.site")
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("want ErrNotPermitted, got %v", err)
	}
	want := `egress: host "evil.example.net" not permitted (site=test.site, mode=enforce)`
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestPolicyCheckAuditAllowsButRecords(t *testing.T) {
	p := NewPolicy(testConfig("audit"), nil)
	if err := p.Check("evil.example.net", "test.site"); err != nil {
		t.Fatalf("audit mode must not refuse: %v", err)
	}
	if got := p.AuditRefusals(); got != 1 {
		t.Fatalf("audit refusals = %d, want 1", got)
	}
}

func TestCheckURLUsesHostnameOnly(t *testing.T) {
	SetConfigWithBuiltin(testConfig("enforce"), []string{"api.anthropic.com"})
	u, _ := url.Parse("https://api.anthropic.com:443/v1/messages?x=1")
	if err := CheckURL(u, "t"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := CheckURLString("wss://chatgpt.com/backend-api/codex/responses", "t"); err == nil {
		t.Fatal("chatgpt.com is not registered in this test and must be refused")
	}
}

func TestNoPolicyInstalledRefusesInEnforce(t *testing.T) {
	resetForTest()
	if err := CheckURLString("https://api.anthropic.com/", "t"); err == nil {
		t.Fatal("with no policy installed, enforce must refuse")
	}
}

func TestPolicyHostCountCoversLoopbackBuiltinAndExtra(t *testing.T) {
	// 3 loopback (127.0.0.1, ::1, localhost) + 2 builtin + 1 configured base URL
	// hostname (openrouter.ai, from testConfig) + 2 extra-allow.
	p := NewPolicy(
		testConfig("enforce", "Extra.Example.com", "second.example.com:443"),
		[]string{"api.anthropic.com", "api.openai.com"},
	)
	if got, want := p.HostCount(), 8; got != want {
		t.Fatalf("HostCount() = %d, want %d", got, want)
	}

	// Duplicates and empties collapse: a builtin repeated in extra-allow, in a
	// different case and with a port, is still one host.
	dup := NewPolicy(testConfig("enforce", "API.Anthropic.com:443", ""), []string{"api.anthropic.com"})
	if got, want := dup.HostCount(), 5; got != want { // 3 loopback + anthropic + openrouter.ai
		t.Fatalf("HostCount() with duplicates = %d, want %d", got, want)
	}

	// No config at all is loopback only.
	if got, want := NewPolicy(nil, nil).HostCount(), 3; got != want {
		t.Fatalf("HostCount() for the empty policy = %d, want %d", got, want)
	}
}

func TestPolicyAdmitsConfiguredProxyURLHosts(t *testing.T) {
	cfg := testConfig("enforce")
	cfg.ProxyURL = "socks5://global-proxy.example:1080"
	cfg.ClaudeKey = []config.ClaudeKey{{APIKey: "k", ProxyURL: "http://claude-proxy.example:3128"}}
	cfg.CodexKey = []config.CodexKey{{APIKey: "k", ProxyURL: "https://codex-proxy.example"}}
	cfg.GeminiKey = []config.GeminiKey{{APIKey: "k", ProxyURL: "socks5h://gemini-proxy.example:1080"}}
	cfg.VertexCompatAPIKey = []config.VertexCompatKey{{APIKey: "k", ProxyURL: "http://vertex-proxy.example"}}
	p := NewPolicy(cfg, nil)
	for _, host := range []string{"global-proxy.example", "claude-proxy.example", "codex-proxy.example", "gemini-proxy.example", "vertex-proxy.example"} {
		if !p.Allowed(host) {
			t.Errorf("configured proxy-url host %q should be admitted", host)
		}
	}
	if p.Allowed("other-proxy.example") {
		t.Error("unconfigured proxy host must not be admitted")
	}
}
