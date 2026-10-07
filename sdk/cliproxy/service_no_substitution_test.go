package cliproxy

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
)

func TestApplyNoSubstitutionConfigSetsBothSwitches(t *testing.T) {
	t.Cleanup(func() {
		thinking.SetRefuseUnsupportedEffort(false)
		executor.SetStripFallbacks(false)
	})
	s := &Service{}
	s.applyNoSubstitutionConfig(&config.Config{NoSubstitution: config.NoSubstitutionConfig{StripFallbacks: true, RefuseUnsupportedEffort: true}})
	if !thinking.RefuseUnsupportedEffort() || !executor.StripFallbacks() {
		t.Fatalf("switches = refuse %v strip %v, want both true", thinking.RefuseUnsupportedEffort(), executor.StripFallbacks())
	}
	s.applyNoSubstitutionConfig(&config.Config{})
	if thinking.RefuseUnsupportedEffort() || executor.StripFallbacks() {
		t.Fatal("a reload with the block absent must turn both off")
	}
	s.applyNoSubstitutionConfig(nil) // must not panic
}
