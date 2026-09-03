package management

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/cachestats"
)

// cacheStatsEventsDefaultLimit bounds a page when the caller names no limit.
const cacheStatsEventsDefaultLimit = 200

// cacheStatsEventsResponse is the payload of GET /v0/management/cache-stats/events.
type cacheStatsEventsResponse struct {
	Enabled   bool               `json:"enabled"`
	LatestSeq int64              `json:"latest_seq"`
	Events    []cachestats.Event `json:"events"`
}

// GetCacheStatsEvents returns retained cache-loss events, oldest first.
//
// `after` selects events with a sequence number greater than it, so a poller
// passes back the `latest_seq` of its previous page. `limit` caps the page and
// defaults to 200; the newest events win when the cap trims.
//
//	GET /v0/management/cache-stats/events?after=<seq>&limit=<n>
func (h *Handler) GetCacheStatsEvents(c *gin.Context) {
	after, errAfter := parseOptionalInt64(c.Query("after"), 0)
	if errAfter != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "after must be an integer"})
		return
	}
	limit, errLimit := parseOptionalInt64(c.Query("limit"), cacheStatsEventsDefaultLimit)
	if errLimit != nil || limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a non-negative integer"})
		return
	}
	store := cachestats.Default()
	page := store.Events(after, int(limit))
	c.JSON(http.StatusOK, cacheStatsEventsResponse{
		Enabled:   store.Enabled(),
		LatestSeq: page.LatestSeq,
		Events:    page.Events,
	})
}

func parseOptionalInt64(raw string, fallback int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
