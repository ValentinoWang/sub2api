package admin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLiandongToolkitStrictBrowserRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{``, `{"goods_id":42,"command":"arbitrary"}`, `{"goods_id":42} {}`, strings.Repeat("x", liandongToolkitMaxJSONBody+1)} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
		var req struct {
			GoodsID int64 `json:"goods_id"`
		}
		present, err := bindLiandongToolkitJSON(c, &req, true)
		require.True(t, !present || err != nil)
	}
}
func TestLiandongToolkitBrowserErrorsDoNotExposeCauses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err    error
		status int
	}{
		{service.ErrLiandongNeedsReconciliation, http.StatusConflict},
		{fmt.Errorf("secret-fixture-value"), http.StatusBadGateway},
		{infraerrors.Unauthorized("INVALID_DEVICE", "device unavailable").WithCause(fmt.Errorf("secret-fixture-value")), http.StatusUnauthorized},
	} {
		r := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(r)
		writeLiandongToolkitError(c, tc.err, "Chrome restock")
		require.Equal(t, tc.status, r.Code)
		require.NotContains(t, r.Body.String(), "secret-fixture-value")
	}
}
