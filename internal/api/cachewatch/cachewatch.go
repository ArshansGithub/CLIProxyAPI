// Package cachewatch embeds the prompt-cache watch page served at /cache.html.
//
// The page is built into the binary rather than fetched like management.html,
// so the management panel auto-updater can never overwrite or remove it. It
// talks only to the /v0/management/cache-stats, cache-keepalive and
// claude-client-versions endpoints with the operator's management key.
package cachewatch

import _ "embed"

//go:embed cache.html
var html []byte

// HTML returns the page body.
func HTML() []byte {
	return html
}
