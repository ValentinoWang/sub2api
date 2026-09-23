package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// codexHarvestNodeRowCap bounds the learning table; rows untouched for 90
// days are dropped on the next feedback.
const codexHarvestNodeRowCap = 10000

type codexHarvestNodeRepository struct{ db *sql.DB }

func NewCodexHarvestNodeRepository(db *sql.DB) service.CodexHarvestNodeRepository {
	return &codexHarvestNodeRepository{db: db}
}

const codexHarvestNodeColumns = `n.id, n.proxy_id, n.proxy_name, n.account_id, COALESCE(a.name, ''), n.identity, n.model,
 n.successes, n.misses, n.network_errors, n.account_errors, n.consecutive_failures,
 n.last_success, n.cooldown_until, n.latency_ms, n.last_result, n.updated_at`

func scanCodexHarvestNodes(rows *sql.Rows) ([]service.CodexHarvestNodeRecord, error) {
	defer func() { _ = rows.Close() }()
	items := []service.CodexHarvestNodeRecord{}
	for rows.Next() {
		var r service.CodexHarvestNodeRecord
		if err := rows.Scan(&r.ID, &r.ProxyID, &r.ProxyName, &r.AccountID, &r.AccountName, &r.Identity, &r.Model,
			&r.Successes, &r.Misses, &r.NetworkErrors, &r.AccountErrors, &r.ConsecutiveFailures,
			&r.LastSuccess, &r.CooldownUntil, &r.LatencyMS, &r.LastResult, &r.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

func (r *codexHarvestNodeRepository) Snapshot(ctx context.Context, scope service.CodexHarvestNodeScope) (int64, []service.CodexHarvestNodeRecord, error) {
	var generation int64
	if err := r.db.QueryRowContext(ctx, `SELECT generation FROM codex_harvest_learning_epoch WHERE id=1`).Scan(&generation); err != nil {
		return 0, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+codexHarvestNodeColumns+` FROM codex_harvest_nodes n
 LEFT JOIN accounts a ON a.id = n.account_id
 WHERE n.account_id=$1 AND n.identity=$2 AND n.model=$3`,
		scope.AccountID, scope.Identity, scope.Model)
	if err != nil {
		return 0, nil, err
	}
	items, err := scanCodexHarvestNodes(rows)
	return generation, items, err
}

func (r *codexHarvestNodeRepository) List(ctx context.Context, offset, limit int) (service.CodexHarvestNodePage, error) {
	page := service.CodexHarvestNodePage{Items: []service.CodexHarvestNodeRecord{}}
	if offset < 0 || limit < 1 || limit > 100 {
		return page, errors.New("invalid node page")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return page, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM codex_harvest_nodes`).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+codexHarvestNodeColumns+` FROM codex_harvest_nodes n
 LEFT JOIN accounts a ON a.id = n.account_id
 ORDER BY n.updated_at DESC, n.id DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return page, err
	}
	page.Items, err = scanCodexHarvestNodes(rows)
	if err != nil {
		return page, err
	}
	return page, tx.Commit()
}

// Reset bumps the learning generation before deleting, so feedback from probes
// already in flight is discarded instead of recreating the reset rows.
func (r *codexHarvestNodeRepository) Reset(ctx context.Context, id int64) error {
	if id < 0 {
		return errors.New("invalid node record ID")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Reset and feedback acquire the epoch row first, in the same lock order.
	if _, err := tx.ExecContext(ctx, `UPDATE codex_harvest_learning_epoch SET generation=generation+1 WHERE id=1`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_harvest_nodes WHERE $1::bigint=0 OR id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Record applies one probe outcome. Account-level failures (401/403/429) are
// counted but never put the proxy itself into cooldown.
func (r *codexHarvestNodeRepository) Record(ctx context.Context, f service.CodexHarvestNodeFeedback) (bool, error) {
	success, miss, network, account := 0, 0, 0, 0
	switch f.Result {
	case "success":
		success = 1
	case "invalid_state", "response_incomplete_or_error", "upstream_error":
		miss = 1
	case "network_error":
		network = 1
	case "account_error", "rate_limited":
		account = 1
	default:
		return false, errors.New("unknown harvest result")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM codex_harvest_learning_epoch WHERE id=1 FOR UPDATE`).Scan(&generation); err != nil {
		return false, err
	}
	if generation != f.Generation {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_harvest_nodes WHERE updated_at < NOW()-INTERVAL '90 days'
 OR id IN (SELECT id FROM codex_harvest_nodes ORDER BY updated_at DESC, id DESC OFFSET $1)`, codexHarvestNodeRowCap-1); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO codex_harvest_nodes
 (proxy_id,proxy_name,account_id,identity,model,successes,misses,network_errors,account_errors,
 consecutive_failures,last_success,cooldown_until,latency_ms,last_result)
 VALUES ($1,$2,$3,$4,$5,$6::int,$7::int,$8::int,$9::int,$7::int+$8::int,
 CASE WHEN $6=1 THEN NOW() END, CASE WHEN $7+$8>0 THEN NOW()+make_interval(secs => $12) END,$10,$11)
 ON CONFLICT (proxy_id,account_id,identity,model) DO UPDATE SET
 proxy_name=EXCLUDED.proxy_name,
 successes=codex_harvest_nodes.successes+$6, misses=codex_harvest_nodes.misses+$7,
 network_errors=codex_harvest_nodes.network_errors+$8, account_errors=codex_harvest_nodes.account_errors+$9,
 consecutive_failures=CASE WHEN $6=1 THEN 0 ELSE codex_harvest_nodes.consecutive_failures+$7+$8 END,
 last_success=CASE WHEN $6=1 THEN NOW() ELSE codex_harvest_nodes.last_success END,
 cooldown_until=CASE WHEN $6=1 THEN NULL WHEN $7+$8>0 THEN EXCLUDED.cooldown_until ELSE codex_harvest_nodes.cooldown_until END,
 latency_ms=CASE WHEN $6=1 OR codex_harvest_nodes.successes=0 THEN $10 ELSE codex_harvest_nodes.latency_ms END,
 last_result=$11, updated_at=NOW()`,
		f.ProxyID, f.ProxyName, f.Scope.AccountID, f.Scope.Identity, f.Scope.Model,
		success, miss, network, account, f.LatencyMS, f.Result, f.CooldownSeconds)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
