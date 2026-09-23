package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// codexHarvestControlsKey stores the harvest speed and the managed proxy pool.
const codexHarvestControlsKey = "openai_codex_harvest_controls_v1"

const (
	codexHarvestPresetStandard = "standard"
	codexHarvestPresetCustom   = "custom"
	codexHarvestMaxPoolSize    = 200
	codexHarvestControlsTTL    = 3 * time.Second
	codexHarvestAccountWindow  = time.Hour
)

// CodexHarvestSpeed bounds how often and how hard the background harvester
// probes. Tickets refresh at 150s and stop being injected at 210s, so every
// preset keeps a round well inside that 60s window.
type CodexHarvestSpeed struct {
	RoundIntervalSeconds      int `json:"round_interval_seconds"`
	ProbeIntervalSeconds      int `json:"probe_interval_seconds"`
	AttemptTimeoutSeconds     int `json:"attempt_timeout_seconds"`
	CooldownSeconds           int `json:"cooldown_seconds"`
	MaxRequestsPerRound       int `json:"max_requests_per_round"`
	MaxProxyAttempts          int `json:"max_proxy_attempts"`
	MaxRequestsPerAccountHour int `json:"max_requests_per_account_hour"`
}

type CodexHarvestBound struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type CodexHarvestControls struct {
	Version  int               `json:"version"`
	Preset   string            `json:"preset"`
	Speed    CodexHarvestSpeed `json:"speed"`
	ProxyIDs []int64           `json:"proxy_ids"`
}

type CodexHarvestRuntime struct {
	Running         bool       `json:"running"`
	NextRoundAt     *time.Time `json:"next_round_at,omitempty"`
	LastRoundAt     *time.Time `json:"last_round_at,omitempty"`
	RequestsUsed    int        `json:"requests_used"`
	RequestBudget   int        `json:"request_budget"`
	CurrentProxy    string     `json:"current_proxy,omitempty"`
	SelectionReason string     `json:"selection_reason,omitempty"`
	IdleReason      string     `json:"idle_reason,omitempty"`
}

type CodexHarvestService struct {
	nodes    CodexHarvestNodeRepository
	settings SettingRepository
	proxies  ProxyRepository
	flow     *codexHarvestFlowRing

	configMu    sync.Mutex
	current     CodexHarvestControls
	configured  bool
	configErr   error
	loadedUntil time.Time

	wake chan struct{}

	runtimeMu   sync.Mutex
	runtime     CodexHarvestRuntime
	lastRequest time.Time

	roundActive atomic.Bool
	tierCursors [2]atomic.Uint64
	explore     atomic.Uint64
	cooldowns   sync.Map // openAICodexTicketKey → time.Time

	hourMu sync.Mutex
	hour   map[int64][]time.Time

	manualMu sync.Mutex
	manual   map[int64]*CodexHarvestManualRun
}

var codexHarvestPresetOrder = []string{"slow", codexHarvestPresetStandard, "fast", "burst"}

func CodexHarvestSpeedPresets() map[string]CodexHarvestSpeed {
	return map[string]CodexHarvestSpeed{
		"slow": {
			RoundIntervalSeconds: 45, ProbeIntervalSeconds: 5, AttemptTimeoutSeconds: 25, CooldownSeconds: 120,
			MaxRequestsPerRound: 4, MaxProxyAttempts: 2, MaxRequestsPerAccountHour: 30,
		},
		codexHarvestPresetStandard: {
			RoundIntervalSeconds: 20, ProbeIntervalSeconds: 2, AttemptTimeoutSeconds: 25, CooldownSeconds: 60,
			MaxRequestsPerRound: 8, MaxProxyAttempts: 3, MaxRequestsPerAccountHour: 60,
		},
		"fast": {
			RoundIntervalSeconds: 10, ProbeIntervalSeconds: 1, AttemptTimeoutSeconds: 20, CooldownSeconds: 30,
			MaxRequestsPerRound: 16, MaxProxyAttempts: 3, MaxRequestsPerAccountHour: 90,
		},
		"burst": {
			RoundIntervalSeconds: 5, ProbeIntervalSeconds: 0, AttemptTimeoutSeconds: 15, CooldownSeconds: 10,
			MaxRequestsPerRound: 30, MaxProxyAttempts: 5, MaxRequestsPerAccountHour: 180,
		},
	}
}

