package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIContinuityRepository struct {
	db        *sql.DB
	encryptor service.SecretEncryptor
}

func NewOpenAIContinuityRepository(db *sql.DB, encryptor service.SecretEncryptor) service.OpenAIContinuityRepository {
	return &openAIContinuityRepository{db: db, encryptor: encryptor}
}

func (r *openAIContinuityRepository) LoadLatestCompleted(
	ctx context.Context,
	continuityID string,
	userID, apiKeyID int64,
	sessionHash string,
) (*service.OpenAIContinuitySnapshot, error) {
	if r == nil || r.db == nil || r.encryptor == nil {
		return nil, errors.New("OpenAI continuity repository is not initialized")
	}
	const query = `
SELECT t.continuity_id, t.user_id, t.api_key_id, t.session_hash,
       turn.sequence, turn.request_id, turn.replay_input_encrypted,
       turn.replay_sha256, COALESCE(turn.upstream_account_id, 0),
       turn.upstream_response_id, t.expires_at
FROM codex_continuity_threads t
JOIN LATERAL (
    SELECT sequence, request_id, replay_input_encrypted, replay_sha256,
           upstream_account_id, upstream_response_id
    FROM codex_continuity_turns
    WHERE thread_id = t.id AND state = 'completed'
    ORDER BY sequence DESC
    LIMIT 1
) turn ON TRUE
WHERE t.continuity_id = $1 AND t.user_id = $2 AND t.api_key_id = $3
  AND t.session_hash = $4 AND t.status = 'active' AND t.expires_at > NOW()`

	var snapshot service.OpenAIContinuitySnapshot
	var encrypted string
	err := r.db.QueryRowContext(ctx, query, continuityID, userID, apiKeyID, sessionHash).Scan(
		&snapshot.ContinuityID,
		&snapshot.UserID,
		&snapshot.APIKeyID,
		&snapshot.SessionHash,
		&snapshot.Sequence,
		&snapshot.RequestID,
		&encrypted,
		&snapshot.ReplaySHA256,
		&snapshot.UpstreamAccountID,
		&snapshot.UpstreamResponseID,
		&snapshot.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest continuity turn: %w", err)
	}
	plaintext, err := r.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt continuity replay: %w", err)
	}
	snapshot.ReplayInput = []byte(plaintext)
	return &snapshot, nil
}

func (r *openAIContinuityRepository) CommitCompleted(ctx context.Context, commit service.OpenAIContinuityCommit) error {
	if r == nil || r.db == nil || r.encryptor == nil {
		return errors.New("OpenAI continuity repository is not initialized")
	}
	if strings.TrimSpace(commit.ContinuityID) == "" || commit.UserID <= 0 || commit.APIKeyID <= 0 ||
		strings.TrimSpace(commit.SessionHash) == "" || strings.TrimSpace(commit.RequestID) == "" || len(commit.ReplayInput) == 0 {
		return errors.New("invalid OpenAI continuity commit")
	}
	encrypted, err := r.encryptor.Encrypt(string(commit.ReplayInput))
	if err != nil {
		return fmt.Errorf("encrypt continuity replay: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin continuity commit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const upsertThread = `
INSERT INTO codex_continuity_threads (
    continuity_id, user_id, api_key_id, session_hash, status, expires_at
) VALUES ($1, $2, $3, $4, 'active', $5)
ON CONFLICT (continuity_id) DO UPDATE SET
    updated_at = NOW(), expires_at = EXCLUDED.expires_at, status = 'active'
WHERE codex_continuity_threads.user_id = EXCLUDED.user_id
  AND codex_continuity_threads.api_key_id = EXCLUDED.api_key_id
  AND codex_continuity_threads.session_hash = EXCLUDED.session_hash
RETURNING id, version`
	var threadID, version int64
	if err := tx.QueryRowContext(ctx, upsertThread, commit.ContinuityID, commit.UserID, commit.APIKeyID, commit.SessionHash, commit.ExpiresAt).Scan(&threadID, &version); err != nil {
		return fmt.Errorf("upsert continuity thread: %w", err)
	}

	var existingSHA string
	err = tx.QueryRowContext(ctx,
		"SELECT replay_sha256 FROM codex_continuity_turns WHERE thread_id = $1 AND request_id = $2",
		threadID, commit.RequestID,
	).Scan(&existingSHA)
	if err == nil {
		if existingSHA != commit.ReplaySHA256 {
			return errors.New("continuity request ID already committed with different replay content")
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check continuity request idempotency: %w", err)
	}

	sequence := version + 1
	const insertTurn = `
INSERT INTO codex_continuity_turns (
    thread_id, sequence, request_id, state, replay_input_encrypted,
    replay_sha256, replay_bytes, upstream_account_id, upstream_response_id
) VALUES ($1, $2, $3, 'completed', $4, $5, $6, $7, $8)`
	if _, err := tx.ExecContext(ctx, insertTurn,
		threadID, sequence, commit.RequestID, encrypted, commit.ReplaySHA256,
		len(commit.ReplayInput), commit.UpstreamAccountID, commit.UpstreamResponseID,
	); err != nil {
		return fmt.Errorf("insert continuity turn: %w", err)
	}
	result, err := tx.ExecContext(ctx,
		"UPDATE codex_continuity_threads SET version = $1, updated_at = NOW(), expires_at = $2 WHERE id = $3 AND version = $4",
		sequence, commit.ExpiresAt, threadID, version,
	)
	if err != nil {
		return fmt.Errorf("advance continuity thread: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read continuity version update: %w", err)
	}
	if rows != 1 {
		return errors.New("continuity sequence conflict")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit continuity turn: %w", err)
	}
	return nil
}
