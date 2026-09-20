package egress

import (
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

var current atomic.Pointer[Policy]

// startupEgress holds the mode and extra-allow list captured at the first
// SetConfig call. They are pinned there for the life of the process: both are
// reachable through PUT /v0/management/config.yaml, and the gate is supposed to
// still bound a process whose management key has leaked. Letting a reload set
// mode: audit (which is fail-open) or append to extra-allow would hand that
// caller the gate's own off switch.
//
// Base-URL and proxy hosts are deliberately not pinned — adding a provider
// while the server runs is ordinary use, and those hosts are re-derived from
// the live config on every reload.
var (
	startupEgressMu sync.Mutex
	startupEgress   *config.EgressConfig
)

// SetConfig rebuilds the process-wide policy from cfg and the built-in hosts.
// Call it after config load and on every config reload.
func SetConfig(cfg *config.Config) { SetConfigWithBuiltin(cfg, BuiltinHosts()) }

// SetConfigWithBuiltin is SetConfig with an explicit built-in list (tests).
//
// The installed mode is logged every time, at startup and on every reload.
// Audit mode is fail-open — it lets an unlisted host through with a warning —
// so it is logged at warn level: running in audit without meaning to is the one
// way the gate can be silently absent, and it should be visible in the log.
func SetConfigWithBuiltin(cfg *config.Config, builtin []string) {
	p := NewPolicyWithEgress(cfg, builtin, pinnedEgress(cfg))
	current.Store(p)
	if p.Mode() == config.EgressModeAudit {
		log.Warnf("egress: mode=audit (fail-open) — %d hosts allowed", p.HostCount())
	} else {
		log.Infof("egress: mode=enforce — %d hosts allowed", p.HostCount())
	}
}

// Current returns the installed policy. Before SetConfig runs it returns a
// loopback-only enforce policy, so nothing leaks during startup.
func Current() *Policy {
	if p := current.Load(); p != nil {
		return p
	}
	return NewPolicy(nil, nil)
}

// CheckURL applies the current policy to u's hostname.
func CheckURL(u *url.URL, site string) error {
	if u == nil {
		return nil
	}
	return Current().Check(u.Hostname(), site)
}

// CheckURLString parses raw and applies the current policy.
func CheckURLString(raw, site string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	return CheckURL(u, site)
}

// pinnedEgress returns the egress settings every policy build must use. The
// first call records what the process started with; later calls get that same
// value back, and a reload that tried to change it is logged and ignored.
func pinnedEgress(cfg *config.Config) config.EgressConfig {
	var incoming config.EgressConfig
	if cfg != nil {
		incoming = cfg.Egress
	}
	incoming = incoming.WithDefaults()

	startupEgressMu.Lock()
	defer startupEgressMu.Unlock()

	if startupEgress == nil {
		pinned := incoming
		startupEgress = &pinned
		return pinned
	}

	pinned := *startupEgress
	if incoming.Mode != pinned.Mode {
		log.Warnf("egress: ignoring config reload that set mode=%q; mode is pinned to %q from startup", incoming.Mode, pinned.Mode)
	}
	if !sameHostList(pinned.ExtraAllow, incoming.ExtraAllow) {
		log.Warnf("egress: ignoring config reload that changed extra-allow (%d host(s) -> %d); extra-allow is pinned to the startup list", len(pinned.ExtraAllow), len(incoming.ExtraAllow))
	}
	return pinned
}

// sameHostList compares two normalized host lists order-insensitively.
func sameHostList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, h := range a {
		counts[h]++
	}
	for _, h := range b {
		counts[h]--
		if counts[h] < 0 {
			return false
		}
	}
	return true
}

func resetForTest() {
	current.Store(nil)
	startupEgressMu.Lock()
	startupEgress = nil
	startupEgressMu.Unlock()
}
