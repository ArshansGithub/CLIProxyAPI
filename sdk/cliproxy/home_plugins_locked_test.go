//go:build locked

package cliproxy

import (
	"context"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestSyncHomePluginsRefusedInLockedBuild(t *testing.T) {
	cfg := &config.Config{}
	cfg.Home.Enabled = true
	cfg.Plugins.Enabled = true
	s := &Service{}
	_, _, didSync, err := s.syncHomePluginsWithClient(context.Background(), cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "locked build") || didSync {
		t.Fatalf("locked build must refuse home plugin sync before any fetch; err=%v didSync=%v", err, didSync)
	}
}
