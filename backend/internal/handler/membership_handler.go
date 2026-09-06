package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/membership"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type MembershipHandler struct {
	engine         *membership.Engine
	paymentService *service.PaymentService
	totpService    *service.TotpService
	userService    *service.UserService
}

func ProvideMembershipHandler(
	engine *membership.Engine,
	paymentService *service.PaymentService,
	totpService *service.TotpService,
	userService *service.UserService,
	emailService *service.EmailService,
	settingService *service.SettingService,
) *MembershipHandler {
	paymentService.SetMembershipEngine(engine)
	engine.Notify = func(ctx context.Context, userID int64, orderID, state string) error {
		user, err := userService.GetByID(ctx, userID)
		if err != nil || strings.TrimSpace(user.Email) == "" {
			return membership.ErrUnavailable
		}
		site := settingService.GetSiteName(ctx)
		subject := fmt.Sprintf("[%s] 会员订单状态更新", site)
		body := fmt.Sprintf("您的会员订单 %s 状态已更新为：%s。请登录 %s 查看详情。", orderID, customerStateLabel(state), site)
		return emailService.SendEmail(ctx, user.Email, subject, body)
	}
	engine.Start()
	return &MembershipHandler{engine: engine, paymentService: paymentService, totpService: totpService, userService: userService}
}

func customerStateLabel(state string) string {
	switch state {
	case "awaiting_input":
		return "待补充信息"
	case "queued":
		return "排队中"
	case "submitted", "processing":
		return "充值中"
	case "review_required":
		return "待复核"
	case "succeeded":
		return "成功"
	case "failed":
		return "失败"
	case "refund_pending":
		return "退款中"
	case "refunded":
		return "已退款"
	default:
		return "已更新"
	}
}

func (h *MembershipHandler) Products(c *gin.Context) {
	products, err := h.engine.Products(c.Request.Context())
	if membershipError(c, err) {
		return
	}
	response.Success(c, membership.CustomerProductsDTO(products))
}

func (h *MembershipHandler) CreateOrder(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	var input membership.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	id, err := h.engine.Create(c.Request.Context(), userID, input, false)
	if membershipError(c, err) {
		return
	}
	response.Created(c, gin.H{"id": id})
}

// CreatePayment creates a payment only for the authenticated owner's membership
// order. Price and order type are derived from the membership domain instead of
// accepting either from the client.
// POST /api/v1/membership/orders/:id/payment
func (h *MembershipHandler) CreatePayment(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	if h.engine == nil || h.paymentService == nil {
		membershipError(c, membership.ErrUnavailable)
		return
	}

	var input MembershipPaymentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}

	// Requote immediately before checkout. PaymentService requotes again while
	// attaching the payment transaction, so a changed price or order state cannot
	// be paid using a stale client value.
	quoteMinor, err := h.engine.PaymentQuote(c.Request.Context(), userID, c.Param("id"))
	if membershipError(c, err) {
		return
	}

	mobile := isMobile(c)
	if input.IsMobile != nil {
		mobile = *input.IsMobile
	}
	result, err := h.paymentService.CreateOrder(c.Request.Context(), service.CreateOrderRequest{
		MembershipOrderID: c.Param("id"),
		UserID:            userID,
		Amount:            float64(quoteMinor) / 100,
		PaymentType:       input.PaymentType,
		OpenID:            input.OpenID,
		ClientIP:          c.ClientIP(),
		IsMobile:          mobile,
		IsWeChatBrowser:   isWeChatBrowser(c),
		SrcHost:           c.Request.Host,
		SrcURL:            c.Request.Referer(),
		ReturnURL:         input.ReturnURL,
		PaymentSource:     input.PaymentSource,
		OrderType:         payment.OrderTypeMembership,
		Locale:            c.GetHeader("Accept-Language"),
	})
	if err != nil {
		membershipPaymentError(c, err)
		return
	}
	ticket, err := h.engine.CreatePaymentRedirectTicket(c.Request.Context(), userID, c.Param("id"), result.OrderID)
	if err != nil {
		membershipPaymentError(c, err)
		return
	}
	response.Success(c, membershipPaymentResponse(result, ticket))
}

// MembershipPaymentInput deliberately excludes amount, plan, order type, and
// provider-resume fields. Membership checkout owns those values and cannot be
// resumed without a membership-order binding.
type MembershipPaymentInput struct {
	PaymentType   string `json:"payment_type" binding:"required"`
	OpenID        string `json:"openid"`
	ReturnURL     string `json:"return_url"`
	PaymentSource string `json:"payment_source"`
	IsMobile      *bool  `json:"is_mobile,omitempty"`
}

