//go:build integration

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRest2BuildProductSchemaAppliesAndEnforcesMembershipInvariants(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, ApplyMigrations(ctx, integrationDB))

	tx := testTx(t)

	var applied int
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT count(*)
FROM schema_migrations
WHERE filename = '238_rest2build_product_schema.sql'
`).Scan(&applied))
	require.Equal(t, 1, applied, "the consolidated rest2build product migration must be recorded")

	rows, err := tx.QueryContext(ctx, `
SELECT sku, for_sale, paused, verified_run_id IS NULL, verified_at IS NULL
FROM membership_products
WHERE sku IN ('chatgpt_pro_20x', 'chatgpt_plus')
ORDER BY sku
`)
	require.NoError(t, err)
	defer rows.Close()

	type product struct {
		sku                 string
		forSale, paused     bool
		missingRun, missing bool
	}
	products := make([]product, 0, 2)
	for rows.Next() {
		var item product
		require.NoError(t, rows.Scan(&item.sku, &item.forSale, &item.paused, &item.missingRun, &item.missing))
		products = append(products, item)
	}
	require.NoError(t, rows.Err())
	require.Len(t, products, 2, "migration must seed exactly the two supported membership SKUs")
	require.Equal(t, []string{"chatgpt_plus", "chatgpt_pro_20x"}, []string{products[0].sku, products[1].sku})
	for _, item := range products {
		require.False(t, item.forSale, "%s must not be for sale before a verification run", item.sku)
		require.True(t, item.paused, "%s must start paused", item.sku)
		require.True(t, item.missingRun, "%s must not have an initial verification run", item.sku)
		require.True(t, item.missing, "%s must not have an initial verification timestamp", item.sku)
	}

	userID := insertMembershipMigrationUser(t, tx, ctx)
	validationOrderID := insertMembershipMigrationOrder(t, tx, ctx, userID, "validation", 0, "created")
	require.NotEmpty(t, validationOrderID)
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_orders (
	id, user_id, sku, kind, idempotency_key, payment_state, price_minor,
	period_days, channel, credential_mode
) VALUES ($1, $2, 'chatgpt_plus', 'validation', $3, 'created', 1, 30, 'gpt', 'account_id')
`, uuid.NewString(), userID, uuid.NewString())
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_orders (
	id, user_id, sku, kind, idempotency_key, payment_state, price_minor,
	period_days, channel, credential_mode
) VALUES ($1, $2, 'chatgpt_plus', 'validation', $3, 'pending', 0, 30, 'gpt', 'account_id')
`, uuid.NewString(), userID, uuid.NewString())

	eventID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_events (id, order_id, action, actor)
VALUES ($1, $2, 'created', 'migration-236-test')
`, eventID, validationOrderID)
	require.NoError(t, err)
	for _, statement := range []string{
		"UPDATE membership_events SET action = 'modified' WHERE id = $1",
		"DELETE FROM membership_events WHERE id = $1",
		"TRUNCATE membership_events CASCADE",
	} {
		args := []any{eventID}
		if statement == "TRUNCATE membership_events CASCADE" {
			args = nil
		}
		err := requireRejectedMembershipStatement(t, tx, ctx, statement, args...)
		require.ErrorContains(t, err, "membership audit events are append-only")
	}

	orderID := insertMembershipMigrationOrder(t, tx, ctx, userID, "customer", 2000, "created")
	couponCode := "late-payment-" + uuid.NewString()
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_coupons (code, discount_minor, max_uses, used, expires_at)
VALUES ($1, 100, 1, 1, now() + interval '1 day')
`, couponCode)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE membership_coupons SET used=2 WHERE code=$1`, couponCode)
	require.NoError(t, err, "migration 237 must allow factual late-payment use to exceed max_uses")
	requireRejectedMembershipStatement(t, tx, ctx, `UPDATE membership_coupons SET used=-1 WHERE code=$1`, couponCode)

	var paymentOrderID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO payment_orders (
	user_id, amount, pay_amount, recharge_code, out_trade_no, payment_type,
	order_type, status, expires_at
) VALUES ($1, 20, 20, $2, $3, 'alipay', 'membership', 'PENDING', now() + interval '10 minutes')
RETURNING id
`, userID, "membership-"+uuid.NewString(), "membership_"+uuid.NewString()).Scan(&paymentOrderID))
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_payment_links (payment_order_id, order_id)
VALUES ($1, $2)
`, paymentOrderID, orderID)
	require.NoError(t, err)
	ticketHash := bytes.Repeat([]byte{0x5a}, 32)
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_payment_redirect_tickets (
	ticket_hash, order_id, payment_order_id, user_id, expires_at
) VALUES ($1, $2, $3, $4, now() + interval '5 minutes')
`, ticketHash, orderID, paymentOrderID, userID)
	require.NoError(t, err)
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_payment_redirect_tickets (
	ticket_hash, order_id, payment_order_id, user_id, expires_at
) VALUES ($1, $2, $3, $4, now() + interval '5 minutes')
`, ticketHash[:31], orderID, paymentOrderID, userID)

	firstTaskID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_tasks (id, order_id, state, channel)
