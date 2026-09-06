package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLiandongRestockPersistsProductMappingAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := &LiandongRestockService{db: db}
	product := LiandongRestockProduct{CNYAmount: 20, USDCredit: 2.78, GoodsID: 42, GrantType: "balance", ExternalURL: "https://ldxp.cn/goods/42", Version: 1, Enabled: true}
	keyInput := "balance:42:20:2.78000000:https://ldxp.cn/goods/42:1"
	digest := sha256.Sum256([]byte(keyInput))
	mappingKey := hex.EncodeToString(digest[:])

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_product_mappings")).WithArgs(product.GoodsID, mappingKey).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_product_mappings")).WithArgs(mappingKey, product.GoodsID, product.CNYAmount, product.GrantType, product.USDCredit, product.ExternalURL, product.Version, product.Enabled, 1).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	require.NoError(t, svc.persistProductMappings(context.Background(), []LiandongRestockProduct{product}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiandongRestockPersistsBatchLifecycleAndReadsStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := &LiandongRestockService{db: db}
	created := "2026-09-06T10:00:00Z"
	batch := &liandongRestockPendingBatch{BatchID: "batch-1", GoodsID: 42, CNYAmount: 20, USDCredit: 2.78, Count: 2, CreatedAt: created}
	codes := []string{"LD-11111111-22222222-33333333-44444444", "LD-AAAAAAAA-BBBBBBBB-CCCCCCCC-DDDDDDDD"}
	digest := sha256.Sum256([]byte(codes[0] + "\n" + codes[1]))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_batches")).WithArgs(batch.BatchID, nil, batch.GoodsID, batch.CNYAmount, batch.USDCredit, len(codes), hex.EncodeToString(digest[:]), batch.RemoteStockBefore, batch.CreatedAt, liandongMappingKey(LiandongRestockProduct{CNYAmount: batch.CNYAmount, USDCredit: batch.USDCredit, GoodsID: batch.GoodsID, GrantType: "balance", Version: 1}), 1, "balance", "", 2, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_segments")).WithArgs(batch.BatchID, 0, 0, len(codes), hex.EncodeToString(digest[:])).WillReturnResult(sqlmock.NewResult(0, 1))
	for i, code := range codes {
		codeDigest := sha256.Sum256([]byte(code))
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_batch_codes")).WithArgs(batch.BatchID, hex.EncodeToString(codeDigest[:]), code[:11], i).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	require.NoError(t, svc.recordBatchPending(context.Background(), batch, codes))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_batches")).WithArgs(batch.BatchID, 7).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.markBatchUploaded(context.Background(), batch.BatchID, 7))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_batches")).WithArgs(batch.BatchID, "failed for test").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, svc.markBatchFailed(context.Background(), batch.BatchID, errTestLiandongPersistence{}))

	createdAt := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT batch_id, goods_id, cny_amount::text, code_count, status")).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"batch_id", "goods_id", "cny_amount", "code_count", "status", "remote_stock_before", "remote_stock_after", "error", "created_at", "uploaded_at", "updated_at"}).
			AddRow("batch-1", int64(42), "20.00", 2, "uploaded", 3, 7, nil, createdAt, createdAt.Add(30*time.Second), updatedAt))
	statuses, readErr := svc.loadBatchStatuses(context.Background(), 20)
	require.NoError(t, readErr)
	require.Len(t, statuses, 1)
	require.Equal(t, "batch-1", statuses[0].BatchID)
	require.Equal(t, 20.0, statuses[0].CNYAmount)
	require.Equal(t, 7, *statuses[0].RemoteStockAfter)
	require.Equal(t, "uploaded", statuses[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

type errTestLiandongPersistence struct{}

func (errTestLiandongPersistence) Error() string { return "failed for test" }
