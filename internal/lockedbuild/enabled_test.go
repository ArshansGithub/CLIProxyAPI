//go:build !locked

package lockedbuild

import "testing"

func TestEnabledIsFalseWithoutTag(t *testing.T) {
	if Enabled {
		t.Fatal("Enabled must be false in a build without -tags locked")
	}
}
