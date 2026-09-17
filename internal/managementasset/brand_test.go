package managementasset

import (
	"strings"
	"testing"
)

const samplePanel = `<!doctype html><html><head><meta charset="UTF-8" /><title>CLI Proxy API Management Center</title></head><body><div id="root"></div></body></html>`

func sampleBadge() LockedBadge {
	return LockedBadge{Version: "v7.2.159-locked.1", PanelTag: "v1.22.18", EgressMode: "enforce", HostCount: 29}
}

func TestBrandMarksTitleStripeAndBadge(t *testing.T) {
	out := string(Brand([]byte(samplePanel), sampleBadge()))
	for _, want := range []string{
		"<title>🔒 CLI Proxy API Management Center · locked build</title>",
		`id="cpa-locked-stripe"`,
		`id="cpa-locked-badge"`,
		"v7.2.159-locked.1",
		"panel v1.22.18",
		"egress enforce · 29 hosts",
		"new MutationObserver(fix)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("branded panel missing %q\n%s", want, out)
		}
	}
	if strings.Count(out, "<title>") != 1 {
		t.Fatalf("title must be rewritten, not duplicated:\n%s", out)
	}
}

func TestBrandEscapesValues(t *testing.T) {
	b := sampleBadge()
	b.Version = `<script>alert(1)</script>`
	out := string(Brand([]byte(samplePanel), b))
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatal("badge values must be HTML-escaped")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("escaped value missing:\n%s", out)
	}
}

func TestBrandOnlyInserts(t *testing.T) {
	// Removing what Brand inserted must give back the reviewed bundle byte for
	// byte: the stamp is an overlay, never an edit of the panel's own markup.
	out := string(Brand([]byte(samplePanel), sampleBadge()))
	start := strings.Index(out, brandHeadStart)
	end := strings.Index(out, brandHeadEnd)
	if start < 0 || end < 0 {
		t.Fatalf("head marker missing:\n%s", out)
	}
	out = out[:start] + out[end+len(brandHeadEnd):]
	start = strings.Index(out, brandBodyStart)
	end = strings.Index(out, brandBodyEnd)
	if start < 0 || end < 0 {
		t.Fatalf("body marker missing:\n%s", out)
	}
	out = out[:start] + out[end+len(brandBodyEnd):]
	out = strings.Replace(out, "<title>🔒 ", "<title>", 1)
	out = strings.Replace(out, " · locked build</title>", "</title>", 1)
	if out != samplePanel {
		t.Fatalf("stamp is not a pure overlay\n got: %s\nwant: %s", out, samplePanel)
	}
}

func TestBrandWithoutAnchorsIsUnchanged(t *testing.T) {
	in := []byte("not html at all")
	if got := Brand(in, sampleBadge()); string(got) != string(in) {
		t.Fatalf("input without </head> and </body> must pass through, got %q", got)
	}
}

func TestPanelTagParsesPin(t *testing.T) {
	if got := panelTagFrom("repo=x\ntag=v1.22.18\nsha256=abc\n"); got != "v1.22.18" {
		t.Fatalf("panelTagFrom = %q, want v1.22.18", got)
	}
	if got := panelTagFrom(""); got != "" {
		t.Fatalf("panelTagFrom(empty) = %q, want empty", got)
	}
}
