package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// What breaks this: a v8 migration that comments out egress, usage-cache-stats,
// overload-retry, or claude-code.cache-keepalive, which would silently disable
// the egress allowlist on the next panel-driven config write.
func TestV8MigrationKeepsForkSections(t *testing.T) {
	raw := []byte(`egress:
  mode: enforce
  extra-allow:
    - callback.example
usage-cache-stats:
  enabled: true
  max-sessions: 500
overload-retry:
  attempts: 3
claude-code:
  cache-keepalive:
    enabled: true
    before-expiry: 5m0s
    probe-5m: auto
`)
	migrated, changed, err := NormalizeConfigLayout(raw, true)
	if err != nil || !changed {
		t.Fatalf("migrate: changed=%v error=%v", changed, err)
	}
	if strings.Contains(string(migrated), "# egress:") || strings.Contains(string(migrated), "# usage-cache-stats:") || strings.Contains(string(migrated), "# overload-retry:") {
		t.Fatalf("a fork section was commented out:\n%s", migrated)
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(migrated, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"egress.mode", "usage-cache-stats.enabled", "overload-retry.attempts", "oauth.providers.claude.claude-code.cache-keepalive.enabled"} {
		if yamlPath(doc.Content[0], key) == nil {
			t.Fatalf("%s missing after migration:\n%s", key, migrated)
		}
	}
	if err = ValidateV8Config(migrated); err != nil {
		t.Fatalf("migrated file is invalid: %v\n%s", err, migrated)
	}
	cfg, err := ParseConfigBytes(migrated)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Egress.Mode != "enforce" || len(cfg.Egress.ExtraAllow) != 1 || !cfg.UsageCacheStats.Enabled || !cfg.ClaudeCode.CacheKeepalive.Enabled {
		t.Fatalf("migration changed effective fork settings: egress=%+v cache-stats=%+v keepalive=%+v", cfg.Egress, cfg.UsageCacheStats, cfg.ClaudeCode.CacheKeepalive)
	}
}
