package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func fakeCodexTicketState(n int) string {
	if n < len(openAICodexTicketStatePrefix) {
		return strings.Repeat("A", n)
	}
	return openAICodexTicketStatePrefix + strings.Repeat("B", n-len(openAICodexTicketStatePrefix))
}

func ticketTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "tok", "chatgpt_account_id": "acc-1"},
	}
}

func ticketTestService(t *testing.T, cfg config.OpenAICodexTicketConfig, upstream HTTPUpstream) *OpenAIGatewayService {
	t.Helper()
	return &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{OpenAICodexTicket: cfg},
		},
		httpUpstream: upstream,
	}
}

func TestApplyOpenAICodexTicket_ReplacesHeader(t *testing.T) {
	state := fakeCodexTicketState(292)
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, nil)
	account := ticketTestAccount(41)
	svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
		AccountID:  41,
		Model:      "gpt-6-astra",
		State:      state,
		Length:     292,
		CapturedAt: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, fakeCodexTicketState(312))
	err := svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h)
	require.NoError(t, err)
	require.Equal(t, state, h.Get(openAICodexTurnStateHeader))
	require.Equal(t, 292, len(h.Get(openAICodexTurnStateHeader)))
}

func TestApplyOpenAICodexTicket_DoesNotReuseOtherModelOrAccount(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, &httpUpstreamRecorder{err: io.EOF})
	a := ticketTestAccount(41)
	b := ticketTestAccount(42)
	astra := fakeCodexTicketState(292)
	svc.storeOpenAICodexTicket(context.Background(), a, &openAICodexTicket{
		AccountID:  41,
		Model:      "gpt-6-astra",
		State:      astra,
		Length:     292,
		CapturedAt: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	})

	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, "keep-ungated")
	err := svc.applyOpenAICodexTicket(context.Background(), a, "gpt-5.5", h)
	require.NoError(t, err)
	require.Equal(t, "keep-ungated", h.Get(openAICodexTurnStateHeader))
	require.False(t, svc.openAICodexTicketBlocksAccount(a, "gpt-5.5"))
	require.True(t, svc.openAICodexTicketBlocksAccount(b, "gpt-6-astra"))
	require.False(t, svc.openAICodexTicketBlocksAccount(a, "gpt-6-astra"))

	h = http.Header{}
	err = svc.applyOpenAICodexTicket(context.Background(), b, "gpt-6-astra", h)
	require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
	require.Empty(t, h.Get(openAICodexTurnStateHeader))
}

func TestLookupOpenAICodexTicket_PrefersNewerExtra(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, TargetLength: 292, TTLSeconds: 3600}, nil)
	account := ticketTestAccount(41)
	oldState := fakeCodexTicketState(292)
	newState := openAICodexTicketStatePrefix + strings.Repeat("C", 286)
	svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
		AccountID:  41,
		Model:      "gpt-6-astra",
		State:      oldState,
		Length:     292,
		CapturedAt: time.Now().Add(-30 * time.Minute),
		ExpiresAt:  time.Now().Add(-time.Minute),
	})
	account.Extra = map[string]any{openAICodexTicketExtraKey("gpt-6-astra"): &openAICodexTicket{
		Model:      "gpt-6-astra",
		State:      newState,
		Length:     292,
		CapturedAt: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	},
	}
	got := svc.lookupOpenAICodexTicket(account, "gpt-6-astra")
	require.NotNil(t, got)
	require.Equal(t, newState, got.State)
	require.True(t, got.valid(time.Now(), 292))
}

func TestApplyOpenAICodexTicket_ExpiredNotInjected(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, &httpUpstreamRecorder{err: io.EOF})
	account := ticketTestAccount(41)
	svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
		AccountID:  41,
		Model:      "gpt-6-astra",
		State:      fakeCodexTicketState(292),
		Length:     292,
		CapturedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(-time.Minute),
	})
	h := http.Header{}
	err := svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h)
	require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
	require.Empty(t, h.Get(openAICodexTurnStateHeader))
}

func TestApplyOpenAICodexTicket_WrongLengthNotInjected(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, nil)
	account := ticketTestAccount(41)
	svc.storeOpenAICodexTicket(context.Background(), account, &openAICodexTicket{
		AccountID:  41,
		Model:      "gpt-6-astra",
		State:      fakeCodexTicketState(312),
		Length:     312,
		CapturedAt: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	})
	h := http.Header{}
	err := svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h)
	require.ErrorIs(t, err, ErrOpenAICodexTicketUnavailable)
	require.Empty(t, h.Get(openAICodexTurnStateHeader))
}

