//go:build !locked

package registry

// Remote catalog sources. Under the locked tag these are empty and refresh is off.
const remoteModelRefreshEnabled = true

var modelsURLs = []string{
	"https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/models.json",
	"https://models.router-for.me/models.json",
}

var codexClientModelsURLs = []string{
	"https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/codex_client_models.json",
	"https://models.router-for.me/codex_client_models.json",
}
