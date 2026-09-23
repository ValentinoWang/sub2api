package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	codexHarvestFlowCap         = 200
	codexHarvestFlowPersistWait = 1500 * time.Millisecond
)

// CodexHarvestFlowEvent is a redacted breadcrumb: it never carries ticket
// state, tokens or proxy credentials.
type CodexHarvestFlowEvent struct {
	ID          string    `json:"id"`
	At          time.Time `json:"at"`
	Stage       string    `json:"stage"`
	Kind        string    `json:"kind"`
	AccountID   int64     `json:"account_id,omitempty"`
	AccountName string    `json:"account_name,omitempty"`
	Model       string    `json:"model,omitempty"`
	ProxyID     int64     `json:"proxy_id,omitempty"`
	ProxyName   string    `json:"proxy_name,omitempty"`
	HTTPStatus  int       `json:"http_status,omitempty"`
	Length      int       `json:"length,omitempty"`
	Accepted    bool      `json:"accepted,omitempty"`
	Manual      bool      `json:"manual,omitempty"`
	Result      string    `json:"result,omitempty"`
	Detail      string    `json:"detail,omitempty"`
}

type CodexHarvestFlowRepository interface {
	List(context.Context, int) ([]CodexHarvestFlowEvent, error)
	Append(context.Context, CodexHarvestFlowEvent) error
}

type codexHarvestFlowRing struct {
	repo   CodexHarvestFlowRepository
	mu     sync.Mutex
	seq    atomic.Uint64
	events []CodexHarvestFlowEvent
}

func newCodexHarvestFlowRing(repo CodexHarvestFlowRepository) *codexHarvestFlowRing {
	return &codexHarvestFlowRing{repo: repo}
}

func (r *codexHarvestFlowRing) hydrate() {
	if r == nil || r.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	events, err := r.repo.List(ctx, codexHarvestFlowCap)
	cancel()
	if err != nil {
		logger.L().Warn("codex harvest flow hydrate failed", zap.Error(err))
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		r.events = append([]CodexHarvestFlowEvent(nil), events...)
	}
}

func (r *codexHarvestFlowRing) record(event CodexHarvestFlowEvent) {
	if r == nil {
		return
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	event.AccountName = clipCodexHarvestText(event.AccountName, 80)
	event.Model = clipCodexHarvestText(event.Model, 64)
	event.ProxyName = clipCodexHarvestText(event.ProxyName, 160)
	event.Result = clipCodexHarvestText(event.Result, 64)
	event.Detail = clipCodexHarvestText(event.Detail, 240)
	r.mu.Lock()
	if event.ID == "" {
		event.ID = fmt.Sprintf("%d-%d", event.At.UnixNano(), r.seq.Add(1))
	}
	r.events = append(r.events, event)
	if overflow := len(r.events) - codexHarvestFlowCap; overflow > 0 {
		r.events = append([]CodexHarvestFlowEvent(nil), r.events[overflow:]...)
	}
	r.mu.Unlock()
	if r.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), codexHarvestFlowPersistWait)
	defer cancel()
	if err := r.repo.Append(ctx, event); err != nil {
		logger.L().Warn("codex harvest flow persist failed", zap.Error(err))
	}
}

func (r *codexHarvestFlowRing) list() []CodexHarvestFlowEvent {
	if r == nil {
		return []CodexHarvestFlowEvent{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]CodexHarvestFlowEvent, len(r.events))
	copy(out, r.events)
	return out
}

func clipCodexHarvestText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func (s *CodexHarvestService) Events() []CodexHarvestFlowEvent {
	if s == nil {
		return []CodexHarvestFlowEvent{}
	}
	return s.flow.list()
}

func (s *CodexHarvestService) recordProbe(account *Account, model string, proxy *Proxy, result codexHarvestProbeResult, targetLength int, manual bool) {
	if s == nil {
		return
	}
	kind := "probe_miss"
	if result.Kind == "success" {
		kind = "probe_hit"
	}
	event := CodexHarvestFlowEvent{
		Stage:      "probe",
		Kind:       kind,
		Model:      model,
		HTTPStatus: result.Status,
		Length:     len(result.State),
		Accepted:   result.Kind == "success",
		Manual:     manual,
		Result:     result.Kind,
		Detail:     describeCodexHarvestOutcome(result, targetLength),
	}
	if account != nil {
		event.AccountID, event.AccountName = account.ID, account.Name
	}
	if proxy != nil {
		event.ProxyID, event.ProxyName = proxy.ID, proxy.Name
	}
	s.flow.record(event)
}

func (s *CodexHarvestService) recordSkip(account *Account, model, result, detail string, manual bool) {
	if s == nil {
		return
	}
	event := CodexHarvestFlowEvent{Stage: "probe", Kind: "skip", Model: model, Manual: manual, Result: result, Detail: detail}
	if account != nil {
		event.AccountID, event.AccountName = account.ID, account.Name
	}
	s.flow.record(event)
}

func (s *CodexHarvestService) recordTicket(account *Account, ticket *openAICodexTicket, manual bool) {
	if s == nil || ticket == nil {
		return
	}
	event := CodexHarvestFlowEvent{
		Stage:     "ticket",
		Kind:      "accept",
		Model:     ticket.Model,
		ProxyID:   ticket.HarvestProxyID,
		ProxyName: ticket.HarvestProxyName,
		Length:    ticket.Length,
		Accepted:  true,
		Manual:    manual,
		Result:    "stored",
		Detail:    fmt.Sprintf("合格门票 %d 字节，已入库", ticket.Length),
	}
	if account != nil {
		event.AccountID, event.AccountName = account.ID, account.Name
	}
	s.flow.record(event)
}

// describeCodexHarvestOutcome explains a probe result in operator terms.
func describeCodexHarvestOutcome(result codexHarvestProbeResult, targetLength int) string {
	length := len(result.State)
	switch result.Kind {
	case "success":
		return fmt.Sprintf("拿到合格门票（%d 字节）", length)
	case "invalid_state":
		if length == 312 || length == 356 {
			return fmt.Sprintf("拿到降级票据（%d 字节），已拒收，换代理重试", length)
		}
		if length == 0 {
			return "响应没有返回门票"
		}
		return fmt.Sprintf("门票不合格（%d 字节，合格应为 %d 字节）", length, targetLength)
	case "rate_limited":
		if result.RetryAfter > 0 {
			return fmt.Sprintf("上游限流，%d 秒后再试", int(result.RetryAfter/time.Second))
		}
		return "上游限流，进入冷却后再试"
	case "account_error":
		return fmt.Sprintf("账号被上游拒绝（HTTP %d），本轮不再为该账号打票", result.Status)
	case "network_error":
		return "代理或网络连接失败"
	case "upstream_error":
		if result.Status == http.StatusServiceUnavailable || result.Status == http.StatusBadGateway {
			return fmt.Sprintf("上游暂时不可用（HTTP %d）", result.Status)
		}
		return fmt.Sprintf("上游返回 HTTP %d", result.Status)
	case "response_incomplete_or_error":
		return "响应没有正常完成，门票不入库"
	case "token_error":
		return "获取账号访问令牌失败"
	default:
		return result.Kind
	}
}
