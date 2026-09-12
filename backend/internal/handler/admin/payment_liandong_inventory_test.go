package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type inventoryHandlerStub struct {
	liandongHandlerStub
	report *service.LiandongInventoryReport
	err    error
}

func (s *inventoryHandlerStub) Inventory(context.Context) (*service.LiandongInventoryReport, error) {
	return s.report, s.err
}

func TestLiandongInventoryHandlerPreservesUnknownAndRedactsFailures(t *testing.T) {
	for _, fail := range []bool{false, true} {
		stub := &inventoryHandlerStub{report: &service.LiandongInventoryReport{Rows: []service.LiandongInventoryRow{{GoodsID: 42, Comparison: "unknown"}}}}
		if fail {
			stub.err = errors.New("private merchant token and driver response")
		}
		h := NewPaymentHandler(nil, nil, stub)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/inventory", h.GetLiandongInventory)
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/inventory", nil))
		require.Equal(t, "no-store", writer.Header().Get("Cache-Control"))
		if fail {
			require.Equal(t, http.StatusServiceUnavailable, writer.Code)
			require.NotContains(t, writer.Body.String(), "private")
		} else {
			require.Equal(t, http.StatusOK, writer.Code)
			require.Contains(t, writer.Body.String(), `"merchant_unsold":null`)
			require.Contains(t, writer.Body.String(), `"identity_verified":false`)
		}
	}
}
