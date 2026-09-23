package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCodexHarvestControls_DefaultsAndValidation(t *testing.T) {
	h := NewCodexHarvestService(nil, nil, &codexPolicyMigrationRepoStub{values: map[string]string{}}, newCodexHarvestProxyRepoStub(testHarvestProxy(1)))
	controls, configured, err := h.Controls(context.Background())
	require.NoError(t, err)
	require.False(t, configured)
	require.Equal(t, DefaultCodexHarvestControls(), controls)
	require.Empty(t, controls.ProxyIDs)

	// Every preset keeps a round inside the 60s window between refresh (150s) and injection stop (210s).
	for name, speed := range CodexHarvestSpeedPresets() {
		require.Less(t, speed.RoundIntervalSeconds, config.OpenAICodexTicketInjectionMaxAgeSeconds-config.OpenAICodexTicketRefreshAgeSeconds, name)
		require.NoError(t, ValidateCodexHarvestControls(CodexHarvestControls{Version: 1, Preset: name, Speed: speed}), name)
	}

	invalid := func(mutate func(*CodexHarvestControls)) error {
		v := DefaultCodexHarvestControls()
		mutate(&v)
		return h.SaveControls(context.Background(), v)
	}
	for name, mutate := range map[string]func(*CodexHarvestControls){
		"version":         func(v *CodexHarvestControls) { v.Version = 2 },
		"unknown preset":  func(v *CodexHarvestControls) { v.Preset = "turbo" },
		"preset mismatch": func(v *CodexHarvestControls) { v.Speed.CooldownSeconds = 61 },
		"below bound":     func(v *CodexHarvestControls) { v.Preset = "custom"; v.Speed.RoundIntervalSeconds = 4 },
		"above bound":     func(v *CodexHarvestControls) { v.Preset = "custom"; v.Speed.MaxRequestsPerAccountHour = 601 },
		"negative proxy":  func(v *CodexHarvestControls) { v.ProxyIDs = []int64{-1} },
		"duplicate proxy": func(v *CodexHarvestControls) { v.ProxyIDs = []int64{1, 1} },
		"unknown proxy":   func(v *CodexHarvestControls) { v.ProxyIDs = []int64{1, 99} },
		"pool too large": func(v *CodexHarvestControls) {
			for i := int64(1); i <= codexHarvestMaxPoolSize+1; i++ {
				v.ProxyIDs = append(v.ProxyIDs, i)
			}
		},
	} {
		err := invalid(mutate)
		require.Error(t, err, name)
		require.True(t, infraerrors.IsBadRequest(err), name)
	}

	saved := DefaultCodexHarvestControls()
	saved.Preset = "fast"
	saved.Speed = CodexHarvestSpeedPresets()["fast"]
	saved.ProxyIDs = []int64{1}
	require.NoError(t, h.SaveControls(context.Background(), saved))
	select {
	case <-h.wake:
	default:
		t.Fatal("saving controls must wake the harvest loop")
	}
	h.loadedUntil = time.Time{}
	controls, configured, err = h.Controls(context.Background())
	require.NoError(t, err)
	require.True(t, configured)
	require.Equal(t, saved, controls)
}

type codexHarvestFailingSettings struct {
	*codexPolicyMigrationRepoStub
	fail bool
}

func (r *codexHarvestFailingSettings) GetValue(ctx context.Context, key string) (string, error) {
	if r.fail {
		return "", errors.New("storage offline")
	}
	return r.codexPolicyMigrationRepoStub.GetValue(ctx, key)
}

func TestCodexHarvestControls_ReadFailureKeepsLastValid(t *testing.T) {
	settings := &codexHarvestFailingSettings{codexPolicyMigrationRepoStub: &codexPolicyMigrationRepoStub{values: map[string]string{}}}
	h := NewCodexHarvestService(nil, nil, settings, newCodexHarvestProxyRepoStub(testHarvestProxy(1)))
	saved := DefaultCodexHarvestControls()
	saved.ProxyIDs = []int64{1}
	require.NoError(t, h.SaveControls(context.Background(), saved))
	settings.fail = true
	h.loadedUntil = time.Time{}
	controls, configured, err := h.Controls(context.Background())
	require.Error(t, err)
	require.True(t, configured)
	require.Equal(t, []int64{1}, controls.ProxyIDs)
}

