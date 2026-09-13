//go:build locked

package api

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/managementasset"
)

// assertControlPanelBody checks the locked build's response, which must be the
// embedded panel verbatim: the on-disk copy is never consulted.
func assertControlPanelBody(t *testing.T, body string, _ string) {
	t.Helper()
	panel, ok := managementasset.EmbeddedPanel()
	if !ok {
		t.Fatal("locked build must embed the panel")
	}
	if body != string(panel) {
		t.Fatalf("management panel body is not the embedded panel (%d bytes served, %d embedded)", len(body), len(panel))
	}
}
