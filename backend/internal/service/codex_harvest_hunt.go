package service

import (
	"context"
	"maps"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const codexHarvestDefaultRoundInterval = 20 * time.Second

// codexHarvestRound is the per-round request budget, charged only when a
// request actually leaves the process.
type codexHarvestRound struct {
	limit   int
	used    atomic.Int64
	stopped sync.Map
}

func (r *codexHarvestRound) Used() int { return int(r.used.Load()) }

func (r *codexHarvestRound) Exhausted() bool { return r.Used() >= r.limit }

func (r *codexHarvestRound) Take() bool {
	for {
		used := r.used.Load()
		if used >= int64(r.limit) {
			return false
		}
		if r.used.CompareAndSwap(used, used+1) {
			return true
		}
	}
}

func (r *codexHarvestRound) Stop(accountID int64) { r.stopped.Store(accountID, struct{}{}) }

func (r *codexHarvestRound) AccountStopped(accountID int64) bool {
	_, stopped := r.stopped.Load(accountID)
	return stopped
}

type codexHarvestHuntOptions struct {
	round       *codexHarvestRound
	maxAttempts int
	manual      bool
	tried       map[int64]bool
}

type codexHarvestHuntResult struct {
	Harvested bool
	Sent      int
	Last      codexHarvestProbeResult
	Stopped   string
}

// SetCodexHarvestService attaches the proxy pool, controls and learning store.
// Without it the harvester stays idle: there is no other source of egress.
func (s *OpenAIGatewayService) SetCodexHarvestService(h *CodexHarvestService) {
	if s == nil {
		return
	}
	s.codexHarvest.Store(h)
	if h != nil {
		h.Wake()
	}
}

func (s *OpenAIGatewayService) codexHarvestService() *CodexHarvestService {
	if s == nil {
		return nil
	}
	return s.codexHarvest.Load()
}

func (s *OpenAIGatewayService) openAICodexTicketHarvestLoop(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		var wake <-chan struct{}
		if h := s.codexHarvestService(); h != nil {
			wake = h.wake
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(0)
		case <-timer.C:
			s.refreshOpenAICodexTickets(ctx)
			timer.Reset(s.armCodexHarvestInterval(ctx))
		}
	}
}

func (s *OpenAIGatewayService) armCodexHarvestInterval(ctx context.Context) time.Duration {
	h := s.codexHarvestService()
	if h == nil {
		return codexHarvestDefaultRoundInterval
	}
	controls, _, _ := h.Controls(ctx)
	delay := time.Duration(controls.Speed.RoundIntervalSeconds) * time.Second
	if delay <= 0 {
		delay = codexHarvestDefaultRoundInterval
	}
	next := time.Now().Add(delay)
	h.setRuntime(func(r *CodexHarvestRuntime) { r.NextRoundAt = &next })
	return delay
}

type codexHarvestPair struct {
	account Account
	model   string
}

