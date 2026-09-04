package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// FirstTopupBonusTier grants bonus_amount when a user's first completed top-up is at least min_amount.
type FirstTopupBonusTier struct {
	MinAmount   float64 `json:"min_amount"`
	BonusAmount float64 `json:"bonus_amount"`
}

// ParseFirstTopupBonusTiers parses the first_topup_bonus_tiers setting (JSON array). Invalid or
// non-positive entries are dropped; the result is sorted by min_amount ascending.
func ParseFirstTopupBonusTiers(raw string) []FirstTopupBonusTier {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tiers []FirstTopupBonusTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		return nil
	}
	out := tiers[:0]
	for _, t := range tiers {
		if t.MinAmount > 0 && t.BonusAmount > 0 {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MinAmount < out[j].MinAmount })
	return out
}

// PickFirstTopupBonus returns the bonus of the highest tier whose min_amount is met, or 0.
func PickFirstTopupBonus(tiers []FirstTopupBonusTier, amount float64) float64 {
	bonus := 0.0
	for _, t := range tiers {
		if amount+1e-9 >= t.MinAmount {
			bonus = t.BonusAmount
		}
	}
	return bonus
}

// FirstTopupBonusTiers reads and parses the configured tiers. Empty when the feature is off.
func (s *SettingService) FirstTopupBonusTiers(ctx context.Context) []FirstTopupBonusTier {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyFirstTopupBonusTiers)
	if err != nil {
		return nil
	}
	return ParseFirstTopupBonusTiers(value)
}
