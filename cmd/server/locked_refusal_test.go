package main

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/lockedbuild"
)

func TestLockedBuildRefusal(t *testing.T) {
	if got := lockedBuildRefusal(false, false, false, false); got != "" {
		t.Fatalf("file store with no home mode must never be refused, got %q", got)
	}
	cases := map[string]string{
		"home":     lockedBuildRefusal(true, false, false, false),
		"postgres": lockedBuildRefusal(false, true, false, false),
		"object":   lockedBuildRefusal(false, false, true, false),
		"git":      lockedBuildRefusal(false, false, false, true),
	}
	for name, got := range cases {
		if lockedbuild.Enabled {
			if !strings.Contains(got, name) || !strings.HasPrefix(got, "locked build: ") {
				t.Errorf("locked build must refuse %s, got %q", name, got)
			}
		} else if got != "" {
			t.Errorf("tagless build must not refuse %s, got %q", name, got)
		}
	}
}
