package service

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLiandongRefundRequiresDurableStorageAndExplicitMerchantReference(t *testing.T) {
	ctx := context.Background()
	svc := &LiandongRestockService{}
	_, err := svc.PrepareUnusedCodeRefund(ctx, LiandongRefundPrepareRequest{ExternalOrderNo: "order", BatchID: "batch", Code: "test-code"})
	require.ErrorIs(t, err, ErrLiandongRefundUnavailable)
	_, err = svc.ConfirmUnusedCodeRefund(ctx, LiandongRefundConfirmRequest{ExternalOrderNo: "order"})
	require.ErrorIs(t, err, ErrLiandongRefundInvalid)
}

func TestLiandongRefundRollsBackAndRedactsStorageFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	svc := &LiandongRestockService{db: db}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock")).WillReturnError(errors.New("driver error code-secret-canary"))
	mock.ExpectRollback()
	_, err = svc.PrepareUnusedCodeRefund(context.Background(), LiandongRefundPrepareRequest{ExternalOrderNo: "order", BatchID: "batch", Code: "code-secret-canary"})
	require.ErrorIs(t, err, ErrLiandongRefundUnavailable)
	require.NotContains(t, err.Error(), "code-secret-canary")
	require.NoError(t, mock.ExpectationsWereMet())
}
