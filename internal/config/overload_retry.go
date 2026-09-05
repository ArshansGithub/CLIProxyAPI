package config

import "time"

// OverloadRetryConfig retries the same credential on an upstream 529
// "overloaded" before failing over to another one. Failing over moves a
// session to a credential that has never seen its prompt-cache prefix, which
// costs a full cache rewrite, while overload bursts usually pass in seconds.
type OverloadRetryConfig struct {
	// Enabled turns the same-credential retry on.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Attempts is how many extra tries the same credential gets after the
	// first 529 before normal failover resumes.
	Attempts int `yaml:"attempts" json:"attempts"`
	// Backoff is the wait before the first retry; it doubles on each retry.
	Backoff time.Duration `yaml:"backoff" json:"backoff"`
	// MaxBackoff caps the doubled wait.
	MaxBackoff time.Duration `yaml:"max-backoff" json:"max-backoff"`
}

const (
	defaultOverloadRetryAttempts   = 3
	defaultOverloadRetryBackoff    = 5 * time.Second
	defaultOverloadRetryMaxBackoff = 20 * time.Second
)

// Normalized fills unset fields with defaults.
func (c OverloadRetryConfig) Normalized() OverloadRetryConfig {
	if c.Attempts <= 0 {
		c.Attempts = defaultOverloadRetryAttempts
	}
	if c.Backoff <= 0 {
		c.Backoff = defaultOverloadRetryBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = defaultOverloadRetryMaxBackoff
	}
	if c.MaxBackoff < c.Backoff {
		c.MaxBackoff = c.Backoff
	}
	return c
}

// WaitFor is the backoff before retry number attempt (zero-based).
func (c OverloadRetryConfig) WaitFor(attempt int) time.Duration {
	c = c.Normalized()
	wait := c.Backoff
	for i := 0; i < attempt && wait < c.MaxBackoff; i++ {
		wait *= 2
	}
	if wait > c.MaxBackoff {
		wait = c.MaxBackoff
	}
	return wait
}
