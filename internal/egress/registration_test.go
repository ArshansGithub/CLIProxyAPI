package egress

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// scanRoots are the packages whose URL literals name upstream provider hosts.
var scanRoots = []string{"internal/runtime/executor", "internal/auth", "sdk/auth", "internal/registry", "internal/client"}

var urlLiteral = regexp.MustCompile(`"(?:https|wss)://([A-Za-z0-9.-]+)`)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}

func loadKnownUnregistered(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	f, err := os.Open(filepath.Join(root, "internal/egress/known-unregistered.txt"))
	if err != nil {
		t.Fatalf("open known-unregistered.txt: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		host := strings.Fields(line)[0] // "host  # reason"
		out[strings.ToLower(host)] = true
	}
	return out
}

func TestEveryProviderHostLiteralIsRegistered(t *testing.T) {
	root := repoRoot(t)
	known := loadKnownUnregistered(t, root)
	registered := map[string]bool{}
	for _, h := range BuiltinHosts() {
		registered[strings.ToLower(h)] = true
	}
	missing := map[string][]string{}
	for _, rel := range scanRoots {
		_ = filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, _ := os.ReadFile(path)
			for _, m := range urlLiteral.FindAllStringSubmatch(string(data), -1) {
				host := strings.ToLower(m[1])
				if registered[host] || known[host] || strings.Contains(host, "example") || host == "localhost" {
					continue
				}
				r, _ := filepath.Rel(root, path)
				missing[host] = append(missing[host], r)
			}
			return nil
		})
	}
	if len(missing) > 0 {
		hosts := make([]string, 0, len(missing))
		for h := range missing {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		for _, h := range hosts {
			t.Errorf("unregistered host %q in %v — add to providers.go or known-unregistered.txt with a reason", h, missing[h])
		}
	}
}
