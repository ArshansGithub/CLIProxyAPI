package egress

import (
	"bytes"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

// captureLog redirects the standard logger into a buffer for the duration of
// fn and returns what was written.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	std := log.StandardLogger()
	prevOut, prevLevel := std.Out, std.Level
	var buf bytes.Buffer
	std.Out = &buf
	std.Level = log.InfoLevel
	t.Cleanup(func() {
		std.Out, std.Level = prevOut, prevLevel
		resetForTest()
	})
	fn()
	return buf.String()
}

// setConfigForTest installs cfg as if the process had just started. SetConfig
// pins egress.mode and extra-allow on its first call, so a test that wants
// those to come from its own config must clear the previous test's pin first.
// Use SetConfigWithBuiltin directly only to exercise a reload.
func setConfigForTest(cfg *config.Config, builtin []string) {
	resetForTest()
	SetConfigWithBuiltin(cfg, builtin)
}

func TestSetConfigLogsEgressModeAndHostCount(t *testing.T) {
	builtin := []string{"api.anthropic.com", "api.openai.com"}

	// enforce: info level, and the count is the installed policy's host count
	// (loopback + builtin + configured base URL + extra-allow).
	out := captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeEnforce, "extra.example.com"), builtin)
	})
	if want := "egress: mode=enforce — 7 hosts allowed"; !strings.Contains(out, want) {
		t.Errorf("enforce log = %q, want it to contain %q", out, want)
	}
	if strings.Contains(out, "level=warning") {
		t.Errorf("enforce should log at info level, got %q", out)
	}
	if got := Current().HostCount(); got != 7 {
		t.Errorf("installed policy HostCount() = %d, want 7", got)
	}

	// audit: warn level, and the fail-open wording is part of the line.
	out = captureLog(t, func() {
		setConfigForTest(testConfig(config.EgressModeAudit, "extra.example.com"), builtin)
	})
	if want := "egress: mode=audit (fail-open) — 7 hosts allowed"; !strings.Contains(out, want) {
		t.Errorf("audit log = %q, want it to contain %q", out, want)
	}
	if !strings.Contains(out, "level=warning") {
		t.Errorf("audit should log at warn level, got %q", out)
	}
}
