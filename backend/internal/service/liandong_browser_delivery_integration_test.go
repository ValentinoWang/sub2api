//go:build integration

package service_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLiandongBrowserDeliveryProofIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("delivery_proof_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	s := service.NewLiandongRestockService(nil, nil, nil, nil, db)
	config := service.LiandongBrowserConfig{Enabled: true}
	var sequence int
	seed := func(t *testing.T) (string, *service.LiandongBrowserBatch) {
		t.Helper()
		sequence++
		config.Enabled = true
		goodsID := int64(100 + sequence)
		config.Products = append(config.Products, service.LiandongBrowserProduct{GoodsID: goodsID, CNYAmount: int(goodsID), USDCredit: float64(goodsID), ExternalURL: fmt.Sprintf("https://wzyp.cn/item/proof%d", sequence), TargetStock: 2, BatchSize: 2, Enabled: true})
		_, err := s.BrowserSaveConfig(ctx, config)
		require.NoError(t, err)
		device, _, err := s.BrowserCreateDevice(ctx, "proof-test", []int64{goodsID})
		require.NoError(t, err)
		_, err = s.BrowserHeartbeat(ctx, device.ID, "verified")
		require.NoError(t, err)
		_, err = s.BrowserInventory(ctx, device.ID, service.LiandongBrowserInventoryReport{GoodsID: goodsID, Complete: true})
		require.NoError(t, err)
		batch, err := s.BrowserClaim(ctx, device.ID, goodsID)
		require.NoError(t, err)
		require.Len(t, batch.Codes, 2)
		_, err = s.BrowserBatchAction(ctx, device.ID, batch.BatchID, "start")
		require.NoError(t, err)
		return device.ID, batch
	}
	decode := func(t *testing.T, b *service.LiandongBrowserBatch, unsold []string, proofs []map[string]any) service.LiandongBrowserInventoryReport {
		t.Helper()
		body := map[string]any{"batch_id": b.BatchID, "goods_id": b.GoodsID, "complete": true, "total": len(unsold), "hashes": unsold, "sold_complete": true, "sold_proofs": proofs}
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		var report service.LiandongBrowserInventoryReport
		require.NoError(t, json.Unmarshal(raw, &report))
		return report
	}
	proofs := func(b *service.LiandongBrowserBatch) []map[string]any {
		return []map[string]any{{"code_hash": b.CodeHashes[0], "card_id": 101}, {"code_hash": b.CodeHashes[1], "card_id": 102}}
	}
	assertPending := func(t *testing.T, batchID string) {
		t.Helper()
		var status string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT status FROM liandong_restock_batches WHERE batch_id=$1", batchID).Scan(&status))
		require.Equal(t, "needs_reconciliation", status)
	}
	assertResolved := func(t *testing.T, id string, b *service.LiandongBrowserBatch, report service.LiandongBrowserInventoryReport, stock int) {
		t.Helper()
		result, err := s.BrowserInventory(ctx, id, report)
		require.NoError(t, err)
		require.True(t, result.BatchResolved)
		require.False(t, result.Blocked)
		require.Equal(t, stock, result.MatchedStock)
		require.False(t, result.RetryEligible)
		raw, err := json.Marshal(result)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		require.Equal(t, float64(1), payload["delivery_proof_version"])
		var stored string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT row_to_json(p)::text FROM liandong_browser_delivery_proofs p WHERE batch_id=$1", b.BatchID).Scan(&stored))
		var audit map[string]any
		require.NoError(t, json.Unmarshal([]byte(stored), &audit))
		require.Equal(t, id, audit["device_id"])
		require.Equal(t, float64(b.GoodsID), audit["goods_id"])
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(b.Codes, "\n")))), audit["code_sha256"])
		require.Len(t, audit["code_hashes"], b.CodeCount)
		require.Len(t, audit["unsold_batch_hashes"], stock)
		for _, code := range b.Codes {
			require.NotContains(t, stored, code)
		}
		_, err = s.BrowserInventory(ctx, id, report)
		require.NoError(t, err)
		var repeated string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT row_to_json(p)::text FROM liandong_browser_delivery_proofs p WHERE batch_id=$1", b.BatchID).Scan(&repeated))
		require.Equal(t, stored, repeated, "replay must preserve the first durable proof")
		var count int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM liandong_restock_batches WHERE goods_id=$1", b.GoodsID).Scan(&count))
		require.Equal(t, 1, count)
	}

	t.Run("mixed unsold and redeemed sold proof", func(t *testing.T) {
		id, b := seed(t)
		var userID int64
		require.NoError(t, db.QueryRowContext(ctx, "INSERT INTO users(email,password_hash,balance) VALUES('delivery@example.invalid','isolated',7) RETURNING id").Scan(&userID))
		_, err := db.ExecContext(ctx, "UPDATE redeem_codes SET status='used',used_by=$2,used_at=NOW() WHERE code=$1", b.Codes[1], userID)
		require.NoError(t, err)
		report := decode(t, b, b.CodeHashes[:1], proofs(b)[1:])
		assertResolved(t, id, b, report, 1)
		var balance float64
		require.NoError(t, db.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", userID).Scan(&balance))
		require.Equal(t, float64(7), balance)
		var stock int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT jsonb_array_length(code_hashes) FROM liandong_browser_inventory WHERE goods_id=$1", b.GoodsID).Scan(&stock))
		require.Equal(t, 1, stock)
	})
	t.Run("all sold but not yet redeemed", func(t *testing.T) {
		id, b := seed(t)
		assertResolved(t, id, b, decode(t, b, nil, proofs(b)), 0)
	})
	t.Run("unsold only client retains stock semantics", func(t *testing.T) {
		id, b := seed(t)
		assertResolved(t, id, b, service.LiandongBrowserInventoryReport{BatchID: b.BatchID, GoodsID: b.GoodsID, Complete: true, Total: 2, Hashes: b.CodeHashes}, 2)
	})
	t.Run("missing code stays pending", func(t *testing.T) {
		id, b := seed(t)
		result, err := s.BrowserInventory(ctx, id, decode(t, b, nil, proofs(b)[:1]))
		require.NoError(t, err)
		require.True(t, result.Blocked)
		require.False(t, result.BatchResolved)
		require.True(t, result.RetryEligible, "the missing original unused code remains eligible, without clearing the batch latch")
		assertPending(t, b.BatchID)
	})
	t.Run("missing redeemed code is not retry eligible", func(t *testing.T) {
		id, b := seed(t)
		_, err := db.ExecContext(ctx, "UPDATE redeem_codes SET status='used',used_by=(SELECT id FROM users WHERE email='delivery@example.invalid'),used_at=NOW() WHERE code=$1", b.Codes[1])
		require.NoError(t, err)
		result, err := s.BrowserInventory(ctx, id, decode(t, b, b.CodeHashes[:1], nil))
		require.NoError(t, err)
		require.True(t, result.Blocked)
		require.False(t, result.BatchResolved)
		require.False(t, result.RetryEligible)
		assertPending(t, b.BatchID)
	})
	t.Run("foreign unsold code prevents retry", func(t *testing.T) {
		id, b := seed(t)
		result, err := s.BrowserInventory(ctx, id, decode(t, b, []string{strings.Repeat("f", 64)}, nil))
		require.NoError(t, err)
		require.False(t, result.IdentityVerified)
		require.False(t, result.RetryEligible)
		assertPending(t, b.BatchID)
	})
	for _, pause := range []string{"disabled", "authorization_failed", "disconnected"} {
		t.Run("retry gate "+pause, func(t *testing.T) {
			id, b := seed(t)
			if pause == "disabled" {
				config.Enabled = false
				_, err = s.BrowserSaveConfig(ctx, config)
			} else {
				_, err = db.ExecContext(ctx, "UPDATE liandong_browser_devices SET paused_reason=$2 WHERE id=$1", id, pause)
			}
			require.NoError(t, err)
			result, err := s.BrowserInventory(ctx, id, decode(t, b, nil, nil))
			require.NoError(t, err)
			require.Equal(t, pause == "disconnected", result.RetryEligible)
			require.True(t, result.Blocked)
			assertPending(t, b.BatchID)
			result, err = s.BrowserInventory(ctx, id, decode(t, b, nil, proofs(b)))
			require.NoError(t, err)
			require.True(t, result.BatchResolved, "pausing writes does not prevent complete delivery reconciliation")
			require.False(t, result.RetryEligible)
		})
	}
	for _, malformed := range []string{"incomplete", "missing batch", "duplicate hash", "duplicate card", "invalid card", "overlap", "unknown hash"} {
		t.Run(malformed, func(t *testing.T) {
			id, b := seed(t)
			p := proofs(b)
			unsold := []string(nil)
			switch malformed {
			case "duplicate hash":
				p[1]["code_hash"] = p[0]["code_hash"]
			case "duplicate card":
				p[1]["card_id"] = p[0]["card_id"]
			case "invalid card":
				p[0]["card_id"] = 0
			case "overlap":
				unsold = b.CodeHashes[:1]
			case "unknown hash":
				p[0]["code_hash"] = strings.Repeat("f", 64)
			}
			report := decode(t, b, unsold, p)
			if malformed == "missing batch" {
				report.BatchID = ""
			}
			if malformed == "incomplete" {
				raw, err := json.Marshal(report)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(raw, &body))
				body["sold_complete"] = false
				raw, err = json.Marshal(body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(raw, &report))
			}
			_, err := s.BrowserInventory(ctx, id, report)
			require.Error(t, err)
			assertPending(t, b.BatchID)
		})
	}
	t.Run("other device cannot complete with exact hashes", func(t *testing.T) {
		_, b := seed(t)
		other, _, err := s.BrowserCreateDevice(ctx, "other", []int64{b.GoodsID})
		require.NoError(t, err)
		_, err = s.BrowserHeartbeat(ctx, other.ID, "verified")
		require.NoError(t, err)
		_, err = s.BrowserInventory(ctx, other.ID, decode(t, b, b.CodeHashes, nil))
		require.Error(t, err)
		assertPending(t, b.BatchID)
		_, err = s.BrowserInventory(ctx, other.ID, service.LiandongBrowserInventoryReport{GoodsID: b.GoodsID, Complete: true, Total: 2, Hashes: b.CodeHashes})
		require.Error(t, err, "omitting batch_id cannot bypass ownership")
		assertPending(t, b.BatchID)
	})
	for _, conflict := range []string{"disabled", "expired", "wrong value", "changed code", "refunded"} {
		t.Run(conflict, func(t *testing.T) {
			id, b := seed(t)
			statement := map[string]string{"disabled": "UPDATE redeem_codes SET status='disabled' WHERE code=$1", "expired": "UPDATE redeem_codes SET expires_at=NOW()-INTERVAL '1 second' WHERE code=$1", "wrong value": "UPDATE redeem_codes SET value=value+1 WHERE code=$1", "changed code": "UPDATE redeem_codes SET code=md5(code) WHERE code=$1"}[conflict]
			if conflict == "refunded" {
				_, err = s.PrepareUnusedCodeRefund(ctx, service.LiandongRefundPrepareRequest{ExternalOrderNo: "proof-refund", BatchID: b.BatchID, Code: b.Codes[0]})
			} else {
				_, err = db.ExecContext(ctx, statement, b.Codes[0])
			}
			require.NoError(t, err)
			result, err := s.BrowserInventory(ctx, id, decode(t, b, nil, proofs(b)))
			require.NoError(t, err)
			require.True(t, result.Blocked)
			require.False(t, result.BatchResolved)
			require.False(t, result.RetryEligible)
			assertPending(t, b.BatchID)
		})
	}
	t.Run("old batch cannot complete a newer batch", func(t *testing.T) {
		id, old := seed(t)
		_, err := s.BrowserInventory(ctx, id, decode(t, old, old.CodeHashes, nil))
		require.NoError(t, err)
		_, err = s.BrowserInventory(ctx, id, service.LiandongBrowserInventoryReport{GoodsID: old.GoodsID, Complete: true})
		require.NoError(t, err)
		current, err := s.BrowserClaim(ctx, id, old.GoodsID)
		require.NoError(t, err)
		_, err = s.BrowserBatchAction(ctx, id, current.BatchID, "start")
		require.NoError(t, err)
		result, err := s.BrowserInventory(ctx, id, decode(t, old, current.CodeHashes, nil))
		require.NoError(t, err)
		require.True(t, result.Blocked)
		require.NotNil(t, result.PendingBatch)
		require.Equal(t, current.BatchID, result.PendingBatch.BatchID)
		assertPending(t, current.BatchID)
		_, err = s.BrowserInventory(ctx, id, decode(t, current, nil, proofs(old)))
		require.Error(t, err, "another batch's sold proofs cannot resolve this batch")
		assertPending(t, current.BatchID)
	})
	t.Run("proof persistence failure cannot finish batch", func(t *testing.T) {
		id, b := seed(t)
		_, err := db.ExecContext(ctx, `CREATE FUNCTION reject_delivery_proof_test() RETURNS TRIGGER AS $$ BEGIN RAISE EXCEPTION 'isolated delivery proof failure'; END; $$ LANGUAGE plpgsql;
            CREATE TRIGGER reject_delivery_proof_test BEFORE INSERT ON liandong_browser_delivery_proofs FOR EACH ROW EXECUTE FUNCTION reject_delivery_proof_test()`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := db.ExecContext(ctx, "DROP TRIGGER reject_delivery_proof_test ON liandong_browser_delivery_proofs; DROP FUNCTION reject_delivery_proof_test()")
			require.NoError(t, err)
		})
		_, err = s.BrowserInventory(ctx, id, decode(t, b, nil, proofs(b)))
		require.Error(t, err)
		assertPending(t, b.BatchID)
		var count int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM liandong_browser_delivery_proofs WHERE batch_id=$1", b.BatchID).Scan(&count))
		require.Zero(t, count)
	})
}
