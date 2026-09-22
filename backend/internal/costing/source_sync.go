package costing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"
)

type SourceSyncResult struct {
	ID                 string         `json:"id"`
	Source             string         `json:"source"`
	ObservedAt         string         `json:"observed_at"`
	Start              string         `json:"start"`
	End                string         `json:"end"`
	AccountCount       int            `json:"account_count"`
	RequestCount       int64          `json:"request_count"`
	StoredRequestCount int64          `json:"stored_request_count"`
	SourceHash         string         `json:"source_hash"`
	Status             string         `json:"status"`
	Report             Reconciliation `json:"report"`
}
type SourceSnapshotStore struct {
	Open func() (*sql.DB, func(), error)
}

// Save stores normalized source facts, not invoices or an inferred subscription tier.
// The database readback verifies the persisted snapshot against the source result.
func (s *SourceSnapshotStore) Save(ctx context.Context, source string, actor int64, q SummaryQuery, report Reconciliation) (SourceSyncResult, error) {
	out := SourceSyncResult{Source: source, Start: q.Start, End: q.End, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Report: report}
	if s == nil || s.Open == nil {
		return out, ErrUnavailable
	}
	if !refOK(source) || actor <= 0 {
		return out, invalid("source identity and verified actor required")
	}
	for _, row := range report.Rows {
		out.RequestCount += row.Requests
	}
	out.AccountCount = len(report.Accounts)
	raw, err := json.Marshal(report)
	if err != nil {
		return out, ErrUnavailable
	}
	hash := sha256.Sum256(raw)
	out.SourceHash = hex.EncodeToString(hash[:])
	id := sha256.Sum256([]byte(source + "|" + q.Start + "|" + q.End + "|" + out.SourceHash))
	out.ID = hex.EncodeToString(id[:])
	db, release, err := s.Open()
	if err != nil || db == nil || release == nil {
		return out, ErrUnavailable
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, ErrUnavailable
	}
	defer tx.Rollback()
	for _, account := range report.Accounts {
		metadata, err := json.Marshal(account)
		if err != nil {
			return out, ErrUnavailable
		}
		digest := sha256.Sum256(metadata)
		_, err = tx.ExecContext(ctx, `INSERT INTO cost_center_account_observations(source,account_id,source_hash,observed_at,payload) VALUES($1,$2,$3,$4,$5::jsonb) ON CONFLICT(source,account_id,source_hash) DO NOTHING`, source, account.ID, hex.EncodeToString(digest[:]), out.ObservedAt, string(metadata))
		if err != nil {
			return out, ErrUnavailable
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cost_center_source_snapshots(snapshot_id,source,scope_start,scope_end,observed_at,actor_id,source_hash,request_count,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb) ON CONFLICT(snapshot_id) DO NOTHING`, out.ID, source, q.Start, q.End, out.ObservedAt, actor, out.SourceHash, out.RequestCount, string(raw))
	if err != nil {
		return out, ErrUnavailable
	}
	var storedHash string
	var storedPayload []byte
	err = tx.QueryRowContext(ctx, `SELECT source_hash,request_count,payload FROM cost_center_source_snapshots WHERE snapshot_id=$1`, out.ID).Scan(&storedHash, &out.StoredRequestCount, &storedPayload)
	if err != nil || storedHash != out.SourceHash || out.StoredRequestCount != out.RequestCount {
		return out, ErrConflict
	}
	var storedReport Reconciliation
	if json.Unmarshal(storedPayload, &storedReport) != nil {
		return out, ErrConflict
	}
	canonical, err := json.Marshal(storedReport)
	if err != nil {
		return out, ErrConflict
	}
	persistedHash := sha256.Sum256(canonical)
	var persistedRequests int64
	for _, row := range storedReport.Rows {
		persistedRequests += row.Requests
	}
	if hex.EncodeToString(persistedHash[:]) != out.SourceHash || persistedRequests != out.RequestCount {
		return out, ErrConflict
	}
	if tx.Commit() != nil {
		return out, ErrUnavailable
	}
	out.Status = "SOURCE_SNAPSHOT_VERIFIED"
	return out, nil
}
