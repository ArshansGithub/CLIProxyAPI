//go:build locked

package registry

import (
	"context"
	"testing"
	"time"
)

func TestLockedBuildHasNoRemoteModelSources(t *testing.T) {
	if remoteModelRefreshEnabled || len(modelsURLs) != 0 || len(codexClientModelsURLs) != 0 || len(devinModelsURLs) != 0 {
		t.Fatal("locked build must not have remote model sources")
	}
	// Must return immediately without starting a goroutine that fetches.
	StartModelsUpdater(context.Background())
	StartCodexClientModelsUpdater(context.Background())
	StartDevinModelsUpdater(context.Background())
}

func TestLockedCatalogUpdaterNeverFetchesAnySource(t *testing.T) {
	fetched := make(chan string, 4)
	updater := &catalogUpdater{
		fetch: func(_ context.Context, source string) ([]byte, error) {
			fetched <- source
			return nil, nil
		},
		publish: func([]byte) ([]string, error) { return nil, nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, source := range []string{"", embeddedCatalogSource, "https://catalog.example/models.json", "/tmp/models.json"} {
		updater.configure(ctx, source)
	}
	select {
	case source := <-fetched:
		t.Fatalf("locked build fetched catalog source %q", source)
	case <-time.After(200 * time.Millisecond):
	}
	if updater.source != "disabled" {
		t.Fatalf("updater source = %q, want disabled", updater.source)
	}
}

// The management refresh route (POST /v0/management/models/refresh) reaches
// refreshOnDemand with a source from config when no updater is running. The
// locked build must skip it rather than fetch.
func TestLockedOnDemandRefreshNeverFetches(t *testing.T) {
	fetched := make(chan string, 4)
	updater := &catalogUpdater{
		fetch: func(_ context.Context, source string) ([]byte, error) {
			fetched <- source
			return nil, nil
		},
		publish: func([]byte) ([]string, error) { return nil, nil },
	}
	for _, source := range []string{"https://catalog.example/models.json", "/tmp/models.json", ""} {
		status, err := updater.refreshOnDemand(context.Background(), source, []string{"https://catalog.example/default.json"})
		if err != nil {
			t.Fatalf("refreshOnDemand(%q) error = %v", source, err)
		}
		if !status.Skipped {
			t.Fatalf("refreshOnDemand(%q) status = %+v, want skipped", source, status)
		}
	}
	select {
	case source := <-fetched:
		t.Fatalf("locked build fetched catalog source %q on demand", source)
	default:
	}
}
