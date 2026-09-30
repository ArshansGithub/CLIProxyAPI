package egress

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

// baseURLConfig builds a config whose only configured host is baseURL's.
func baseURLConfig(mode, baseURL string, extra ...string) *config.Config {
	cfg := &config.Config{}
	cfg.Egress = config.EgressConfig{Mode: mode, ExtraAllow: extra}.WithDefaults()
	cfg.OpenAICompatibility = []config.OpenAICompatibility{{Name: "or", BaseURL: baseURL}}
	return cfg
}

// A reload must not be able to turn the gate fail-open. PUT /config.yaml can
// write egress.mode, so if a reload were honoured, anyone holding the
// management key could switch the gate off instead of going around it.
func TestReloadCannotWeakenModeToAudit(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	out := captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeEnforce), builtin)
		SetConfigWithBuiltin(testConfig(config.EgressModeAudit), builtin)
	})

	if got := Current().Mode(); got != config.EgressModeEnforce {
		t.Errorf("mode after reload = %q, want it pinned to %q", got, config.EgressModeEnforce)
	}
	// logrus escapes the quotes inside msg=..., so match the unquoted parts.
	for _, want := range []string{"ignoring config reload that set mode=", "audit", "pinned to"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want it to contain %q", out, want)
		}
	}
	// The gate must still actually refuse, not merely report enforce.
	if err := Current().Check("evil.example.net", "test.site"); err == nil {
		t.Error("enforce policy should still refuse an unlisted host after the reload")
	}
}

// A reload must not be able to widen the allowlist either.
func TestReloadCannotAddExtraAllowHosts(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	out := captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeEnforce), builtin)
		SetConfigWithBuiltin(testConfig(config.EgressModeEnforce, "exfil.example.net"), builtin)
	})

	if Current().Allowed("exfil.example.net") {
		t.Error("extra-allow added by a reload should be ignored")
	}
	if want := "ignoring config reload that changed extra-allow"; !strings.Contains(out, want) {
		t.Errorf("log = %q, want it to contain %q", out, want)
	}
}

// extra-allow present at startup is the supported way to admit a host, so it
// must survive reloads rather than being dropped along with the reload's own.
func TestStartupExtraAllowSurvivesReload(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeEnforce, "proxy.internal.example.com"), builtin)
		SetConfigWithBuiltin(testConfig(config.EgressModeEnforce), builtin)
	})

	if !Current().Allowed("proxy.internal.example.com") {
		t.Error("extra-allow from startup should still be admitted after a reload")
	}
}

// Pinning is a pin, not a one-way tightening: a process started in audit stays
// in audit. Tightening needs a restart, same as loosening.
func TestStartupAuditModeIsPinned(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeAudit), builtin)
		SetConfigWithBuiltin(testConfig(config.EgressModeEnforce), builtin)
	})

	if got := Current().Mode(); got != config.EgressModeAudit {
		t.Errorf("mode after reload = %q, want it pinned to %q", got, config.EgressModeAudit)
	}
}

// Pinning must not break ordinary use: adding a provider while the server runs
// still admits that provider's host on reload.
func TestReloadStillAdmitsNewBaseURLHosts(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	captureLog(t, func() {
		setConfigForTest(baseURLConfig(config.EgressModeEnforce, "https://openrouter.ai/api/v1"), builtin)
		SetConfigWithBuiltin(baseURLConfig(config.EgressModeEnforce, "https://new-provider.example.com/v1"), builtin)
	})

	if !Current().Allowed("new-provider.example.com") {
		t.Error("a base URL added by a reload should still be admitted")
	}
}

// An unchanged reload is the common case and must stay quiet.
func TestUnchangedReloadLogsNoPinWarning(t *testing.T) {
	builtin := []string{"api.anthropic.com"}
	out := captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeEnforce, "extra.example.com"), builtin)
		SetConfigWithBuiltin(testConfig(config.EgressModeEnforce, "extra.example.com"), builtin)
	})

	if strings.Contains(out, "ignoring config reload") {
		t.Errorf("unchanged reload should not warn, got %q", out)
	}
}
