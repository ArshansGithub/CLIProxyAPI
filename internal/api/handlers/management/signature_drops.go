package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/signaturedrops"
)

// GetSignatureDrops reports the requests whose thinking history the Claude
// executor stripped because a CAIS signature carried an unrecognized
// generation identifier. Zero is the healthy state; anything else means the
// allowlist in internal/signature needs the new identifier confirmed and added.
func (h *Handler) GetSignatureDrops(c *gin.Context) {
	c.JSON(http.StatusOK, signaturedrops.Default().Snapshot())
}

// DeleteSignatureDrops clears the counter, for instance after the allowlist
// was amended and the operator wants a clean read on whether drops continue.
func (h *Handler) DeleteSignatureDrops(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "cleared_requests": signaturedrops.Default().Reset()})
}
