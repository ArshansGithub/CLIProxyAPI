package config

import (
	"fmt"
	"net"
	"strings"
)

const (
	// EgressModeEnforce refuses connections to hosts outside the policy.
	EgressModeEnforce = "enforce"
	// EgressModeAudit logs would-be refusals and lets the connection proceed.
	EgressModeAudit = "audit"
)

// EgressConfig controls the outbound host policy. See internal/egress.
type EgressConfig struct {
	// Mode is "enforce" (default) or "audit".
	Mode string `yaml:"mode" json:"mode"`
	// ExtraAllow lists hostnames admitted in addition to provider hosts and
	// configured base URLs. Ports are ignored; matching is case-insensitive.
	ExtraAllow []string `yaml:"extra-allow" json:"extra-allow"`
}

// WithDefaults fills the mode and normalizes ExtraAllow to lowercase hostnames.
func (c EgressConfig) WithDefaults() EgressConfig {
	out := c
	if strings.TrimSpace(out.Mode) == "" {
		out.Mode = EgressModeEnforce
	}
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	normalized := make([]string, 0, len(c.ExtraAllow))
	for _, raw := range c.ExtraAllow {
		host := NormalizeEgressHost(raw)
		if host != "" {
			normalized = append(normalized, host)
		}
	}
	out.ExtraAllow = normalized
	return out
}

// Validate rejects unknown modes.
func (c EgressConfig) Validate() error {
	switch c.Mode {
	case EgressModeEnforce, EgressModeAudit:
		return nil
	}
	return fmt.Errorf("egress.mode must be %q or %q, got %q", EgressModeEnforce, EgressModeAudit, c.Mode)
}

// NormalizeEgressHost lowercases, trims, and strips a port. Returns "" for empty input.
func NormalizeEgressHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	return strings.TrimSuffix(host, ".")
}
