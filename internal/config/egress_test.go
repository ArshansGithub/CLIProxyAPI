package config

import "testing"

func TestEgressConfigDefaults(t *testing.T) {
	cfg := EgressConfig{}.WithDefaults()
	if cfg.Mode != EgressModeEnforce {
		t.Fatalf("default mode = %q, want %q", cfg.Mode, EgressModeEnforce)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestEgressConfigValidateRejectsUnknownMode(t *testing.T) {
	cfg := EgressConfig{Mode: "yolo"}.WithDefaults()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestEgressConfigNormalizesExtraAllow(t *testing.T) {
	cfg := EgressConfig{ExtraAllow: []string{" Example.COM ", "", "api.example.com:443"}}.WithDefaults()
	want := []string{"example.com", "api.example.com"}
	if len(cfg.ExtraAllow) != len(want) {
		t.Fatalf("ExtraAllow = %v, want %v", cfg.ExtraAllow, want)
	}
	for i := range want {
		if cfg.ExtraAllow[i] != want[i] {
			t.Fatalf("ExtraAllow[%d] = %q, want %q", i, cfg.ExtraAllow[i], want[i])
		}
	}
}

func TestNormalizeEgressHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"[::1]", "::1"},
		{"[::1]:443", "::1"},
		{"::1", "::1"},
		{"Example.COM.", "example.com"},
		{"api.example.com:443", "api.example.com"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeEgressHost(tc.in); got != tc.want {
			t.Errorf("NormalizeEgressHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
