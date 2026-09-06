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
	ldxp := v1.Group("/admin/tools/ldxp")
	ldxp.Use(gin.HandlerFunc(adminAuth))
	ldxp.Use(panelRateLimiter.Global())
	ldxp.Use(gin.HandlerFunc(auditLog))
	ldxp.Use(middleware.AdminComplianceGuard(settingService))
	{
		ldxp.GET("/installation", h.GetInstallation)
		ldxp.POST("/installation", h.Install)
		ldxp.GET("/status", h.GetStatus)
		ldxp.PUT("/config", h.UpdateConfig)
		ldxp.POST("/config/test", h.TestConfig)
		ldxp.GET("/goods", h.GetGoods)

		jobs := ldxp.Group("/jobs")
		{
			jobs.POST("/preview", h.Preview)
			jobs.POST("/run", h.Run)
			jobs.GET("/:id", h.GetJob)
			jobs.POST("/:id/resume", h.Resume)
			jobs.GET("/:id/export", h.Export)
		}
	}
}
