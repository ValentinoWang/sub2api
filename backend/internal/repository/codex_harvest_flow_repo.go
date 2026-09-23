package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const codexHarvestFlowPersistCap = 200

type codexHarvestFlowRepository struct{ db *sql.DB }

func NewCodexHarvestFlowRepository(db *sql.DB) service.CodexHarvestFlowRepository {
	return &codexHarvestFlowRepository{db: db}
}

// List returns up to limit events, oldest first, matching the in-memory ring.
func (r *codexHarvestFlowRepository) List(ctx context.Context, limit int) ([]service.CodexHarvestFlowEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("harvest flow repository unavailable")
	}
	if limit < 1 || limit > codexHarvestFlowPersistCap {
		limit = codexHarvestFlowPersistCap
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_id, at, stage, kind, account_id, account_name, model, proxy_id, proxy_name,
 http_status, length, accepted, manual, result, detail
 FROM codex_harvest_flow_events ORDER BY at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	newestFirst := make([]service.CodexHarvestFlowEvent, 0, limit)
	for rows.Next() {
		var e service.CodexHarvestFlowEvent
		if err := rows.Scan(&e.ID, &e.At, &e.Stage, &e.Kind, &e.AccountID, &e.AccountName, &e.Model, &e.ProxyID, &e.ProxyName,
			&e.HTTPStatus, &e.Length, &e.Accepted, &e.Manual, &e.Result, &e.Detail); err != nil {
			return nil, err
		}
		newestFirst = append(newestFirst, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]service.CodexHarvestFlowEvent, len(newestFirst))
	for i := range newestFirst {
		out[len(newestFirst)-1-i] = newestFirst[i]
	}
	return out, nil
}

// Append stores one event and trims the table to the newest 200 rows.
func (r *codexHarvestFlowRepository) Append(ctx context.Context, e service.CodexHarvestFlowEvent) error {
	if r == nil || r.db == nil {
		return errors.New("harvest flow repository unavailable")
	}
	if e.ID == "" {
		return errors.New("harvest flow event id required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO codex_harvest_flow_events
 (event_id, at, stage, kind, account_id, account_name, model, proxy_id, proxy_name, http_status, length, accepted, manual, result, detail)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
 ON CONFLICT (event_id) DO NOTHING`,
		e.ID, e.At, e.Stage, e.Kind, e.AccountID, e.AccountName, e.Model, e.ProxyID, e.ProxyName,
		e.HTTPStatus, e.Length, e.Accepted, e.Manual, e.Result, e.Detail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM codex_harvest_flow_events WHERE id IN (
 SELECT id FROM codex_harvest_flow_events ORDER BY at DESC, id DESC OFFSET $1)`, codexHarvestFlowPersistCap); err != nil {
		return err
	}
	return tx.Commit()
}