var codexHarvestSpeedBounds = []struct {
	name     string
	min, max int
	value    func(CodexHarvestSpeed) int
}{
	{"round_interval_seconds", 5, 600, func(s CodexHarvestSpeed) int { return s.RoundIntervalSeconds }},
	{"probe_interval_seconds", 0, 60, func(s CodexHarvestSpeed) int { return s.ProbeIntervalSeconds }},
	{"attempt_timeout_seconds", 5, 60, func(s CodexHarvestSpeed) int { return s.AttemptTimeoutSeconds }},
	{"cooldown_seconds", 5, 3600, func(s CodexHarvestSpeed) int { return s.CooldownSeconds }},
	{"max_requests_per_round", 1, 100, func(s CodexHarvestSpeed) int { return s.MaxRequestsPerRound }},
	{"max_proxy_attempts", 1, 10, func(s CodexHarvestSpeed) int { return s.MaxProxyAttempts }},
	{"max_requests_per_account_hour", 1, 600, func(s CodexHarvestSpeed) int { return s.MaxRequestsPerAccountHour }},
}

func CodexHarvestSpeedBounds() map[string]CodexHarvestBound {
	out := make(map[string]CodexHarvestBound, len(codexHarvestSpeedBounds))
	for _, field := range codexHarvestSpeedBounds {
		out[field.name] = CodexHarvestBound{Min: field.min, Max: field.max}
	}
	return out
}

func DefaultCodexHarvestControls() CodexHarvestControls {
	return CodexHarvestControls{
		Version:  1,
		Preset:   codexHarvestPresetStandard,
		Speed:    CodexHarvestSpeedPresets()[codexHarvestPresetStandard],
		ProxyIDs: []int64{},
	}
}

