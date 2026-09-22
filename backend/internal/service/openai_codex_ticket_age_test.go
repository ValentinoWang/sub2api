package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAICodexTicketAgeBoundaries(t *testing.T) {
	captured := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	for _, ttl := range []time.Duration{240 * time.Second, time.Hour} {
		for _, age := range []int{149, 150, 209, 210, 239, 240} {
			t.Run(fmt.Sprintf("ttl=%s/age=%d", ttl, age), func(t *testing.T) {
				ticket := &openAICodexTicket{
					Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292,
					CapturedAt: captured, ExpiresAt: captured.Add(ttl),
				}
				now := captured.Add(time.Duration(age) * time.Second)
				require.Equal(t, age < 240, ticket.valid(now, 292))
				require.Equal(t, age < 210, ticket.injectable(now, 292))
				require.Equal(t, age >= 150, ticket.needsRefresh(now, 90*time.Second))
				account := ticketTestAccount(41)
				account.Extra = map[string]any{openAICodexTicketExtraKey(ticket.Model): ticket}
				statuses := OpenAICodexTicketStatuses(account, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true}, now)
				require.Equal(t, age < 210, statuses[0].Ready)
				require.Equal(t, age >= 210, statuses[0].Blocked)
				require.Equal(t, int64(240-age), statuses[0].RemainingSeconds)
				if age < 240 {
					require.Equal(t, captured.Add(240*time.Second), *statuses[0].ExpiresAt)
				} else {
					require.Nil(t, statuses[0].ExpiresAt)
				}
			})
		}
	}
}

func TestOpenAICodexTicketAgeRejectsUnknownOrFutureCapture(t *testing.T) {
	now := time.Now().UTC()
	for _, captured := range []time.Time{{}, now.Add(time.Second)} {
		ticket := &openAICodexTicket{State: fakeCodexTicketState(292), Length: 292, CapturedAt: captured, ExpiresAt: now.Add(time.Hour)}
		require.False(t, ticket.valid(now, 292))
		require.False(t, ticket.injectable(now, 292))
		require.True(t, ticket.needsRefresh(now, 90*time.Second))
	}
	var missing *openAICodexTicket
	require.False(t, missing.injectable(now, 292))
	require.True(t, missing.needsRefresh(now, 90*time.Second))
}

func TestOpenAICodexTicketAgeRespectsEarlierExpiry(t *testing.T) {
	now := time.Now().UTC()
	ticket := &openAICodexTicket{State: fakeCodexTicketState(292), Length: 292, CapturedAt: now, ExpiresAt: now.Add(120 * time.Second)}
	require.Equal(t, ticket.ExpiresAt, ticket.effectiveExpiresAt())
	require.False(t, ticket.needsRefresh(now.Add(29*time.Second), 90*time.Second))
	require.True(t, ticket.needsRefresh(now.Add(30*time.Second), 90*time.Second))
	require.False(t, ticket.injectable(now.Add(120*time.Second), 292))
	// A configured shorter refresh lead must not postpone preparation past 150s.
	ticket.ExpiresAt = now.Add(time.Hour)
	require.True(t, ticket.needsRefresh(now.Add(150*time.Second), time.Second))
}

func TestOpenAICodexTicketAgeConfig(t *testing.T) {
	for _, cfg := range []config.OpenAICodexTicketConfig{{}, {TTLSeconds: 3600, RefreshBeforeSeconds: 600}, {TTLSeconds: -1, RefreshBeforeSeconds: -1}} {
		got := ticketTestService(t, cfg, nil).openAICodexTicketConfig()
		require.Equal(t, 240, got.TTLSeconds)
		require.Equal(t, 90, got.RefreshBeforeSeconds)
	}
	got := ticketTestService(t, config.OpenAICodexTicketConfig{TTLSeconds: 120, RefreshBeforeSeconds: 30}, nil).openAICodexTicketConfig()
	require.Equal(t, 120, got.TTLSeconds)
	require.Equal(t, 30, got.RefreshBeforeSeconds)
}

