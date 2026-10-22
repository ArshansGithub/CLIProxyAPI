//go:build !locked

package api

import (
	"strings"
	"testing"
)

// assertControlPanelBody checks the default build's response, which is the
// management asset read from the static directory on disk.
func assertControlPanelBody(t *testing.T, body string, wantDiskMarker string) {
	t.Helper()
	if !strings.Contains(body, wantDiskMarker) {
		t.Fatalf("management panel body missing: %s", body)
	}
}
