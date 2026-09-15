package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/costing"
	"github.com/gin-gonic/gin"
	"net/http"
)

// registerCostCenterRoutes takes the authenticated/audited admin group, never a public group.
// These are calculator endpoints only; no procurement or account writes are exposed.
func registerCostCenterRoutes(admin *gin.RouterGroup) {
	handler := costing.HTTPHandler{Authorize: func(_ *http.Request) bool { return true }}
	serve := func(c *gin.Context) { handler.ServeHTTP(c.Writer, c.Request) }
	admin.GET("/cost-center/catalog", serve)
	admin.POST("/cost-center/compare", serve)
}
