package costing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math"
	"time"
)

type ObservedQuotaWindow struct {
	Pool            string  `json:"pool"`
	Window          string  `json:"window"`
	UsedPercent     float64 `json:"used_percent"`
	DurationSeconds int64   `json:"duration_seconds"`
	ResetAt         int64   `json:"reset_at"`
}
type QuotaObservation struct {
	AccountID    int64                 `json:"account_id"`
	ObservedAt   string                `json:"observed_at"`
	Kind         string                `json:"kind"`
	Windows      []ObservedQuotaWindow `json:"windows"`
	QualityFlags []string              `json:"quality_flags"`
}

// QuotaStore stores observations in their own table, never in the low-frequency money ledger.
// Missing observations and reset attribution cannot be inferred from a pair of percentages.
type QuotaStore struct {
	Open func() (*sql.DB, func(), error)
}

func (s *QuotaStore) Record(ctx context.Context, q QuotaObservation) error {
	if s == nil || s.Open == nil {
		return ErrUnavailable
	}
	at, err := parseTime(q.ObservedAt)
	if q.AccountID <= 0 || err != nil || at.After(time.Now().Add(time.Second)) || (q.Kind != "snapshot" && q.Kind != "after_reset") || len(q.Windows) == 0 || len(q.Windows) > 20 {
		return invalid("quota observation scope")
	}
	seen := map[string]bool{}
	q.QualityFlags = []string{}
	unknownDuration := false
	for _, w := range q.Windows {
		key := w.Pool + ":" + w.Window
		if !refOK(w.Pool) || !refOK(w.Window) || seen[key] || math.IsNaN(w.UsedPercent) || math.IsInf(w.UsedPercent, 0) || w.UsedPercent < 0 || w.UsedPercent > 100 || w.DurationSeconds < 0 || w.ResetAt < at.Unix() {
			return invalid("quota window observation")
		}
		seen[key] = true
		unknownDuration = unknownDuration || w.DurationSeconds == 0
	}
	if unknownDuration {
		q.QualityFlags = append(q.QualityFlags, "WINDOW_DURATION_UNKNOWN")
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return invalid("quota observation encoding")
	}
	hash := sha256.Sum256(raw)
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `INSERT INTO cost_center_quota_observations(event_key,account_id,observed_at,kind,payload) VALUES($1,$2,$3,$4,$5::jsonb) ON CONFLICT(event_key) DO NOTHING`, hex.EncodeToString(hash[:]), q.AccountID, q.ObservedAt, q.Kind, string(raw))
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *QuotaStore) List(ctx context.Context, accountID int64, start, end string) ([]QuotaObservation, error) {
	out := []QuotaObservation{}
	if s == nil || s.Open == nil {
		return out, ErrUnavailable
	}
	if accountID <= 0 || !validPeriod(start, end) {
		return out, invalid("quota history account and interval required")
	}
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return out, ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT payload FROM cost_center_quota_observations WHERE account_id=$1 AND observed_at >= $2 AND observed_at < $3 ORDER BY observed_at,event_key LIMIT 1001`, accountID, start, end)
	if err != nil {
		return out, ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var q QuotaObservation
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &q) != nil {
			return nil, ErrUnavailable
		}
		out = append(out, q)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	if len(out) > 1000 {
		return nil, ErrLimit
	}
	return out, nil
}
