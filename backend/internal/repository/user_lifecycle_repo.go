package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userLifecycleRepository struct {
	db *sql.DB
}

// NewUserLifecycleRepository stores the once-per-user lifecycle email ledger with plain SQL
// (no ent entity), mirroring the continuity repository.
func NewUserLifecycleRepository(db *sql.DB) service.UserLifecycleRepository {
	return &userLifecycleRepository{db: db}
}

const lifecycleWelcomeSQL = `
SELECT u.id, u.email, COALESCE(u.username, ''), u.created_at
FROM users u
WHERE u.deleted_at IS NULL
  AND u.status = 'active'
  AND u.created_at >= $2
  AND NOT EXISTS (SELECT 1 FROM user_lifecycle_emails l WHERE l.user_id = u.id AND l.event = $1)
ORDER BY u.id
LIMIT $3`

// Last activity is derived from api_keys.last_used_at (cheap, indexed per user) rather than
// scanning usage_logs. Users who never used a key fall back to their registration time.
const lifecycleInactiveSQL = `
SELECT u.id, u.email, COALESCE(u.username, ''), u.created_at, la.last_active
FROM users u
LEFT JOIN LATERAL (
    SELECT MAX(k.last_used_at) AS last_active FROM api_keys k WHERE k.user_id = u.id
) la ON TRUE
WHERE u.deleted_at IS NULL
  AND u.status = 'active'
  AND u.created_at <= $2
  AND COALESCE(la.last_active, u.created_at) <= $3
  AND NOT EXISTS (SELECT 1 FROM user_lifecycle_emails l WHERE l.user_id = u.id AND l.event = $1)
ORDER BY u.id
LIMIT $4`

func (r *userLifecycleRepository) ListWelcomeCandidates(ctx context.Context, event string, since time.Time, limit int) ([]service.LifecycleCandidate, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, lifecycleWelcomeSQL, event, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []service.LifecycleCandidate
	for rows.Next() {
		var c service.LifecycleCandidate
		if err := rows.Scan(&c.UserID, &c.Email, &c.Username, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *userLifecycleRepository) ListInactiveCandidates(ctx context.Context, event string, createdBefore, inactiveBefore time.Time, limit int) ([]service.LifecycleCandidate, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, lifecycleInactiveSQL, event, createdBefore, inactiveBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []service.LifecycleCandidate
	for rows.Next() {
		var c service.LifecycleCandidate
		var last sql.NullTime
		if err := rows.Scan(&c.UserID, &c.Email, &c.Username, &c.CreatedAt, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			t := last.Time
			c.LastActiveAt = &t
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *userLifecycleRepository) MarkSent(ctx context.Context, userID int64, event string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_lifecycle_emails (user_id, event, sent_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id, event) DO UPDATE SET sent_at = EXCLUDED.sent_at`,
		userID, event)
	return err
}