func TestApplyOpenAICodexTicketAgeGate(t *testing.T) {
	for _, source := range []string{"memory", "persisted"} {
		for _, age := range []int{209, 210, 239, 240} {
			for _, failClosed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/age=%d/closed=%t", source, age, failClosed), func(t *testing.T) {
					account := ticketTestAccount(41)
					svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: failClosed}, nil)
					now := time.Now().UTC()
					ticket := &openAICodexTicket{Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292, CapturedAt: now.Add(-time.Duration(age) * time.Second), ExpiresAt: now.Add(time.Hour)}
					if source == "memory" {
						svc.openaiCodexTickets.Store(openAICodexTicketKey(account.ID, ticket.Model), ticket)
					} else {
						account.Extra = map[string]any{openAICodexTicketExtraKey(ticket.Model): ticket}
					}
					headers := http.Header{}
					err := svc.applyOpenAICodexTicket(context.Background(), account, ticket.Model, headers)
					blocked := age >= 210 && failClosed
					if blocked {
						require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
					} else {
						require.NoError(t, err)
					}
					require.Equal(t, blocked, svc.openAICodexTicketBlocksAccount(account, ticket.Model))
					if age < 210 {
						require.Equal(t, ticket.State, headers.Get(openAICodexTurnStateHeader))
					} else {
						require.Empty(t, headers.Get(openAICodexTurnStateHeader))
					}
				})
			}
		}
	}
}

func TestRefreshOpenAICodexTicketAgeWindow(t *testing.T) {
	for _, age := range []int{149, 150, 210, 240} {
		t.Run(fmt.Sprintf("age=%d", age), func(t *testing.T) {
			account := ticketTestAccount(41)
			account.Status = StatusActive
			now := time.Now().UTC()
			ticket := &openAICodexTicket{Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292, CapturedAt: now.Add(-time.Duration(age) * time.Second), ExpiresAt: now.Add(time.Hour)}
			account.Extra = map[string]any{openAICodexTicketExtraKey(ticket.Model): ticket}
			calls := 0
			upstream := &codexTicketFuncUpstream{do: func(*http.Request) (*http.Response, error) {
				calls++
				return nil, io.EOF
			}}
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true, TTLSeconds: 3600, RefreshBeforeSeconds: 600, Models: []string{ticket.Model}, HarvestProxyURL: "http://proxy.example.com:8080"}, upstream)
			svc.accountRepo = &codexTicketRefreshRepo{accounts: []Account{*account}}
			svc.refreshOpenAICodexTickets(context.Background())
			require.Equal(t, age >= 150, calls == 1)
			// A failed refresh retains a usable ticket only until the injection cutoff.
			headers := http.Header{}
			err := svc.applyOpenAICodexTicket(context.Background(), account, ticket.Model, headers)
			if age < 210 {
				require.NoError(t, err)
				require.Equal(t, ticket.State, headers.Get(openAICodexTurnStateHeader))
			} else {
				require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
			}
		})
	}
}

func TestStoreOpenAICodexTicketDoesNotRenewDuplicate(t *testing.T) {
	for _, source := range []string{"memory", "persisted"} {
		t.Run(source, func(t *testing.T) {
			now := time.Now().UTC()
			account := ticketTestAccount(41)
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, nil)
			old := &openAICodexTicket{Model: "gpt-6-astra", State: fakeCodexTicketState(292), Length: 292, CapturedAt: now.Add(-210 * time.Second), ExpiresAt: now.Add(time.Hour)}
			key := openAICodexTicketKey(account.ID, old.Model)
			if source == "memory" {
				svc.openaiCodexTickets.Store(key, old)
			} else {
				account.Extra = map[string]any{openAICodexTicketExtraKey(old.Model): old}
			}
			incoming := *old
			incoming.CapturedAt = now
			svc.storeOpenAICodexTicket(context.Background(), account, &incoming)
			got := svc.lookupOpenAICodexTicket(account, old.Model)
			require.Equal(t, old.CapturedAt, got.CapturedAt)
			require.Equal(t, now.Add(30*time.Second), got.ExpiresAt)
			require.False(t, got.injectable(now, 292))
			require.Equal(t, now, incoming.CapturedAt, "published/input snapshots must not be mutated")
			incoming.State = incoming.State[:291] + "C"
			svc.storeOpenAICodexTicket(context.Background(), account, &incoming)
			got = svc.lookupOpenAICodexTicket(account, old.Model)
			require.True(t, got.injectable(now, 292))
			require.Equal(t, now.Add(240*time.Second), got.ExpiresAt)
		})
	}
}
