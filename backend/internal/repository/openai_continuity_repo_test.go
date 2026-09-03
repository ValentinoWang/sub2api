package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIContinuityRepositoryLoadLatestCompletedReadsClientWindowID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewOpenAIContinuityRepository(db, continuityTestEncryptor{})
	expiresAt := time.Now().Add(24 * time.Hour).UTC()
	replay := `[{"type":"message"}]`

	mock.ExpectQuery(regexp.QuoteMeta("SELECT t.continuity_id, t.user_id, t.api_key_id, t.session_hash,")).
		WithArgs("continuity-id", int64(1), int64(2), "session-hash").
		WillReturnRows(sqlmock.NewRows([]string{
			"continuity_id", "user_id", "api_key_id", "session_hash",
			"sequence", "request_id", "replay_input_encrypted", "replay_sha256",
			"upstream_account_id", "upstream_response_id", "client_window_id", "expires_at",
		}).AddRow(
			"continuity-id", int64(1), int64(2), "session-hash",
			int64(3), "resp_1", "encrypted:"+replay, "sha",
			int64(9), "resp_1", "window-1", expiresAt,
		))

	snapshot, err := repo.LoadLatestCompleted(context.Background(), "continuity-id", 1, 2, "session-hash")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, []byte(replay), snapshot.ReplayInput)
	require.Equal(t, "window-1", snapshot.ClientWindowID)
	require.NoError(t, mock.ExpectationsWereMet())
}

type continuityTestEncryptor struct{}

func (continuityTestEncryptor) Encrypt(plaintext string) (string, error) {
	return "encrypted:" + plaintext, nil
}

func (continuityTestEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext[len("encrypted:"):], nil
}

func TestOpenAIContinuityRepositoryCommitCompletedIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewOpenAIContinuityRepository(db, continuityTestEncryptor{})
	expiresAt := time.Now().Add(24 * time.Hour).UTC()
	commit := service.OpenAIContinuityCommit{
		ContinuityID:       "continuity-id",
		UserID:             1,
		APIKeyID:           2,
		SessionHash:        "session-hash",
		RequestID:          "resp_1",
		ReplayInput:        []byte(`[{"type":"message"}]`),
		ReplaySHA256:       "sha",
		UpstreamAccountID:  9,
		UpstreamResponseID: "resp_1",
		ClientWindowID:     "window-1",
		ExpiresAt:          expiresAt,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO codex_continuity_threads (")).
		WithArgs(commit.ContinuityID, commit.UserID, commit.APIKeyID, commit.SessionHash, expiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(7, 2))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT replay_sha256 FROM codex_continuity_turns WHERE thread_id = $1 AND request_id = $2")).
		WithArgs(int64(7), commit.RequestID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO codex_continuity_turns (")).
		WithArgs(int64(7), int64(3), commit.RequestID, "encrypted:"+string(commit.ReplayInput), commit.ReplaySHA256, len(commit.ReplayInput), commit.UpstreamAccountID, commit.UpstreamResponseID, commit.ClientWindowID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE codex_continuity_threads SET version = $1, updated_at = NOW(), expires_at = $2 WHERE id = $3 AND version = $4")).
		WithArgs(int64(3), expiresAt, int64(7), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.CommitCompleted(context.Background(), commit))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIContinuityRepositoryCommitCompletedIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewOpenAIContinuityRepository(db, continuityTestEncryptor{})
	commit := service.OpenAIContinuityCommit{
		ContinuityID: "continuity-id", UserID: 1, APIKeyID: 2,
		SessionHash: "session-hash", RequestID: "resp_1",
		ReplayInput: []byte(`[]`), ReplaySHA256: "sha",
		UpstreamAccountID: 9, UpstreamResponseID: "resp_1",
		ExpiresAt: time.Now().Add(24 * time.Hour).UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO codex_continuity_threads (")).
		WithArgs(commit.ContinuityID, commit.UserID, commit.APIKeyID, commit.SessionHash, commit.ExpiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(7, 3))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT replay_sha256 FROM codex_continuity_turns WHERE thread_id = $1 AND request_id = $2")).
		WithArgs(int64(7), commit.RequestID).
		WillReturnRows(sqlmock.NewRows([]string{"replay_sha256"}).AddRow(commit.ReplaySHA256))
	mock.ExpectCommit()

	require.NoError(t, repo.CommitCompleted(context.Background(), commit))
	require.NoError(t, mock.ExpectationsWereMet())
}
