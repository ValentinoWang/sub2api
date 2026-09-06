package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterLiandongToolRoutesRequiresAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := adminhandler.NewLiandongToolkitHandler(nil, nil)
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
			return
		}
		c.Next()
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	RegisterLiandongToolRoutes(router.Group("/api/v1"), handler, adminAuth, auditLog, nil, nil)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/tools/ldxp/installation"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/installation"},
		{http.MethodGet, "/api/v1/admin/tools/ldxp/status"},
		{http.MethodPut, "/api/v1/admin/tools/ldxp/config"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/config/test"},
		{http.MethodGet, "/api/v1/admin/tools/ldxp/goods"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/jobs/preview"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/jobs/run"},
		{http.MethodGet, "/api/v1/admin/tools/ldxp/jobs/job-1"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/jobs/job-1/resume"},
		{http.MethodGet, "/api/v1/admin/tools/ldxp/jobs/job-1/export"},
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestRegisterLiandongToolRoutesAppliesAuthAuditAndComplianceChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var auditCalls int
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Next()
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) {
		auditCalls++
		c.Next()
	})
	RegisterLiandongToolRoutes(
		router.Group("/api/v1"),
		adminhandler.NewLiandongToolkitHandler(nil, nil),
		adminAuth,
		auditLog,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/tools/ldxp/installation", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, auditCalls)
}
