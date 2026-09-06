//go:build unit

package handler

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/membership"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderRejectsMembershipFieldsOutsideMembershipRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{
		`{"payment_type":"alipay","order_type":"membership"}`,
		`{"payment_type":"alipay","membership_order_id":"membership-order-id"}`,
	} {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders", bytes.NewBufferString(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})

			(&PaymentHandler{}).CreateOrder(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			var response struct {
				Reason string `json:"reason"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, "MEMBERSHIP_PAYMENT_ROUTE_REQUIRED", response.Reason)
		})
	}
}

func TestMembershipPaymentResponseDoesNotExposeProviderFields(t *testing.T) {
	expiresAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	response := membershipPaymentResponse(&service.CreateOrderResponse{
		OrderID:      42,
		Amount:       12.34,
		PayAmount:    12.34,
		Status:       service.OrderStatusPending,
		PaymentType:  payment.TypeAlipay,
		OutTradeNo:   "membership-trade",
		PayURL:       "https://pay.example.test/checkout",
		QRCode:       "https://pay.example.test/qr",
		Currency:     "CNY",
		ExpiresAt:    expiresAt,
		ClientSecret: "provider-client-secret",
		IntentID:     "provider-intent-id",
		CountryCode:  "CN",
		PaymentEnv:   "production",
		ResumeToken:  "provider-resume-token",
		PaymentMode:  "redirect",
		ResultType:   payment.CreatePaymentResultOrderCreated,
	}, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))

	body, err := json.Marshal(response)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(body, &fields))
	require.Equal(t, float64(42), fields["order_id"])
	require.Equal(t, "CNY", fields["currency"])
	require.Equal(t, "/api/v1/membership/payment/redirect?ticket=AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE", fields["redirect_url"])
	for _, hidden := range []string{
		"client_secret", "intent_id", "country_code", "payment_env", "resume_token",
		"payment_mode", "result_type", "oauth", "jsapi", "jsapi_payload", "pay_url", "qr_code", "out_trade_no",
	} {
		require.NotContains(t, fields, hidden)
	}
}

func TestMembershipPaymentResponseBuildsOnlyBoundSiteRedirect(t *testing.T) {
	ticket := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	response := membershipPaymentResponse(&service.CreateOrderResponse{OrderID: 17}, ticket)
	require.Equal(t, "/api/v1/membership/payment/redirect?ticket=AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI", response.RedirectURL)
	require.Empty(t, membershipPaymentRedirectURL(""))
}

func TestMembershipPaymentRedirectTicketReissuesOnlyForAuthenticatedPendingOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT l.payment_order_id").
		WithArgs("membership-order-1", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"payment_order_id"}).AddRow(int64(9)))
	mock.ExpectExec("DELETE FROM membership_payment_redirect_tickets").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg(), "membership-order-1", int64(42), int64(9), 300).
		WillReturnResult(sqlmock.NewResult(0, 1))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/membership/orders/membership-order-1/payment/redirect-ticket", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "membership-order-1"}}
	ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})

	(&MembershipHandler{engine: &membership.Engine{DB: db}}).ReissuePaymentRedirectTicket(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data struct {
			RedirectURL string `json:"redirect_url"`
			OrderID     int64  `json:"order_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, int64(9), body.Data.OrderID)
	require.Regexp(t, `^/api/v1/membership/payment/redirect\?ticket=[A-Za-z0-9_-]{43}$`, body.Data.RedirectURL)
	require.NotContains(t, recorder.Body.String(), "pay_url")
	require.NotContains(t, recorder.Body.String(), "qr_code")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipPaymentRedirectAllowsNativeNavigationWithSingleUseTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ticket := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))

	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE membership_payment_redirect_tickets").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}).AddRow("membership-order-1", int64(9), int64(42)))
	mock.ExpectQuery("SELECT po.pay_url,po.qr_code").
		WithArgs("membership-order-1", int64(42), int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"pay_url", "qr_code"}).AddRow("https://pay.example.test/checkout?private=1", nil))
	mock.ExpectCommit()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/membership/payment/redirect?ticket="+ticket, nil)

	(&MembershipHandler{engine: &membership.Engine{DB: db}}).RedirectPayment(ctx)

	require.Equal(t, http.StatusFound, recorder.Code)
	require.Equal(t, "https://pay.example.test/checkout?private=1", recorder.Header().Get("Location"))
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipPaymentRedirectFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name         string
		consumeRows  *sqlmock.Rows
		providerRows *sqlmock.Rows
		wantStatus   int
	}{
		{name: "expired or already consumed", consumeRows: sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}), wantStatus: http.StatusNotFound},
		{name: "wrong membership payment binding", consumeRows: sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}).AddRow("membership-order-1", int64(9), int64(42)), providerRows: sqlmock.NewRows([]string{"pay_url", "qr_code"}), wantStatus: http.StatusNotFound},
		{name: "non web provider target", consumeRows: sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}).AddRow("membership-order-1", int64(9), int64(42)), providerRows: sqlmock.NewRows([]string{"pay_url", "qr_code"}).AddRow("weixin://wxpay/bizpayurl?private=1", nil), wantStatus: http.StatusNotFound},
		{name: "missing provider target", consumeRows: sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}).AddRow("membership-order-1", int64(9), int64(42)), providerRows: sqlmock.NewRows([]string{"pay_url", "qr_code"}).AddRow(nil, nil), wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			ticket := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
			mock.ExpectBegin()
			mock.ExpectQuery("UPDATE membership_payment_redirect_tickets").WithArgs(sqlmock.AnyArg()).WillReturnRows(tc.consumeRows)
			if tc.providerRows != nil {
				mock.ExpectQuery("SELECT po.pay_url,po.qr_code").WithArgs("membership-order-1", int64(42), int64(9)).WillReturnRows(tc.providerRows)
				mock.ExpectCommit()
			}

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/membership/payment/redirect?ticket="+ticket, nil)

			(&MembershipHandler{engine: &membership.Engine{DB: db}}).RedirectPayment(ctx)

			require.Equal(t, tc.wantStatus, recorder.Code)
			require.Empty(t, recorder.Header().Get("Location"))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMembershipPaymentRedirectCannotReplayConsumedTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ticket := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32))
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE membership_payment_redirect_tickets").WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"order_id", "payment_order_id", "user_id"}).AddRow("membership-order-1", int64(9), int64(42)))
	mock.ExpectQuery("SELECT po.pay_url,po.qr_code").WithArgs("membership-order-1", int64(42), int64(9)).WillReturnRows(sqlmock.NewRows([]string{"pay_url", "qr_code"}).AddRow("https://pay.example.test/checkout", nil))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE membership_payment_redirect_tickets").WithArgs(sqlmock.AnyArg()).WillReturnError(sql.ErrNoRows)

	h := &MembershipHandler{engine: &membership.Engine{DB: db}}
	for _, wantStatus := range []int{http.StatusFound, http.StatusNotFound} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/membership/payment/redirect?ticket="+ticket, nil)
		h.RedirectPayment(ctx)
		require.Equal(t, wantStatus, recorder.Code)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipPaymentErrorDoesNotExposeProviderDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/membership/orders/order-1/payment", nil)

	providerErr := infraerrors.ServiceUnavailable("PROVIDER_SECRET_REASON", "provider rejected request").WithMetadata(map[string]string{
		"provider": "supplier.internal.example",
		"trace_id": "private-trace-id",
	})
	membershipPaymentError(ctx, providerErr)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var body struct {
		Message  string            `json:"message"`
		Reason   string            `json:"reason"`
		Metadata map[string]string `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "Membership payment is currently unavailable", body.Message)
	require.Equal(t, "MEMBERSHIP_PAYMENT_UNAVAILABLE", body.Reason)
	require.Nil(t, body.Metadata)

	require.NotContains(t, recorder.Body.String(), "provider rejected request")
	require.NotContains(t, recorder.Body.String(), "PROVIDER_SECRET_REASON")
	require.NotContains(t, recorder.Body.String(), "supplier.internal.example")
	require.NotContains(t, recorder.Body.String(), "private-trace-id")
}