func TestApplyOpenAICodexTicket_FailOpenSkipsInject(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:    true,
		FailClosed: false,
	}, nil)
	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, "client-state")
	err := svc.applyOpenAICodexTicket(context.Background(), ticketTestAccount(41), "gpt-6-astra", h)
	require.NoError(t, err)
	require.Equal(t, "client-state", h.Get(openAICodexTurnStateHeader))
	require.False(t, svc.openAICodexTicketBlocksAccount(ticketTestAccount(41), "gpt-6-astra"))
}

func TestApplyOpenAICodexTicket_DisabledNoop(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: false, FailClosed: true}, nil)
	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, "client-state")
	err := svc.applyOpenAICodexTicket(context.Background(), ticketTestAccount(41), "gpt-6-astra", h)
	require.NoError(t, err)
	require.Equal(t, "client-state", h.Get(openAICodexTurnStateHeader))
}

func TestHarvestOpenAICodexTicket_SwitchesProxyAfterDegradedTicket(t *testing.T) {
	state292 := fakeCodexTicketState(292)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		codexHarvestResponse(http.StatusOK, fakeCodexTicketState(312)),
		codexHarvestResponse(http.StatusOK, state292),
	}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, upstream)
	first, second := testHarvestProxy(1), testHarvestProxy(2)
	harvest, nodes := newTicketHarvest(t, first, second)
	svc.SetCodexHarvestService(harvest)
	account := ticketTestAccount(41)
	controls, _, err := harvest.Controls(context.Background())
	require.NoError(t, err)

	result := svc.huntCodexHarvestTicket(context.Background(), harvest, account, "gpt-6-astra", []Proxy{first, second}, controls, codexHarvestHuntOptions{maxAttempts: 3})
	require.True(t, result.Harvested)
	require.Equal(t, 2, result.Sent)
	ticket := svc.lookupOpenAICodexTicket(account, "gpt-6-astra")
	require.NotNil(t, ticket)
	require.Equal(t, state292, ticket.State)
	require.NotEqual(t, ticket.HarvestProxyID, int64(0))
	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, "stale")
	require.NoError(t, svc.applyOpenAICodexTicket(context.Background(), account, "gpt-6-astra", h))
	require.Equal(t, state292, h.Get(openAICodexTurnStateHeader))
	require.Len(t, upstream.requests, 2)
	require.Empty(t, upstream.requests[0].Header.Get(openAICodexTurnStateHeader))
	require.Equal(t, openAICodexAstraMinVersion, upstream.requests[0].Header.Get("version"))
	require.Equal(t, HTTPUpstreamProfileOpenAIHarvest, HTTPUpstreamProfileFromContext(upstream.requests[0].Context()))
	require.True(t, upstream.requests[0].Close)
	// The degraded ticket and the good one came from different pool proxies.
	require.Len(t, nodes.feedback, 2)
	require.Equal(t, "invalid_state", nodes.feedback[0].Result)
	require.Equal(t, "success", nodes.feedback[1].Result)
	require.NotEqual(t, nodes.feedback[0].ProxyID, nodes.feedback[1].ProxyID)
	require.Equal(t, nodes.feedback[1].ProxyID, ticket.HarvestProxyID)
	for _, proxy := range []Proxy{first, second} {
		if proxy.ID == ticket.HarvestProxyID {
			require.Equal(t, proxy.URL(), upstream.lastProxyURL)
		}
	}
}

