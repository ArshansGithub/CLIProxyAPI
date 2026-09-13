//go:build locked

package registry

import (
	"context"
	"testing"
)

func TestLockedBuildHasNoRemoteModelSources(t *testing.T) {
	if remoteModelRefreshEnabled || len(modelsURLs) != 0 || len(codexClientModelsURLs) != 0 {
		t.Fatal("locked build must not have remote model sources")
	}
	// Must return immediately without starting a goroutine that fetches.
	StartModelsUpdater(context.Background())
	StartCodexClientModelsUpdater(context.Background())
}
