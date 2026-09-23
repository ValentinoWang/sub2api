//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func cleanCodexHarvestTables(t *testing.T) {
	t.Helper()
	_, err := integrationDB.Exec(`DELETE FROM codex_harvest_nodes; DELETE FROM codex_harvest_flow_events`)
	require.NoError(t, err)
}

func TestCodexHarvestNodeRepository_RecordSnapshotAndReset(t *testing.T) {
	ctx := context.Background()
	cleanCodexHarvestTables(t)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{
		Name: "codex-harvest-node", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "x"},
	})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, account.ID)
		cleanCodexHarvestTables(t)
	})
	repo := NewCodexHarvestNodeRepository(integrationDB)
	scope := service.CodexHarvestNodeScope{AccountID: account.ID, Identity: "id-1", Model: "gpt-6-astra"}
	generation, records, err := repo.Snapshot(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, records)

	feedback := func(proxyID int64, result string) service.CodexHarvestNodeFeedback {
		return service.CodexHarvestNodeFeedback{Scope: scope, ProxyID: proxyID, ProxyName: fmt.Sprintf("proxy-%d", proxyID),
			Generation: generation, Result: result, LatencyMS: 120, CooldownSeconds: 60}
	}
	for _, result := range []string{"invalid_state", "network_error", "success", "rate_limited"} {
		ok, err := repo.Record(ctx, feedback(7, result))
		require.NoError(t, err)
		require.True(t, ok)
	}
	ok, err := repo.Record(ctx, feedback(8, "upstream_error"))
	require.NoError(t, err)
	require.True(t, ok)
	_, err = repo.Record(ctx, feedback(8, "bogus"))
	require.Error(t, err)

	_, records, err = repo.Snapshot(ctx, scope)
	require.NoError(t, err)
	byProxy := map[int64]service.CodexHarvestNodeRecord{}
	for _, r := range records {
		byProxy[r.ProxyID] = r
	}
	p7 := byProxy[7]
	require.Equal(t, "codex-harvest-node", p7.AccountName)
	require.Equal(t, int64(1), p7.Successes)
	require.Equal(t, int64(1), p7.Misses)
	require.Equal(t, int64(1), p7.NetworkErrors)
	require.Equal(t, int64(1), p7.AccountErrors)
	require.Zero(t, p7.ConsecutiveFailures, "a success resets the failure streak; account errors never add to it")
	require.Nil(t, p7.CooldownUntil, "account-level 429 does not cool the proxy")
	require.NotNil(t, p7.LastSuccess)
	require.Equal(t, "rate_limited", p7.LastResult)
	p8 := byProxy[8]
	require.Equal(t, 1, p8.ConsecutiveFailures)
	require.NotNil(t, p8.CooldownUntil)
	require.WithinDuration(t, time.Now().Add(time.Minute), *p8.CooldownUntil, 10*time.Second)

	// Another identity or model never sees this learning.
	_, other, err := repo.Snapshot(ctx, service.CodexHarvestNodeScope{AccountID: account.ID, Identity: "id-2", Model: "gpt-6-astra"})
	require.NoError(t, err)
	require.Empty(t, other)

	page, err := repo.List(ctx, 0, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Len(t, page.Items, 1)
	_, err = repo.List(ctx, 0, 101)
	require.Error(t, err)

	require.NoError(t, repo.Reset(ctx, p8.ID))
	newGeneration, records, err := repo.Snapshot(ctx, scope)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Greater(t, newGeneration, generation)
	// Feedback from before the reset is dropped instead of recreating the row.
	ok, err = repo.Record(ctx, feedback(8, "success"))
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, repo.Reset(ctx, 0))
	_, records, err = repo.Snapshot(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestCodexHarvestFlowRepository_AppendTrimAndOrder(t *testing.T) {
	ctx := context.Background()
	cleanCodexHarvestTables(t)
	t.Cleanup(func() { cleanCodexHarvestTables(t) })
	repo := NewCodexHarvestFlowRepository(integrationDB)
	require.Error(t, repo.Append(ctx, service.CodexHarvestFlowEvent{}))
	start := time.Now().Add(-time.Hour)
	for i := 0; i < codexHarvestFlowPersistCap+5; i++ {
		require.NoError(t, repo.Append(ctx, service.CodexHarvestFlowEvent{
			ID: fmt.Sprintf("evt-%03d", i), At: start.Add(time.Duration(i) * time.Second), Stage: "probe", Kind: "probe_miss",
			AccountID: 1, AccountName: "a", Model: "gpt-6-astra", ProxyID: 3, ProxyName: "p", HTTPStatus: 503, Result: "upstream_error", Manual: i%2 == 0,
		}))
	}
	// Re-appending an existing event is a no-op.
	require.NoError(t, repo.Append(ctx, service.CodexHarvestFlowEvent{ID: "evt-204", At: time.Now(), Stage: "probe"}))
	events, err := repo.List(ctx, 0)
	require.NoError(t, err)
	require.Len(t, events, codexHarvestFlowPersistCap)
	require.Equal(t, "evt-005", events[0].ID, "oldest retained first")
	require.Equal(t, "evt-204", events[len(events)-1].ID)
	require.True(t, events[len(events)-1].Manual)
	require.Equal(t, 503, events[0].HTTPStatus)
	last, err := repo.List(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, []string{"evt-202", "evt-203", "evt-204"}, []string{last[0].ID, last[1].ID, last[2].ID})
}

func TestCodexHarvestMigrationDropsLegacyProxySetting(t *testing.T) {
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM settings WHERE key='openai_codex_ticket_harvest_proxy_url'`).Scan(&count))
	require.Zero(t, count)
	var generation int64
	require.NoError(t, integrationDB.QueryRow(`SELECT generation FROM codex_harvest_learning_epoch WHERE id=1`).Scan(&generation))
	require.Positive(t, generation)
}
