package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLiandongRestockRoutesKeepAdminAuditAndComplianceGuards(t *testing.T) {
	source, err := os.ReadFile("payment.go")
	require.NoError(t, err)
	text := string(source)
	start := strings.Index(text, "liandong := v1.Group(\"/admin/liandong/restock\")")
	require.NotEqual(t, -1, start)
	end := strings.Index(text[start:], "\n\t\t}")
	require.NotEqual(t, -1, end)
	block := text[start : start+end]
	require.Contains(t, block, "liandong.Use(gin.HandlerFunc(adminAuth))")
	require.Contains(t, block, "liandong.Use(gin.HandlerFunc(auditLog))")
	require.Contains(t, block, "liandong.Use(middleware.AdminComplianceGuard(settingService))")
}

func TestLiandongRestockRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin access required")
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	jwtAuth := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })
	adminPayment := adminhandler.NewPaymentHandler(nil, nil)
	RegisterPaymentRoutes(
		router.Group("/api/v1"), &handler.PaymentHandler{}, &handler.PaymentWebhookHandler{}, adminPayment,
		jwtAuth, adminAuth, auditLog, nil, &servermiddleware.PanelRateLimiter{},
	)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/liandong/restock/status"},
		{http.MethodPut, "/api/v1/admin/liandong/restock/config"},
		{http.MethodPut, "/api/v1/admin/liandong/restock/policies"},
		{http.MethodPost, "/api/v1/admin/liandong/restock/run"},
		{http.MethodPost, "/api/v1/admin/liandong/restock/enable"},
	}
	for _, route := range routes {
		for _, tc := range []struct {
			name       string
			auth       string
			wantStatus int
		}{
			{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
			{name: "non-admin", auth: "Bearer user-token", wantStatus: http.StatusForbidden},
		} {
			t.Run(route.method+route.path+"/"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, nil)
				if tc.auth != "" {
					req.Header.Set("Authorization", tc.auth)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				require.Equal(t, tc.wantStatus, rec.Code)
			})
		}
	}
}