func TestHarvestOpenAICodexTicket_HTTP503DoesNotAbortHunt(t *testing.T) {
	state292 := fakeCodexTicketState(292)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"overloaded"}`))},
		codexHarvestResponse(http.StatusOK, state292),
	}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{
		Enabled:      true,
		TargetLength: 292,
		TTLSeconds:   3600,
		FailClosed:   true,
	}, upstream)
	harvest, _ := newTicketHarvest(t, testHarvestProxy(1), testHarvestProxy(2))
	svc.SetCodexHarvestService(harvest)
	controls, _, _ := harvest.Controls(context.Background())
	account := ticketTestAccount(41)
	result := svc.huntCodexHarvestTicket(context.Background(), harvest, account, "gpt-6-astra", []Proxy{testHarvestProxy(1), testHarvestProxy(2)}, controls, codexHarvestHuntOptions{maxAttempts: 3})
	require.True(t, result.Harvested)
	ticket := svc.lookupOpenAICodexTicket(account, "gpt-6-astra")
	require.NotNil(t, ticket)
	require.Equal(t, state292, ticket.State)
	require.Len(t, upstream.requests, 2)
}

func TestLookupOpenAICodexTicket_HydratesFromExtra(t *testing.T) {
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, TargetLength: 292, TTLSeconds: 3600}, nil)
	state := fakeCodexTicketState(292)
	account := ticketTestAccount(9)
	account.Extra = map[string]any{
		openAICodexTicketExtraKey("gpt-6-astra"): map[string]any{
			"state":       state,
			"length":      292,
			"model":       "gpt-6-astra",
			"captured_at": time.Now().Add(-time.Minute),
			"expires_at":  time.Now().Add(time.Hour),
		},
	}
	got := svc.lookupOpenAICodexTicket(account, "gpt-6-astra")
	require.NotNil(t, got)
	require.Equal(t, state, got.State)
	require.True(t, got.valid(time.Now(), 292))
}

func TestOpenAICodexTicketStatuses_ReportsRemainingTTL(t *testing.T) {
	now := time.Now().UTC()
	account := ticketTestAccount(41)
	account.Extra = map[string]any{
		openAICodexTicketExtraKey("gpt-6-astra"): map[string]any{
			"state":       fakeCodexTicketState(292),
			"length":      292,
			"model":       "gpt-6-astra",
			"captured_at": now.Add(-60 * time.Second),
			"expires_at":  now.Add(50 * time.Minute),
		},
	}
	got := OpenAICodexTicketStatuses(account, config.OpenAICodexTicketConfig{Enabled: true, FailClosed: true}, now)
	require.Len(t, got, 2)
	require.Equal(t, "gpt-6-astra", got[0].Model)
	require.True(t, got[0].Ready)
	require.Equal(t, int64(180), got[0].RemainingSeconds)
	require.Equal(t, now.Add(180*time.Second), *got[0].ExpiresAt)
	require.Equal(t, "gpt-5.6-sol", got[1].Model)
	require.False(t, got[1].Ready)
}

func TestExtractOpenAICodexTicketModel(t *testing.T) {
	require.Equal(t, "gpt-6-astra", extractOpenAICodexTicketModel([]byte(`{"model":"gpt-6-astra"}`)))
	require.Empty(t, extractOpenAICodexTicketModel([]byte(`{}`)))
}

// These stubs exercise the real continuous refresh path with both default models
// completing together. Run under -race to catch writes to the shared account maps.
type codexTicketRefreshRepo struct {
	AccountRepository
	accounts []Account
	mu       sync.Mutex
	updates  map[string]any
}

func (r *codexTicketRefreshRepo) ListByPlatform(context.Context, string) ([]Account, error) {
	return r.accounts, nil
}
func (r *codexTicketRefreshRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updates == nil {
		r.updates = make(map[string]any)
	}
	for k, v := range updates {
		r.updates[k] = v
	}
	return nil
}

type codexTicketSequentialUpstream struct {
	HTTPUpstream
	inFlight    atomic.Int64
	maxInFlight atomic.Int64
	started     atomic.Int64
}

func (u *codexTicketSequentialUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	current := u.inFlight.Add(1)
	defer u.inFlight.Add(-1)
	for {
		seen := u.maxInFlight.Load()
		if current <= seen || u.maxInFlight.CompareAndSwap(seen, current) {
			break
		}
	}
	u.started.Add(1)
	time.Sleep(5 * time.Millisecond)
	return codexHarvestResponse(http.StatusOK, fakeCodexTicketState(292)), nil
}

// Rounds probe one account/model at a time, and each probe works on its own
// copy of the account maps so the listed snapshot is never mutated.
func TestRefreshOpenAICodexTickets_SequentialModelsPreserveAccountSnapshot(t *testing.T) {
	account := ticketTestAccount(41)
	account.Status = StatusActive
	account.Extra = map[string]any{"existing": true}
	repo := &codexTicketRefreshRepo{accounts: []Account{*account}}
	upstream := &codexTicketSequentialUpstream{}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, upstream)
	harvest, _ := newTicketHarvest(t, testHarvestProxy(1))
	svc.SetCodexHarvestService(harvest)
	svc.accountRepo = repo
	svc.refreshOpenAICodexTickets(context.Background())
	require.Equal(t, int64(2), upstream.started.Load())
	require.Equal(t, int64(1), upstream.maxInFlight.Load())
	require.Equal(t, map[string]any{"existing": true}, account.Extra)
	require.Len(t, repo.updates, 2)
	for _, model := range []string{openAICodexTicketDefaultModel, openAICodexTicketDefaultSolModel} {
		ticket := svc.lookupOpenAICodexTicket(account, model)
		require.NotNil(t, ticket)
		require.True(t, ticket.valid(time.Now(), 292))
	}
	// Valid tickets do not produce another probe on the next cycle.
	svc.refreshOpenAICodexTickets(context.Background())
	require.Equal(t, int64(2), upstream.started.Load())
}
func TestOpenAICodexTicketStatuses_RespectRuntimeConfiguration(t *testing.T) {
	account := ticketTestAccount(41)
	require.Empty(t, OpenAICodexTicketStatuses(account, config.OpenAICodexTicketConfig{}, time.Now()))
	cfg := config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"custom-model"}}
	status := OpenAICodexTicketStatuses(account, cfg, time.Now())
	require.Len(t, status, 1)
	require.Equal(t, "custom-model", status[0].Model)
	require.False(t, status[0].Blocked)
	cfg.FailClosed = true
	require.True(t, OpenAICodexTicketStatuses(account, cfg, time.Now())[0].Blocked)
}
func TestProbeOpenAICodexTicket_RejectsInvalidState(t *testing.T) {
	cases := map[string]*http.Response{
		"degraded":   codexHarvestResponse(http.StatusOK, fakeCodexTicketState(312)),
		"prefix":     codexHarvestResponse(http.StatusOK, strings.Repeat("X", 292)),
		"missing":    codexHarvestResponse(http.StatusOK, ""),
		"incomplete": {StatusCode: http.StatusOK, Header: http.Header{openAICodexTurnStateHeader: []string{fakeCodexTicketState(292)}}, Body: io.NopCloser(strings.NewReader("data: {}\n\n"))},
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{responses: []*http.Response{response}}
			svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, upstream)
			harvest, _ := newTicketHarvest(t, testHarvestProxy(1))
			svc.SetCodexHarvestService(harvest)
			controls, _, _ := harvest.Controls(context.Background())
			account := ticketTestAccount(41)
			result := svc.huntCodexHarvestTicket(context.Background(), harvest, account, "gpt-6-astra", []Proxy{testHarvestProxy(1)}, controls, codexHarvestHuntOptions{maxAttempts: 1})
			require.False(t, result.Harvested)
			require.Nil(t, svc.lookupOpenAICodexTicket(account, "gpt-6-astra"))
		})
	}
}
func TestOpenAICodexTicket_RequiresActualLengthAndExpiry(t *testing.T) {
	ticket := &openAICodexTicket{State: fakeCodexTicketState(312), Length: 292, ExpiresAt: time.Now().Add(time.Hour)}
	require.False(t, ticket.valid(time.Now(), 292))
	ticket.State = fakeCodexTicketState(292)
	ticket.ExpiresAt = time.Time{}
	require.False(t, ticket.valid(time.Now(), 292))
}

// /responses/compact 的出站模型被 Forward 改写为 gateway.openai_compact_model
// （默认非空），门票门控必须按该出站模型判定。否则对门控模型发 compact 请求时，
// 所有无票账号都会被 fail_closed 误判为不可调度，而这些请求实际不需要票。
func TestOpenAICodexTicketGate_CompactRequestUsesForwardOutboundModel(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		OpenAICompactModel: "gpt-5.5",
		OpenAICodexTicket: config.OpenAICodexTicketConfig{
			Enabled:      true,
			TargetLength: 292,
			TTLSeconds:   3600,
			FailClosed:   true,
			Models:       []string{"gpt-6-astra"},
		},
	}}}
	account := ticketTestAccount(41) // 无票

	// 出站模型预测必须与 Forward 的解析链一致。
	require.Equal(t, "gpt-6-astra", svc.openAICodexTicketOutboundModel(account, "gpt-6-astra", false))
	require.Equal(t, "gpt-5.5", svc.openAICodexTicketOutboundModel(account, "gpt-6-astra", true))

	// 普通请求：出站仍是门控模型且无票 → fail_closed 必须拦号。
	require.True(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra", false))

	// compact 请求：出站已被改写成非门控的 gpt-5.5 → 不得拦号。
	require.False(t, svc.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-6-astra", true))

	// 回归锚点：按客户端原始模型判定（旧实现的口径）在 compact 下必然误拦。
	require.True(t, svc.openAICodexTicketBlocksAccount(account, canonicalOpenAIAccountSchedulingModel(account, "gpt-6-astra")))
}
