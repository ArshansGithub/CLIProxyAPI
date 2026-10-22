package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/effortpin"
	log "github.com/sirupsen/logrus"
)

// GetEffortPin reports whether bare ids for the pinned models are refused,
// the pinned set, and the refusal counters since start.
func (h *Handler) GetEffortPin(c *gin.Context) {
	c.JSON(http.StatusOK, effortpin.Default().Snapshot())
}

// PutEffortPin flips the running process's allow-bare switch. The configured
// value returns on restart or config reload; the cache watch page uses this.
func (h *Handler) PutEffortPin(c *gin.Context) {
	var body struct {
		AllowBare *bool `json:"allow_bare"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.AllowBare == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"allow_bare\": true|false}"})
		return
	}
	snap := effortpin.Default().SetAllowBare(*body.AllowBare)
	log.WithFields(log.Fields{"mode": map[bool]string{true: "allow-bare", false: "refuse-bare"}[*body.AllowBare]}).Warn("effort-pin: switch changed from the management API |")
	c.JSON(http.StatusOK, snap)
}
