package lockedbuild

import "testing"

func TestEnabledIsAConstant(t *testing.T) {
	// The value depends on the build tag; the test only asserts the symbol exists
	// and is usable as a bool in both builds.
	if Enabled && !Enabled {
		t.Fatal("unreachable")
	}
}
