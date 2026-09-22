package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssessResetCreditHistory(t *testing.T) {
	now := time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name string
		age  time.Duration
		kind string
		want string
	}{
		{"recent use", 3 * time.Minute, "used", "recent"},
		{"exact ten minute boundary", 10 * time.Minute, "used", "clear"},
		{"old use", time.Hour, "used", "clear"},
		{"grant is not usage", time.Minute, "granted", "clear"},
		{"unknown event kind", time.Minute, "unknown", "unknown"},
		{"future timestamp", -time.Minute, "used", "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := []resetCreditHistoryEvent{{ID: "private-id", Kind: test.kind, OccurredAt: now.Add(-test.age).Format(time.RFC3339Nano)}}
			check := assessResetCreditHistory([]resetCreditHistoryPage{{AsOf: now, WindowStart: now.Add(-30 * 24 * time.Hour), Events: &events}}, now)
			require.Equal(t, test.want, check.Status)
			body, err := json.Marshal(check)
			require.NoError(t, err)
			require.NotContains(t, string(body), "private-id")
		})
	}
}

func TestResetCreditHistoryVerifiedTimestamp(t *testing.T) {
	// Sanitized shape and timestamp from the read-only upstream probe; IDs are fixtures.
	page, err := parseResetCreditHistoryPage([]byte(`{
		"as_of":"2026-09-16T06:09:15.411143Z",
		"window_start":"2026-08-17T06:09:15.411143Z",
		"events":[
			{"id":"fixture-used","kind":"used","occurred_at":"2026-09-15T16:40:32.907674Z"},
			{"id":"fixture-grant-1","kind":"granted","occurred_at":"2026-09-05T04:19:53.891578Z"},
			{"id":"fixture-grant-2","kind":"granted","occurred_at":"2026-09-04T02:31:28.138700Z"},
			{"id":"fixture-grant-3","kind":"granted","occurred_at":"2026-08-22T00:16:32.769223Z"}
		],
		"next_cursor":null
	}`))
	require.NoError(t, err)
	check := assessResetCreditHistory([]resetCreditHistoryPage{page}, page.AsOf)
	require.Equal(t, "clear", check.Status)
	require.Equal(t, 1, check.UsedCount)
	require.Zero(t, check.RecentUseCount)
	require.True(t, check.HistoryComplete)
	at, err := time.Parse(time.RFC3339Nano, check.LastUsedAt)
	require.NoError(t, err)
	require.Equal(t, "2026-09-16 00:40:32", at.In(time.FixedZone("UTC+08:00", 8*60*60)).Format("2006-01-02 15:04:05"))
	// Move only the evaluation time to verify the reminder without consuming a real card.
	for _, test := range []struct {
		age  time.Duration
		want string
	}{
		{9*time.Minute + 59*time.Second, "recent"},
		{10 * time.Minute, "clear"},
	} {
		page.AsOf = at.Add(test.age)
		got := assessResetCreditHistory([]resetCreditHistoryPage{page}, page.AsOf)
		require.Equal(t, test.want, got.Status)
		require.Equal(t, check.LastUsedAt, got.LastUsedAt)
	}
}

func TestResetCreditHistoryUnknownAndConfirmationChanges(t *testing.T) {
	now := time.Now().UTC()
	events := []resetCreditHistoryEvent{}
	page := resetCreditHistoryPage{AsOf: now, WindowStart: now.Add(-time.Hour), Events: &events}
	clear := assessResetCreditHistory([]resetCreditHistoryPage{page}, now)
	require.Equal(t, "clear", clear.Status)
	for _, invalid := range []resetCreditHistoryPage{
		{AsOf: now.Add(-3 * time.Minute), WindowStart: now.Add(-time.Hour), Events: &events},
		{AsOf: now, WindowStart: now.Add(-time.Minute), Events: &events},
		{AsOf: now, WindowStart: now.Add(-time.Hour)},
	} {
		require.Equal(t, "unknown", assessResetCreditHistory([]resetCreditHistoryPage{invalid}, now).Status)
	}
	cursor := "more"
	page.NextCursor = &cursor
	require.Equal(t, "unknown", assessResetCreditHistory([]resetCreditHistoryPage{page}, now).Status)
	page.NextCursor = nil
	events = append(events, resetCreditHistoryEvent{ID: "used-1", Kind: "used", OccurredAt: now.Add(-time.Minute).Format(time.RFC3339Nano)})
	first := assessResetCreditHistory([]resetCreditHistoryPage{page}, now)
	second := assessResetCreditHistory([]resetCreditHistoryPage{page}, now.Add(time.Second))
	require.Equal(t, first.ConfirmationKey, second.ConfirmationKey, "re-query alone must not invalidate acknowledgement")
	require.NotEqual(t, clear.ConfirmationKey, first.ConfirmationKey)
	events = append(events, resetCreditHistoryEvent{ID: "used-2", Kind: "used", OccurredAt: now.Format(time.RFC3339Nano)})
	changed := assessResetCreditHistory([]resetCreditHistoryPage{page}, now)
	require.NotEqual(t, first.ConfirmationKey, changed.ConfirmationKey, "another use requires new acknowledgement")
	check := assessResetCreditHistory([]resetCreditHistoryPage{page, page}, now)
	require.Equal(t, 2, check.UsedCount, "duplicate page entries must not count twice")
	_, err := parseResetCreditHistoryPage([]byte(`{"events":[]}`))
	require.Error(t, err)
}

func TestCheckResetCreditHistoryReadsPagesWithoutConsuming(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"chatgpt_account_id": "private-account"}}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{100: account}}
	cache := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fake-token"}}
	tokens := NewOpenAITokenProvider(repo, cache, nil)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/backend-api/wham/rate-limit-reset-credits/history", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		events := []resetCreditHistoryEvent{{ID: "used-event", Kind: "used", OccurredAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}}
		page := resetCreditHistoryPage{AsOf: now, WindowStart: now.Add(-30 * 24 * time.Hour), Events: &events}
		if r.URL.Query().Get("cursor") == "" {
			cursor := "opaque+/cursor"
			page.NextCursor = &cursor
		} else {
			require.Equal(t, "opaque+/cursor", r.URL.Query().Get("cursor"))
		}
		require.NoError(t, json.NewEncoder(w).Encode(page))
	}))
	defer srv.Close()
	svc := NewOpenAIQuotaService(repo, nil, tokens, newQuotaRedirectingFactory(srv), nil)
	check := svc.CheckResetCreditHistory(context.Background(), 100)
	require.Equal(t, 2, requests)
	require.Equal(t, "recent", check.Status)
	require.Equal(t, 1, check.RecentUseCount)
	require.True(t, check.HistoryComplete)
}
