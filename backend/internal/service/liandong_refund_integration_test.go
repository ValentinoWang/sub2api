//go:build integration

package service_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLiandongUnusedCodeRefundIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("refund_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	codes := repository.NewRedeemCodeRepository(client)
	users := repository.NewUserRepository(client, db)
	redeemer := service.NewRedeemService(codes, users, nil, nil, nil, client, nil, nil)
	refunds := service.NewLiandongRestockService(nil, nil, nil, nil, db)
	var sequence int
	seed := func(t *testing.T) (service.LiandongRefundPrepareRequest, *service.RedeemCode, int64) {
		t.Helper()
		sequence++
		req := service.LiandongRefundPrepareRequest{ExternalOrderNo: fmt.Sprintf("order-%d", sequence), BatchID: fmt.Sprintf("batch-%d", sequence), Code: fmt.Sprintf("TEST-REFUND-%08d", sequence)}
		code := &service.RedeemCode{Code: req.Code, Type: service.RedeemTypeBalance, Value: 5, Status: service.StatusUnused}
		require.NoError(t, codes.Create(ctx, code))
		digest := sha256.Sum256([]byte(req.Code))
		hash := hex.EncodeToString(digest[:])
		_, err := db.ExecContext(ctx, `INSERT INTO liandong_restock_batches (batch_id, goods_id, cny_amount, grant_value, code_count, code_sha256, status, created_at) VALUES ($1,42,5,5,1,$2,'uploaded',NOW())`, req.BatchID, hash)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO liandong_restock_batch_codes (batch_id,code_sha256,code_hint,ordinal) VALUES ($1,$2,'test',0)`, req.BatchID, hash)
		require.NoError(t, err)
		user, err := client.User.Create().SetEmail(fmt.Sprintf("refund-%d@example.invalid", sequence)).SetPasswordHash("test-only").Save(ctx)
		require.NoError(t, err)
		return req, code, user.ID
	}

	t.Run("reservation replay confirmation and permanent guards", func(t *testing.T) {
		req, code, userID := seed(t)
		prepared, err := refunds.PrepareUnusedCodeRefund(ctx, req)
		require.NoError(t, err)
		require.Equal(t, "reserved", prepared.Status)
		replayed, err := refunds.PrepareUnusedCodeRefund(ctx, req)
		require.NoError(t, err)
		require.Equal(t, prepared, replayed)
		_, err = redeemer.Redeem(ctx, userID, req.Code)
		require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
		_, err = refunds.ConfirmUnusedCodeRefund(ctx, service.LiandongRefundConfirmRequest{ExternalOrderNo: req.ExternalOrderNo})
		require.ErrorIs(t, err, service.ErrLiandongRefundInvalid)
		confirm := service.LiandongRefundConfirmRequest{ExternalOrderNo: req.ExternalOrderNo, MerchantRefundReference: "merchant-ref-1"}
		confirmed, err := refunds.ConfirmUnusedCodeRefund(ctx, confirm)
		require.NoError(t, err)
		require.Equal(t, "merchant_reference_recorded", confirmed.Status)
		replayed, err = refunds.ConfirmUnusedCodeRefund(ctx, confirm)
		require.NoError(t, err)
		require.Equal(t, confirmed, replayed)
		confirm.MerchantRefundReference = "different-ref"
		_, err = refunds.ConfirmUnusedCodeRefund(ctx, confirm)
		require.ErrorIs(t, err, service.ErrLiandongRefundConflict)
		req.ExternalOrderNo = "different-order"
		_, err = refunds.PrepareUnusedCodeRefund(ctx, req)
		require.ErrorIs(t, err, service.ErrLiandongRefundConflict)
		code.Status = service.StatusUnused
		require.Error(t, codes.Update(ctx, code))
		_, err = codes.BatchUpdate(ctx, []int64{code.ID}, service.RedeemCodeBatchUpdateFields{Status: &code.Status})
		require.Error(t, err)
		require.Error(t, codes.Delete(ctx, code.ID))
		for _, statement := range []string{
			`DELETE FROM liandong_code_refunds WHERE redeem_code_id = $1`,
			`UPDATE liandong_code_refunds SET status='reserved', merchant_refund_reference=NULL, confirmed_at=NULL WHERE redeem_code_id=$1`,
			`UPDATE liandong_code_refunds SET external_order_no='changed' WHERE redeem_code_id=$1`,
		} {
			_, err = db.ExecContext(ctx, statement, code.ID)
			require.Error(t, err)
		}
		user, err := users.GetByID(ctx, userID)
		require.NoError(t, err)
		require.Zero(t, user.Balance)
	})

	t.Run("reject redeemed and mismatched batch without debiting balance", func(t *testing.T) {
		req, _, userID := seed(t)
		_, err := redeemer.Redeem(ctx, userID, req.Code)
		require.NoError(t, err)
		_, err = refunds.PrepareUnusedCodeRefund(ctx, req)
		require.ErrorIs(t, err, service.ErrLiandongRefundNotEligible)
		req.BatchID = "unrelated-batch"
		_, err = refunds.PrepareUnusedCodeRefund(ctx, req)
		require.ErrorIs(t, err, service.ErrLiandongRefundNotEligible)
		user, err := users.GetByID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, float64(5), user.Balance)
	})

	t.Run("reservation storage failure rolls back the freeze", func(t *testing.T) {
		req, _, userID := seed(t)
		_, err := db.ExecContext(ctx, `CREATE FUNCTION reject_test_refund_insert() RETURNS TRIGGER AS $$ BEGIN RAISE EXCEPTION 'isolated reservation fault'; END; $$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_test_refund_insert BEFORE INSERT ON liandong_code_refunds FOR EACH ROW EXECUTE FUNCTION reject_test_refund_insert()`)
		require.NoError(t, err)
		_, prepareErr := refunds.PrepareUnusedCodeRefund(ctx, req)
		_, err = db.ExecContext(ctx, `DROP TRIGGER reject_test_refund_insert ON liandong_code_refunds; DROP FUNCTION reject_test_refund_insert()`)
		require.NoError(t, err)
		require.ErrorIs(t, prepareErr, service.ErrLiandongRefundUnavailable)
		_, err = redeemer.Redeem(ctx, userID, req.Code)
		require.NoError(t, err)
		user, err := users.GetByID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, float64(5), user.Balance)
	})

	t.Run("database constraints reject duplicate and invalid state records", func(t *testing.T) {
		req, code, _ := seed(t)
		_, err := refunds.PrepareUnusedCodeRefund(ctx, req)
		require.NoError(t, err)
		for _, statement := range []string{
			`INSERT INTO liandong_code_refunds (external_order_no,batch_id,code_sha256,redeem_code_id) SELECT 'duplicate-order',batch_id,code_sha256,redeem_code_id FROM liandong_code_refunds WHERE redeem_code_id=$1`,
			`UPDATE liandong_code_refunds SET status='money_refunded' WHERE redeem_code_id=$1`,
			`UPDATE liandong_code_refunds SET status='merchant_reference_recorded' WHERE redeem_code_id=$1`,
			`UPDATE liandong_code_refunds SET merchant_refund_reference='unexpected-reference' WHERE redeem_code_id=$1`,
		} {
			_, err := db.ExecContext(ctx, statement, code.ID)
			require.Error(t, err)
		}
	})

	t.Run("truncate bypass red case and permanent guard", func(t *testing.T) {
		req, code, _ := seed(t)
		_, err := refunds.PrepareUnusedCodeRefund(ctx, req)
		require.NoError(t, err)
		// Reproduce the missing statement-trigger failure inside a rolled-back
		// transaction in the disposable test database, retaining all durable rows.
		red, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = red.Rollback() }()
		_, err = red.ExecContext(ctx, `DROP TRIGGER liandong_refund_truncate_guard ON liandong_code_refunds`)
		require.NoError(t, err)
		_, err = red.ExecContext(ctx, `TRUNCATE liandong_code_refunds`)
		require.NoError(t, err, "row triggers alone do not protect TRUNCATE")
		_, err = red.ExecContext(ctx, `UPDATE redeem_codes SET status='unused' WHERE id=$1`, code.ID)
		require.NoError(t, err, "clearing the refund table would reactivate rights")
		require.NoError(t, red.Rollback())
		_, err = db.ExecContext(ctx, `TRUNCATE liandong_code_refunds`)
		require.Error(t, err, "statement trigger must reject TRUNCATE")
		_, err = db.ExecContext(ctx, `TRUNCATE redeem_codes CASCADE`)
		require.Error(t, err, "cascading TRUNCATE must also reject clearing refunds")
		frozen, err := codes.GetByID(ctx, code.ID)
		require.NoError(t, err)
		require.Equal(t, "disabled", frozen.Status)
	})

	t.Run("parallel redemption and refund grant at most once", func(t *testing.T) {
		for i := 0; i < 12; i++ {
			req, _, userID := seed(t)
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(3)
			var refundErr error
			var redeemErrors [2]error
			go func() { defer wg.Done(); <-start; _, refundErr = refunds.PrepareUnusedCodeRefund(ctx, req) }()
			for j := range redeemErrors {
				go func(j int) { defer wg.Done(); <-start; _, redeemErrors[j] = redeemer.Redeem(ctx, userID, req.Code) }(j)
			}
			close(start)
			wg.Wait()
			grants := 0
			for _, err := range redeemErrors {
				if err == nil {
					grants++
				} else {
					require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
				}
			}
			user, err := users.GetByID(ctx, userID)
			require.NoError(t, err)
			if refundErr == nil {
				require.Zero(t, grants)
				require.Zero(t, user.Balance)
			} else {
				require.ErrorIs(t, refundErr, service.ErrLiandongRefundNotEligible)
				require.Equal(t, 1, grants)
				require.Equal(t, float64(5), user.Balance)
			}
		}
	})

	t.Run("same order and same code concurrent replay", func(t *testing.T) {
		req, _, _ := seed(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var results [6]*service.LiandongCodeRefund
		var errs [6]error
		for i := range results {
			wg.Add(1)
			go func(i int) { defer wg.Done(); <-start; results[i], errs[i] = refunds.PrepareUnusedCodeRefund(ctx, req) }(i)
		}
		close(start)
		wg.Wait()
		for i := range results {
			require.NoError(t, errs[i])
			require.Equal(t, results[0], results[i])
		}
		other, _, _ := seed(t)
		other.ExternalOrderNo = req.ExternalOrderNo
		_, err := refunds.PrepareUnusedCodeRefund(ctx, other)
		require.ErrorIs(t, err, service.ErrLiandongRefundConflict)
		ordinary, err := codes.GetByCode(ctx, other.Code)
		require.NoError(t, err)
		ordinary.Notes = "ordinary code management remains available"
		require.NoError(t, codes.Update(ctx, ordinary))
		require.NoError(t, codes.Delete(ctx, ordinary.ID))
	})

	t.Run("already waiting admin update cannot reactivate committed refund", func(t *testing.T) {
		req, code, _ := seed(t)
		blocker, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = blocker.Rollback() }()
		_, err = blocker.ExecContext(ctx, `SELECT id FROM redeem_codes WHERE id=$1 FOR UPDATE`, code.ID)
		require.NoError(t, err)
		prepared := make(chan error, 1)
		go func() { _, err := refunds.PrepareUnusedCodeRefund(ctx, req); prepared <- err }()
		waitForLock := func(pattern string) {
			t.Helper()
			require.Eventually(t, func() bool {
				var waiting bool
				err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE $1)`, pattern).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond)
		}
		waitForLock("%FOR UPDATE OF r%")
		updated := make(chan error, 1)
		go func() { updated <- codes.Update(ctx, code) }()
		waitForLock("UPDATE %redeem_codes%")
		require.NoError(t, blocker.Commit())
		require.NoError(t, <-prepared)
		require.Error(t, <-updated)
		frozen, err := codes.GetByID(ctx, code.ID)
		require.NoError(t, err)
		require.Equal(t, "disabled", frozen.Status)
	})
}