// ValidateCodexHarvestControls checks shape only; proxy existence is checked on save.
func ValidateCodexHarvestControls(v CodexHarvestControls) error {
	invalid := func(format string, args ...any) error {
		return infraerrors.BadRequest("INVALID_CODEX_HARVEST_CONTROLS", fmt.Sprintf(format, args...))
	}
	if v.Version != 1 {
		return invalid("unsupported harvest settings version")
	}
	if v.Preset != codexHarvestPresetCustom && !slices.Contains(codexHarvestPresetOrder, v.Preset) {
		return invalid("unknown harvest speed preset")
	}
	if preset, ok := CodexHarvestSpeedPresets()[v.Preset]; ok && preset != v.Speed {
		return invalid("harvest speed does not match the selected preset")
	}
	for _, field := range codexHarvestSpeedBounds {
		value := field.value(v.Speed)
		if value < field.min || value > field.max {
			return invalid("%s must be between %d and %d", field.name, field.min, field.max)
		}
	}
	if len(v.ProxyIDs) > codexHarvestMaxPoolSize {
		return invalid("harvest proxy pool is limited to %d proxies", codexHarvestMaxPoolSize)
	}
	seen := make(map[int64]struct{}, len(v.ProxyIDs))
	for _, id := range v.ProxyIDs {
		if id <= 0 {
			return invalid("harvest proxy IDs must be positive")
		}
		if _, dup := seen[id]; dup {
			return invalid("harvest proxy IDs must be unique")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func NewCodexHarvestService(nodes CodexHarvestNodeRepository, flows CodexHarvestFlowRepository, settings SettingRepository, proxies ProxyRepository) *CodexHarvestService {
	s := &CodexHarvestService{
		nodes:    nodes,
		settings: settings,
		proxies:  proxies,
		flow:     newCodexHarvestFlowRing(flows),
		current:  DefaultCodexHarvestControls(),
		wake:     make(chan struct{}, 1),
		hour:     make(map[int64][]time.Time),
		manual:   make(map[int64]*CodexHarvestManualRun),
	}
	s.flow.hydrate()
	return s
}

func ProvideCodexHarvestService(nodes CodexHarvestNodeRepository, flows CodexHarvestFlowRepository, settings SettingRepository, proxies ProxyRepository) *CodexHarvestService {
	return NewCodexHarvestService(nodes, flows, settings, proxies)
}

// Controls returns the saved controls, or the standard preset with an empty
// pool when nothing has been saved. A read failure keeps the last valid value.
func (s *CodexHarvestService) Controls(ctx context.Context) (CodexHarvestControls, bool, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if time.Now().Before(s.loadedUntil) {
		return cloneCodexHarvestControls(s.current), s.configured, s.configErr
	}
	v, configured, err := s.readControls(ctx)
	if err == nil {
		s.current, s.configured = v, configured
	}
	s.configErr = err
	s.loadedUntil = time.Now().Add(codexHarvestControlsTTL)
	return cloneCodexHarvestControls(s.current), s.configured, err
}

func (s *CodexHarvestService) readControls(ctx context.Context) (CodexHarvestControls, bool, error) {
	if s.settings == nil {
		return DefaultCodexHarvestControls(), false, nil
	}
	query, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, err := s.settings.GetValue(query, codexHarvestControlsKey)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && raw == "") {
		return DefaultCodexHarvestControls(), false, nil
	}
	if err != nil {
		return CodexHarvestControls{}, false, err
	}
	var v CodexHarvestControls
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return CodexHarvestControls{}, false, err
	}
	if v.ProxyIDs == nil {
		v.ProxyIDs = []int64{}
	}
	if err := ValidateCodexHarvestControls(v); err != nil {
		return CodexHarvestControls{}, false, err
	}
	return v, true, nil
}

// SaveControls validates, requires every pool member to exist, persists and
// wakes the harvest loop so a new pool or speed takes effect immediately.
func (s *CodexHarvestService) SaveControls(ctx context.Context, v CodexHarvestControls) error {
	if v.ProxyIDs == nil {
		v.ProxyIDs = []int64{}
	}
	if err := ValidateCodexHarvestControls(v); err != nil {
		return err
	}
	if len(v.ProxyIDs) > 0 {
		if s.proxies == nil {
			return errors.New("proxy repository unavailable")
		}
		query, cancel := context.WithTimeout(ctx, 5*time.Second)
		found, err := s.proxies.ListByIDs(query, v.ProxyIDs)
		cancel()
		if err != nil {
			return err
		}
		if len(found) != len(v.ProxyIDs) {
			return infraerrors.BadRequest("CODEX_HARVEST_UNKNOWN_PROXY", "harvest proxy pool contains unknown proxies")
		}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.configMu.Lock()
	query, cancel := context.WithTimeout(ctx, 3*time.Second)
	err = s.settings.Set(query, codexHarvestControlsKey, string(raw))
	cancel()
	if err == nil {
		s.current, s.configured, s.configErr = cloneCodexHarvestControls(v), true, nil
		s.loadedUntil = time.Now().Add(codexHarvestControlsTTL)
	}
	s.configMu.Unlock()
	if err != nil {
		return err
	}
	s.Wake()
	return nil
}

func cloneCodexHarvestControls(v CodexHarvestControls) CodexHarvestControls {
	v.ProxyIDs = slices.Clone(v.ProxyIDs)
	if v.ProxyIDs == nil {
		v.ProxyIDs = []int64{}
	}
	return v
}

func (s *CodexHarvestService) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// CodexHarvestPoolMember describes one configured pool entry for the admin page.
type CodexHarvestPoolMember struct {
	ProxyID int64  `json:"proxy_id"`
	Name    string `json:"name,omitempty"`
	Usable  bool   `json:"usable"`
	Reason  string `json:"reason,omitempty"`
}

// Pool resolves the configured IDs to usable proxies, preserving the saved
// order. Inactive, expired and deleted proxies are reported but never used.
func (s *CodexHarvestService) Pool(ctx context.Context, ids []int64, now time.Time) ([]Proxy, []CodexHarvestPoolMember, error) {
	members := make([]CodexHarvestPoolMember, 0, len(ids))
	if len(ids) == 0 {
		return nil, members, nil
	}
	if s.proxies == nil {
		return nil, members, errors.New("proxy repository unavailable")
	}
	query, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	found, err := s.proxies.ListByIDs(query, ids)
	if err != nil {
		return nil, members, err
	}
	byID := make(map[int64]Proxy, len(found))
	for _, p := range found {
		byID[p.ID] = p
	}
	usable := make([]Proxy, 0, len(ids))
	for _, id := range ids {
		p, ok := byID[id]
		member := CodexHarvestPoolMember{ProxyID: id}
		switch {
		case !ok:
			member.Reason = "deleted"
		case !p.IsActive():
			member.Name, member.Reason = p.Name, "inactive"
		case p.IsExpired(now):
			member.Name, member.Reason = p.Name, "expired"
		default:
			member.Name, member.Usable = p.Name, true
			usable = append(usable, p)
		}
		members = append(members, member)
	}
	return usable, members, nil
}

func (s *CodexHarvestService) setRuntime(update func(*CodexHarvestRuntime)) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	update(&s.runtime)
}

func (s *CodexHarvestService) Runtime() CodexHarvestRuntime {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	return s.runtime
}

// takeAccountHour charges one sent request against the rolling hourly limit.
// Switching proxies never returns quota.
func (s *CodexHarvestService) takeAccountHour(accountID int64, limit int, now time.Time) bool {
	s.hourMu.Lock()
	defer s.hourMu.Unlock()
	sent := pruneCodexHarvestHour(s.hour[accountID], now)
	if len(sent) >= limit {
		s.hour[accountID] = sent
		return false
	}
	s.hour[accountID] = append(sent, now)
	return true
}

func (s *CodexHarvestService) accountHourUsed(accountID int64, now time.Time) int {
	s.hourMu.Lock()
	defer s.hourMu.Unlock()
	sent := pruneCodexHarvestHour(s.hour[accountID], now)
	if len(sent) == 0 {
		delete(s.hour, accountID)
	} else {
		s.hour[accountID] = sent
	}
	return len(sent)
}

func pruneCodexHarvestHour(sent []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-codexHarvestAccountWindow)
	first := 0
	for first < len(sent) && !sent[first].After(cutoff) {
		first++
	}
	return sent[first:]
}

