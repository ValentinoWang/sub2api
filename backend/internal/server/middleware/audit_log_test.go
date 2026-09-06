package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeriveAuditAction(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{"PUT", "/api/v1/admin/accounts/:id", "admin.accounts.update"},
		{"POST", "/api/v1/admin/accounts", "admin.accounts.create"},
		{"DELETE", "/api/v1/admin/backups/:id", "admin.backups.delete"},
		{"GET", "/api/v1/admin/users/:id/api-keys", "admin.users.api_keys.read"},
		{"POST", "/api/v1/admin/redeem-codes/batch", "admin.redeem_codes.batch.create"},
	}
	for _, tc := range cases {
		if got := deriveAuditAction(tc.method, tc.path); got != tc.want {
			t.Fatalf("deriveAuditAction(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

type auditCaptureRepository struct {
	mu   sync.Mutex
	logs []*service.AuditLog
}

func (r *auditCaptureRepository) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}
func (r *auditCaptureRepository) Insert(_ context.Context, log *service.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}
func (r *auditCaptureRepository) List(context.Context, *service.AuditLogFilter) (*service.AuditLogList, error) {
	return &service.AuditLogList{}, nil
}
func (r *auditCaptureRepository) GetByID(context.Context, int64) (*service.AuditLog, error) {
	return nil, service.ErrAuditLogNotFound
}
func (r *auditCaptureRepository) Count(context.Context) (int64, error) { return 0, nil }
func (r *auditCaptureRepository) TruncateAll(context.Context) error    { return nil }
func (r *auditCaptureRepository) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func TestPromptAuditAdminOperationsUseOmittedBodiesAndAllowlistedDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
		c.Set(string(ContextKeyUserRole), "admin")
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	router.PUT("/api/v1/admin/prompt-audit/config", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{
			"result": "failed", "error_code": "prompt_audit_config_conflict", "config_version": int64(9),
			"token": "audit-canary-secret", "raw_prompt": "audit-canary-prompt", "nested": map[string]any{"unsafe": true},
		})
		c.JSON(http.StatusConflict, gin.H{"ok": false})
	})
	router.POST("/api/v1/admin/prompt-audit/endpoints/probe", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{
			"result": "success", "guard_endpoint_id": "guard-1", "http_status": 200,
			"latency_ms": 12, "token_applied": true,
		})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPut, "/api/v1/admin/prompt-audit/config", bytes.NewBufferString(`{"expected_config_version":8,"token":"audit-canary-secret"}`)),
		httptest.NewRequest(http.MethodPost, "/api/v1/admin/prompt-audit/endpoints/probe", bytes.NewBufferString(`{"endpoint":{"token":"audit-canary-secret"}}`)),
	} {
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
	}
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 2)

	byAction := make(map[string]*service.AuditLog, len(logs))
	for _, entry := range logs {
		byAction[entry.Action] = entry
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		require.NotContains(t, entry.RequestBody, "audit-canary")
		require.NotContains(t, entry.Extra, "token")
		require.NotContains(t, entry.Extra, "raw_prompt")
		require.NotContains(t, entry.Extra, "nested")
	}

	config := byAction["admin.prompt_audit.config.update"]
	require.NotNil(t, config)
	require.Equal(t, http.StatusConflict, config.StatusCode)
	require.Equal(t, "failed", config.Extra["result"])
	require.Equal(t, "prompt_audit_config_conflict", config.Extra["error_code"])
	require.EqualValues(t, 9, config.Extra["config_version"])

	probe := byAction["admin.prompt_audit.endpoint.probe"]
	require.NotNil(t, probe)
	require.Equal(t, http.StatusOK, probe.StatusCode)
	require.Equal(t, "success", probe.Extra["result"])
	require.Equal(t, "guard-1", probe.Extra["guard_endpoint_id"])
	require.Equal(t, true, probe.Extra["token_applied"])
}

func TestPromptAuditMutationAuditRoutesHaveStableActionsAndOmitBodies(t *testing.T) {
	expected := map[string]string{
		"PUT /api/v1/admin/prompt-audit/config":                   "admin.prompt_audit.config.update",
		"POST /api/v1/admin/prompt-audit/endpoints/probe":         "admin.prompt_audit.endpoint.probe",
		"DELETE /api/v1/admin/prompt-audit/events/:id":            "admin.prompt_audit.event.delete",
		"POST /api/v1/admin/prompt-audit/events/batch-delete":     "admin.prompt_audit.events.batch_delete",
		"POST /api/v1/admin/prompt-audit/events/delete-preview":   "admin.prompt_audit.events.delete_preview",
		"POST /api/v1/admin/prompt-audit/events/delete-by-filter": "admin.prompt_audit.events.filter_delete",
	}
	for route, action := range expected {
		require.Equal(t, action, auditActionOverrides[route])
		_, omitted := auditBodyOmittedRoutes[route]
		require.Truef(t, omitted, "%s must not persist its credential or confirmation-bearing body", route)
	}
}

func TestPasskeyLoginAuditUsesCanonicalLoginActionAndOmitsCredentialBody(t *testing.T) {
	route := "POST /api/v1/auth/passkey/login/finish"
	require.Equal(t, service.AuditActionLogin, auditActionOverrides[route])
	require.Contains(t, auditBodyOmittedRoutes, route)
}

func TestMembershipCredentialRoutesOmitAuditBodies(t *testing.T) {
	for _, route := range []string{
		"PUT /api/v1/membership/orders/:id/credential",
		"POST /api/v1/admin/membership/cdks/import",
	} {
		require.Contains(t, auditBodyOmittedRoutes, route)
	}
}