// refreshOpenAICodexTickets runs one bounded round: schedulable accounts first,
// each tier with its own rotating cursor, probes strictly one at a time.
func (s *OpenAIGatewayService) refreshOpenAICodexTickets(ctx context.Context) {
	h := s.codexHarvestService()
	if h == nil || s.accountRepo == nil || s.httpUpstream == nil || ctx.Err() != nil {
		return
	}
	if !s.openAICodexTicketEnabledContext(ctx) {
		h.setRuntime(func(r *CodexHarvestRuntime) { r.IdleReason = "disabled" })
		return
	}
	if !h.roundActive.CompareAndSwap(false, true) {
		return
	}
	defer h.roundActive.Store(false)
	controls, _, _ := h.Controls(ctx)
	now := time.Now()
	pool, _, err := h.Pool(ctx, controls.ProxyIDs, now)
	if err != nil {
		logger.L().Warn("openai_codex_ticket harvest pool unavailable", zap.Error(err))
		h.setRuntime(func(r *CodexHarvestRuntime) { r.IdleReason = "pool_unavailable" })
		return
	}
	if len(pool) == 0 {
		h.setRuntime(func(r *CodexHarvestRuntime) { r.IdleReason = "empty_pool" })
		return
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		logger.L().Warn("openai_codex_ticket list accounts failed", zap.Error(err))
		return
	}
	cfg := s.openAICodexTicketConfig()
	refreshBefore := time.Duration(cfg.RefreshBeforeSeconds) * time.Second
	tiers := [2][]codexHarvestPair{}
	seen := make(map[int64]bool, len(accounts))
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	for i := range accounts {
		account := accounts[i]
		if seen[account.ID] || account.Status != StatusActive || !isOpenAICodexTicketAccount(&account) {
			continue
		}
		seen[account.ID] = true
		tier := 1
		if account.IsSchedulable() {
			tier = 0
		}
		for _, model := range cfg.Models {
			model = normalizeOpenAICodexTicketModel(model)
			if model == "" {
				continue
			}
			if _, cooling := h.coolingDown(account.ID, model, now); cooling {
				continue
			}
			if t := s.lookupOpenAICodexTicket(&account, model); t.valid(now, cfg.TargetLength) && !t.needsRefresh(now, refreshBefore) {
				continue
			}
			tiers[tier] = append(tiers[tier], codexHarvestPair{account: account, model: model})
		}
	}
	round := &codexHarvestRound{limit: controls.Speed.MaxRequestsPerRound}
	h.setRuntime(func(r *CodexHarvestRuntime) {
		r.Running, r.RequestsUsed, r.RequestBudget, r.NextRoundAt, r.IdleReason = true, 0, round.limit, nil, ""
	})
	defer func() {
		finished := time.Now()
		h.setRuntime(func(r *CodexHarvestRuntime) {
			r.Running, r.CurrentProxy, r.LastRoundAt = false, "", &finished
			if tiers[0] == nil && tiers[1] == nil {
				r.IdleReason = "all_ready"
			} else if round.Exhausted() {
				r.IdleReason = "round_budget"
			}
		})
	}()
	for tier, pairs := range tiers {
		if len(pairs) == 0 {
			continue
		}
		cursor := &h.tierCursors[tier]
		start := int(cursor.Load() % uint64(len(pairs)))
		for offset := 0; offset < len(pairs) && ctx.Err() == nil && !round.Exhausted(); offset++ {
			index := (start + offset) % len(pairs)
			cursor.Store(uint64((index + 1) % len(pairs)))
			pair := pairs[index]
			if round.AccountStopped(pair.account.ID) {
				continue
			}
			acc := pair.account
			// Token/header helpers may update account metadata; each probe owns its maps.
			acc.Extra = maps.Clone(pair.account.Extra)
			acc.Credentials = maps.Clone(pair.account.Credentials)
			s.huntCodexHarvestTicket(ctx, h, &acc, pair.model, pool, controls, codexHarvestHuntOptions{
				round:       round,
				maxAttempts: controls.Speed.MaxProxyAttempts,
			})
		}
	}
	if used := round.Used(); used > 0 {
		logger.L().Info("openai_codex_ticket probe cycle", zap.Int("probed", used), zap.Int("budget", round.limit))
	}
}

// huntCodexHarvestTicket tries up to opts.maxAttempts proxies for one
// account/model. Singleflight covers the whole hunt so a manual run and the
// background round never probe the same pair at the same time.
func (s *OpenAIGatewayService) huntCodexHarvestTicket(ctx context.Context, h *CodexHarvestService, account *Account, model string, pool []Proxy, controls CodexHarvestControls, opts codexHarvestHuntOptions) codexHarvestHuntResult {
	key := openAICodexTicketKey(account.ID, model)
	value, _, _ := s.openaiCodexTicketFlight.Do(key, func() (any, error) {
		return s.huntCodexHarvestTicketLocked(ctx, h, account, model, pool, controls, opts), nil
	})
	result, _ := value.(codexHarvestHuntResult)
	return result
}

