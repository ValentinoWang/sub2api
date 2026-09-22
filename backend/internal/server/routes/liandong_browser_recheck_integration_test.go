//go:build integration

package routes_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

func TestLiandongBrowserRecheckIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pg, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("recheck_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), testcontainers.WithWaitStrategyAndDeadline(2*time.Minute, wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2*time.Minute), wait.ForListeningPort("5432/tcp").WithStartupTimeout(2*time.Minute)))
	if pg != nil {
		t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
	}
	require.NoError(t, err)
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	svc := service.NewLiandongRestockService(nil, nil, nil, nil, db)
	_, err = svc.BrowserSaveConfig(ctx, service.LiandongBrowserConfig{Enabled: true, Products: []service.LiandongBrowserProduct{{GoodsID: 42, CNYAmount: 5, USDCredit: 5, ExternalURL: "https://wzyp.cn/item/recheck-test", TargetStock: 2, BatchSize: 2, Enabled: true}}})
	require.NoError(t, err)
	var adminID int64
	require.NoError(t, db.QueryRowContext(ctx, "INSERT INTO users(email,password_hash,role,balance) VALUES('recheck-admin@example.invalid','isolated','admin',7) RETURNING id").Scan(&adminID))
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var audited atomic.Int64
	adminAuth := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer isolated-admin" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: adminID})
		c.Set(string(middleware.ContextKeyUserRole), "admin")
		c.Next()
	})
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { audited.Add(1); c.Next() })
	routes.RegisterLiandongToolRoutes(router.Group("/api/v1"), adminhandler.NewLiandongToolkitHandler(svc), adminAuth, audit, nil, middleware.NewPanelRateLimiter(rdb, nil))
	request := func(method, path, key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	decode := func(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var body struct {
			Data struct {
				Recheck map[string]any `json:"recheck"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body.Data.Recheck
	}
	newDevice := func(t *testing.T) (*service.LiandongBrowserDevice, string) {
		t.Helper()
		redisServer.FastForward(2 * time.Minute)
		d, key, err := svc.BrowserCreateDevice(ctx, "recheck-test", []int64{42})
		require.NoError(t, err)
		return d, key
	}
	queuePath := func(id string) string { return "/admin/tools/ldxp/browser/devices/" + id + "/recheck" }
	claimPath := "/ldxp/device/recheck/claim"
	resultPath := func(id string) string { return "/ldxp/device/recheck/" + id + "/result" }
	snapshot := func(t *testing.T) string {
		t.Helper()
		var raw string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT jsonb_build_object(
          'devices',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM liandong_browser_devices d),
          'config',(SELECT jsonb_agg(to_jsonb(c)) FROM liandong_browser_config c),
          'inventory',(SELECT jsonb_agg(to_jsonb(i) ORDER BY goods_id) FROM liandong_browser_inventory i),
          'runtime',(SELECT jsonb_agg(to_jsonb(r) ORDER BY device_id) FROM liandong_browser_runtime r),
          'batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY batch_id) FROM liandong_restock_batches b),
          'segments',(SELECT jsonb_agg(to_jsonb(s) ORDER BY batch_id,segment_no) FROM liandong_restock_segments s),
          'codes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM redeem_codes r),
          'users',(SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM users u))::text`).Scan(&raw))
		return raw
	}
	statusRecheck := func(t *testing.T, id string) map[string]any {
		t.Helper()
		w := request(http.MethodGet, "/admin/tools/ldxp/browser/status", "isolated-admin", "")
		require.Equal(t, 200, w.Code)
		var body struct {
			Data struct {
				Devices          []map[string]any `json:"devices"`
				RecheckSupported bool             `json:"recheck_supported"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Data.RecheckSupported)
		for _, d := range body.Data.Devices {
			if d["id"] == id {
				if d["recheck"] == nil {
					return nil
				}
				return d["recheck"].(map[string]any)
			}
		}
		t.Fatal("device missing")
		return nil
	}
	passed := `{"state":"passed","reason":"","resumed":true}`
	failed := `{"state":"failed","reason":"verification_required","resumed":false}`

	t.Run("queue claim restart and result replay preserve authority", func(t *testing.T) {
		d, key := newDevice(t)
		_, err := svc.BrowserHeartbeat(ctx, d.ID, "failed")
		require.NoError(t, err)
		before := snapshot(t)
		require.Nil(t, statusRecheck(t, d.ID))
		first := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", ""))
		require.Equal(t, "queued", first["state"])
		require.Equal(t, float64(adminID), first["requested_by"])
		require.False(t, first["resumed"].(bool))
		require.Equal(t, first, decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}")))
		claimed := decode(t, request(http.MethodPost, claimPath, key, "{}"))
		require.Equal(t, first["id"], claimed["id"])
		require.Equal(t, "checking", claimed["state"])
		require.Equal(t, claimed, decode(t, request(http.MethodPost, claimPath, key, "")))
		require.Equal(t, claimed, decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}")))
		finished := decode(t, request(http.MethodPost, resultPath(first["id"].(string)), key, passed))
		require.Equal(t, "passed", finished["state"])
		require.Equal(t, true, finished["resumed"])
		require.Contains(t, finished, "finished_at")
		require.Equal(t, finished, decode(t, request(http.MethodPost, resultPath(first["id"].(string)), key, passed)))
		require.Equal(t, finished, statusRecheck(t, d.ID))
		require.Equal(t, before, snapshot(t))
		_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.Error(t, err)
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.Error(t, err)
		require.Equal(t, 409, request(http.MethodPost, resultPath(first["id"].(string)), key, failed).Code)
		require.Nil(t, decode(t, request(http.MethodPost, claimPath, key, "{}")))
		next := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		require.NotEqual(t, first["id"], next["id"])
		require.Greater(t, audited.Load(), int64(0))
	})
	t.Run("failed result replays without implicit recovery", func(t *testing.T) {
		d, key := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		decode(t, request(http.MethodPost, claimPath, key, "{}"))
		before := snapshot(t)
		got := decode(t, request(http.MethodPost, resultPath(q["id"].(string)), key, failed))
		require.Equal(t, "failed", got["state"])
		require.Equal(t, got, decode(t, request(http.MethodPost, resultPath(q["id"].(string)), key, failed)))
		require.Equal(t, before, snapshot(t))
		require.Equal(t, 409, request(http.MethodPost, resultPath(q["id"].(string)), key, passed).Code)
	})
	t.Run("result cannot skip queued claim", func(t *testing.T) {
		d, key := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		require.Equal(t, 409, request(http.MethodPost, resultPath(q["id"].(string)), key, passed).Code)
		require.Equal(t, "queued", statusRecheck(t, d.ID)["state"])
	})
	t.Run("concurrent clicks share one active request", func(t *testing.T) {
		d, _ := newDevice(t)
		var wg sync.WaitGroup
		responses := make(chan *httptest.ResponseRecorder, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				responses <- request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}")
			}()
		}
		wg.Wait()
		close(responses)
		ids := map[string]bool{}
		for w := range responses {
			q := decode(t, w)
			ids[q["id"].(string)] = true
		}
		require.Len(t, ids, 1)
		var count int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM liandong_browser_rechecks WHERE device_id=$1", d.ID).Scan(&count))
		require.Equal(t, 1, count)
	})
	t.Run("queued expiry is failed and cannot be claimed", func(t *testing.T) {
		d, key := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		_, err := db.ExecContext(ctx, "UPDATE liandong_browser_rechecks SET requested_at=NOW()-INTERVAL '11 minutes' WHERE id=$1", q["id"])
		require.NoError(t, err)
		require.Nil(t, decode(t, request(http.MethodPost, claimPath, key, "{}")))
		expired := statusRecheck(t, d.ID)
		require.Equal(t, "failed", expired["state"])
		require.Equal(t, "state_invalid", expired["reason"])
		require.Equal(t, false, expired["resumed"])
		fresh := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		require.NotEqual(t, q["id"], fresh["id"])
	})
	t.Run("checking survives worker restart beyond queue expiry", func(t *testing.T) {
		d, key := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		claimed := decode(t, request(http.MethodPost, claimPath, key, "{}"))
		_, err := db.ExecContext(ctx, "UPDATE liandong_browser_rechecks SET requested_at=NOW()-INTERVAL '11 minutes' WHERE id=$1", q["id"])
		require.NoError(t, err)
		replayed := decode(t, request(http.MethodPost, claimPath, key, "{}"))
		require.Equal(t, q["id"], replayed["id"])
		require.Equal(t, claimed["started_at"], replayed["started_at"])
		require.Equal(t, "checking", replayed["state"])
	})
	t.Run("device cannot report another device request", func(t *testing.T) {
		d, key := newDevice(t)
		_, otherKey := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		decode(t, request(http.MethodPost, claimPath, key, "{}"))
		require.Nil(t, decode(t, request(http.MethodPost, claimPath, otherKey, "{}")))
		require.Equal(t, 404, request(http.MethodPost, resultPath(q["id"].(string)), otherKey, passed).Code)
		require.Equal(t, "checking", statusRecheck(t, d.ID)["state"])
	})
	t.Run("authentication and revocation remain enforced", func(t *testing.T) {
		d, key := newDevice(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		decode(t, request(http.MethodPost, claimPath, key, "{}"))
		require.NoError(t, svc.BrowserRevokeDevice(ctx, d.ID))
		require.Equal(t, 401, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}").Code)
		for _, invalid := range []string{"", key, "ldxpd_" + strings.Repeat("a", 64)} {
			require.Equal(t, 401, request(http.MethodPost, claimPath, invalid, "{}").Code)
			require.Equal(t, 401, request(http.MethodPost, resultPath(q["id"].(string)), invalid, passed).Code)
		}
		require.Equal(t, 401, request(http.MethodPost, queuePath("unknown"), "isolated-admin", "{}").Code)
		require.Equal(t, 401, request(http.MethodPost, queuePath(d.ID), "", "{}").Code)
	})
	for _, body := range []string{`{"requested_by":1}`, `{"device_id":"foreign"}`, `{"token":"private-synthetic"}`} {
		t.Run("reject queue or claim fields "+body, func(t *testing.T) {
			d, key := newDevice(t)
			require.Equal(t, 400, request(http.MethodPost, queuePath(d.ID), "isolated-admin", body).Code)
			require.Equal(t, 400, request(http.MethodPost, claimPath, key, body).Code)
			require.Nil(t, statusRecheck(t, d.ID))
		})
	}
	for _, body := range []string{`{"state":"passed","reason":"","resumed":false,"device_id":"foreign"}`, `{"state":"passed","reason":"","resumed":false,"requested_by":1}`, `{"state":"passed","reason":"network_error","resumed":false}`, `{"state":"failed","reason":"network_error","resumed":true}`, `{"state":"failed","reason":"private-synthetic","resumed":false}`, `{"state":"failed","reason":"","resumed":false}`, `{"state":"checking","reason":"","resumed":false}`, `{"state":"passed","resumed":false}`, `{"state":"passed","reason":""}`} {
		t.Run("reject result "+body, func(t *testing.T) {
			d, key := newDevice(t)
			q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
			decode(t, request(http.MethodPost, claimPath, key, "{}"))
			w := request(http.MethodPost, resultPath(q["id"].(string)), key, body)
			require.Equal(t, 400, w.Code)
			require.NotContains(t, w.Body.String(), "private-synthetic")
			require.Equal(t, "checking", statusRecheck(t, d.ID)["state"])
		})
	}
	t.Run("existing uncertain batch is untouched", func(t *testing.T) {
		d, key := newDevice(t)
		_, err := svc.BrowserHeartbeat(ctx, d.ID, "verified")
		require.NoError(t, err)
		_, err = svc.BrowserInventory(ctx, d.ID, service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true})
		require.NoError(t, err)
		b, err := svc.BrowserClaim(ctx, d.ID, 42)
		require.NoError(t, err)
		_, err = svc.BrowserBatchAction(ctx, d.ID, b.BatchID, "start")
		require.NoError(t, err)
		before := snapshot(t)
		q := decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		decode(t, request(http.MethodPost, claimPath, key, "{}"))
		decode(t, request(http.MethodPost, resultPath(q["id"].(string)), key, passed))
		require.Equal(t, before, snapshot(t))
		_, err = svc.BrowserClaim(ctx, d.ID, 42)
		require.ErrorIs(t, err, service.ErrLiandongNeedsReconciliation)
	})
	t.Run("one pending request unique index remains a database guard", func(t *testing.T) {
		d, _ := newDevice(t)
		decode(t, request(http.MethodPost, queuePath(d.ID), "isolated-admin", "{}"))
		_, err := db.ExecContext(ctx, `INSERT INTO liandong_browser_rechecks(id,device_id,requested_by,state) VALUES($1,$2,$3,'queued')`, fmt.Sprintf("%032d", 1), d.ID, adminID)
		require.Error(t, err)
	})
}
