package egress

import (
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]map[string]struct{}{}
)

// Register records hosts a provider's code may contact. Safe from init().
func Register(provider string, hosts ...string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	set, ok := registry[provider]
	if !ok {
		set = map[string]struct{}{}
		registry[provider] = set
	}
	for _, h := range hosts {
		set[h] = struct{}{}
	}
}

// BuiltinHosts returns every registered host, deduplicated across providers
// and sorted, for NewPolicy.
func BuiltinHosts() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	seen := map[string]struct{}{}
	var out []string
	for _, set := range registry {
		for h := range set {
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}
	sort.Strings(out)
	return out
}

// Built-in provider hosts. Each line corresponds to URL literals in the
// package named in the comment; registration_test.go fails if a literal in
// executor/auth/registry/client code is missing here and not excused in
// known-unregistered.txt.
func init() {
	Register("claude", // internal/runtime/executor/claude_*, internal/auth/claude
		"api.anthropic.com", "console.anthropic.com", "claude.ai", "platform.claude.com")
	Register("codex", // internal/runtime/executor/codex_*, internal/auth/codex, sdk/auth/codex_device.go, internal/client/codex/live
		"chatgpt.com", "api.openai.com", "auth.openai.com", "platform.openai.com")
	Register("xai", // internal/auth/xai, internal/runtime/executor/xai_*
		"api.x.ai", "auth.x.ai", "cli-chat-proxy.grok.com")
	Register("kimi", // internal/auth/kimi
		"api.kimi.com", "auth.kimi.com")
	Register("gemini", // internal/auth/gemini*, internal/auth/antigravity, internal/runtime/executor/gemini_*, internal/runtime/executor/antigravity_*
		"cloudcode-pa.googleapis.com", "daily-cloudcode-pa.googleapis.com",
		"daily-cloudcode-pa.sandbox.googleapis.com", "www.googleapis.com", "oauth2.googleapis.com",
		"aiplatform.googleapis.com", "generativelanguage.googleapis.com", "accounts.google.com",
		// vertexaisearch.cloud.google.com is not a URL literal (it's a Host
		// comparison in internal/runtime/executor/helps/antigravity_grounding_urls.go
		// used to validate a grounding redirect before the proxy itself follows
		// it with an outbound HEAD request); registered anyway so enforce mode
		// doesn't silently break Antigravity grounding-citation resolution.
		"vertexaisearch.cloud.google.com")
}
