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

func TestProxySubscriptionManagementRoutesWithoutRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewProxyHandler(nil)
	router := gin.New()
	// Mirrors registerProxyRoutes: static subscription paths coexist with /:id routes.
	proxies := router.Group("/api/v1/admin/proxies")
	proxies.POST("/subscriptions/import", h.ImportSubscription)
	proxies.GET("/subscriptions", h.ListSubscriptions)
	proxies.POST("/subscriptions/:subscription_id/refresh", h.RefreshSubscription)
	proxies.PUT("/subscriptions/:subscription_id", h.UpdateSubscription)
	proxies.GET("/:id", h.GetByID)
	proxies.PUT("/:id", h.Update)
	proxies.POST("/:id/test", h.Test)

	serve := func(method, path, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		return recorder
	}
	list := serve(http.MethodGet, "/api/v1/admin/proxies/subscriptions", "")
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"data":[]`)
	require.Equal(t, http.StatusServiceUnavailable, serve(http.MethodPost, "/api/v1/admin/proxies/subscriptions/abc/refresh", "").Code)
	require.Equal(t, http.StatusBadRequest, serve(http.MethodPut, "/api/v1/admin/proxies/subscriptions/abc", `{}`).Code)
	require.Equal(t, http.StatusServiceUnavailable, serve(http.MethodPut, "/api/v1/admin/proxies/subscriptions/abc", `{"refresh_interval_minutes":60}`).Code)
}