func (s *OpenAIGatewayService) huntCodexHarvestTicketLocked(ctx context.Context, h *CodexHarvestService, account *Account, model string, pool []Proxy, controls CodexHarvestControls, opts codexHarvestHuntOptions) (out codexHarvestHuntResult) {
	cfg := s.openAICodexTicketConfig()
	if opts.tried == nil {
		opts.tried = map[int64]bool{}
	}
	defer h.setRuntime(func(r *CodexHarvestRuntime) { r.CurrentProxy = "" })
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil || strings.TrimSpace(token) == "" {
		out.Last = codexHarvestProbeResult{Kind: "token_error", Err: err}
		h.recordProbe(account, model, nil, out.Last, cfg.TargetLength, opts.manual)
		h.setCooldown(account.ID, model, time.Now().Add(time.Duration(controls.Speed.CooldownSeconds)*time.Second))
		return out
	}
	scope := CodexHarvestNodeScope{AccountID: account.ID, Identity: codexHarvestIdentity(account), Model: model}
	var generation int64
	var records []CodexHarvestNodeRecord
	learning := h.nodes != nil
	if learning {
		query, cancel := context.WithTimeout(ctx, 5*time.Second)
		generation, records, err = h.nodes.Snapshot(query, scope)
		cancel()
		if err != nil {
			learning = false
			logger.L().Warn("openai_codex_ticket harvest learning unavailable", zap.Error(err))
		}
	}
	for attempt := 0; attempt < opts.maxAttempts && ctx.Err() == nil; attempt++ {
		if opts.round != nil && (opts.round.Exhausted() || opts.round.AccountStopped(account.ID)) {
			break
		}
		proxy, reason, ok := h.chooseHarvestProxy(s.lookupOpenAICodexTicket(account, model), pool, records, opts.tried, time.Now())
		if !ok {
			out.Stopped = "no_proxy"
			break
		}
		opts.tried[proxy.ID] = true
		h.setRuntime(func(r *CodexHarvestRuntime) { r.CurrentProxy, r.SelectionReason = proxy.Name, reason })
		if !h.waitHarvestPace(ctx, controls) {
			break
		}
		started := time.Now()
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(controls.Speed.AttemptTimeoutSeconds)*time.Second)
		result := s.requestCodexHarvestProbe(attemptCtx, account, token, model, proxy.URL(), func() bool {
			return h.reserveHarvestRequest(account.ID, controls, opts)
		})
		// An attempt deadline is a slow proxy, not a cancellation: classify against the parent.
		result.Kind = classifyCodexHarvestProbe(ctx, cfg.TargetLength, result)
		cancel()
		if !result.Sent {
			if result.Kind == "not_sent" {
				out.Stopped = "budget"
			}
			break
		}
		out.Sent++
		out.Last = result
		if learning && result.Kind != "cancelled" {
			query, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if _, err := h.nodes.Record(query, CodexHarvestNodeFeedback{
				Scope: scope, ProxyID: proxy.ID, ProxyName: proxy.Name, Generation: generation,
				Result: result.Kind, LatencyMS: time.Since(started).Milliseconds(), CooldownSeconds: controls.Speed.CooldownSeconds,
			}); err != nil {
				logger.L().Warn("openai_codex_ticket harvest learning feedback failed", zap.Error(err))
			}
			cancel()
		}
		if result.Kind == "cancelled" {
			break
		}
		h.recordProbe(account, model, &proxy, result, cfg.TargetLength, opts.manual)
		if result.Kind == "success" {
			now := time.Now()
			ticket := &openAICodexTicket{
				AccountID:        account.ID,
				Model:            model,
				State:            result.State,
				Length:           len(result.State),
				CapturedAt:       now,
				ExpiresAt:        now.Add(time.Duration(cfg.TTLSeconds) * time.Second),
				Attempts:         out.Sent,
				HarvestProxyID:   proxy.ID,
				HarvestProxyName: proxy.Name,
			}
			s.storeOpenAICodexTicket(ctx, account, ticket)
			h.clearCooldown(account.ID, model)
			h.recordTicket(account, ticket, opts.manual)
			logger.L().Info("openai_codex_ticket harvested", zap.Int64("account_id", account.ID),
				zap.String("model", model), zap.Int64("proxy_id", proxy.ID), zap.Int("attempts", out.Sent))
			out.Harvested = true
			return out
		}
		if result.Kind == "account_error" || result.Kind == "rate_limited" {
			if opts.round != nil {
				opts.round.Stop(account.ID)
			}
			out.Stopped = result.Kind
			break
		}
		h.setRuntime(func(r *CodexHarvestRuntime) { r.SelectionReason = "switch_after_" + result.Kind })
	}
	if out.Sent > 0 && ctx.Err() == nil {
		cooldown := time.Duration(controls.Speed.CooldownSeconds) * time.Second
		if out.Last.RetryAfter > cooldown {
			cooldown = out.Last.RetryAfter
		}
		h.setCooldown(account.ID, model, time.Now().Add(cooldown))
		logger.L().Info("openai_codex_ticket probe miss", zap.Int64("account_id", account.ID), zap.String("model", model),
			zap.String("reason", out.Last.Kind), zap.Int("http", out.Last.Status), zap.Int("attempts", out.Sent))
	}
	return out
}

