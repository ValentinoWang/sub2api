//go:build integration

package service_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLiandongBrowserPersistentSafetyIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("browser_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	s := service.NewLiandongRestockService(nil, nil, nil, nil, db)
	_, err = s.BrowserPublicProducts(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Chrome 商品目录尚未配置")
	_, err = s.BrowserSaveConfig(ctx, service.LiandongBrowserConfig{Products: []service.LiandongBrowserProduct{}})
	require.NoError(t, err)
	empty, err := s.BrowserPublicProducts(ctx)
	require.NoError(t, err)
	require.Empty(t, empty)
	config := service.LiandongBrowserConfig{Enabled: true, Products: []service.LiandongBrowserProduct{{GoodsID: 42, CNYAmount: 5, USDCredit: 5, ExternalURL: "https://wzyp.cn/item/test", TargetStock: 4, BatchSize: 2, Enabled: true}, {GoodsID: 43, CNYAmount: 10, USDCredit: 10, ExternalURL: "https://wzyp.cn/item/test10", TargetStock: 4, BatchSize: 2, Enabled: true}}}
	_, err = s.BrowserSaveConfig(ctx, config)
	require.NoError(t, err)
	d, key, err := s.BrowserCreateDevice(ctx, "test-device", []int64{42})
	require.NoError(t, err)
	var stored string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT key_sha256 FROM liandong_browser_devices WHERE id=$1`, d.ID).Scan(&stored))
	require.NotEqual(t, key, stored)
	require.Len(t, stored, 64)
	id, err := s.BrowserAuthenticate(ctx, key)
	require.NoError(t, err)
	require.Equal(t, d.ID, id)
	cfg, _, err := s.BrowserConfig(ctx, id)
	require.NoError(t, err)
	require.Len(t, cfg.Products, 1)
	_, err = s.BrowserHeartbeat(ctx, id, "verified")
	require.NoError(t, err)
	_, err = s.BrowserInventory(ctx, id, service.LiandongBrowserInventoryReport{GoodsID: 43, Complete: true})
	require.Error(t, err)
	report := service.LiandongBrowserInventoryReport{GoodsID: 42, Complete: true}
	inv, err := s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.True(t, inv.IdentityVerified)
	b, err := s.BrowserClaim(ctx, id, 42)
	require.NoError(t, err)
	require.Len(t, b.Codes, 2)
	_, err = s.BrowserHeartbeat(ctx, id, "failed")
	require.NoError(t, err)
	_, err = s.BrowserHeartbeat(ctx, id, "verified")
	require.NoError(t, err)
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.Equal(t, "claimed", inv.PendingBatch.Status)
	_, err = s.BrowserResume(ctx, id)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE liandong_browser_devices SET last_seen_at=NOW()-INTERVAL '3 minutes' WHERE id=$1`, id)
	require.NoError(t, err)
	disconnected, err := s.BrowserHeartbeat(ctx, id, "verified")
	require.NoError(t, err)
	require.Equal(t, "disconnected", disconnected.PausedReason)
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.Equal(t, "claimed", inv.PendingBatch.Status)
	_, err = s.BrowserResume(ctx, id)
	require.NoError(t, err)
	// Independent service instances must reuse the already committed batch.
	var wg sync.WaitGroup
	results := make(chan string, 6)
	failures := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := service.NewLiandongRestockService(nil, nil, nil, nil, db)
			for n := 0; n < 30; n++ {
				batch, e := other.BrowserClaim(ctx, id, 42)
				if errors.Is(e, service.ErrLiandongRunBusy) {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				if e != nil {
					failures <- e
					return
				}
				results <- batch.BatchID
				return
			}
			failures <- errors.New("lease retry exhausted")
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	for bid := range results {
		require.Equal(t, b.BatchID, bid)
	}
	var batchCount, codeCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM liandong_restock_batches`).Scan(&batchCount))
	require.Equal(t, 1, batchCount)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM redeem_codes`).Scan(&codeCount))
	require.Equal(t, 2, codeCount)
	started, err := s.BrowserBatchAction(ctx, id, b.BatchID, "start")
	require.NoError(t, err)
	require.Equal(t, "uncertain", started.Status)
	_, err = s.BrowserClaim(ctx, id, 42)
	require.ErrorIs(t, err, service.ErrLiandongNeedsReconciliation)
	_, err = s.BrowserBatchAction(ctx, id, b.BatchID, "imported")
	require.NoError(t, err)
	// Equal counts with unrelated known codes never verify the batch's identities.
	manual := []string{"MANUAL-ONE", "MANUAL-TWO"}
	hashes := []string{}
	for _, code := range manual {
		_, err = db.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,status) VALUES($1,'balance',5,'unused')`, code)
		require.NoError(t, err)
		h := sha256.Sum256([]byte(code))
		hashes = append(hashes, hex.EncodeToString(h[:]))
	}
	report.Total = 2
	report.Hashes = hashes
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.True(t, inv.IdentityVerified)
	require.True(t, inv.Blocked)
	require.False(t, inv.BatchResolved)
	report.Hashes = b.CodeHashes
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.True(t, inv.BatchResolved)
	report.BatchID = b.BatchID
	report.Hashes = b.CodeHashes[:1]
	report.Total = 1
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.True(t, inv.BatchResolved)
	report.BatchID = ""
	report.Hashes = b.CodeHashes
	report.Total = 2
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	public, err := s.BrowserPublicProducts(ctx)
	require.NoError(t, err)
	require.Len(t, public, 1)
	require.Equal(t, "5元额度", public[0].Title)
	config.Enabled = false
	_, err = s.BrowserSaveConfig(ctx, config)
	require.NoError(t, err)
	public, err = s.BrowserPublicProducts(ctx)
	require.NoError(t, err)
	require.Len(t, public, 1)
	_, err = db.ExecContext(ctx, `UPDATE liandong_browser_inventory SET observed_at=NOW()-INTERVAL '1 day'`)
	require.NoError(t, err)
	_, err = s.BrowserHeartbeat(ctx, id, "failed")
	require.NoError(t, err)
	public, err = s.BrowserPublicProducts(ctx)
	require.NoError(t, err)
	require.Len(t, public, 1)
	config.Enabled = true
	_, err = s.BrowserSaveConfig(ctx, config)
	require.NoError(t, err)
	_, err = s.BrowserHeartbeat(ctx, id, "verified")
	require.NoError(t, err)
	_, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	_, err = s.BrowserResume(ctx, id)
	require.NoError(t, err)
	b2, err := s.BrowserClaim(ctx, id, 42)
	require.NoError(t, err)
	require.NotEqual(t, b.BatchID, b2.BatchID)
	config.Products[0].Enabled = false
	_, err = s.BrowserSaveConfig(ctx, config)
	require.NoError(t, err)
	_, err = s.BrowserBatchAction(ctx, id, b2.BatchID, "start")
	require.Error(t, err)
	config.Products[0].Enabled = true
	_, err = s.BrowserSaveConfig(ctx, config)
	require.NoError(t, err)
	_, err = s.BrowserBatchAction(ctx, id, b2.BatchID, "start")
	require.Error(t, err)
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	_, err = s.BrowserBatchAction(ctx, id, b2.BatchID, "start")
	require.NoError(t, err)
	_, err = s.BrowserBatchAction(ctx, id, b2.BatchID, "authorization_failed")
	require.NoError(t, err)
	hb, err := s.BrowserHeartbeat(ctx, id, "verified")
	require.NoError(t, err)
	require.Equal(t, "authorization_failed", hb.PausedReason)
	_, err = s.BrowserResume(ctx, id)
	require.Error(t, err)
	report.Hashes = append(b.CodeHashes, b2.CodeHashes...)
	report.Total = 4
	inv, err = s.BrowserInventory(ctx, id, report)
	require.NoError(t, err)
	require.True(t, inv.BatchResolved)
	_, err = s.BrowserResume(ctx, id)
	require.NoError(t, err)
	// Manual stock remains outside the automatic batch ledger.
	var manualInBatch int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM liandong_restock_batch_codes bc JOIN redeem_codes r ON r.id=bc.redeem_code_id WHERE r.code LIKE 'MANUAL%'`).Scan(&manualInBatch))
	require.Zero(t, manualInBatch)
	// A ledger insertion failure rolls back the batch and every generated code.
	d10, _, err := s.BrowserCreateDevice(ctx, "rollback-device", []int64{43})
	require.NoError(t, err)
	_, err = s.BrowserHeartbeat(ctx, d10.ID, "verified")
	require.NoError(t, err)
	_, err = s.BrowserInventory(ctx, d10.ID, service.LiandongBrowserInventoryReport{GoodsID: 43, Complete: true})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE FUNCTION reject_browser_test_code() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.value=10 THEN RAISE EXCEPTION 'injected code persistence failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_browser_test_code BEFORE INSERT ON redeem_codes FOR EACH ROW EXECUTE FUNCTION reject_browser_test_code()`)
	require.NoError(t, err)
	_, err = s.BrowserClaim(ctx, d10.ID, 43)
	require.Error(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM liandong_restock_batches WHERE goods_id=43`).Scan(&batchCount))
	require.Zero(t, batchCount)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM redeem_codes WHERE value=10`).Scan(&codeCount))
	require.Zero(t, codeCount)
	require.Error(t, s.RunOnce(ctx, true))
	require.NoError(t, s.BrowserRevokeDevice(ctx, id))
	_, err = s.BrowserAuthenticate(ctx, key)
	require.Error(t, err)
}
