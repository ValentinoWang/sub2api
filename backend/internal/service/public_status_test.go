package service

import (
	"testing"
	"time"
)

func timelinePoint(status string, latencyMs int, at time.Time) UserMonitorTimelinePoint {
	point := UserMonitorTimelinePoint{Status: status, CheckedAt: at}
	if latencyMs >= 0 {
		point.LatencyMs = &latencyMs
	}
	return point
}

func TestApplyPublicStatusTimelineSummarisesSamples(t *testing.T) {
	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	// Newest first, as ListUserView returns it.
	timeline := []UserMonitorTimelinePoint{
		timelinePoint(MonitorStatusOperational, 40, base.Add(3*time.Minute)),
		timelinePoint(MonitorStatusFailed, -1, base.Add(2*time.Minute)),
		timelinePoint(MonitorStatusDegraded, 120, base.Add(time.Minute)),
		timelinePoint(MonitorStatusOperational, 20, base),
	}

	var item PublicStatusItem
	applyPublicStatusTimeline(&item, timeline)

	if item.Samples != 4 {
		t.Fatalf("samples: want 4, got %d", item.Samples)
	}
	if item.SuccessRate == nil || *item.SuccessRate != 75 {
		t.Errorf("success rate: want 75, got %v", item.SuccessRate)
	}
	if item.LatencyP50Ms == nil || item.LatencyP95Ms == nil {
		t.Fatalf("latency percentiles must be set when successful samples carry latency")
	}
	if *item.LatencyP50Ms > *item.LatencyP95Ms {
		t.Errorf("p50 (%d) must not exceed p95 (%d)", *item.LatencyP50Ms, *item.LatencyP95Ms)
	}
	if item.WindowStart == nil || !item.WindowStart.Equal(base) {
		t.Errorf("window start must be the oldest sample, got %v", item.WindowStart)
	}
	if item.LastCheckedAt == nil || !item.LastCheckedAt.Equal(base.Add(3*time.Minute)) {
		t.Errorf("last checked must be the newest sample, got %v", item.LastCheckedAt)
	}
}

func TestApplyPublicStatusTimelineEdgeCases(t *testing.T) {
	t.Run("empty timeline leaves the item untouched", func(t *testing.T) {
		var item PublicStatusItem
		applyPublicStatusTimeline(&item, nil)
		if item.Samples != 0 || item.SuccessRate != nil || item.LatencyP50Ms != nil {
			t.Errorf("empty timeline must not populate any metric: %+v", item)
		}
	})

	t.Run("all failures report zero success and no latency", func(t *testing.T) {
		now := time.Now().UTC()
		var item PublicStatusItem
		applyPublicStatusTimeline(&item, []UserMonitorTimelinePoint{
			timelinePoint(MonitorStatusFailed, -1, now),
			timelinePoint(MonitorStatusError, -1, now.Add(-time.Minute)),
		})
		if item.SuccessRate == nil || *item.SuccessRate != 0 {
			t.Errorf("success rate: want 0, got %v", item.SuccessRate)
		}
		if item.LatencyP50Ms != nil || item.LatencyP95Ms != nil {
			t.Errorf("no successful sample means no latency percentiles")
		}
	})

	t.Run("single sample reports both percentiles", func(t *testing.T) {
		var item PublicStatusItem
		applyPublicStatusTimeline(&item, []UserMonitorTimelinePoint{
			timelinePoint(MonitorStatusOperational, 55, time.Now().UTC()),
		})
		if item.LatencyP50Ms == nil || *item.LatencyP50Ms != 55 {
			t.Errorf("p50: want 55, got %v", item.LatencyP50Ms)
		}
		if item.LatencyP95Ms == nil || *item.LatencyP95Ms != 55 {
			t.Errorf("p95: want 55, got %v", item.LatencyP95Ms)
		}
	})
}

func TestPercentileIndexStaysInRange(t *testing.T) {
	for _, n := range []int{0, 1, 2, 5, 20} {
		for _, p := range []float64{0, 0.5, 0.95, 1} {
			idx := percentileIndex(n, p)
			if idx < 0 {
				t.Errorf("percentileIndex(%d, %v) = %d, must never be negative", n, p, idx)
			}
			if n > 0 && idx >= n {
				t.Errorf("percentileIndex(%d, %v) = %d, must stay inside the slice", n, p, idx)
			}
		}
	}
}

func TestNormalizePublicStatusRejectsUnknownValues(t *testing.T) {
	for _, status := range []string{MonitorStatusOperational, MonitorStatusDegraded, MonitorStatusFailed, MonitorStatusError} {
		if got := normalizePublicStatus(status); got != status {
			t.Errorf("known status %q must pass through, got %q", status, got)
		}
	}
	for _, status := range []string{"", "pending", "SOMETHING_ELSE"} {
		if got := normalizePublicStatus(status); got != "unknown" {
			t.Errorf("status %q must normalise to unknown, got %q", status, got)
		}
	}
}
