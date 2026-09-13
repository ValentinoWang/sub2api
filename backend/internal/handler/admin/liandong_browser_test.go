package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLiandongBrowserDeviceCredentialCannotBeReplacedByAdminBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLiandongToolkitHandler(service.NewLiandongRestockService(nil, nil, nil, nil))
	r := gin.New()
	r.Use(h.BrowserDeviceAuth)
	r.GET("/config", h.BrowserDeviceConfig)
	for _, auth := range []string{"", "Bearer admin-api-key", "Bearer ldxpd_" + strings.Repeat("a", 64)} {
		req := httptest.NewRequest(http.MethodGet, "/config", nil)
		req.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}
func TestLiandongBrowserClaimRejectsPrivilegeExpansionFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLiandongToolkitHandler(service.NewLiandongRestockService(nil, nil, nil, nil))
	r := gin.New()
	r.POST("/claim", h.BrowserClaim)
	for _, body := range []string{`{"goods_id":42,"usd_credit":100000}`, `{"goods_id":42,"user_id":1}`, `{"goods_id":42,"server_url":"https://evil.invalid"}`, `{"goods_id":42,"merchant_token":"secret"}`} {
		req := httptest.NewRequest(http.MethodPost, "/claim", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.NotContains(t, w.Body.String(), "secret")
	}
}

func TestLiandongBrowserStockTargetRejectsPrivilegeExpansion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewLiandongToolkitHandler(service.NewLiandongRestockService(nil, nil, nil, nil))
	router := gin.New()
	router.POST("/stock-target", h.BrowserDeviceStockTarget)
	for _, body := range []string{`{"goods_ids":[42],"target_stock":999,"enabled":true}`, `{"goods_ids":[42],"target_stock":999,"usd_credit":999}`, `{"goods_ids":[42],"target_stock":1000}`, `{"goods_ids":[],"target_stock":999}`} {
		req := httptest.NewRequest(http.MethodPost, "/stock-target", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
}
