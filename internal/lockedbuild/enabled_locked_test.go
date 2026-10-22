//go:build locked

package lockedbuild

import "testing"

func TestEnabledIsTrueWithTag(t *testing.T) {
	if !Enabled {
		t.Fatal("Enabled must be true in a build with -tags locked")
	}
}
