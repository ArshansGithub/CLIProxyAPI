package effortpin

import (
	"strings"
	"testing"
)

var pinned = []string{"claude-opus-5-5", "claude-sonnet-5-5", "gpt-6.1-sol", "gpt-6-luna"}

func configured(allowBare bool) *Store {
	s := NewStore()
	s.Configure(true, pinned, allowBare)
	return s
}

func TestRefusesBarePinnedIDsWithOrWithoutPrefix(t *testing.T) {
	s := configured(false)
	for _, m := range []string{"claude-opus-5-5", "scoped/claude-opus-5-5", "side/claude-opus-5-5[1m]", "gpt-6.1-sol", "GPT-6-Luna", " claude-sonnet-5-5 "} {
		refused, msg := s.Refuse(m)
		if !refused {
			t.Fatalf("%q must be refused", m)
		}
		if !strings.Contains(msg, "-medium") || !strings.Contains(msg, "/cache.html") {
			t.Fatalf("%q: message must name the slugs and the switch: %s", m, msg)
		}
	}
	if snap := s.Snapshot(); snap.Refusals != 6 || snap.LastRefusedModel != "claude-sonnet-5-5" || !snap.Active {
		t.Fatalf("snapshot after refusals: %+v", snap)
	}
}

func TestPassesSlugsUnpinnedModelsAndDisabled(t *testing.T) {
	s := configured(false)
	for _, m := range []string{"claude-opus-5-5-medium", "scoped/claude-opus-5-5-medium[1m]", "gpt-6.1-sol-high", "gpt-6.1-sol(high)", "claude-fable-5-1[1m]", "claude-fable-5-1", "cf-glm53-flash", "gemini-3.1-pro-low", ""} {
		if refused, _ := s.Refuse(m); refused {
			t.Fatalf("%q must pass", m)
		}
	}
	if snap := s.Snapshot(); snap.Refusals != 0 {
		t.Fatalf("no refusal expected: %+v", snap)
	}
	off := NewStore()
	off.Configure(false, pinned, false)
	if refused, _ := off.Refuse("claude-opus-5-5"); refused {
		t.Fatal("a disabled store must not refuse")
	}
}

func TestAllowBareSwitchCountsInsteadOfRefusing(t *testing.T) {
	s := configured(false)
	s.SetAllowBare(true)
	if refused, _ := s.Refuse("scoped/claude-opus-5-5"); refused {
		t.Fatal("bare id must pass while the switch is on")
	}
	snap := s.Snapshot()
	if snap.Active || snap.AllowedBare != 1 || snap.LastAllowedModel != "scoped/claude-opus-5-5" || snap.Refusals != 0 {
		t.Fatalf("snapshot with switch on: %+v", snap)
	}
	s.SetAllowBare(false)
	if refused, _ := s.Refuse("scoped/claude-opus-5-5"); !refused {
		t.Fatal("bare id must be refused again once the switch is off")
	}
	// Configure resets the runtime switch to the configured value.
	s.SetAllowBare(true)
	s.Configure(true, pinned, false)
	if refused, _ := s.Refuse("gpt-6.1-sol"); !refused {
		t.Fatal("Configure must reset the runtime switch")
	}
}
