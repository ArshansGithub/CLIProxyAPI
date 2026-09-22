//go:build locked

package meta

import (
	"os"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/egress"
	log "github.com/sirupsen/logrus"
)

// mintURLFromEnv honours META_MINT_URL only when the egress policy already
// admits its host. Upstream reads the variable unconditionally, which lets the
// process environment redirect a POST that carries the Meta device-code token
// as a bearer header. The gate would refuse an off-allowlist host at dial time
// anyway; checking here keeps the token from being attached to a request the
// operator never configured, and says why in the log instead of failing later.
func mintURLFromEnv() string {
	raw := strings.TrimSpace(os.Getenv("META_MINT_URL"))
	if raw == "" {
		return ""
	}
	if err := egress.CheckURLString(raw, "meta.mint"); err != nil {
		log.Warnf("meta auth: ignoring META_MINT_URL in the locked build: %v", err)
		return ""
	}
	return raw
}
