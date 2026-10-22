package signature

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestClaudeCAIS_LiveChannel18CaptureIsAccepted validates the fork's rule
// against real signatures rather than the synthetic encoder, using whatever
// request-body captures the local proxy has on disk (they rotate, so the test
// skips rather than fails when none hold a CAIS signature). On 2026-09-29 the
// capture carried 44 channel-18 model-free envelopes from claude-fable-5-1.
func TestClaudeCAIS_LiveChannel18CaptureIsAccepted(t *testing.T) {
	home, _ := os.UserHomeDir()
	matches, _ := filepath.Glob(filepath.Join(home, ".cli-proxy-api", "logs", "request-log-parts-request-body-*", "*.tmp"))
	if len(matches) == 0 {
		t.Skip("no live capture on this machine")
	}
	re := regexp.MustCompile(`"signature":"(C[A-Za-z0-9+/=]{20,})"`)
	var sigs [][][]byte
	for _, path := range matches {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sigs = append(sigs, re.FindAllSubmatch(body, -1)...)
	}
	if len(sigs) == 0 {
		t.Skip("captures hold no CAIS signatures")
	}
	seen18 := 0
	for _, m := range sigs {
		sig := string(m[1])
		info, err := InspectClaudeCAISSignature(sig)
		if err != nil {
			t.Fatalf("live signature rejected: %v", err)
		}
		if info.ChannelID == 18 {
			seen18++
		}
		if got := DetectSignatureProviderForBlock(sig, SignatureBlockKindClaudeThinking); got != SignatureProviderClaude {
			t.Fatalf("live signature provider = %q, want %q", got, SignatureProviderClaude)
		}
	}
	if seen18 == 0 {
		t.Skipf("%d live signatures accepted, none on channel 18 in the current captures", len(sigs))
	}
	t.Logf("%d live signatures accepted, %d on channel 18", len(sigs), seen18)
}
