//go:build locked

package pluginhost

import (
	"strings"
	"testing"
)

func TestLockedLoaderRefusesToOpen(t *testing.T) {
	loader := defaultPluginLoader()
	_, err := loader.Open(pluginFile{Path: "/tmp/x.dylib"}, nil)
	if err == nil || !strings.Contains(err.Error(), "locked build") {
		t.Fatalf("want locked-build error, got %v", err)
	}
}