// MembershipPaymentResponse is the customer checkout contract. The provider
// destination remains server-side and is reached only through RedirectPayment.
type MembershipPaymentResponse struct {
	OrderID     int64     `json:"order_id"`
	Amount      float64   `json:"amount"`
	PayAmount   float64   `json:"pay_amount"`
	Status      string    `json:"status"`
	PaymentType string    `json:"payment_type"`
	Currency    string    `json:"currency,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	RedirectURL string    `json:"redirect_url"`
}

func membershipPaymentResponse(result *service.CreateOrderResponse, redirectTicket string) *MembershipPaymentResponse {
	if result == nil {
		return nil
	}
	return &MembershipPaymentResponse{
		OrderID:     result.OrderID,
		Amount:      result.Amount,
		PayAmount:   result.PayAmount,
		Status:      result.Status,
		PaymentType: result.PaymentType,
		Currency:    result.Currency,
		ExpiresAt:   result.ExpiresAt,
		RedirectURL: membershipPaymentRedirectURL(redirectTicket),
	}
}

// ReissuePaymentRedirectTicket restores a local checkout URL for a still
// pending payment after the original one was lost, expired, or cleared by a
// page refresh. It never accepts a payment-order identifier from the client.
// POST /api/v1/membership/orders/:id/payment/redirect-ticket
func (h *MembershipHandler) ReissuePaymentRedirectTicket(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	if h.engine == nil {
		membershipPaymentError(c, membership.ErrUnavailable)
		return
	}
	ticket, paymentOrderID, err := h.engine.ReissuePaymentRedirectTicket(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		membershipPaymentError(c, err)
		return
	}
	response.Success(c, gin.H{"redirect_url": membershipPaymentRedirectURL(ticket), "order_id": paymentOrderID})
}

// membershipPaymentError keeps provider errors out of the customer checkout
// contract. Provider reasons, messages, and metadata may disclose operational
// details that are not actionable to a membership customer.
func membershipPaymentError(c *gin.Context, _ error) {
	response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Membership payment is currently unavailable", "MEMBERSHIP_PAYMENT_UNAVAILABLE", nil)
}

func membershipPaymentRedirectURL(ticket string) string {
	if strings.TrimSpace(ticket) == "" {
		return ""
	}
	return "/api/v1/membership/payment/redirect?ticket=" + url.QueryEscape(ticket)
}

// RedirectPayment consumes a short-lived ticket before looking up the provider
// target. It is intentionally public because browser navigation cannot attach
// the Authorization header stored by the panel client.
// GET /api/v1/membership/payment/redirect?ticket=:ticket
func (h *MembershipHandler) RedirectPayment(c *gin.Context) {
	if h.engine == nil {
		membershipPaymentRedirectError(c)
		return
	}
	target, err := h.engine.ConsumePaymentRedirectTicket(c.Request.Context(), c.Query("ticket"))
	if err != nil {
		membershipPaymentRedirectError(c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Redirect(http.StatusFound, target)
}

func membershipPaymentRedirectError(c *gin.Context) {
	response.ErrorWithDetails(c, http.StatusNotFound, "Membership payment redirect unavailable", "MEMBERSHIP_PAYMENT_REDIRECT_UNAVAILABLE", nil)
}

func (h *MembershipHandler) PutCredential(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	var input membership.CredentialInput
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.PutCredential(c.Request.Context(), userID, c.Param("id"), input)) {
		return
	}
	response.Success(c, gin.H{"accepted": true, "expires_in_seconds": int(membership.CredentialTTL.Seconds())})
}

func (h *MembershipHandler) Orders(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	orders, err := h.engine.Orders(c.Request.Context(), userID, false)
	if membershipError(c, err) {
		return
	}
	response.Success(c, membership.CustomerOrdersDTO(orders))
}

func (h *MembershipHandler) Order(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	order, err := h.engine.Order(c.Request.Context(), userID, c.Param("id"), false)
	if membershipError(c, err) {
		return
	}
	response.Success(c, membership.CustomerOrderDTOFrom(order))
}

func (h *MembershipHandler) RequestRefund(c *gin.Context) {
	userID, ok := membershipUser(c)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.RequestRefund(c.Request.Context(), userID, c.Param("id"), input.Reason)) {
		return
	}
	response.Accepted(c, gin.H{"state": "review_required"})
}

func (h *MembershipHandler) AdminOverview(c *gin.Context) {
	result, err := h.engine.AdminOverview(c.Request.Context())
	if membershipError(c, err) {
		return
	}
	response.Success(c, membership.AdminOverviewDTOFrom(result))
}

func (h *MembershipHandler) AdminOrder(c *gin.Context) {
	order, err := h.engine.Order(c.Request.Context(), 0, c.Param("id"), true)
	if membershipError(c, err) {
		return
	}
	response.Success(c, membership.AdminOrderDTOFrom(order))
}

func (h *MembershipHandler) UpdateProduct(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	var input membership.ProductUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.UpdateProduct(c.Request.Context(), actor, c.Param("sku"), input)) {
		return
	}
	response.Success(c, gin.H{"updated": true})
}

func (h *MembershipHandler) CheckAvailability(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	if membershipError(c, h.engine.CheckAvailability(c.Request.Context(), actor, c.Param("sku"))) {
		return
	}
	response.Success(c, gin.H{"checked": true})
}

func (h *MembershipHandler) CreateValidationOrder(c *gin.Context) {
	_, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	var input membership.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	id, err := h.engine.Create(c.Request.Context(), subject.UserID, input, true)
	if membershipError(c, err) {
		return
	}
	response.Created(c, gin.H{"id": id})
}

func (h *MembershipHandler) VerifyProduct(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	var input struct {
		RunID    string `json:"run_id"`
		Evidence string `json:"evidence_ref"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.VerifyProduct(c.Request.Context(), actor, c.Param("sku"), input.RunID, input.Evidence)) {
		return
	}
	response.Success(c, gin.H{"verified": true})
}

