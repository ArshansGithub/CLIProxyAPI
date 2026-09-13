//go:build locked

package registry

// The locked build never fetches catalogs; it serves the embedded snapshot
// pinned in models/MODELS_VERSION.
const remoteModelRefreshEnabled = false

var modelsURLs []string

var codexClientModelsURLs []string
