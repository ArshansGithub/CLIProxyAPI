package egress

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawTransport matches raw http.Transport / websocket.Dialer literals and
// bare .Proxy assignments (the [^=] excludes == comparisons). A guarded
// transport can be de-guarded by a later .Proxy = ... assignment that
// bypasses egress.WrapProxyFunc, so that pattern must be caught too.
//
// Spec §4.4 also names the two shapes that reach the network without ever
// writing an `&http.Transport{}` literal: an http.Client built with a
// Transport field, and `new(http.Transport)`. Both are matched here so a file
// cannot construct an unguarded client by spelling it differently.
var rawTransport = regexp.MustCompile(`&?http\.Transport\{|&?websocket\.Dialer\{|\.Proxy\s*=[^=]|http\.Client\{[^}]*Transport:|new\(http\.Transport\)|proxy\.SOCKS5\(|proxy\.FromURL\(`)

func TestNoUnguardedTransportConstruction(t *testing.T) {
	root := repoRoot(t)
	allowed := map[string]bool{}
	f, err := os.Open(filepath.Join(root, "internal/egress/coverage_allowlist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			allowed[line] = true
		}
	}
	f.Close()
	for _, rel := range []string{"internal", "sdk", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, _ := os.ReadFile(path)
			if !rawTransport.Match(data) {
				return nil
			}
			r, _ := filepath.Rel(root, path)
			if !allowed[r] {
				t.Errorf("%s constructs a raw transport/dialer or assigns .Proxy directly; guard it with egress and add it to coverage_allowlist.txt", r)
			}
			return nil
		})
	}
}
