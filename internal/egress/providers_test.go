package egress

import (
	"sort"
	"testing"
)

// TestBuiltinHostsDeduplicatesAcrossProviders registers the same host under
// two different (throwaway) provider names and asserts BuiltinHosts returns
// it exactly once, in sorted order. Uses "dup.example.test" so it can never
// collide with a real provider host or with known-unregistered.txt.
//
// The registry is process-global and Register has no Unregister; the two
// throwaway provider entries made here are left registered for the lifetime
// of the test binary. That's acceptable: TestEveryProviderHostLiteralIsRegistered
// only flags literals that are *not* registered, so an extra registered host
// that never appears as a literal is harmless to it.
func TestBuiltinHostsDeduplicatesAcrossProviders(t *testing.T) {
	const dupHost = "dup.example.test"
	Register("_test-provider-a", dupHost)
	Register("_test-provider-b", dupHost)

	hosts := BuiltinHosts()

	count := 0
	for _, h := range hosts {
		if h == dupHost {
			count++
		}
	}
	if count != 1 {
		t.Errorf("BuiltinHosts() contains %q %d times, want exactly 1 (got %v)", dupHost, count, hosts)
	}

	if !sort.StringsAreSorted(hosts) {
		t.Errorf("BuiltinHosts() is not sorted: %v", hosts)
	}
}
