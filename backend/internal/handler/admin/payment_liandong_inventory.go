package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) GetLiandongInventory(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	svc, ok := h.liandong.(interface {
		Inventory(context.Context) (*service.LiandongInventoryReport, error)
	})
	if !ok {
		response.Error(c, http.StatusServiceUnavailable, "Liandong inventory is unavailable")
		return
	}
	result, err := svc.Inventory(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Liandong inventory could not be observed")
		return
	}
	response.Success(c, result)
}
