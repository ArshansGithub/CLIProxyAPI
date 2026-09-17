// Package egress decides which outbound hosts the proxy may connect to.
//
// The policy is the union of built-in provider hosts (providers.go), the
// hostname of every configured base URL, loopback, and egress.extra-allow.
// Enforcement points live in transport.go.
package egress

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

// ErrNotPermitted is wrapped by every refusal.
var ErrNotPermitted = errors.New("egress: host not permitted")

// Policy is an immutable host set plus a mode.
type Policy struct {
	hosts         map[string]struct{}
	mode          string
	auditRefusals atomic.Int64
}

// NewPolicy builds a policy from config and the built-in provider host list.
func NewPolicy(cfg *config.Config, builtin []string) *Policy {
	p := &Policy{hosts: map[string]struct{}{}, mode: config.EgressModeEnforce}
	add := func(raw string) {
		if h := config.NormalizeEgressHost(raw); h != "" {
			p.hosts[h] = struct{}{}
		}
	}
	for _, h := range []string{"127.0.0.1", "::1", "localhost"} {
		add(h)
	}
	for _, h := range builtin {
		add(h)
	}
	if cfg != nil {
		p.mode = cfg.Egress.WithDefaults().Mode
		for _, h := range cfg.Egress.ExtraAllow {
			add(h)
		}
		for _, u := range append(configuredBaseURLs(cfg), configuredProxyURLs(cfg)...) {
			if parsed, err := url.Parse(strings.TrimSpace(u)); err == nil {
				add(parsed.Hostname())
			}
		}
	}
	return p
}

// Mode returns "enforce" or "audit".
func (p *Policy) Mode() string { return p.mode }

// HostCount returns how many distinct hosts the policy admits: loopback, the
// built-in provider hosts, every configured base URL's hostname, and
// egress.extra-allow, after normalization and de-duplication.
func (p *Policy) HostCount() int { return len(p.hosts) }

// Allowed reports whether host (with or without port, any case) is in the set.
func (p *Policy) Allowed(host string) bool {
	_, ok := p.hosts[config.NormalizeEgressHost(host)]
	return ok
}

// Check returns nil when host is allowed. In enforce mode a refusal is an
// error wrapping ErrNotPermitted; in audit mode it is logged and counted.
func (p *Policy) Check(host, site string) error {
	if p.Allowed(host) {
		return nil
	}
	if p.mode == config.EgressModeAudit {
		p.auditRefusals.Add(1)
		log.Warnf("egress audit: host %q would be refused (site=%s)", config.NormalizeEgressHost(host), site)
		return nil
	}
	err := fmt.Errorf("egress: host %q not permitted (site=%s, mode=%s)", config.NormalizeEgressHost(host), site, p.mode)
	log.Errorf("%v", err)
	return &refusal{msg: err.Error()}
}

type refusal struct{ msg string }

func (r *refusal) Error() string        { return r.msg }
func (r *refusal) Is(target error) bool { return target == ErrNotPermitted }

// AuditRefusals returns how many audit-mode refusals this policy has logged.
func (p *Policy) AuditRefusals() int64 { return p.auditRefusals.Load() }

// configuredBaseURLs collects every base-url field a user can set in config.
// Extend this list when upstream adds a provider block with a base URL.
func configuredBaseURLs(cfg *config.Config) []string {
	var out []string
	for _, e := range cfg.OpenAICompatibility {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.CodexKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.ClaudeKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.GeminiKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.InteractionsKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.VertexCompatAPIKey {
		out = append(out, e.BaseURL)
	}
	for _, e := range cfg.XAIKey {
		out = append(out, e.BaseURL)
	}
	return out
}

// configuredProxyURLs collects every proxy-url a user can set in config. A
// forward proxy is where the socket actually goes, so its host must be
// admitted like a base-url host. Per-credential proxy_url values in auth
// files are not config and need egress.extra-allow.
func configuredProxyURLs(cfg *config.Config) []string {
	out := []string{cfg.ProxyURL}
	for _, e := range cfg.ClaudeKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.CodexKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.GeminiKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.VertexCompatAPIKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.XAIKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.InteractionsKey {
		out = append(out, e.ProxyURL)
	}
	for _, e := range cfg.OpenAICompatibility {
		for _, k := range e.APIKeyEntries {
			out = append(out, k.ProxyURL)
		}
	}
	return out
}
