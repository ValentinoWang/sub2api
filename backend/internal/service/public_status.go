package service

import (
	"context"
	"sort"
	"time"
)

// PublicStatusItem is the crawler-safe, unauthenticated view of one enabled channel monitor.
// It deliberately omits account, quota, group and API-key details.
type PublicStatusItem struct {
	Name          string     `json:"name"`
	Provider      string     `json:"provider"`
	Model         string     `json:"model"`
	Status        string     `json:"status"` // operational | degraded | failed | error | unknown
	Samples       int        `json:"samples"`
	SuccessRate   *float64   `json:"success_rate,omitempty"` // percentage over the sampled window
	LatencyP50Ms  *int       `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms  *int       `json:"latency_p95_ms,omitempty"`
	WindowStart   *time.Time `json:"window_start,omitempty"` // oldest sample in the window
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}

// PublicStatusSummary is returned by GET /api/v1/public/status.
type PublicStatusSummary struct {
	Enabled     bool               `json:"enabled"`
	GeneratedAt time.Time          `json:"generated_at"`
	Overall     string             `json:"overall"` // operational | degraded | failed | unknown
	Items       []PublicStatusItem `json:"items"`
}

// PublicStatus aggregates the enabled monitors' recent probe history into a read-only summary.
// Numbers are derived from the same scheduled server-side probes that power the channel monitor
// page, so every figure carries a sample count and window start rather than a bare claim.
func (s *ChannelMonitorService) PublicStatus(ctx context.Context) (*PublicStatusSummary, error) {
	views, err := s.ListUserView(ctx)
	if err != nil {
		return nil, err
	}
	summary := &PublicStatusSummary{
		Enabled:     true,
		GeneratedAt: time.Now().UTC(),
		Overall:     "unknown",
		Items:       make([]PublicStatusItem, 0, len(views)),
	}
	worst := 0
	rank := map[string]int{"operational": 1, "degraded": 2, "failed": 3, "error": 3}
	for _, v := range views {
		if v == nil {
			continue
		}
		item := PublicStatusItem{
			Name:     v.Name,
			Provider: v.Provider,
			Model:    v.PrimaryModel,
			Status:   normalizePublicStatus(v.PrimaryStatus),
		}
		applyPublicStatusTimeline(&item, v.Timeline)
		if r, ok := rank[item.Status]; ok && r > worst {
			worst = r
		}
		summary.Items = append(summary.Items, item)
	}
	switch worst {
	case 1:
		summary.Overall = MonitorStatusOperational
	case 2:
		summary.Overall = MonitorStatusDegraded
	case 3:
		summary.Overall = MonitorStatusFailed
	}
	return summary, nil
}

func normalizePublicStatus(status string) string {
	switch status {
	case MonitorStatusOperational, MonitorStatusDegraded, MonitorStatusFailed, MonitorStatusError:
		return status
	default:
		return "unknown"
	}
}

// applyPublicStatusTimeline fills sample count, success rate and latency percentiles from the
// primary model's recent history (newest first).
func applyPublicStatusTimeline(item *PublicStatusItem, timeline []UserMonitorTimelinePoint) {
	if len(timeline) == 0 {
		return
	}
	latencies := make([]int, 0, len(timeline))
	success := 0
	var oldest, newest time.Time
	for _, p := range timeline {
		if oldest.IsZero() || p.CheckedAt.Before(oldest) {
			oldest = p.CheckedAt
		}
		if newest.IsZero() || p.CheckedAt.After(newest) {
			newest = p.CheckedAt
		}
		if p.Status == MonitorStatusOperational || p.Status == MonitorStatusDegraded {
			success++
			if p.LatencyMs != nil && *p.LatencyMs >= 0 {
				latencies = append(latencies, *p.LatencyMs)
			}
		}
	}
	item.Samples = len(timeline)
	rate := float64(success) / float64(len(timeline)) * 100
	rate = float64(int(rate*10+0.5)) / 10
	item.SuccessRate = &rate
	if !oldest.IsZero() {
		o := oldest.UTC()
		item.WindowStart = &o
	}
	if !newest.IsZero() {
		n := newest.UTC()
		item.LastCheckedAt = &n
	}
	if len(latencies) > 0 {
		sort.Ints(latencies)
		p50 := latencies[percentileIndex(len(latencies), 0.5)]
		p95 := latencies[percentileIndex(len(latencies), 0.95)]
		item.LatencyP50Ms = &p50
		item.LatencyP95Ms = &p95
	}
}

// percentileIndex maps a percentile to an index into a sorted slice of length n.
// n <= 0 returns 0 so a caller that forgets the emptiness check cannot index out of range.
func percentileIndex(n int, p float64) int {
	if n <= 1 {
		return 0
	}
	idx := int(p * float64(n-1))
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}
