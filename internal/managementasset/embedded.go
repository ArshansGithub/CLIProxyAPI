//go:build !locked

package managementasset

// EmbeddedPanel reports no embedded panel in the default build; the runtime
// updater in updater.go supplies the asset instead.
func EmbeddedPanel() ([]byte, bool) { return nil, false }

// PanelVersion is empty in the default build.
func PanelVersion() string { return "" }
