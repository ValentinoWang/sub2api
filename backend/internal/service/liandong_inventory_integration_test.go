//go:build integration

package service_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLiandongInventoryPostgresHashJoinAndGoodsIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("inventory_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("isolated-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))

	for _, batch := range []struct {
		id      string
		goodsID int64
		count   int
	}{{"inventory-a", 42, 4}, {"inventory-b", 42, 3}, {"inventory-other", 43, 1}} {
		_, err := db.ExecContext(ctx, `INSERT INTO liandong_restock_batches (batch_id, goods_id, cny_amount, grant_value, code_count, code_sha256, status, created_at) VALUES ($1,$2,5,5,$3,$4,'uploaded',NOW())`, batch.id, batch.goodsID, batch.count, "batch-digest")
		require.NoError(t, err)
	}

	expired := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	usedAt := time.Now().Add(-time.Minute)
	for _, code := range []struct {
		value     string
		batch     string
		ordinal   int
		status    string
		expiresAt *time.Time
		usedAt    *time.Time
	}{
		{"INVENTORY-UNUSED", "inventory-a", 0, "unused", nil, nil},
		{"INVENTORY-FUTURE", "inventory-a", 1, "unused", &future, nil},
		{"INVENTORY-USED", "inventory-a", 2, "used", nil, nil},
		{"INVENTORY-USED-AT", "inventory-a", 3, "unused", nil, &usedAt},
		{"INVENTORY-DISABLED", "inventory-b", 0, "disabled", nil, nil},
		{"INVENTORY-EXPIRED", "inventory-b", 1, "unused", &expired, nil},
		{"INVENTORY-MISSING", "inventory-b", 2, "", nil, nil},
		{"INVENTORY-OTHER-GOODS", "inventory-other", 0, "unused", nil, nil},
	} {
		if code.status != "" {
			// Deliberately unrelated notes ensure batch ownership comes from the
			// registered SHA-256 digest of the actual code, never notes text.
			_, err := client.RedeemCode.Create().SetCode(code.value).SetStatus(code.status).
				SetNotes("unrelated inventory annotation").SetNillableExpiresAt(code.expiresAt).SetNillableUsedAt(code.usedAt).Save(ctx)
			require.NoError(t, err)
		}
		digest := sha256.Sum256([]byte(code.value))
		_, err := db.ExecContext(ctx, `INSERT INTO liandong_restock_batch_codes (batch_id,code_sha256,code_hint,ordinal) VALUES ($1,$2,'same-hint',$3)`, code.batch, hex.EncodeToString(digest[:]), code.ordinal)
		require.NoError(t, err)
	}
	_, err = client.RedeemCode.Create().SetCode("INVENTORY-UNREGISTERED").SetStatus("unused").SetNotes("inventory-a goods_id=42").Save(ctx)
	require.NoError(t, err)

	// No merchant token is configured: this integration exercises migrated local
	// storage through the public service without sending any merchant request.
	cfg := &config.Config{LiandongRestock: config.LiandongRestockConfig{ProductsJSON: `[{"goods_id":42,"cny_amount":5,"usd_credit":5},{"goods_id":43,"cny_amount":10,"usd_credit":10},{"goods_id":44,"cny_amount":20,"usd_credit":20}]`}}
	svc := service.NewLiandongRestockService(repository.NewSettingRepository(client), nil, cfg, nil, db)
	report, err := svc.Inventory(ctx)
	require.NoError(t, err)
	require.Len(t, report.Rows, 3)
	counts := map[int64]service.LiandongLocalInventory{
		42: {Batches: 2, AllocatedCodes: 7, CreatedCodes: 6, UnusedCodes: 2, UsedCodes: 2, DisabledCodes: 1, OtherCodes: 1, MissingCodes: 1},
		43: {Batches: 1, AllocatedCodes: 1, CreatedCodes: 1, UnusedCodes: 1},
		44: {},
	}
	for _, row := range report.Rows {
		require.Empty(t, row.LocalError)
		require.NotNil(t, row.Local)
		expected, ok := counts[row.GoodsID]
		require.True(t, ok)
		require.Equal(t, expected, *row.Local, "goods ID %d", row.GoodsID)
		require.Nil(t, row.MerchantUnsold)
		require.Nil(t, row.QuantityDelta)
		require.False(t, row.IdentityVerified)
		require.Equal(t, "unknown", row.Comparison)
	}
}
