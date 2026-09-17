package managementasset

import (
	"bytes"
	"fmt"
	"html"
	"strconv"
	"strings"
)

// LockedBadge is what the locked build stamps onto the management panel so a
// person looking at the page can tell, without reading logs or running -v,
// that they are talking to the privacy-locked fork and which policy it is
// enforcing right now.
type LockedBadge struct {
	Version    string // buildinfo.Version, e.g. v7.2.159-locked.1
	PanelTag   string // pinned panel tag from PANEL_VERSION
	EgressMode string // "enforce" or "audit"
	HostCount  int    // hosts the egress policy currently admits
}

// Marker comments bracket every inserted block so tests can prove the stamp is
// a pure overlay: strip the blocks and the reviewed bundle comes back byte for
// byte. The panel bundle itself is never edited, so PANEL_VERSION's sha256 and
// `make panel-verify` keep describing exactly what is embedded.
const (
	brandHeadStart = "<!-- cpa-locked:head -->"
	brandHeadEnd   = "<!-- /cpa-locked:head -->"
	brandBodyStart = "<!-- cpa-locked:body -->"
	brandBodyEnd   = "<!-- /cpa-locked:body -->"
	brandTitleTag  = "<title>"
	brandTitleEnd  = "</title>"
	brandTitlePre  = "🔒 "
	brandTitleSuf  = " · locked build"
)

// brandTitleScript keeps the lock on the tab title after the panel's own code
// rewrites document.title on load and on route changes. fix() is idempotent,
// so the mutation it causes does not loop.
const brandTitleScript = `<script>(function(){var p="🔒 ",s=" · locked build",t=document.querySelector("title");if(!t)return;function fix(){var v=document.title,c=v;if(c.indexOf(p)===0)c=c.slice(p.length);if(c.slice(-s.length)===s)c=c.slice(0,-s.length);if(v!==p+c+s)document.title=p+c+s}fix();new MutationObserver(fix).observe(t,{childList:true,characterData:true,subtree:true})})();</script>`

const brandStyle = `<style>
#cpa-locked-stripe{position:fixed;top:0;left:0;right:0;height:4px;z-index:2147483000;pointer-events:none;background:linear-gradient(90deg,#f59e0b 0%,#22c55e 100%)}
#cpa-locked-badge{position:fixed;right:12px;bottom:12px;z-index:2147483000;display:inline-flex;align-items:center;gap:.45em;padding:6px 12px;border-radius:999px;border:1px solid #f59e0b;background:#0f172a;color:#f8fafc;font:600 12px/1.3 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;letter-spacing:.01em;box-shadow:0 4px 14px rgba(0,0,0,.35);white-space:nowrap;cursor:default;user-select:none}
#cpa-locked-badge span{font-weight:400;color:#cbd5e1}
#cpa-locked-badge.cpa-locked-audit{border-color:#ef4444}
#cpa-locked-stripe.cpa-locked-audit{background:linear-gradient(90deg,#ef4444 0%,#f59e0b 100%)}
</style>`

// Brand overlays the locked-build stamp on a panel bundle: a lock prefix on the
// document title, a coloured stripe across the top of the page, and a fixed
// badge naming the build, the panel pin, and the live egress mode. Input that
// lacks the </head> or </body> anchor is returned unchanged.
func Brand(panel []byte, b LockedBadge) []byte {
	headAt := bytes.LastIndex(panel, []byte("</head>"))
	bodyAt := bytes.LastIndex(panel, []byte("</body>"))
	if headAt < 0 || bodyAt < 0 || bodyAt < headAt {
		return panel
	}
	audit := ""
	if b.EgressMode != "enforce" {
		audit = " cpa-locked-audit"
	}
	badge := fmt.Sprintf(
		`<div id="cpa-locked-stripe" class="%[1]s"></div><div id="cpa-locked-badge" class="%[1]s" title="Privacy-locked CLIProxyAPI build: no runtime downloads, outbound hosts allowlisted">🔒 PRIVACY-LOCKED BUILD<span>· %[2]s · panel %[3]s · egress %[4]s · %[5]s hosts</span></div>`,
		strings.TrimSpace(audit),
		html.EscapeString(strings.TrimSpace(b.Version)),
		html.EscapeString(strings.TrimSpace(b.PanelTag)),
		html.EscapeString(strings.TrimSpace(b.EgressMode)),
		strconv.Itoa(b.HostCount),
	)

	var out bytes.Buffer
	out.Grow(len(panel) + len(brandStyle) + len(badge) + 256)
	out.Write(panel[:headAt])
	out.WriteString(brandHeadStart)
	out.WriteString(brandStyle)
	out.WriteString(brandHeadEnd)
	out.Write(panel[headAt:bodyAt])
	out.WriteString(brandBodyStart)
	out.WriteString(badge)
	out.WriteString(brandTitleScript)
	out.WriteString(brandBodyEnd)
	out.Write(panel[bodyAt:])
	return retitle(out.Bytes())
}

// retitle prefixes the first <title> with a lock and suffixes it with the
// build's name, so browser tabs and history entries show the difference too.
func retitle(page []byte) []byte {
	start := bytes.Index(page, []byte(brandTitleTag))
	if start < 0 {
		return page
	}
	start += len(brandTitleTag)
	end := bytes.Index(page[start:], []byte(brandTitleEnd))
	if end < 0 {
		return page
	}
	end += start
	var out bytes.Buffer
	out.Grow(len(page) + 24)
	out.Write(page[:start])
	out.WriteString(brandTitlePre)
	out.Write(page[start:end])
	out.WriteString(brandTitleSuf)
	out.Write(page[end:])
	return out.Bytes()
}

// PanelTag returns the `tag=` value from the PANEL_VERSION pin, or "" when the
// build has no embedded panel.
func PanelTag() string { return panelTagFrom(PanelVersion()) }

func panelTagFrom(pin string) string {
	for _, line := range strings.Split(pin, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "tag="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
