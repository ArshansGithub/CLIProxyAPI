//go:build locked

package api

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/egress"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/managementasset"
)

// assertControlPanelBody checks the locked build's response: the embedded
// panel with the locked-build stamp overlaid at serve time. The on-disk copy
// is never consulted, and the embedded bytes themselves are never edited.
func assertControlPanelBody(t *testing.T, body string, _ string) {
	t.Helper()
	panel, ok := managementasset.EmbeddedPanel()
	if !ok {
		t.Fatal("locked build must embed the panel")
	}
	policy := egress.Current()
	want := string(managementasset.Brand(panel, managementasset.LockedBadge{
		Version:    buildinfo.Version,
		PanelTag:   managementasset.PanelTag(),
		EgressMode: policy.Mode(),
		HostCount:  policy.HostCount(),
	}))
	if body != want {
		t.Fatalf("management panel body is not the stamped embedded panel (%d bytes served, %d expected)", len(body), len(want))
	}
	for _, marker := range []string{`id="cpa-locked-badge"`, "🔒 PRIVACY-LOCKED BUILD", "panel v1."} {
		if !strings.Contains(body, marker) {
			t.Fatalf("served panel lacks locked-build marker %q", marker)
		}
	}
}