func TestCodexHarvestPool_ReportsUnusableMembersAndKeepsOrder(t *testing.T) {
	expiredAt := time.Now().Add(-time.Minute)
	inactive := testHarvestProxy(2)
	inactive.Status = "inactive"
	expired := testHarvestProxy(3)
	expired.ExpiresAt = &expiredAt
	h := NewCodexHarvestService(nil, nil, nil, newCodexHarvestProxyRepoStub(testHarvestProxy(1), inactive, expired, testHarvestProxy(5)))
	usable, members, err := h.Pool(context.Background(), []int64{5, 2, 3, 4, 1}, time.Now())
	require.NoError(t, err)
	require.Equal(t, []int64{5, 1}, []int64{usable[0].ID, usable[1].ID})
	reasons := map[int64]string{}
	for _, m := range members {
		reasons[m.ProxyID] = m.Reason
		require.Equal(t, m.Reason == "", m.Usable)
	}
	require.Equal(t, map[int64]string{5: "", 2: "inactive", 3: "expired", 4: "deleted", 1: ""}, reasons)
}

func TestRankCodexHarvestProxies(t *testing.T) {
	now := time.Now()
	recent := now.Add(-time.Hour)
	stale := now.Add(-8 * 24 * time.Hour)
	cooling := now.Add(time.Minute)
	pool := []Proxy{testHarvestProxy(1), testHarvestProxy(2), testHarvestProxy(3), testHarvestProxy(4), testHarvestProxy(5)}
	records := []CodexHarvestNodeRecord{
		{ProxyID: 2, Successes: 1, Misses: 3, LastSuccess: &recent},
		{ProxyID: 3, Successes: 5, LastSuccess: &recent},
		{ProxyID: 4, Successes: 9, LastSuccess: &stale},
		{ProxyID: 5, CooldownUntil: &cooling},
	}
	ranked := rankCodexHarvestProxies(pool, records, map[int64]bool{}, 0, now)
	ids := make([]int64, 0, len(ranked))
	for _, p := range ranked {
		ids = append(ids, p.ID)
	}
	// Recent successes first by success rate, cooling proxies excluded, the rest in rotation order.
	require.Equal(t, []int64{3, 2, 1, 4}, ids)
	ranked = rankCodexHarvestProxies(pool, records, map[int64]bool{3: true, 2: true}, 3, now)
	require.Equal(t, int64(4), ranked[0].ID, "the explore cursor rotates proxies without recent success")
	require.Empty(t, rankCodexHarvestProxies(pool, records, map[int64]bool{1: true, 2: true, 3: true, 4: true}, 0, now))
}

func TestChooseHarvestProxy_PrefersTicketProxyInsidePool(t *testing.T) {
	h := NewCodexHarvestService(nil, nil, nil, nil)
	pool := []Proxy{testHarvestProxy(1), testHarvestProxy(2)}
	proxy, reason, ok := h.chooseHarvestProxy(&openAICodexTicket{HarvestProxyID: 2}, pool, nil, map[int64]bool{}, time.Now())
	require.True(t, ok)
	require.Equal(t, int64(2), proxy.ID)
	require.Equal(t, "ticket_sticky", reason)
	// A proxy removed from the pool is never used again, even if it issued the ticket.
	proxy, reason, ok = h.chooseHarvestProxy(&openAICodexTicket{HarvestProxyID: 9}, pool, nil, map[int64]bool{}, time.Now())
	require.True(t, ok)
	require.NotEqual(t, int64(9), proxy.ID)
	require.Equal(t, "explore", reason)
	_, _, ok = h.chooseHarvestProxy(nil, pool, nil, map[int64]bool{1: true, 2: true}, time.Now())
	require.False(t, ok)
}

func TestCodexHarvestAccountHourWindow(t *testing.T) {
	h := NewCodexHarvestService(nil, nil, nil, nil)
	start := time.Now()
	require.True(t, h.takeAccountHour(7, 2, start))
	require.True(t, h.takeAccountHour(7, 2, start.Add(time.Second)))
	require.False(t, h.takeAccountHour(7, 2, start.Add(2*time.Second)))
	require.True(t, h.takeAccountHour(8, 2, start), "limits are per account")
	require.Equal(t, 2, h.accountHourUsed(7, start.Add(time.Minute)))
	require.True(t, h.takeAccountHour(7, 2, start.Add(time.Hour+time.Millisecond)), "requests leave the window after an hour")
}

