package routes

import (
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterLiandongToolRoutes registers the LDXP administrator tool surface
// under the supplied API-v1 group. It intentionally does not modify the
// application's global router or dependency graph.
func RegisterLiandongToolRoutes(
	v1 *gin.RouterGroup,
	h *adminhandler.LiandongToolkitHandler,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	v1.GET("/ldxp/products", h.BrowserPublicProducts)
	device := v1.Group("/ldxp/device")
	device.Use(h.BrowserDeviceAuth)
	device.Use(panelRateLimiter.LDXPDevice())
	device.GET("/config", h.BrowserDeviceConfig)
	device.POST("/stock-target", h.BrowserDeviceStockTarget)
	device.POST("/inventory", h.BrowserInventory)
	device.POST("/claim", h.BrowserClaim)
	device.POST("/batches/:id/start", h.BrowserStart)
	device.POST("/batches/:id/result", h.BrowserResult)
	device.POST("/heartbeat", h.BrowserHeartbeat)
	device.POST("/runtime", h.BrowserRuntime)
	device.POST("/recheck/claim", h.BrowserClaimRecheck)
	device.POST("/recheck/:id/result", h.BrowserFinishRecheck)
	device.POST("/resume", h.BrowserDeviceResume)
	ldxp := v1.Group("/admin/tools/ldxp")
	ldxp.Use(gin.HandlerFunc(adminAuth))
	if auditLog != nil {
		ldxp.Use(gin.HandlerFunc(auditLog))
	}
	ldxp.Use(panelRateLimiter.LDXP())
	ldxp.Use(middleware.AdminComplianceGuard(settingService))
	{
		browser := ldxp.Group("/browser")
		browser.GET("/status", h.BrowserStatus)
		browser.PUT("/config", h.BrowserSaveConfig)
		browser.POST("/devices", h.BrowserCreateDevice)
		browser.DELETE("/devices/:id", h.BrowserRevokeDevice)
		browser.POST("/devices/:id/recheck", h.BrowserRequestRecheck)
		browser.POST("/resume", h.BrowserAdminResume)
	}
}
