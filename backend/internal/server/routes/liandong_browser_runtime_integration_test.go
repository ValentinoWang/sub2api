//go:build integration

package routes_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestLiandongBrowserRuntimeIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("runtime_report_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), testcontainers.WithWaitStrategyAndDeadline(2*time.Minute, wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2*time.Minute), wait.ForListeningPort("5432/tcp").WithStartupTimeout(2*time.Minute)))
	if container != nil {
		t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	}
	require.NoError(t, err)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	svc := service.NewLiandongRestockService(nil, nil, nil, nil, db)
	_, err = svc.BrowserSaveConfig(ctx, service.LiandongBrowserConfig{Enabled: true, Products: []service.LiandongBrowserProduct{{GoodsID: 42, CNYAmount: 5, USDCredit: 5, ExternalURL: "https://wzyp.cn/item/runtime-test", TargetStock: 2, BatchSize: 2, Enabled: true}}})
	require.NoError(t, err)
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminAuth := middleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() })
	routes.RegisterLiandongToolRoutes(router.Group("/api/v1"), adminhandler.NewLiandongToolkitHandler(svc), adminAuth, nil, nil, middleware.NewPanelRateLimiter(rdb, nil))
	post := func(key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ldxp/device/runtime", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	device := func(t *testing.T) (*service.LiandongBrowserDevice, string) {
		t.Helper()
		d, key, err := svc.BrowserCreateDevice(ctx, "isolated-runtime-test", []int64{42})
		require.NoError(t, err)
		return d, key
	}
	statusDevice := func(t *testing.T, id string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/tools/ldxp/browser/status", nil))
		require.Equal(t, http.StatusOK, w.Code)
		var envelope struct {
			Data struct {
				Devices []map[string]any `json:"devices"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		for _, row := range envelope.Data.Devices {
			if row["id"] == id {
				return row
			}
		}
		t.Fatal("device missing from status")
		return nil
	}
	snapshot := func(t *testing.T) string {
		t.Helper()
		var value string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT jsonb_build_object(
            'devices',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM liandong_browser_devices d),
            'config',(SELECT jsonb_agg(to_jsonb(c)) FROM liandong_browser_config c),
            'inventory',(SELECT jsonb_agg(to_jsonb(i) ORDER BY goods_id) FROM liandong_browser_inventory i),
            'batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY batch_id) FROM liandong_restock_batches b),
            'segments',(SELECT jsonb_agg(to_jsonb(s) ORDER BY batch_id,segment_no) FROM liandong_restock_segments s),
            'codes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM redeem_codes r),
            'users',(SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM users u))::text`).Scan(&value))
		return value
	}
	paused := `{"state":"paused","reason":"non_json","browser_verification_required":true,"checked_at":"2026-09-16T00:00:00+08:00","next_check_at":"2026-09-15T16:05:00Z"}`
	running := `{"state":"running","reason":"","browser_verification_required":false}`

	t.Run("report is absent until actually received", func(t *testing.T) {
		d, _ := device(t)
		row := statusDevice(t, d.ID)
		require.Nil(t, row["runtime"])
		require.Nil(t, row["last_seen_at"])
		require.Nil(t, row["authorization_verified_at"])
	})
	t.Run("paused report readback and recovery change only observation", func(t *testing.T) {
		d, key := device(t)
		before := snapshot(t)
		started := time.Now()
		w := post(key, paused)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, before, snapshot(t))
		runtime := statusDevice(t, d.ID)["runtime"].(map[string]any)
		require.Equal(t, "http", runtime["execution_mode"])
		require.Equal(t, "paused", runtime["state"])
		require.Equal(t, "non_json", runtime["reason"])
		require.Equal(t, true, runtime["browser_verification_required"])
		reported, err := time.Parse(time.RFC3339Nano, runtime["reported_at"].(string))
		require.NoError(t, err)
		require.False(t, reported.Before(started.Add(-time.Second)))
		require.False(t, reported.After(time.Now().Add(time.Second)))
		require.Contains(t, runtime, "checked_at")
		require.Contains(t, runtime, "next_check_at")
		checked, err := time.Parse(time.RFC3339Nano, runtime["checked_at"].(string))
		require.NoError(t, err)
		require.True(t, checked.Equal(time.Date(2026, 9, 15, 16, 0, 0, 0, time.UTC)))
		w = post(key, running)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, before, snapshot(t))
		runtime = statusDevice(t, d.ID)["runtime"].(map[string]any)
		require.Equal(t, "running", runtime["state"])
		require.Equal(t, "", runtime["reason"])
		require.Equal(t, false, runtime["browser_verification_required"])
		require.NotContains(t, runtime, "checked_at")
	})
	for _, mode := range []string{"http", "browser"} {
		t.Run("execution mode "+mode+" is observational", func(t *testing.T) {
			d, key := device(t)
			_, err := svc.BrowserHeartbeat(ctx, d.ID, "failed")
			require.NoError(t, err)
			before := snapshot(t)
			body := `{"state":"paused","reason":"verification_required","browser_verification_required":true,"execution_mode":"` + mode + `"}`
			w := post(key, body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var response struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Equal(t, mode, response.Data["execution_mode"])
			require.Equal(t, mode, statusDevice(t, d.ID)["runtime"].(map[string]any)["execution_mode"])
			var stored string
			require.NoError(t, db.QueryRowContext(ctx, "SELECT report->>'execution_mode' FROM liandong_browser_runtime WHERE device_id=$1", d.ID).Scan(&stored))
			require.Equal(t, mode, stored)
			require.Equal(t, http.StatusOK, post(key, `{"state":"running","reason":"","browser_verification_required":false,"execution_mode":"`+mode+`"}`).Code)
			require.Equal(t, before, snapshot(t))
			require.Equal(t, "authorization_failed", statusDevice(t, d.ID)["paused_reason"])
			_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
			require.Error(t, err)
			_, err = svc.BrowserClaim(ctx, d.ID, 42)
			require.Error(t, err)
		})
	}
	t.Run("stored report without mode reads as http without rewriting it", func(t *testing.T) {
		d, _ := device(t)
		_, err := db.ExecContext(ctx, "INSERT INTO liandong_browser_runtime(device_id,report) VALUES($1,$2::jsonb)", d.ID, running)
		require.NoError(t, err)
		var before, after string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT report::text FROM liandong_browser_runtime WHERE device_id=$1", d.ID).Scan(&before))
		require.Equal(t, "http", statusDevice(t, d.ID)["runtime"].(map[string]any)["execution_mode"])
		require.NoError(t, db.QueryRowContext(ctx, "SELECT report::text FROM liandong_browser_runtime WHERE device_id=$1", d.ID).Scan(&after))
		require.Equal(t, before, after)
	})
	t.Run("service rejects unknown execution mode independently of http binding", func(t *testing.T) {
		d, _ := device(t)
		var report service.LiandongBrowserRuntimeReport
		require.NoError(t, json.Unmarshal([]byte(`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":"untrusted"}`), &report))
		_, err := svc.BrowserReportRuntime(ctx, d.ID, report)
		require.Error(t, err)
		require.Nil(t, statusDevice(t, d.ID)["runtime"])
	})
	t.Run("running report cannot supply merchant authorization", func(t *testing.T) {
		d, key := device(t)
		before := snapshot(t)
		require.Equal(t, http.StatusOK, post(key, running).Code)
		require.Equal(t, before, snapshot(t))
		_, err := svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.Error(t, err)
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.Error(t, err)
	})
	t.Run("running report cannot clear failed authorization", func(t *testing.T) {
		d, key := device(t)
		_, err := svc.BrowserHeartbeat(ctx, d.ID, "failed")
		require.NoError(t, err)
		before := snapshot(t)
		require.Equal(t, http.StatusOK, post(key, running).Code)
		require.Equal(t, before, snapshot(t))
		require.Equal(t, "authorization_failed", statusDevice(t, d.ID)["paused_reason"])
		_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.Error(t, err)
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.Error(t, err)
	})
	t.Run("running report cannot refresh expired authorization", func(t *testing.T) {
		d, key := device(t)
		_, err := svc.BrowserHeartbeat(ctx, d.ID, "verified")
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, "UPDATE liandong_browser_devices SET last_seen_at=NOW()-INTERVAL '5 minutes',authorization_verified_at=NOW()-INTERVAL '5 minutes' WHERE id=$1", d.ID)
		require.NoError(t, err)
		before := snapshot(t)
		require.Equal(t, http.StatusOK, post(key, running).Code)
		require.Equal(t, before, snapshot(t))
		_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.Error(t, err)
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.Error(t, err)
	})
	t.Run("verification required can be reported while restock is disabled", func(t *testing.T) {
		d, key := device(t)
		_, err := db.ExecContext(ctx, "UPDATE liandong_browser_config SET enabled=FALSE WHERE id=TRUE")
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := db.ExecContext(ctx, "UPDATE liandong_browser_config SET enabled=TRUE WHERE id=TRUE")
			require.NoError(t, err)
		})
		before := snapshot(t)
		require.Equal(t, http.StatusOK, post(key, `{"state":"paused","reason":"verification_required","browser_verification_required":true}`).Code)
		require.Equal(t, before, snapshot(t))
		require.Equal(t, "verification_required", statusDevice(t, d.ID)["runtime"].(map[string]any)["reason"])
	})
	t.Run("wrong or revoked device is rejected", func(t *testing.T) {
		d, key := device(t)
		require.NoError(t, svc.BrowserRevokeDevice(ctx, d.ID))
		before := snapshot(t)
		for _, invalid := range []string{"", "admin-not-a-device", "ldxpd_" + strings.Repeat("a", 64), key} {
			require.Equal(t, http.StatusUnauthorized, post(invalid, paused).Code)
		}
		require.Equal(t, before, snapshot(t))
		require.Nil(t, statusDevice(t, d.ID)["runtime"])
	})
	for _, malformed := range []string{
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":""}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":null}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":"HTTP"}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":"chrome"}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":"https://example.invalid"}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":false}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":1}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":{}}`,
		`{"state":"running","reason":"","browser_verification_required":false,"execution_mode":[]}`,
		`{"state":"running","reason":"non_json","browser_verification_required":false,"execution_mode":"browser"}`,
		`{"state":"paused","reason":"network_error","browser_verification_required":true,"execution_mode":"browser"}`,
		`{"state":"running","reason":""}`,
		`{"state":"running","browser_verification_required":false}`,
		`{"state":"running","reason":null,"browser_verification_required":false}`,
		`{"state":"running","reason":"","browser_verification_required":null}`,
		`{"reason":"non_json","browser_verification_required":true}`,
		`null`,
		`{"state":"unknown","reason":"non_json","browser_verification_required":true}`,
		`{"state":"paused","reason":"","browser_verification_required":false}`,
		`{"state":"paused","reason":"synthetic-private-error","browser_verification_required":false}`,
		`{"state":"running","reason":"non_json","browser_verification_required":false}`,
		`{"state":"running","reason":"","browser_verification_required":true}`,
		`{"state":"paused","reason":"network_error","browser_verification_required":true}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":"true"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"checked_at":"yesterday"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"next_check_at":"2026-09-16"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"message":"synthetic-private-error"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"url":"https://example.invalid/private"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"token":"synthetic-private-token"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"device_id":"another-device"}`,
		`{"state":"paused","reason":"non_json","browser_verification_required":true,"reported_at":"2026-09-16T00:00:00Z"}`,
	} {
		t.Run("reject "+malformed, func(t *testing.T) {
			d, key := device(t)
			before := snapshot(t)
			w := post(key, malformed)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.NotContains(t, w.Body.String(), "synthetic-private")
			require.Equal(t, before, snapshot(t))
			require.Nil(t, statusDevice(t, d.ID)["runtime"])
		})
	}
	t.Run("uncertain batch remains latched after runtime recovery", func(t *testing.T) {
		d, key := device(t)
		_, err := svc.BrowserHeartbeat(ctx, d.ID, "verified")
		require.NoError(t, err)
		_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.NoError(t, err)
		batch, err := svc.BrowserClaim(ctx, d.ID, 42)
		require.NoError(t, err)
		_, err = svc.BrowserBatchAction(ctx, d.ID, batch.BatchID, "start")
		require.NoError(t, err)
		before := snapshot(t)
		require.Equal(t, http.StatusOK, post(key, paused).Code)
		require.Equal(t, http.StatusOK, post(key, running).Code)
		require.Equal(t, before, snapshot(t))
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.ErrorIs(t, err, service.ErrLiandongNeedsReconciliation)
	})
	t.Run("runtime uses existing device rate budget", func(t *testing.T) {
		_, key := device(t)
		for i := 0; i < 120; i++ {
			require.Equal(t, http.StatusOK, post(key, running).Code)
		}
		require.Equal(t, http.StatusTooManyRequests, post(key, running).Code)
	})
}