func harvestRoundService(t *testing.T, upstream HTTPUpstream, accounts []Account, mutate func(*CodexHarvestControls), proxies ...Proxy) (*OpenAIGatewayService, *CodexHarvestService, *codexTicketRefreshRepo) {
	t.Helper()
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}}, upstream)
	h, _ := newTicketHarvest(t, proxies...)
	if mutate != nil {
		controls, _, err := h.Controls(context.Background())
		require.NoError(t, err)
		controls.Preset = codexHarvestPresetCustom
		mutate(&controls)
		require.NoError(t, h.SaveControls(context.Background(), controls))
	}
	svc.SetCodexHarvestService(h)
	repo := &codexTicketRefreshRepo{accounts: accounts}
	svc.accountRepo = repo
	return svc, h, repo
}

func activeTicketAccount(id int64) Account {
	account := *ticketTestAccount(id)
	account.Status = StatusActive
	account.Schedulable = true
	account.Credentials = map[string]any{"access_token": "tok", "chatgpt_account_id": fmt.Sprintf("acc-%d", id)}
	return account
}

func TestRefreshOpenAICodexTickets_RoundBudgetAndHourLimit(t *testing.T) {
	upstream := &httpUpstreamRecorder{err: io.EOF}
	svc, h, _ := harvestRoundService(t, upstream, []Account{activeTicketAccount(1), activeTicketAccount(2)}, func(c *CodexHarvestControls) {
		c.Speed.MaxRequestsPerRound = 3
		c.Speed.MaxProxyAttempts = 2
		c.Speed.MaxRequestsPerAccountHour = 1
	}, testHarvestProxy(1), testHarvestProxy(2))
	svc.refreshOpenAICodexTickets(context.Background())
	// Each account may only send one request per hour, even though the round and proxy limits allow more.
	require.Len(t, upstream.requests, 2)
	require.Equal(t, 1, h.accountHourUsed(1, time.Now()))
	require.Equal(t, 1, h.accountHourUsed(2, time.Now()))
	runtime := h.Runtime()
	require.False(t, runtime.Running)
	require.Equal(t, 2, runtime.RequestsUsed)
	require.Equal(t, 3, runtime.RequestBudget)
}

func TestRefreshOpenAICodexTickets_RoundBudgetCapsRequests(t *testing.T) {
	upstream := &httpUpstreamRecorder{err: io.EOF}
	svc, h, _ := harvestRoundService(t, upstream, []Account{activeTicketAccount(1), activeTicketAccount(2), activeTicketAccount(3)}, func(c *CodexHarvestControls) {
		c.Speed.MaxRequestsPerRound = 2
		c.Speed.MaxProxyAttempts = 1
	}, testHarvestProxy(1))
	svc.refreshOpenAICodexTickets(context.Background())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "round_budget", h.Runtime().IdleReason)
	// The cursor moves on so the account skipped this round goes first next time.
	upstream.requests = nil
	h.cooldowns.Range(func(key, _ any) bool { h.cooldowns.Delete(key); return true })
	svc.refreshOpenAICodexTickets(context.Background())
	require.NotEmpty(t, upstream.requests)
}

func TestRefreshOpenAICodexTickets_SchedulableAccountsFirst(t *testing.T) {
	upstream := &httpUpstreamRecorder{err: io.EOF}
	deferred := activeTicketAccount(1)
	deferred.Schedulable = false
	svc, _, _ := harvestRoundService(t, upstream, []Account{deferred, activeTicketAccount(2)}, func(c *CodexHarvestControls) {
		c.Speed.MaxRequestsPerRound = 1
		c.Speed.MaxProxyAttempts = 1
	}, testHarvestProxy(1))
	svc.refreshOpenAICodexTickets(context.Background())
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "acc-2", upstream.requests[0].Header.Get("chatgpt-account-id"), "the schedulable account goes first")
}

