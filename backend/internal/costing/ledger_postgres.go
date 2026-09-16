package costing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SQLLedgerStore uses a caller-supplied database; it does not migrate schema or open a network connection at construction.
// Open supports a separately configured cost database. Release MUST release only this operation's resources.
// An application-managed pool can return a no-op release. Never close the main app pool here.
type SQLLedgerStore struct {
	Open func() (*sql.DB, func(), error)
}

func (s *SQLLedgerStore) connect() (*sql.DB, func(), error) {
	if s == nil || s.Open == nil {
		return nil, nil, ErrUnavailable
	}
	db, close, e := s.Open()
	if e != nil || db == nil || close == nil {
		return nil, nil, ErrUnavailable
	}
	return db, close, nil
}

type sqlQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readSQLLedger(ctx context.Context, q sqlQuery) ([]LedgerEvent, error) {
	rows, e := q.QueryContext(ctx, `SELECT payload FROM cost_center_events ORDER BY seq ASC LIMIT 10001`)
	if e != nil {
		return nil, fmt.Errorf("%w: events read", ErrUnavailable)
	}
	defer rows.Close()
	events := []LedgerEvent{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, ErrUnavailable
		}
		var event LedgerEvent
		if e = json.Unmarshal(raw, &event); e != nil {
			return nil, ErrUnavailable
		}
		events = append(events, event)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	if e = ValidateEventStream(events); e != nil {
		return nil, e
	}
	return events, nil
}
func (s *SQLLedgerStore) Snapshot(ctx context.Context) ([]LedgerEvent, error) {
	db, release, e := s.connect()
	if e != nil {
		return nil, e
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return readSQLLedger(ctx, db)
}
func (s *SQLLedgerStore) Transact(ctx context.Context, fn func([]LedgerEvent) (*LedgerEvent, error)) error {
	db, release, e := s.connect()
	if e != nil {
		return e
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	// Every supported writer takes this row lock before taking its READ COMMITTED snapshot.
	var version int
	if e = tx.QueryRowContext(ctx, `SELECT schema_version FROM cost_center_lock WHERE id=1 FOR UPDATE`).Scan(&version); e != nil || version != 1 {
		return ErrUnavailable
	}
	events, e := readSQLLedger(ctx, tx)
	if e != nil {
		return e
	}
	next, e := fn(events)
	if e != nil {
		return e
	}
	if next != nil {
		raw, e := json.Marshal(next)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO cost_center_events(seq,event_id,idempotency_key,request_hash,actor_id,recorded_at,kind,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`, next.Sequence, next.ID, next.Key, next.RequestHash, next.Actor, next.RecordedAt, next.Command.Kind, string(raw))
		if e != nil {
			return ErrUnavailable
		}
	}
	if e = tx.Commit(); e != nil {
		return ErrUnavailable
	}
	return nil
}
