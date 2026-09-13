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
	handler := adminhandler.NewLiandongToolkitHandler(nil)
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
		{http.MethodGet, "/api/v1/admin/tools/ldxp/browser/status"},
		{http.MethodPut, "/api/v1/admin/tools/ldxp/browser/config"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/browser/devices"},
		{http.MethodDelete, "/api/v1/admin/tools/ldxp/browser/devices/id"},
		{http.MethodPost, "/api/v1/admin/tools/ldxp/browser/resume"},
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
		adminhandler.NewLiandongToolkitHandler(nil),
		adminAuth,
		auditLog,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/tools/ldxp/browser/status", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, 1, auditCalls)
}

func TestRegisterLiandongToolRoutesAuditsBeforeDegradedSideEffectRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var auditCalls int
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 17})
		c.Set(string(servermiddleware.ContextKeyUserRole), "admin")
		c.Next()
	})
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) {
		auditCalls++
		c.Next()
	})
	RegisterLiandongToolRoutes(
		router.Group("/api/v1"),
		adminhandler.NewLiandongToolkitHandler(nil),
		adminAuth,
		auditLog,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tools/ldxp/browser/devices", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, 1, auditCalls)
}

func TestRegisterLiandongToolRoutesRemovesServerToolOperations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	auth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterLiandongToolRoutes(router.Group("/api/v1"), adminhandler.NewLiandongToolkitHandler(nil), auth, nil, nil, nil)
	for _, path := range []string{"installation", "status", "config", "config/test", "goods", "jobs/preview", "jobs/run", "jobs/id", "jobs/id/resume", "jobs/id/export"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(method, "/api/v1/admin/tools/ldxp/"+path, nil))
			require.Equal(t, http.StatusNotFound, recorder.Code, method+" "+path)
		}
	}
}
