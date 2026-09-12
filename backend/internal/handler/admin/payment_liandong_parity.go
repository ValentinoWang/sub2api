package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) GetLiandongCommerceParity(c *gin.Context) {
	svc, ok := h.liandong.(interface {
		CommerceParity(context.Context) (map[string]any, error)
	})
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Commerce parity is unavailable")
		return
	}
	data, err := svc.CommerceParity(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Unable to read commerce parity")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, data)
}
