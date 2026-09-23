package service

import (
	"context"
	"sort"
	"time"
)

type CodexHarvestTicketView struct {
	Model            string     `json:"model"`
	Ready            bool       `json:"ready"`
	Blocked          bool       `json:"blocked"`
	Length           int        `json:"length,omitempty"`
	RemainingSeconds int64      `json:"remaining_seconds"`
	AgeSeconds       int64      `json:"age_seconds,omitempty"`
	ProxyName        string     `json:"proxy_name,omitempty"`
	CooldownUntil    *time.Time `json:"cooldown_until,omitempty"`
}

type CodexHarvestAccountView struct {
	ID          int64                    `json:"id"`
	Name        string                   `json:"name"`
	Status      string                   `json:"status"`
	Schedulable bool                     `json:"schedulable"`
	HourUsed    int                      `json:"hour_used"`
	HourLimit   int                      `json:"hour_limit"`
	Tickets     []CodexHarvestTicketView `json:"tickets"`
	Manual      *CodexHarvestManualRun   `json:"manual,omitempty"`
}

type CodexHarvestSnapshot struct {
	GeneratedAt   time.Time                    `json:"generated_at"`
	Enabled       bool                         `json:"enabled"`
	FailClosed    bool                         `json:"fail_closed"`
	Models        []string                     `json:"models"`
	TargetLength  int                          `json:"target_length"`
	Controls      CodexHarvestControls         `json:"controls"`
	Configured    bool                         `json:"configured"`
	SettingsError string                       `json:"settings_error,omitempty"`
	Presets       map[string]CodexHarvestSpeed `json:"presets"`
	PresetOrder   []string                     `json:"preset_order"`
	Bounds        map[string]CodexHarvestBound `json:"bounds"`
	Pool          []CodexHarvestPoolMember     `json:"pool"`
	PoolError     string                       `json:"pool_error,omitempty"`
	Runtime       CodexHarvestRuntime          `json:"runtime"`
	Accounts      []CodexHarvestAccountView    `json:"accounts"`
	Events        []CodexHarvestFlowEvent      `json:"events"`
}

// CodexHarvestSnapshot assembles the admin page view. It never includes
// ticket state, tokens or proxy credentials.
func (s *OpenAIGatewayService) CodexHarvestSnapshot(ctx context.Context) (CodexHarvestSnapshot, error) {
	h := s.codexHarvestService()
	if h == nil {
		return CodexHarvestSnapshot{}, ErrCodexHarvestUnavailable
	}
	now := time.Now()
	cfg := s.openAICodexTicketConfig()
	out := CodexHarvestSnapshot{
		GeneratedAt:  now,
		Enabled:      s.openAICodexTicketEnabledContext(ctx),
		FailClosed:   cfg.FailClosed,
		Models:       append([]string(nil), cfg.Models...),
		TargetLength: cfg.TargetLength,
		Presets:      CodexHarvestSpeedPresets(),
		PresetOrder:  append([]string(nil), codexHarvestPresetOrder...),
		Bounds:       CodexHarvestSpeedBounds(),
		Runtime:      h.Runtime(),
		Accounts:     []CodexHarvestAccountView{},
		Events:       h.Events(),
	}
	controls, configured, err := h.Controls(ctx)
	out.Controls, out.Configured = controls, configured
	if err != nil {
		out.SettingsError = "harvest settings unavailable; using the last valid values"
	}
	_, members, err := h.Pool(ctx, controls.ProxyIDs, now)
	out.Pool = members
	if err != nil {
		out.PoolError = "proxy pool unavailable"
	}
	if s.accountRepo == nil {
		return out, nil
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return out, err
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	runs := h.ManualRuns()
	seen := map[int64]bool{}
	for i := range accounts {
		account := accounts[i]
		if seen[account.ID] || !isOpenAICodexTicketAccount(&account) {
			continue
		}
		seen[account.ID] = true
		view := CodexHarvestAccountView{
			ID: account.ID, Name: account.Name, Status: account.Status, Schedulable: account.IsSchedulable(),
			HourUsed: h.accountHourUsed(account.ID, now), HourLimit: controls.Speed.MaxRequestsPerAccountHour,
			Tickets: make([]CodexHarvestTicketView, 0, len(cfg.Models)),
		}
		if run, ok := runs[account.ID]; ok {
			view.Manual = &run
		}
		for _, model := range cfg.Models {
			model = normalizeOpenAICodexTicketModel(model)
			if model == "" {
				continue
			}
			ticketView := CodexHarvestTicketView{Model: model}
			if ticket := s.lookupOpenAICodexTicket(&account, model); ticket.valid(now, cfg.TargetLength) {
				ticketView.Ready = ticket.injectable(now, cfg.TargetLength)
				ticketView.Length = ticket.Length
				if remaining := int64(ticket.effectiveExpiresAt().Sub(now) / time.Second); remaining > 0 {
					ticketView.RemainingSeconds = remaining
				}
				ticketView.AgeSeconds = int64(now.Sub(ticket.CapturedAt) / time.Second)
				ticketView.ProxyName = ticket.HarvestProxyName
			}
			ticketView.Blocked = out.Enabled && cfg.FailClosed && !ticketView.Ready
			if until, cooling := h.coolingDown(account.ID, model, now); cooling {
				ticketView.CooldownUntil = &until
			}
			view.Tickets = append(view.Tickets, ticketView)
		}
		out.Accounts = append(out.Accounts, view)
	}
	return out, nil
}