// chooseHarvestProxy prefers the proxy that issued the current ticket, then
// the learning order. Only pool members are ever returned.
func (h *CodexHarvestService) chooseHarvestProxy(current *openAICodexTicket, pool []Proxy, records []CodexHarvestNodeRecord, tried map[int64]bool, now time.Time) (Proxy, string, bool) {
	if current != nil && current.HarvestProxyID > 0 && !tried[current.HarvestProxyID] {
		for _, p := range pool {
			if p.ID == current.HarvestProxyID {
				return p, "ticket_sticky", true
			}
		}
	}
	ranked := rankCodexHarvestProxies(pool, records, tried, h.explore.Add(1)-1, now)
	if len(ranked) == 0 {
		return Proxy{}, "", false
	}
	reason := "explore"
	for _, r := range records {
		if r.ProxyID == ranked[0].ID && r.LastSuccess != nil && r.LastSuccess.After(now.Add(-codexHarvestRecentSuccess)) {
			reason = "recent_success"
		}
	}
	return ranked[0], reason, true
}

// waitHarvestPace keeps the configured minimum gap between any two probes.
func (h *CodexHarvestService) waitHarvestPace(ctx context.Context, controls CodexHarvestControls) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		h.runtimeMu.Lock()
		delay := time.Until(h.lastRequest.Add(time.Duration(controls.Speed.ProbeIntervalSeconds) * time.Second))
		h.runtimeMu.Unlock()
		if delay <= 0 {
			return true
		}
		timer := time.NewTimer(min(delay, 250*time.Millisecond))
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

func (h *CodexHarvestService) reserveHarvestRequest(accountID int64, controls CodexHarvestControls, opts codexHarvestHuntOptions) bool {
	if opts.round != nil && (opts.round.AccountStopped(accountID) || opts.round.Exhausted()) {
		return false
	}
	now := time.Now()
	if !h.takeAccountHour(accountID, controls.Speed.MaxRequestsPerAccountHour, now) {
		return false
	}
	if opts.round != nil && !opts.round.Take() {
		h.refundAccountHour(accountID, now)
		return false
	}
	h.runtimeMu.Lock()
	h.lastRequest = now
	if opts.round != nil {
		h.runtime.RequestsUsed = opts.round.Used()
	}
	h.runtimeMu.Unlock()
	return true
}

func (h *CodexHarvestService) refundAccountHour(accountID int64, at time.Time) {
	h.hourMu.Lock()
	defer h.hourMu.Unlock()
	sent := h.hour[accountID]
	for i := len(sent) - 1; i >= 0; i-- {
		if sent[i].Equal(at) {
			h.hour[accountID] = append(sent[:i], sent[i+1:]...)
			return
		}
	}
}

// CodexHarvestManualRequest bounds one admin-triggered run for one account.
type CodexHarvestManualRequest struct {
	Models          []string `json:"models"`
	MaxAttempts     int      `json:"max_attempts"`
	IntervalSeconds int      `json:"interval_seconds"`
}

