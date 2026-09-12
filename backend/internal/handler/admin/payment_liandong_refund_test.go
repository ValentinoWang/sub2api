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

type liandongRefundHandlerStub struct {
	liandongHandlerStub
	err      error
	prepared bool
}

func (s *liandongRefundHandlerStub) PrepareUnusedCodeRefund(context.Context, service.LiandongRefundPrepareRequest) (*service.LiandongCodeRefund, error) {
	s.prepared = true
	return &service.LiandongCodeRefund{Status: "reserved"}, s.err
}

func (s *liandongRefundHandlerStub) ConfirmUnusedCodeRefund(context.Context, service.LiandongRefundConfirmRequest) (*service.LiandongCodeRefund, error) {
	return &service.LiandongCodeRefund{Status: "merchant_reference_recorded"}, s.err
}

func TestLiandongRefundHandlersKeepCodesAndInternalErrorsOutOfResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		err        error
		status     int
	}{
		{"reserved", `{"code":"refund-secret-canary","batch_id":"batch-1","external_order_no":"order-1"}`, nil, http.StatusOK},
		{"malformed", `{"code":"refund-secret-canary",`, nil, http.StatusBadRequest},
		{"driver failure", `{"code":"refund-secret-canary"}`, errors.New("database error refund-secret-canary"), http.StatusServiceUnavailable},
		{"conflict", `{"code":"refund-secret-canary"}`, service.ErrLiandongRefundConflict, http.StatusConflict},
		{"used", `{"code":"refund-secret-canary"}`, service.ErrLiandongRefundNotEligible, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &liandongRefundHandlerStub{err: tc.err}
			h := NewPaymentHandler(nil, nil, stub)
			router := gin.New()
			router.POST("/prepare", h.PrepareLiandongUnusedCodeRefund)
			request := httptest.NewRequest(http.MethodPost, "/prepare", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.status, recorder.Code)
			require.NotContains(t, recorder.Body.String(), "refund-secret-canary")
			require.NotContains(t, recorder.Body.String(), "database error")
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			if tc.name == "malformed" {
				require.False(t, stub.prepared)
			}
		})
	}
}
