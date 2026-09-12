//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration240AlignsObservedSchemasWithoutRewritingRows(t *testing.T) {
	ctx := context.Background()
	migration, err := dbmigrations.FS.ReadFile("240_canonical_legacy_schema_alignment.sql")
	require.NoError(t, err)
	var canonicalShape []string
	for _, prior := range []struct {
		name, functions string
		legacyColumn    bool
	}{
		{"development", migration240PriorDevFunctions, false},
		{"production", migration240PriorProdFunctions, true},
	} {
		t.Run(prior.name, func(t *testing.T) {
			tx := testTx(t)
			_, err := tx.ExecContext(ctx, prior.functions)
			require.NoError(t, err)
			if !prior.legacyColumn {
				_, err = tx.ExecContext(ctx, "ALTER TABLE groups DROP COLUMN models_list_config")
				require.NoError(t, err)
			}
			var groupID int64
			require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO groups (name,platform,model_allowlist) VALUES (' DeepMath ','openai','{"enabled":true,"models":["keep-current"]}') RETURNING id`).Scan(&groupID))
			if prior.legacyColumn {
				_, err = tx.ExecContext(ctx, `UPDATE groups SET models_list_config='{"enabled":true,"models":["keep-legacy"]}' WHERE id=$1`, groupID)
				require.NoError(t, err)
			}
			var accountIDs []int64
			for _, kind := range []string{"oauth", "apikey"} {
				var id int64
				require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts (name,platform,type,credentials,extra) VALUES ($1,'openai',$2,'{"fixture":"preserve"}','{"sentinel":"preserve","openai_ws_allow_store_recovery":true}') RETURNING id`, "alignment-"+kind, kind).Scan(&id))
				_, err = tx.ExecContext(ctx, `INSERT INTO account_groups (account_id,group_id) VALUES ($1,$2)`, id, groupID)
				require.NoError(t, err)
				accountIDs = append(accountIDs, id)
			}
			// This explicitly establishes the observed policy difference before repair.
			var recovery bool
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT (extra->>'openai_ws_allow_store_recovery')::boolean FROM accounts WHERE id=$1`, accountIDs[0]).Scan(&recovery))
			require.Equal(t, !prior.legacyColumn, recovery)
			readSnapshot := func() []string {
				t.Helper()
				queries := []string{
					`SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY id),'[]')::text FROM accounts a`,
					`SELECT COALESCE(jsonb_agg(to_jsonb(g)-'models_list_config' ORDER BY id),'[]')::text FROM groups g`,
					`SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY filename),'[]')::text FROM schema_migrations m`,
					`SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY id),'[]')::text FROM scheduler_outbox o`,
				}
				values := make([]string, len(queries))
				for i, query := range queries {
					require.NoError(t, tx.QueryRowContext(ctx, query).Scan(&values[i]))
				}
				return values
			}
			before := readSnapshot()
			for replay := 0; replay < 2; replay++ {
				_, err = tx.ExecContext(ctx, string(migration))
				require.NoError(t, err)
				require.Equal(t, before, readSnapshot(), "migration must preserve all account/group data, history, and outbox")
			}
			var legacy string
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT models_list_config::text FROM groups WHERE id=$1`, groupID).Scan(&legacy))
			if prior.legacyColumn {
				require.JSONEq(t, `{"enabled":true,"models":["keep-legacy"]}`, legacy)
			} else {
				require.JSONEq(t, `{}`, legacy)
			}
			shape := migration240LogicalShape(ctx, t, tx)
			if canonicalShape == nil {
				canonicalShape = shape
			} else {
				require.Equal(t, canonicalShape, shape)
			}
			for _, id := range accountIDs {
				_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(extra,'{sentinel}','"updated"') WHERE id=$1`, id)
				require.NoError(t, err)
				var kind, mode, sentinel string
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT type,(extra->>'openai_ws_allow_store_recovery')::boolean,extra->>('openai_'||type||'_responses_websockets_v2_mode'),extra->>'sentinel' FROM accounts WHERE id=$1`, id).Scan(&kind, &recovery, &mode, &sentinel))
				require.Equal(t, kind == "apikey", recovery)
				require.Equal(t, "ctx_pool", mode)
				require.Equal(t, "updated", sentinel)
			}
			for _, kind := range []string{"oauth", "apikey"} {
				var id int64
				require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts (name,platform,type,extra) VALUES ($1,'openai',$2,'{"sentinel":"new-member"}') RETURNING id`, fmt.Sprintf("new-%s-%s", prior.name, kind), kind).Scan(&id))
				_, err = tx.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES ($1,$2)`, id, groupID)
				require.NoError(t, err)
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT (extra->>'openai_ws_allow_store_recovery')::boolean FROM accounts WHERE id=$1`, id).Scan(&recovery))
				require.Equal(t, kind == "apikey", recovery)
				var events int
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM scheduler_outbox WHERE account_id=$1 AND event_type='account_changed'`, id).Scan(&events))
				require.Greater(t, events, 0)
			}
		})
	}
}

func migration240LogicalShape(ctx context.Context, t *testing.T, tx *sql.Tx) []string {
	t.Helper()
	queries := []string{
		`SELECT jsonb_agg(jsonb_build_array(column_name,data_type,is_nullable,column_default) ORDER BY column_name)::text FROM information_schema.columns WHERE table_schema='public' AND table_name='groups' AND column_name IN ('models_list_config','model_allowlist')`,
		`SELECT pg_get_functiondef('public.enforce_deepmath_openai_ws_policy_on_account()'::regprocedure)`,
		`SELECT pg_get_functiondef('public.enforce_deepmath_openai_ws_policy_on_membership()'::regprocedure)`,
		`SELECT jsonb_agg(pg_get_triggerdef(oid) ORDER BY tgname)::text FROM pg_trigger WHERE tgname IN ('accounts_enforce_deepmath_openai_ws_policy','account_groups_enforce_deepmath_openai_ws_policy')`,
	}
	shape := make([]string, len(queries))
	for i, query := range queries {
		require.NoError(t, tx.QueryRowContext(ctx, query).Scan(&shape[i]))
	}
	require.JSONEq(t, `[["model_allowlist","jsonb","NO","'{}'::jsonb"],["models_list_config","jsonb","NO","'{}'::jsonb"]]`, shape[0])
	return shape
}