VALUES ($1, $2, 'queued', 'gpt')
`, firstTaskID, orderID)
	require.NoError(t, err)
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_tasks (id, order_id, state, channel)
VALUES ($1, $2, 'queued', 'gpt')
`, uuid.NewString(), orderID)

	activeTarget := "sha256:migration-236-active-target"
	activeTargetOrderID := insertMembershipMigrationOrder(t, tx, ctx, userID, "customer", 2000, "created")
	activeTargetTaskID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_tasks (id, order_id, state, channel, target_hash)
VALUES ($1, $2, 'queued', 'gpt', $3)
`, activeTargetTaskID, activeTargetOrderID, activeTarget)
	require.NoError(t, err)
	rejectedTargetOrderID := insertMembershipMigrationOrder(t, tx, ctx, userID, "customer", 2000, "created")
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_tasks (id, order_id, state, channel, target_hash)
VALUES ($1, $2, 'processing', 'gpt', $3)
`, uuid.NewString(), rejectedTargetOrderID, activeTarget)
	_, err = tx.ExecContext(ctx, "UPDATE membership_tasks SET state = 'succeeded' WHERE id = $1", activeTargetTaskID)
	require.NoError(t, err)
	reusedTargetOrderID := insertMembershipMigrationOrder(t, tx, ctx, userID, "customer", 2000, "created")
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_tasks (id, order_id, state, channel, target_hash)
VALUES ($1, $2, 'queued', 'gpt', $3)
`, uuid.NewString(), reusedTargetOrderID, activeTarget)
	require.NoError(t, err, "a target may be reused after the previous task reaches a terminal state")

	cdkID := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_cdks (id, sku, channel, ciphertext, fingerprint)
VALUES ($1, 'chatgpt_plus', 'gpt', 'ciphertext', $2)
`, cdkID, uuid.NewString())
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_attempts (id, task_id, cdk_id, state)
VALUES ($1, $2, $3, 'intent')
`, uuid.NewString(), firstTaskID, cdkID)
	require.NoError(t, err)
	requireRejectedMembershipStatement(t, tx, ctx, `
INSERT INTO membership_attempts (id, task_id, cdk_id, state)
VALUES ($1, $2, $3, 'submitted')
`, uuid.NewString(), firstTaskID, cdkID)
	_, err = tx.ExecContext(ctx, "UPDATE membership_attempts SET state = 'failed' WHERE task_id = $1", firstTaskID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
INSERT INTO membership_attempts (id, task_id, cdk_id, state)
VALUES ($1, $2, $3, 'submitted')
`, uuid.NewString(), firstTaskID, cdkID)
	require.NoError(t, err, "a failed attempt must not block a replacement attempt")
}

func insertMembershipMigrationUser(t *testing.T, tx *sql.Tx, ctx context.Context) int64 {
	t.Helper()

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, role, status, balance, concurrency)
VALUES ($1, 'hash', 'user', 'active', 0, 1)
RETURNING id
`, "migration-236-"+uuid.NewString()+"@example.invalid").Scan(&userID))
	return userID
}

func insertMembershipMigrationOrder(t *testing.T, tx *sql.Tx, ctx context.Context, userID int64, kind string, priceMinor int64, paymentState string) string {
	t.Helper()

	orderID := uuid.NewString()
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO membership_orders (
	id, user_id, sku, kind, idempotency_key, payment_state, price_minor,
	period_days, channel, credential_mode
) VALUES ($1, $2, 'chatgpt_plus', $3, $4, $5, $6, 30, 'gpt', 'account_id')
RETURNING id
`, orderID, userID, kind, uuid.NewString(), paymentState, priceMinor).Scan(&orderID))
	return orderID
}

func requireRejectedMembershipStatement(t *testing.T, tx *sql.Tx, ctx context.Context, statement string, args ...any) error {
	t.Helper()

	const savepoint = "membership_236_rejected_statement"
	_, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, statement, args...)
	require.Error(t, err, "statement must be rejected: %s", statement)
	_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint)
	require.NoError(t, rollbackErr)
	_, releaseErr := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
	require.NoError(t, releaseErr)
	return err
}