type CodexHarvestManualRun struct {
	AccountID  int64      `json:"account_id"`
	Models     []string   `json:"models"`
	Running    bool       `json:"running"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Attempts   int        `json:"attempts"`
	Harvested  []string   `json:"harvested"`
	Result     string     `json:"result,omitempty"`
}

var (
	ErrCodexHarvestDisabled       = infraerrors.Conflict("CODEX_HARVEST_DISABLED", "codex ticket harvesting is disabled")
	ErrCodexHarvestEmptyPool      = infraerrors.Conflict("CODEX_HARVEST_EMPTY_POOL", "harvest proxy pool has no usable proxy")
	ErrCodexHarvestManualRunning  = infraerrors.Conflict("CODEX_HARVEST_MANUAL_RUNNING", "a manual harvest is already running for this account")
	ErrCodexHarvestAccountInvalid = infraerrors.BadRequest("CODEX_HARVEST_ACCOUNT_INVALID", "account cannot hold codex tickets")
	ErrCodexHarvestUnavailable    = infraerrors.New(http.StatusServiceUnavailable, "CODEX_HARVEST_UNAVAILABLE", "codex harvest service unavailable")
)

const (
	codexHarvestManualMaxAttempts     = 20
	codexHarvestManualDefaultAttempts = 6
	codexHarvestManualMaxInterval     = 60
	codexHarvestManualDefaultInterval = 5
	codexHarvestManualMaxDuration     = 15 * time.Minute
)

func (s *OpenAIGatewayService) normalizeCodexHarvestManualRequest(req CodexHarvestManualRequest) (CodexHarvestManualRequest, error) {
	if req.MaxAttempts == 0 {
		req.MaxAttempts = codexHarvestManualDefaultAttempts
	}
	if req.IntervalSeconds == 0 {
		req.IntervalSeconds = codexHarvestManualDefaultInterval
	}
	if req.MaxAttempts < 1 || req.MaxAttempts > codexHarvestManualMaxAttempts {
		return req, infraerrors.BadRequest("INVALID_CODEX_HARVEST_MANUAL", "max_attempts must be between 1 and 20")
	}
	if req.IntervalSeconds < 1 || req.IntervalSeconds > codexHarvestManualMaxInterval {
		return req, infraerrors.BadRequest("INVALID_CODEX_HARVEST_MANUAL", "interval_seconds must be between 1 and 60")
	}
	configured := s.openAICodexTicketConfig().Models
	if len(req.Models) == 0 {
		req.Models = append([]string(nil), configured...)
	}
	models := make([]string, 0, len(req.Models))
	for _, model := range req.Models {
		model = normalizeOpenAICodexTicketModel(model)
		known := false
		for _, item := range configured {
			if normalizeOpenAICodexTicketModel(item) == model {
				known = true
			}
		}
		if !known {
			return req, infraerrors.BadRequest("INVALID_CODEX_HARVEST_MANUAL", "model is not a ticket-gated model")
		}
		if !containsString(models, model) {
			models = append(models, model)
		}
	}
	req.Models = models
	return req, nil
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// StartCodexHarvestManual starts a bounded background run for one account and
// returns immediately; progress is visible in the flow list and run status.
func (s *OpenAIGatewayService) StartCodexHarvestManual(ctx context.Context, accountID int64, req CodexHarvestManualRequest) (CodexHarvestManualRun, error) {
	h := s.codexHarvestService()
	if h == nil || s.accountRepo == nil || s.httpUpstream == nil {
		return CodexHarvestManualRun{}, ErrCodexHarvestUnavailable
	}
	if !s.openAICodexTicketEnabledContext(ctx) {
		return CodexHarvestManualRun{}, ErrCodexHarvestDisabled
	}
	req, err := s.normalizeCodexHarvestManualRequest(req)
	if err != nil {
		return CodexHarvestManualRun{}, err
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return CodexHarvestManualRun{}, err
	}
	if account == nil || account.Status != StatusActive || !isOpenAICodexTicketAccount(account) {
		return CodexHarvestManualRun{}, ErrCodexHarvestAccountInvalid
	}
	controls, _, _ := h.Controls(ctx)
	pool, _, err := h.Pool(ctx, controls.ProxyIDs, time.Now())
	if err != nil {
		return CodexHarvestManualRun{}, err
	}
	if len(pool) == 0 {
		return CodexHarvestManualRun{}, ErrCodexHarvestEmptyPool
	}
	run := &CodexHarvestManualRun{AccountID: accountID, Models: req.Models, Running: true, StartedAt: time.Now(), Harvested: []string{}}
	h.manualMu.Lock()
	if current := h.manual[accountID]; current != nil && current.Running {
		h.manualMu.Unlock()
		return CodexHarvestManualRun{}, ErrCodexHarvestManualRunning
	}
	h.manual[accountID] = run
	snapshot := *run
	h.manualMu.Unlock()
	runCtx, cancel := context.WithTimeout(s.codexHarvestBaseContext(), codexHarvestManualMaxDuration)
	go func() {
		defer cancel()
		s.runCodexHarvestManual(runCtx, h, account, req, pool, controls, run)
	}()
	return snapshot, nil
}

func (s *OpenAIGatewayService) runCodexHarvestManual(ctx context.Context, h *CodexHarvestService, account *Account, req CodexHarvestManualRequest, pool []Proxy, controls CodexHarvestControls, run *CodexHarvestManualRun) {
	result := "exhausted"
	defer func() {
		finished := time.Now()
		h.manualMu.Lock()
		run.Running, run.FinishedAt = false, &finished
		if ctx.Err() != nil && result != "harvested" {
			result = "cancelled"
		}
		run.Result = result
		h.manualMu.Unlock()
	}()
	harvested := 0
	for _, model := range req.Models {
		tried := map[int64]bool{}
		for attempt := 0; attempt < req.MaxAttempts && ctx.Err() == nil; attempt++ {
			if len(tried) >= len(pool) {
				tried = map[int64]bool{}
			}
			acc := *account
			acc.Extra = maps.Clone(account.Extra)
			acc.Credentials = maps.Clone(account.Credentials)
			hunt := s.huntCodexHarvestTicket(ctx, h, &acc, model, pool, controls, codexHarvestHuntOptions{
				maxAttempts: 1, manual: true, tried: tried,
			})
			h.manualMu.Lock()
			run.Attempts += hunt.Sent
			if hunt.Harvested {
				run.Harvested = append(run.Harvested, model)
			}
			h.manualMu.Unlock()
			if hunt.Harvested {
				harvested++
				break
			}
			if hunt.Stopped == "budget" {
				h.recordSkip(account, model, "hour_budget", "每小时打票次数已用完，手动打票停止", true)
				result = "hour_budget"
				return
			}
			if hunt.Stopped == "account_error" {
				result = "account_error"
				return
			}
			if hunt.Sent == 0 && hunt.Stopped == "no_proxy" {
				h.recordSkip(account, model, "no_proxy", "打票代理都在冷却中，手动打票停止", true)
				result = "no_proxy"
				return
			}
			if attempt+1 < req.MaxAttempts {
				wait := time.Duration(req.IntervalSeconds) * time.Second
				if hunt.Last.RetryAfter > wait {
					wait = hunt.Last.RetryAfter
				}
				timer := time.NewTimer(wait)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}
	}
	if harvested == len(req.Models) {
		result = "harvested"
	} else if harvested > 0 {
		result = "partial"
	}
}

// ManualRuns returns the latest manual run per account.
func (h *CodexHarvestService) ManualRuns() map[int64]CodexHarvestManualRun {
	h.manualMu.Lock()
	defer h.manualMu.Unlock()
	out := make(map[int64]CodexHarvestManualRun, len(h.manual))
	for id, run := range h.manual {
		copyRun := *run
		copyRun.Models = append([]string(nil), run.Models...)
		copyRun.Harvested = append([]string(nil), run.Harvested...)
		out[id] = copyRun
	}
	return out
}
