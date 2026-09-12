package admin

import (
	"context"
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type liandongRefundOperator interface {
	PrepareUnusedCodeRefund(context.Context, service.LiandongRefundPrepareRequest) (*service.LiandongCodeRefund, error)
	ConfirmUnusedCodeRefund(context.Context, service.LiandongRefundConfirmRequest) (*service.LiandongCodeRefund, error)
}

func (h *PaymentHandler) PrepareLiandongUnusedCodeRefund(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req service.LiandongRefundPrepareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid Liandong refund request")
		return
	}
	svc, ok := h.liandong.(liandongRefundOperator)
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Liandong refund is unavailable")
		return
	}
	result, err := svc.PrepareUnusedCodeRefund(c.Request.Context(), req)
	if err != nil {
		respondLiandongRefundError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PaymentHandler) ConfirmLiandongUnusedCodeRefund(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req service.LiandongRefundConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid Liandong refund request")
		return
	}
	svc, ok := h.liandong.(liandongRefundOperator)
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Liandong refund is unavailable")
		return
	}
	result, err := svc.ConfirmUnusedCodeRefund(c.Request.Context(), req)
	if err != nil {
		respondLiandongRefundError(c, err)
		return
	}
	response.Success(c, result)
}

func respondLiandongRefundError(c *gin.Context, err error) {
	// Never expose driver/binding errors; these requests carry a redeem code.
	switch {
	case errors.Is(err, service.ErrLiandongRefundInvalid):
		response.BadRequest(c, service.ErrLiandongRefundInvalid.Error())
	case errors.Is(err, service.ErrLiandongRefundConflict):
		response.Error(c, http.StatusConflict, service.ErrLiandongRefundConflict.Error())
	case errors.Is(err, service.ErrLiandongRefundNotEligible):
		response.Error(c, http.StatusConflict, service.ErrLiandongRefundNotEligible.Error())
	case errors.Is(err, service.ErrLiandongRefundNotFound):
		response.Error(c, http.StatusNotFound, service.ErrLiandongRefundNotFound.Error())
	default:
		response.Error(c, http.StatusServiceUnavailable, service.ErrLiandongRefundUnavailable.Error())
	}
}

func (h *PaymentHandler) liandongService(c *gin.Context) liandongRestockOperator {
	if h.liandong == nil {
		response.Error(c, http.StatusServiceUnavailable, "Liandong restock is not available")
		return nil
	}
	return h.liandong
}

// GetLiandongRestockStatus returns the fixed-product channel status.
func (h *PaymentHandler) GetLiandongRestockStatus(c *gin.Context) {
	svc := h.liandongService(c)
	if svc == nil {
		return
	}
	status, err := svc.Status(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// UpdateLiandongRestockConfig stores encrypted merchant settings and mappings.
func (h *PaymentHandler) UpdateLiandongRestockConfig(c *gin.Context) {
	svc := h.liandongService(c)
	if svc == nil {
		return
	}
	var req service.LiandongRestockConfigurationUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid Liandong configuration: "+err.Error())
		return
	}
	status, err := svc.UpdateConfiguration(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// UpdateLiandongRestockPolicies updates inventory thresholds and enablement.
func (h *PaymentHandler) UpdateLiandongRestockPolicies(c *gin.Context) {
	svc := h.liandongService(c)
	if svc == nil {
		return
	}
	var req []service.LiandongRestockPolicyUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid Liandong policies: "+err.Error())
		return
	}
	status, err := svc.UpdatePolicies(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// RunLiandongRestockNow triggers one reconciliation cycle.
func (h *PaymentHandler) RunLiandongRestockNow(c *gin.Context) {
	svc := h.liandongService(c)
	if svc == nil {
		return
	}
	if err := svc.RunOnce(c.Request.Context(), true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	status, err := svc.Status(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// SetLiandongRestockEnabled changes the persisted worker flag.
func (h *PaymentHandler) SetLiandongRestockEnabled(c *gin.Context) {
	svc := h.liandongService(c)
	if svc == nil {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid Liandong enablement request: "+err.Error())
		return
	}
	status, err := svc.SetEnabled(c.Request.Context(), req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, status)
}
