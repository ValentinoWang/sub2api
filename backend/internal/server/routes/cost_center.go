package routes

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/costing"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// RegisterCostCenterRoutes always uses the existing verified admin, audit and compliance middleware.
// ledgerDSN is an explicit deployment setting, never accepted from an HTTP body.
// Empty DSN disables persistence rather than falling back to an unreviewed production database.
func RegisterCostCenterRoutes(v1 *gin.RouterGroup, auth middleware.AdminAuthMiddleware,
	audit middleware.AuditLogMiddleware, settings *service.SettingService,
	limiter *middleware.PanelRateLimiter, ledgerDSN string) {
	var ledger *costing.Ledger
	if strings.TrimSpace(ledgerDSN) != "" {
		ledger = &costing.Ledger{Store: &costing.SQLLedgerStore{Open: func() (*sql.DB, func(), error) {
			db, err := sql.Open("postgres", ledgerDSN)
			if err != nil {
				return nil, nil, err
			}
			// Low-frequency admin ledger only. A bounded connection per operation avoids a new unmanaged process-wide pool.
			// A later DI-managed shared pool can use the same store without changing the ledger contract.
			db.SetMaxOpenConns(1)
			db.SetMaxIdleConns(0)
			return db, func() { _ = db.Close() }, nil
		}}}
	}
	admin := v1.Group("/admin/cost-center")
	admin.Use(gin.HandlerFunc(auth), limiter.Global(), gin.HandlerFunc(audit), middleware.AdminComplianceGuard(settings))
	serve := func(c *gin.Context) {
		value, ok := c.Get(string(middleware.ContextKeyUser))
		subject, valid := value.(middleware.AuthSubject)
		allowed := ok && valid && subject.UserID > 0 && c.GetString(string(middleware.ContextKeyUserRole)) == "admin" && !c.IsAborted()
		h := costing.HTTPHandler{Authorize: func(*http.Request) bool { return allowed }, Actor: func(*http.Request) int64 { return subject.UserID }, Ledger: ledger}
		h.ServeHTTP(c.Writer, c.Request)
	}
	admin.GET("/catalog", serve)
	admin.POST("/compare", serve)
	admin.GET("/ledger/health", serve)
	admin.GET("/ledger/events", serve)
	admin.GET("/ledger/summary", serve)
	admin.POST("/ledger/commands", serve)
}
