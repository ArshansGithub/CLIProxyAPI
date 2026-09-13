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