func TestMembershipAdminVerifyAndReviewOmitAuditBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		method string
		route  string
		path   string
		status int
	}{
		{method: http.MethodPost, route: "/api/v1/admin/membership/products/:sku/verify", path: "/api/v1/admin/membership/products/product-1/verify", status: http.StatusBadRequest},
		{method: http.MethodPost, route: "/api/v1/admin/membership/orders/:id/review", path: "/api/v1/admin/membership/orders/order-1/review", status: http.StatusConflict},
	}

	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	for _, tc := range cases {
		status := tc.status
		router.POST(tc.route, func(c *gin.Context) { c.Status(status) })
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(`{"evidence_ref":"https://example.invalid/evidence?token=url-secret","session":"session-secret","cdk":"CDK-ABCD-1234-EFGH-5678","token":"jwt-secret"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, tc.status, recorder.Code)
	}
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, len(cases))
	byRoute := make(map[string]*service.AuditLog, len(logs))
	for _, entry := range logs {
		byRoute[entry.Path] = entry
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		require.NotNil(t, entry.ActorUserID)
		require.EqualValues(t, 77, *entry.ActorUserID)
		require.Equal(t, service.RoleAdmin, entry.ActorRole)
		for _, secret := range []string{"session-secret", "CDK-ABCD-1234-EFGH-5678", "jwt-secret", "https://example.invalid", "url-secret"} {
			require.NotContains(t, entry.RequestBody, secret)
		}
	}
	for _, tc := range cases {
		entry := byRoute[tc.route]
		require.NotNil(t, entry)
		require.Equal(t, tc.status, entry.StatusCode)
	}
}

// Ollama 会话保存的请求体整体就是浏览器 Cookie 明文，键级脱敏清单曾漏掉裸键
// "session"，必须走整体不入库路径，防止会话凭证长期留存在 audit_logs。
func TestOllamaCloudUsageSessionRouteOmitsAuditBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.Contains(t, auditBodyOmittedRoutes, "PUT /api/v1/admin/accounts/:id/ollama-cloud-usage/session")

	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
		c.Set(string(ContextKeyUserRole), "admin")
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	router.PUT("/api/v1/admin/accounts/:id/ollama-cloud-usage/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/7/ollama-cloud-usage/session",
		bytes.NewBufferString(`{"session":"wos-session=audit-canary-cookie; __Secure-authjs.session-token.0=audit-canary-shard"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 1)
	require.Equal(t, "<credential-bearing body omitted>", logs[0].RequestBody)
	require.NotContains(t, logs[0].RequestBody, "audit-canary")
}

func TestLiandongSensitiveReadsHaveStableActionsAndOmitQueryMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := []struct {
		route  string
		path   string
		action string
	}{
		{"/api/v1/admin/tools/ldxp/installation", "/api/v1/admin/tools/ldxp/installation", "admin.tools.ldxp.installation.read"},
		{"/api/v1/admin/tools/ldxp/status", "/api/v1/admin/tools/ldxp/status", "admin.tools.ldxp.status.read"},
		{"/api/v1/admin/tools/ldxp/goods", "/api/v1/admin/tools/ldxp/goods", "admin.tools.ldxp.goods.read"},
		{"/api/v1/admin/tools/ldxp/jobs/:id", "/api/v1/admin/tools/ldxp/jobs/job-1", "admin.tools.ldxp.jobs.read"},
	}
	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	for _, item := range paths {
		route := item.route
		router.GET(route, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	}
	for _, item := range paths {
		request := httptest.NewRequest(http.MethodGet, item.path+"?url=https%3A%2F%2Fuser%3Apass%40example.invalid%2F%3Ftoken%3Dquery-secret", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
	}
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, len(paths))
	byAction := make(map[string]*service.AuditLog, len(logs))
	for _, entry := range logs {
		byAction[entry.Action] = entry
		require.Empty(t, entry.RequestBody)
		require.NotContains(t, entry.Path, "?")
		require.NotContains(t, entry.Path, "query-secret")
		require.NotContains(t, entry.Extra, "query")
		require.NotContains(t, entry.Extra, "query-secret")
	}
	for _, item := range paths {
		require.NotNil(t, byAction[item.action])
	}
}

func TestLiandongLimiterRejectionIsAuditedWithoutSensitiveMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()
	limiter := &PanelRateLimiter{
		limiter:        &fakePanelAllower{err: errors.New("redis unavailable")},
		settingService: nil,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	router.Use(limiter.LDXP())
	router.PUT("/api/v1/admin/tools/ldxp/config", func(c *gin.Context) { c.Status(http.StatusOK) })

	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tools/ldxp/config",
		bytes.NewBufferString(`{"merchant_token":"merchant-secret","code_secret":"code-secret","external_url":"https://user:pass@example.invalid/?token=url-secret"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 1)
	require.Equal(t, "admin.tools.ldxp.config.update", logs[0].Action)
	require.Equal(t, http.StatusServiceUnavailable, logs[0].StatusCode)
	require.Equal(t, "<credential-bearing body omitted>", logs[0].RequestBody)
	for _, secret := range []string{"merchant-secret", "code-secret", "url-secret", "user:pass"} {
		require.NotContains(t, logs[0].RequestBody, secret)
		require.NotContains(t, logs[0].Path, secret)
		require.NotContains(t, logs[0].Extra, secret)
	}
}