func (s *CodexHarvestService) coolingDown(accountID int64, model string, now time.Time) (time.Time, bool) {
	key := openAICodexTicketKey(accountID, model)
	value, ok := s.cooldowns.Load(key)
	until, _ := value.(time.Time)
	if !ok || !until.After(now) {
		s.cooldowns.Delete(key)
		return time.Time{}, false
	}
	return until, true
}

func (s *CodexHarvestService) setCooldown(accountID int64, model string, until time.Time) {
	s.cooldowns.Store(openAICodexTicketKey(accountID, model), until)
}

func (s *CodexHarvestService) clearCooldown(accountID int64, model string) {
	s.cooldowns.Delete(openAICodexTicketKey(accountID, model))
}

func (s *CodexHarvestService) ListNodes(ctx context.Context, offset, limit int) (CodexHarvestNodePage, error) {
	if s.nodes == nil {
		return CodexHarvestNodePage{Items: []CodexHarvestNodeRecord{}}, nil
	}
	query, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.nodes.List(query, offset, limit)
}

func (s *CodexHarvestService) ResetNodes(ctx context.Context, id int64) error {
	if s.nodes == nil {
		return errors.New("harvest learning storage unavailable")
	}
	query, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.nodes.Reset(query, id); err != nil {
		return err
	}
	s.explore.Store(0)
	return nil
}
