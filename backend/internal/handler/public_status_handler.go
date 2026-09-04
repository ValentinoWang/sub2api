package handler

import (
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const publicStatusCacheTTL = 30 * time.Second

// PublicStatusHandler serves the unauthenticated, aggregated monitor summary that backs the
// public /status page. Gated by the public_status_page_enabled setting and cached briefly so
// anonymous traffic cannot hammer the history tables.
type PublicStatusHandler struct {
	monitorService *service.ChannelMonitorService
	settingService *service.SettingService

	mu       sync.Mutex
	cached   *service.PublicStatusSummary
	cachedAt time.Time
}

func NewPublicStatusHandler(monitorService *service.ChannelMonitorService, settingService *service.SettingService) *PublicStatusHandler {
	return &PublicStatusHandler{monitorService: monitorService, settingService: settingService}
}

// Get handles GET /api/v1/public/status.
func (h *PublicStatusHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	if h.settingService == nil || !h.settingService.IsPublicStatusPageEnabled(ctx) {
		response.Success(c, gin.H{"enabled": false, "items": []any{}})
		return
	}
	if h.monitorService == nil {
		response.Success(c, gin.H{"enabled": true, "overall": "unknown", "items": []any{}})
		return
	}

	h.mu.Lock()
	if h.cached != nil && time.Since(h.cachedAt) < publicStatusCacheTTL {
		cached := h.cached
		h.mu.Unlock()
		c.Header("Cache-Control", "public, max-age=30")
		response.Success(c, cached)
		return
	}
	h.mu.Unlock()

	summary, err := h.monitorService.PublicStatus(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.mu.Lock()
	h.cached = summary
	h.cachedAt = time.Now()
	h.mu.Unlock()
	c.Header("Cache-Control", "public, max-age=30")
	response.Success(c, summary)
}
