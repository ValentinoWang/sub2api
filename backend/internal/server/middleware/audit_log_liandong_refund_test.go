package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLiandongRefundAuditOmitsEvenMalformedCredentialBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repository, nil)
	audit.Start()
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	for _, suffix := range []string{"prepare", "confirm"} {
		path := "/api/v1/admin/liandong/restock/refunds/" + suffix
		router.POST(path, func(c *gin.Context) { c.Status(http.StatusBadRequest) })
		for _, body := range []string{`{"code":"refund-audit-canary"}`, `{"code":"refund-audit-canary",`} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		}
	}
	audit.Stop()
	repository.mu.Lock()
	defer repository.mu.Unlock()
	require.Len(t, repository.logs, 4)
	for _, entry := range repository.logs {
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		require.NotContains(t, entry.RequestBody, "refund-audit-canary")
	}
}
