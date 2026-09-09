package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestDeleteProxyIfUnusedRejectsBackupReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM proxies WHERE id = $1 AND deleted_at IS NULL FOR UPDATE")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM accounts WHERE proxy_id = $1 AND deleted_at IS NULL")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM proxies WHERE backup_proxy_id = $1 AND deleted_at IS NULL")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err = deleteProxyIfUnusedOnClient(context.Background(), client, 9)

	require.ErrorIs(t, err, service.ErrProxyBackupInUse)
	require.NoError(t, mock.ExpectationsWereMet())
}
