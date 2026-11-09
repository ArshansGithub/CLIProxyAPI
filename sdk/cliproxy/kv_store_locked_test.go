//go:build locked

package cliproxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/kvstore"
)

// The local kv store is held under the locked tag: an enabled config must not
// open <auth-dir>/kv.db, and callers must see in-memory mode.
func TestLockedBuildKeepsLocalKVStoreClosed(t *testing.T) {
	authDir := t.TempDir()
	service := &Service{}
	service.applyKVStoreConfig(&config.Config{AuthDir: authDir})
	t.Cleanup(func() { _ = kvstore.ShutdownLocal() })

	if _, errStat := os.Stat(filepath.Join(authDir, config.DefaultKVStorePath)); !os.IsNotExist(errStat) {
		t.Fatalf("kv.db exists under the locked build (stat error %v)", errStat)
	}
	client, kvMode, errClient := kvstore.CurrentKVClient()
	if client != nil || kvMode || errClient != nil {
		t.Fatalf("CurrentKVClient = (%v, %v, %v), want in-memory mode", client, kvMode, errClient)
	}
}
