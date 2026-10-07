package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNoSubstitutionConfigDecodesFromYAML(t *testing.T) {
	var cfg Config
	src := []byte("no-substitution:\n  strip-fallbacks: true\n  refuse-unsupported-effort: true\n")
	if err := yaml.Unmarshal(src, &cfg); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if !cfg.NoSubstitution.StripFallbacks || !cfg.NoSubstitution.RefuseUnsupportedEffort {
		t.Fatalf("decoded %+v, want both true", cfg.NoSubstitution)
	}
}

func TestNoSubstitutionConfigDefaultsOff(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("api-keys: []\n"), &cfg); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if cfg.NoSubstitution.StripFallbacks || cfg.NoSubstitution.RefuseUnsupportedEffort {
		t.Fatalf("defaults %+v, want both false", cfg.NoSubstitution)
	}
}

func TestNoSubstitutionIsAForkRootSection(t *testing.T) {
	for _, s := range forkRootSections {
		if s == "no-substitution" {
			return
		}
	}
	t.Fatalf("forkRootSections = %v, missing no-substitution", forkRootSections)
}
