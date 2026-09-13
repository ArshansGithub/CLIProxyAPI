//go:build locked

package managementasset

import (
	"context"
	"path/filepath"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// ManagementFileName exposes the control panel asset filename.
const ManagementFileName = "management.html"

// SetCurrentConfig is retained for call-site compatibility; the locked build
// never syncs the panel so the config is not needed.
func SetCurrentConfig(_ *config.Config) {}

// StartAutoUpdater is a no-op: the locked build embeds a reviewed panel.
func StartAutoUpdater(_ context.Context, _ string) {}

// StaticDir mirrors the default build's path derivation so callers that only
// display the path keep working. Nothing is written there.
func StaticDir(configFilePath string) string { return staticDirFor(configFilePath) }

// FilePath returns where the default build would keep the asset.
func FilePath(configFilePath string) string {
	return filepath.Join(staticDirFor(configFilePath), ManagementFileName)
}

// EnsureLatestManagementHTML never downloads; it reports whether the embedded
// panel exists.
func EnsureLatestManagementHTML(_ context.Context, _ string, _ string, _ string) bool {
	_, ok := EmbeddedPanel()
	return ok
}