func TestHuntCodexHarvestTicket_RateLimitCooldownAndAccountStop(t *testing.T) {
	limited := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"300"}}, Body: io.NopCloser(strings.NewReader("{}"))}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{limited}}
	svc, h, _ := harvestRoundService(t, upstream, []Account{activeTicketAccount(1)}, func(c *CodexHarvestControls) {
		c.Speed.MaxProxyAttempts = 3
		c.Speed.CooldownSeconds = 30
	}, testHarvestProxy(1), testHarvestProxy(2), testHarvestProxy(3))
	svc.refreshOpenAICodexTickets(context.Background())
	require.Len(t, upstream.requests, 1, "429 stops the account for this round instead of trying other proxies")
	until, cooling := h.coolingDown(1, "gpt-6-astra", time.Now())
	require.True(t, cooling)
	require.Greater(t, time.Until(until), 4*time.Minute, "Retry-After beyond the configured cooldown is honored")
	svc.refreshOpenAICodexTickets(context.Background())
	require.Len(t, upstream.requests, 1, "a cooling account/model is skipped by later rounds")
}

func TestRefreshOpenAICodexTickets_EmptyPoolSendsNothing(t *testing.T) {
	upstream := &httpUpstreamRecorder{}
	svc, h, _ := harvestRoundService(t, upstream, []Account{activeTicketAccount(1)}, nil)
	svc.refreshOpenAICodexTickets(context.Background())
	require.Empty(t, upstream.requests)
	require.Equal(t, "empty_pool", h.Runtime().IdleReason)
}

func TestHuntCodexHarvestTicket_LearningFeedbackAndStickiness(t *testing.T) {
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		codexHarvestResponse(http.StatusOK, fakeCodexTicketState(292)),
		codexHarvestResponse(http.StatusOK, fakeCodexTicketState(292)),
	}}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true, Models: []string{"gpt-6-astra"}}, upstream)
	pool := []Proxy{testHarvestProxy(1), testHarvestProxy(2), testHarvestProxy(3)}
	h, nodes := newTicketHarvest(t, pool...)
	svc.SetCodexHarvestService(h)
	controls, _, _ := h.Controls(context.Background())
	account := ticketTestAccount(41)
	require.True(t, svc.huntCodexHarvestTicket(context.Background(), h, account, "gpt-6-astra", pool, controls, codexHarvestHuntOptions{maxAttempts: 1}).Harvested)
	first := svc.lookupOpenAICodexTicket(account, "gpt-6-astra")
	require.NotNil(t, first)
	require.Equal(t, codexHarvestIdentity(account), nodes.records[0].Identity)
	require.NotEmpty(t, nodes.records[0].Identity)
	// The refresh goes back to the proxy that issued the current ticket.
	require.True(t, svc.huntCodexHarvestTicket(context.Background(), h, account, "gpt-6-astra", pool, controls, codexHarvestHuntOptions{maxAttempts: 1}).Harvested)
	require.Len(t, nodes.feedback, 2)
	require.Equal(t, first.HarvestProxyID, nodes.feedback[1].ProxyID)
	require.Equal(t, int64(2), nodes.records[0].Successes)

	// Feedback from before a reset is discarded.
	require.NoError(t, h.ResetNodes(context.Background(), 0))
	stale := nodes.feedback[0]
	recorded, err := nodes.Record(context.Background(), stale)
	require.NoError(t, err)
	require.False(t, recorded)
}

func TestHuntCodexHarvestTicket_TokenFailureCoolsDownWithoutRequest(t *testing.T) {
	upstream := &httpUpstreamRecorder{}
	svc := ticketTestService(t, config.OpenAICodexTicketConfig{Enabled: true}, upstream)
	h, _ := newTicketHarvest(t, testHarvestProxy(1))
	controls, _, _ := h.Controls(context.Background())
	account := ticketTestAccount(41)
	account.Credentials = map[string]any{}
	result := svc.huntCodexHarvestTicket(context.Background(), h, account, "gpt-6-astra", []Proxy{testHarvestProxy(1)}, controls, codexHarvestHuntOptions{maxAttempts: 2})
	require.False(t, result.Harvested)
	require.Zero(t, result.Sent)
	require.Empty(t, upstream.requests)
	_, cooling := h.coolingDown(41, "gpt-6-astra", time.Now())
	require.True(t, cooling)
	events := h.Events()
	require.Equal(t, "token_error", events[len(events)-1].Result)
}

