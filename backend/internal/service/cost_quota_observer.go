package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
)

// Only normalized window observations leave the quota service. Credit IDs, account
// credentials and upstream payloads are never sent to the procurement ledger.
func (s *OpenAIQuotaService) observeCostQuota(ctx context.Context, id int64, usage *OpenAIQuotaUsage, kind string) {
	if s.costQuotaObserver == nil || usage == nil {
		return
	}
	at := time.Unix(usage.FetchedAt, 0).UTC()
	if usage.FetchedAt <= 0 {
		at = time.Now().UTC()
	}
	q := costing.QuotaObservation{AccountID: id, ObservedAt: at.Format(time.RFC3339Nano), Kind: kind, Windows: []costing.ObservedQuotaWindow{}}
	add := func(pool string, limit *OpenAIRateLimit) {
		if limit == nil {
			return
		}
		for _, item := range []struct {
			id     string
			window *OpenAIRateLimitWindow
		}{{"primary", limit.PrimaryWindow}, {"secondary", limit.SecondaryWindow}} {
			if w := item.window; w != nil {
				reset := w.ResetAt
				if reset == 0 && w.ResetAfterSeconds >= 0 {
					reset = at.Unix() + w.ResetAfterSeconds
				}
				q.Windows = append(q.Windows, costing.ObservedQuotaWindow{Pool: pool, Window: item.id, UsedPercent: w.UsedPercent, DurationSeconds: w.LimitWindowSeconds, ResetAt: reset})
			}
		}
	}
	add("codex", usage.RateLimit)
	// Additional pools need an explicit source identifier; never merge them with Codex.
	for _, limit := range usage.AdditionalRateLimits {
		if limit.MeteredFeature != "" {
			add(limit.MeteredFeature, limit.RateLimit)
		}
	}
	if len(q.Windows) == 0 {
		return
	}
	if err := s.costQuotaObserver(ctx, q); err != nil {
		slog.Warn("cost quota observation was not persisted", "account_id", id, "reason", "cost_observation_unavailable")
	}
}