func (h *MembershipHandler) ImportCDKs(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	var input struct {
		SKU       string   `json:"sku"`
		Codes     []string `json:"codes"`
		CostMinor int64    `json:"cost_minor"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.ImportCDKs(c.Request.Context(), actor, input.SKU, input.Codes, input.CostMinor)) {
		return
	}
	response.Created(c, gin.H{"imported": len(input.Codes)})
}

func (h *MembershipHandler) Review(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	var input struct {
		Action   string `json:"action"`
		Evidence string `json:"evidence_ref"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.Review(c.Request.Context(), actor, c.Param("id"), input.Action, input.Evidence)) {
		return
	}
	response.Success(c, gin.H{"accepted": true})
}

func (h *MembershipHandler) SaveCoupon(c *gin.Context) {
	actor, ok := h.requireStepUp(c)
	if !ok {
		return
	}
	var input struct {
		Code          string    `json:"code"`
		DiscountMinor int64     `json:"discount_minor"`
		MaxUses       int       `json:"max_uses"`
		ExpiresAt     time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		membershipError(c, membership.ErrInvalid)
		return
	}
	if membershipError(c, h.engine.SaveCoupon(c.Request.Context(), actor, input.Code, input.DiscountMinor, input.MaxUses, input.ExpiresAt)) {
		return
	}
	response.Created(c, gin.H{"saved": true})
}

func (h *MembershipHandler) requireStepUp(c *gin.Context) (string, bool) {
	if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
		return "", false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Authorization required")
		return "", false
	}
	return strconv.FormatInt(subject.UserID, 10), true
}

func membershipUser(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authorization required")
		return 0, false
	}
	return subject.UserID, true
}

func membershipError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, membership.ErrInvalid):
		response.ErrorWithDetails(c, http.StatusBadRequest, "Invalid membership request", "MEMBERSHIP_INVALID_INPUT", nil)
	case errors.Is(err, membership.ErrConsent):
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Explicit credential consent is required", "MEMBERSHIP_CONSENT_REQUIRED", nil)
	case errors.Is(err, membership.ErrCredential):
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Credential expired or unavailable", "CREDENTIAL_EXPIRED", nil)
	case errors.Is(err, membership.ErrNotFound):
		response.ErrorWithDetails(c, http.StatusNotFound, "Membership order not found", "MEMBERSHIP_NOT_FOUND", nil)
	case errors.Is(err, membership.ErrConflict), errors.Is(err, membership.ErrReview):
		response.ErrorWithDetails(c, http.StatusConflict, "Membership order requires review", "REVIEW_REQUIRED", nil)
	case errors.Is(err, membership.ErrUnavailable):
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "Membership service unavailable", "MEMBERSHIP_UNAVAILABLE", nil)
	default:
		response.ErrorWithDetails(c, http.StatusInternalServerError, "Membership service unavailable", "MEMBERSHIP_INTERNAL_ERROR", nil)
	}
	return true
}