func TestCodexHarvestFlowRing_CapHydrateAndRedaction(t *testing.T) {
	repo := &codexHarvestFlowRepoStub{}
	for i := 0; i < 3; i++ {
		repo.events = append(repo.events, CodexHarvestFlowEvent{ID: "old-" + string(rune('a'+i)), At: time.Now(), Stage: "probe"})
	}
	h := NewCodexHarvestService(nil, repo, nil, nil)
	require.Len(t, h.Events(), 3, "events are restored from storage on start")
	for i := 0; i < codexHarvestFlowCap+10; i++ {
		h.recordProbe(ticketTestAccount(1), "gpt-6-astra", &Proxy{ID: 1, Name: "p1", Password: "secret"}, codexHarvestProbeResult{Kind: "success", Status: 200, State: fakeCodexTicketState(292)}, 292, false)
	}
	events := h.Events()
	require.Len(t, events, codexHarvestFlowCap)
	raw, err := json.Marshal(events)
	require.NoError(t, err)
	require.NotContains(t, string(raw), fakeCodexTicketState(292))
	require.NotContains(t, string(raw), "secret")
	require.Equal(t, "拿到合格门票（292 字节）", events[len(events)-1].Detail)
	require.Len(t, repo.events, 3+codexHarvestFlowCap+10)
}

func TestValidateCodexProbeResponse(t *testing.T) {
	require.NoError(t, validateCodexProbeResponse([]byte(codexHarvestCompletedBody)))
	require.NoError(t, validateCodexProbeResponse([]byte(`{"status":"completed"}`)))
	for name, body := range map[string]string{
		"done only":  "data: [DONE]\n\n",
		"error":      "data: {\"type\":\"error\",\"error\":{\"message\":\"x\"}}\n\n" + codexHarvestCompletedBody,
		"incomplete": "data: {\"type\":\"response.incomplete\"}\n\n",
		"bad json":   "data: {oops}\n\n",
		"empty":      "",
	} {
		require.Error(t, validateCodexProbeResponse([]byte(body)), name)
	}
}

func TestCodexHarvestRetryAfter(t *testing.T) {
	now := time.Now()
	require.Equal(t, 30*time.Second, codexHarvestRetryAfter("30", now))
	require.Equal(t, 24*time.Hour, codexHarvestRetryAfter("999999", now))
	require.Zero(t, codexHarvestRetryAfter("-1", now))
	require.Zero(t, codexHarvestRetryAfter("", now))
	future := now.Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	require.InDelta(t, float64(2*time.Minute), float64(codexHarvestRetryAfter(future, now)), float64(2*time.Second))
}

func TestClassifyCodexHarvestProbe(t *testing.T) {
	ok := fakeCodexTicketState(292)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for want, tc := range map[string]struct {
		ctx    context.Context
		result codexHarvestProbeResult
	}{
		"success":                      {context.Background(), codexHarvestProbeResult{Sent: true, Status: 200, State: ok}},
		"invalid_state":                {context.Background(), codexHarvestProbeResult{Sent: true, Status: 200, State: fakeCodexTicketState(312)}},
		"response_incomplete_or_error": {context.Background(), codexHarvestProbeResult{Sent: true, Status: 200, State: ok, Err: errors.New("x")}},
		"account_error":                {context.Background(), codexHarvestProbeResult{Sent: true, Status: 403}},
		"rate_limited":                 {context.Background(), codexHarvestProbeResult{Sent: true, Status: 429}},
		"upstream_error":               {context.Background(), codexHarvestProbeResult{Sent: true, Status: 503}},
		"network_error":                {context.Background(), codexHarvestProbeResult{Sent: true, Err: errors.New("dial")}},
		"not_sent":                     {context.Background(), codexHarvestProbeResult{}},
		"cancelled":                    {cancelled, codexHarvestProbeResult{Sent: true, Err: context.Canceled}},
	} {
		require.Equal(t, want, classifyCodexHarvestProbe(tc.ctx, 292, tc.result), want)
	}
}

