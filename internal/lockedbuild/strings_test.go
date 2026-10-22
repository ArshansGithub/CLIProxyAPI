//go:build locked

package lockedbuild

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// forbidden are byte strings that must not survive into a locked binary.
var forbidden = []string{
	"api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center",
	"cpamc.router-for.me",
	"models.router-for.me",
	"raw.githubusercontent.com/router-for-me/models",
}

func TestLockedBinaryHasNoForbiddenHosts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the server binary")
	}
	root, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	out := filepath.Join(t.TempDir(), "cliproxyapi")
	cmd := exec.Command("go", "build", "-tags", "locked", "-o", out, "./cmd/server")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, b)
	}
	bin, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range forbidden {
		if bytes.Contains(bin, []byte(s)) {
			t.Errorf("locked binary still contains %q", s)
		}
	}
}
