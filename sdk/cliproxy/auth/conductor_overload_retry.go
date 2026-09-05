package auth

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// upstreamOverloadStatus is the status Anthropic returns while shedding load.
const upstreamOverloadStatus = 529

// overloadRetryConfig is the active same-credential overload retry policy, or
// nil when the feature is off.
func (m *Manager) overloadRetryConfig() *internalconfig.OverloadRetryConfig {
	cfg := m.runtimeConfigSnapshot()
	if cfg == nil || !cfg.OverloadRetry.Enabled {
		return nil
	}
	c := cfg.OverloadRetry.Normalized()
	return &c
}

// overloadRetryWait reports whether err is an upstream overload that the same
// credential should retry, and how long to wait first. attempt counts the
// retries already made for this credential on this request.
func (m *Manager) overloadRetryWait(ctx context.Context, err error, attempt int) (time.Duration, bool) {
	if err == nil || ctx == nil || ctx.Err() != nil {
		return 0, false
	}
	if statusCodeFromError(err) != upstreamOverloadStatus {
		return 0, false
	}
	cfg := m.overloadRetryConfig()
	if cfg == nil || attempt >= cfg.Attempts {
		return 0, false
	}
	return cfg.WaitFor(attempt), true
}

// waitForOverloadRetry sleeps for wait unless the request is cancelled first.
func waitForOverloadRetry(ctx context.Context, wait time.Duration) bool {
	if wait <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func logOverloadRetry(ctx context.Context, auth *Auth, model string, attempt, attempts int, wait time.Duration) {
	entry := logEntryWithRequestID(ctx)
	if entry == nil {
		entry = log.NewEntry(log.StandardLogger())
	}
	authID := ""
	if auth != nil {
		authID = auth.ID
	}
	entry.Infof("overload sticky retry | auth=%s model=%s attempt=%d/%d wait=%s: upstream 529, retrying the same credential to keep its prompt cache", authID, model, attempt+1, attempts, wait)
}
