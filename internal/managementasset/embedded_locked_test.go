//go:build locked

package managementasset

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEmbeddedPanelMatchesPin(t *testing.T) {
	data, ok := EmbeddedPanel()
	if !ok || len(data) == 0 {
		t.Fatal("locked build must embed the panel")
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	want := ""
	for _, line := range strings.Split(panelVersionText, "\n") {
		if strings.HasPrefix(line, "sha256=") {
			want = strings.TrimPrefix(line, "sha256=")
		}
	}
	if want == "" || got != want {
		t.Fatalf("embedded panel sha256 %s, PANEL_VERSION says %q", got, want)
	}
}
