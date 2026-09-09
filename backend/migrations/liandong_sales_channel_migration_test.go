package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRest2BuildProductSchemaDefinesDurableInventorySchema(t *testing.T) {
	content, err := FS.ReadFile("238_rest2build_product_schema.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	for _, table := range []string{
		"CREATE TABLE IF NOT EXISTS liandong_product_mappings",
		"CREATE TABLE IF NOT EXISTS liandong_restock_batches",
		"CREATE TABLE IF NOT EXISTS liandong_restock_batch_codes",
	} {
		require.Contains(t, sql, table)
	}
	require.Contains(t, sql, "mapping_key VARCHAR(128) NOT NULL UNIQUE")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS uq_liandong_product_mappings_active_goods ON liandong_product_mappings(goods_id) WHERE enabled")
	require.NotContains(t, sql, "grant_type IN ('balance', 'subscription')")
	require.Contains(t, sql, "ADD CONSTRAINT liandong_product_mappings_balance_grant_check CHECK (grant_type = 'balance' AND group_id IS NULL AND validity_days IS NULL)")
	require.Contains(t, sql, "ADD CONSTRAINT liandong_restock_batches_grant_type_balance_check CHECK (grant_type = 'balance')")
	require.Contains(t, sql, "status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'uploaded', 'failed', 'needs_reconciliation'))")
	require.Contains(t, sql, "ALTER COLUMN status TYPE VARCHAR(32)")
	require.Contains(t, sql, "code_sha256 VARCHAR(64) NOT NULL")
	require.Contains(t, sql, "REFERENCES liandong_restock_batches(batch_id) ON DELETE CASCADE")
	require.Contains(t, sql, "PRIMARY KEY (batch_id, ordinal)")
	require.Contains(t, sql, "UNIQUE (batch_id, code_sha256)")
	require.Less(t, strings.Index(sql, "ADD COLUMN IF NOT EXISTS job_id"), strings.Index(sql, "CREATE INDEX IF NOT EXISTS idx_liandong_restock_batches_job"))
	for _, legacy := range []string{
		"231_codex_continuity.sql",
		"232_codex_continuity_client_window.sql",
		"233_user_lifecycle_emails.sql",
		"234_user_first_topup_bonus.sql",
		"235_liandong_sales_channel.sql",
		"236_membership_fulfillment.sql",
		"237_membership_coupon_late_payment.sql",
	} {
		require.Contains(t, sql, legacy)
	}
}
