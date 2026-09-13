//go:build locked

package registry

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// TestEmbeddedCatalogsMatchModelsVersionPin guards the model snapshot pin. The
// locked build never refreshes the catalogs from the network, so the bytes
// compiled into the binary are the only ones that will ever be served, and
// MODELS_VERSION is the record of which snapshot was reviewed. If someone
// swaps a catalog without re-recording its digest, this fails.
func TestEmbeddedCatalogsMatchModelsVersionPin(t *testing.T) {
	pins := readModelsVersionPins(t)
	for _, tc := range []struct {
		key  string
		data []byte
	}{
		{"models.json.sha256", embeddedModelsJSON},
		{"codex_client_models.json.sha256", embeddedCodexClientModelsJSON},
	} {
		want, ok := pins[tc.key]
		if !ok {
			t.Errorf("models/MODELS_VERSION has no %s= line", tc.key)
			continue
		}
		sum := sha256.Sum256(tc.data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s: embedded catalog digest %s does not match the pin %s in models/MODELS_VERSION; "+
				"re-review the snapshot and update the pin (see make refresh-models)", tc.key, got, want)
		}
	}
}

// readModelsVersionPins parses the key=value lines of models/MODELS_VERSION.
// The file is read relative to the package directory, which is the test's
// working directory, so it stays a plain data file rather than a second embed.
func readModelsVersionPins(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open("models/MODELS_VERSION")
	if err != nil {
		t.Fatalf("read models/MODELS_VERSION: %v", err)
	}
	defer func() { _ = f.Close() }()
	pins := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		pins[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan models/MODELS_VERSION: %v", err)
	}
	return pins
}
