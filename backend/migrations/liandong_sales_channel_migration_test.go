package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiandongSalesChannelMigrationDefinesDurableInventorySchema(t *testing.T) {
	content, err := FS.ReadFile("235_liandong_sales_channel.sql")
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
	require.Contains(t, sql, "CHECK (grant_type IN ('balance', 'subscription'))")
	require.Contains(t, sql, "CHECK ((grant_type = 'balance' AND group_id IS NULL AND validity_days IS NULL) OR (grant_type = 'subscription' AND group_id IS NOT NULL AND validity_days IS NOT NULL AND validity_days > 0))")
	require.Contains(t, sql, "status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'uploaded', 'failed'))")
	require.Contains(t, sql, "code_sha256 VARCHAR(64) NOT NULL")
	require.Contains(t, sql, "REFERENCES liandong_restock_batches(batch_id) ON DELETE CASCADE")
	require.Contains(t, sql, "PRIMARY KEY (batch_id, ordinal)")
	require.Contains(t, sql, "UNIQUE (batch_id, code_sha256)")
}
