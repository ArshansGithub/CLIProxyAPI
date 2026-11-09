//go:build !locked

package meta

import (
	"os"
	"strings"
)

// mintURLFromEnv returns upstream's META_MINT_URL override verbatim.
func mintURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("META_MINT_URL"))
}