func TestStartCodexHarvestManual(t *testing.T) {
	account := activeTicketAccount(41)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		codexHarvestResponse(http.StatusOK, fakeCodexTicketState(312)),
		codexHarvestResponse(http.StatusOK, fakeCodexTicketState(292)),
	}}
	svc, h, _ := harvestRoundService(t, upstream, []Account{account}, nil, testHarvestProxy(1), testHarvestProxy(2))

	_, err := svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{MaxAttempts: 21})
	require.True(t, infraerrors.IsBadRequest(err))
	_, err = svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{Models: []string{"gpt-4o"}})
	require.True(t, infraerrors.IsBadRequest(err))

	run, err := svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{MaxAttempts: 3, IntervalSeconds: 1})
	require.NoError(t, err)
	require.True(t, run.Running)
	require.Equal(t, []string{"gpt-6-astra"}, run.Models)
	require.Eventually(t, func() bool { return !h.ManualRuns()[41].Running }, 10*time.Second, 20*time.Millisecond)
	final := h.ManualRuns()[41]
	require.Equal(t, "harvested", final.Result)
	require.Equal(t, 2, final.Attempts)
	require.Equal(t, []string{"gpt-6-astra"}, final.Harvested)
	require.Len(t, upstream.requests, 2)
	manualEvents := 0
	for _, e := range h.Events() {
		if e.Manual {
			manualEvents++
		}
	}
	require.GreaterOrEqual(t, manualEvents, 2)
}

func TestStartCodexHarvestManual_Preconditions(t *testing.T) {
	account := activeTicketAccount(41)
	svc, _, _ := harvestRoundService(t, &httpUpstreamRecorder{}, []Account{account}, nil)
	_, err := svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{})
	require.ErrorIs(t, err, ErrCodexHarvestEmptyPool)

	svc, h, _ := harvestRoundService(t, &codexTicketFuncUpstream{do: func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}, []Account{account}, nil, testHarvestProxy(1))
	svc.cfg.Gateway.OpenAICodexTicket.Enabled = false
	_, err = svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{})
	require.ErrorIs(t, err, ErrCodexHarvestDisabled)
	svc.cfg.Gateway.OpenAICodexTicket.Enabled = true
	svc.StartOpenAICodexTicketHarvester()
	t.Cleanup(svc.StopOpenAICodexTicketHarvester)
	h.manualMu.Lock()
	h.manual[41] = &CodexHarvestManualRun{AccountID: 41, Running: true}
	h.manualMu.Unlock()
	_, err = svc.StartCodexHarvestManual(context.Background(), 41, CodexHarvestManualRequest{})
	require.ErrorIs(t, err, ErrCodexHarvestManualRunning)
}

func TestCodexHarvestSnapshot_NeverExposesStateOrCredentials(t *testing.T) {
	account := activeTicketAccount(41)
	account.Name = "pro-1"
	state := fakeCodexTicketState(292)
	now := time.Now()
	account.Extra = map[string]any{openAICodexTicketExtraKey("gpt-6-astra"): &openAICodexTicket{
		Model: "gpt-6-astra", State: state, Length: 292, CapturedAt: now.Add(-30 * time.Second), ExpiresAt: now.Add(time.Hour),
		HarvestProxyID: 1, HarvestProxyName: "harvest-1",
	}}
	account.Credentials["access_token"] = "secret-access-token"
	secretProxy := testHarvestProxy(1)
	secretProxy.Username, secretProxy.Password = "user", "proxy-secret"
	svc, h, _ := harvestRoundService(t, &httpUpstreamRecorder{}, []Account{account}, nil, secretProxy)
	svc.cfg.Gateway.OpenAICodexTicket.FailClosed = true
	svc.cfg.Gateway.OpenAICodexTicket.Models = []string{"gpt-6-astra", "gpt-5.6-sol"}
	h.setCooldown(41, "gpt-5.6-sol", now.Add(time.Minute))
	snapshot, err := svc.CodexHarvestSnapshot(context.Background())
	require.NoError(t, err)
	require.True(t, snapshot.Enabled)
	require.Len(t, snapshot.Pool, 1)
	require.True(t, snapshot.Pool[0].Usable)
	require.Len(t, snapshot.Accounts, 1)
	tickets := snapshot.Accounts[0].Tickets
	require.True(t, tickets[0].Ready)
	require.False(t, tickets[0].Blocked)
	require.Equal(t, "harvest-1", tickets[0].ProxyName)
	require.False(t, tickets[1].Ready)
	require.True(t, tickets[1].Blocked)
	require.NotNil(t, tickets[1].CooldownUntil)
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(raw), state)
	require.NotContains(t, string(raw), "proxy-secret")
	require.NotContains(t, string(raw), "secret-access-token")
}
