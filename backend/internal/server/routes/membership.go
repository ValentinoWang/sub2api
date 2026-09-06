package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterMembershipRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	adminAuth middleware.AdminAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	public := v1.Group("/membership")
	public.Use(panelRateLimiter.PublicIP())
	public.GET("/products", h.Membership.Products)
	public.GET("/payment/redirect", h.Membership.RedirectPayment)

	customer := v1.Group("/membership")
	customer.Use(gin.HandlerFunc(jwtAuth))
	customer.Use(middleware.BackendModeUserGuard(settingService))
	customer.Use(panelRateLimiter.Global())
	if auditLog != nil {
		customer.Use(gin.HandlerFunc(auditLog))
	}
	{
		customer.POST("/orders", limitMembershipBody(16<<10), h.Membership.CreateOrder)
		customer.POST("/orders/:id/payment", limitMembershipBody(8<<10), h.Membership.CreatePayment)
		customer.POST("/orders/:id/payment/redirect-ticket", h.Membership.ReissuePaymentRedirectTicket)
		customer.GET("/orders", h.Membership.Orders)
		customer.GET("/orders/:id", h.Membership.Order)
		customer.PUT("/orders/:id/credential", limitMembershipBody(70<<10), h.Membership.PutCredential)
		customer.POST("/orders/:id/refund", limitMembershipBody(4<<10), h.Membership.RequestRefund)
	}

	admin := v1.Group("/admin/membership")
	admin.Use(gin.HandlerFunc(adminAuth))
	if auditLog != nil {
		admin.Use(gin.HandlerFunc(auditLog))
	}
	admin.Use(panelRateLimiter.Global())
	admin.Use(middleware.AdminComplianceGuard(settingService))
	{
		admin.GET("", h.Membership.AdminOverview)
		admin.GET("/orders/:id", h.Membership.AdminOrder)
		admin.PUT("/products/:sku", limitMembershipBody(16<<10), h.Membership.UpdateProduct)
		admin.POST("/products/:sku/availability", h.Membership.CheckAvailability)
		admin.POST("/products/:sku/verify", limitMembershipBody(8<<10), h.Membership.VerifyProduct)
		admin.POST("/validation-orders", limitMembershipBody(16<<10), h.Membership.CreateValidationOrder)
		admin.POST("/cdks/import", limitMembershipBody(128<<10), h.Membership.ImportCDKs)
		admin.POST("/orders/:id/review", limitMembershipBody(8<<10), h.Membership.Review)
		admin.POST("/coupons", limitMembershipBody(8<<10), h.Membership.SaveCoupon)
	}
}

func limitMembershipBody(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		}
		c.Next()
	}
}
