package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type liandongHandlerStub struct {
	status    *service.LiandongRestockStatus
	statusErr error
	runErr    error
	runCalled bool
}

func (s *liandongHandlerStub) Status(context.Context) (*service.LiandongRestockStatus, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	return s.status, nil
}
func (s *liandongHandlerStub) UpdateConfiguration(context.Context, service.LiandongRestockConfigurationUpdate) (*service.LiandongRestockStatus, error) {
	return s.status, s.statusErr
}
func (s *liandongHandlerStub) UpdatePolicies(context.Context, []service.LiandongRestockPolicyUpdate) (*service.LiandongRestockStatus, error) {
	return s.status, s.statusErr
}
func (s *liandongHandlerStub) RunOnce(context.Context, bool) error {
	s.runCalled = true
	return s.runErr
}
func (s *liandongHandlerStub) SetEnabled(context.Context, bool) (*service.LiandongRestockStatus, error) {
	return s.status, s.statusErr
}

func newLiandongHandlerTestRouter(h *PaymentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/status", h.GetLiandongRestockStatus)
	r.PUT("/config", h.UpdateLiandongRestockConfig)
	r.PUT("/policies", h.UpdateLiandongRestockPolicies)
	r.POST("/run", h.RunLiandongRestockNow)
	r.POST("/enable", h.SetLiandongRestockEnabled)
	return r
}

func TestLiandongAdminResponsesNeverExposeSecretsOrCodes(t *testing.T) {
	const token = "merchant-token-must-stay-secret"
	const secret = "code-secret-must-stay-secret"
	const fullCode = "LD-12345678-ABCDEFGH-IJKLMNOP-QRSTUVWX"
	stub := &liandongHandlerStub{status: &service.LiandongRestockStatus{
		IntegrationMode: "sales_channel", PaymentReadiness: "NOT_READY", Configured: true,
		MerchantTokenConfigured: true, CodeSecretConfigured: true, Enabled: false,
		Products: []service.LiandongRestockProduct{{CNYAmount: 20, USDCredit: 2.78, GoodsID: 42, Threshold: 5, RestockCount: 3}},
		Batches:  []service.LiandongRestockBatchStatus{{BatchID: "batch-1", GoodsID: 42, CodeCount: 3, Status: "uploaded", Error: ""}},
	}}
	h := NewPaymentHandler(nil, nil, stub)
	r := newLiandongHandlerTestRouter(h)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/status", ""},
		{http.MethodPut, "/config", `{"merchant_token":"` + token + `","generate_code_secret":false,"products":[{"cny_amount":20,"usd_credit":2.78,"goods_id":42}]}`},
		{http.MethodPut, "/policies", `[{"cny_amount":20,"threshold":5,"restock_count":3,"enabled":false}]`},
		{http.MethodPost, "/run", ""},
		{http.MethodPost, "/enable", `{"enabled":false}`},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotContains(t, rec.Body.String(), token)
			require.NotContains(t, rec.Body.String(), secret)
			require.NotContains(t, rec.Body.String(), fullCode)
		})
	}
	require.True(t, stub.runCalled)
}

func TestLiandongAdminHandlerReturnsUnavailableWithoutService(t *testing.T) {
	h := NewPaymentHandler(nil, nil)
	r := newLiandongHandlerTestRouter(h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestLiandongAdminHandlerPropagatesRunFailure(t *testing.T) {
	stub := &liandongHandlerStub{
		status: &service.LiandongRestockStatus{},
		runErr: errors.New("upstream unavailable"),
	}
	h := NewPaymentHandler(nil, nil, stub)
	r := newLiandongHandlerTestRouter(h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/run", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":500`)
}
