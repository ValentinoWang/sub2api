package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImportProxySubscriptionUnavailableWithoutRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewProxyHandler(nil)
	router := gin.New()
	router.POST("/api/v1/admin/proxies/subscriptions/import", h.ImportSubscription)

	body := []byte(`{"name":"test","url":"https://subscription.example.test/secret"}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/subscriptions/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "subscription.example.test")
}

func TestImportProxySubscriptionRejectsMalformedRequestWithoutEcho(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewProxyHandler(nil)
	router := gin.New()
	router.POST("/api/v1/admin/proxies/subscriptions/import", h.ImportSubscription)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/subscriptions/import", bytes.NewBufferString(`{"url":"sensitive-value"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "sensitive-value")
}
