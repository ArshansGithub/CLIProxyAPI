package egress

import (
	"net/url"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

var current atomic.Pointer[Policy]

// SetConfig rebuilds the process-wide policy from cfg and the built-in hosts.
// Call it after config load and on every config reload.
func SetConfig(cfg *config.Config) { SetConfigWithBuiltin(cfg, BuiltinHosts()) }

// SetConfigWithBuiltin is SetConfig with an explicit built-in list (tests).
func SetConfigWithBuiltin(cfg *config.Config, builtin []string) {
	current.Store(NewPolicy(cfg, builtin))
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
