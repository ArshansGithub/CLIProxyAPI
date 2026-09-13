package egress

import (
	"net/url"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	log "github.com/sirupsen/logrus"
)

var current atomic.Pointer[Policy]

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
	p := NewPolicy(cfg, builtin)
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

func resetForTest() { current.Store(nil) }
