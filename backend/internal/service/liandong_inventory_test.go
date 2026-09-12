package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type liandongMerchantStockFixture struct {
	mu    sync.Mutex
	codes map[int64]map[string]struct{}
}

func (f *liandongMerchantStockFixture) add(t *testing.T, request *http.Request) {
	t.Helper()
	var payload struct {
		GoodsID int64  `json:"goods_id"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		t.Error(err)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.codes == nil {
		f.codes = make(map[int64]map[string]struct{})
	}
	if f.codes[payload.GoodsID] == nil {
		f.codes[payload.GoodsID] = make(map[string]struct{})
	}
	for _, code := range strings.Split(payload.Content, "\n") {
		if code != "" {
			f.codes[payload.GoodsID][code] = struct{}{}
		}
	}
}

func (f *liandongMerchantStockFixture) writeTotal(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	var payload struct {
		GoodsID int64 `json:"goods_id"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		t.Error(err)
		return
	}
	f.mu.Lock()
	total := len(f.codes[payload.GoodsID])
	f.mu.Unlock()
	if err := json.NewEncoder(writer).Encode(map[string]any{"code": 1, "data": map[string]int{"total": total}}); err != nil {
		t.Error(err)
	}
}

func TestLiandongInventoryShowsCountsWithoutClaimingIdentityOrClearingLatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/merchantApi/goodsCardStorage/list", r.URL.Path)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "0", payload["status"])
		_, _ = w.Write([]byte(`{"code":1,"data":{"total":1}}`))
	}))
	defer server.Close()
	svc, _, redeem := newLiandongTestService(server.URL)
	svc.memoryBatches = map[string]*liandongMemoryBatch{"batch": {
		Batch: liandongRestockPendingBatch{GoodsID: 42}, Codes: []string{"unused", "used", "disabled", "expired", "missing"},
	}}
	expired := time.Now().Add(-time.Hour)
	redeem.codes = map[string]*RedeemCode{
		"unused": {Code: "unused", Status: StatusUnused}, "used": {Code: "used", Status: StatusUsed},
		"disabled": {Code: "disabled", Status: "disabled"}, "expired": {Code: "expired", Status: StatusUnused, ExpiresAt: &expired},
	}
	state := &LiandongRestockState{Products: svc.products, ReconciliationRequired: true}
	require.NoError(t, svc.saveState(context.Background(), state))
	report, err := svc.Inventory(context.Background())
	require.NoError(t, err)
	require.True(t, report.ReconciliationRequired)
	require.Len(t, report.Rows, 1)
	row := report.Rows[0]
	require.Equal(t, &LiandongLocalInventory{Batches: 1, AllocatedCodes: 5, CreatedCodes: 4, UnusedCodes: 1, UsedCodes: 1, DisabledCodes: 1, OtherCodes: 1, MissingCodes: 1}, row.Local)
	require.Equal(t, 0, *row.QuantityDelta)
	require.False(t, row.IdentityVerified)
	require.Equal(t, "quantity_difference_identity_unknown", row.Comparison)
	saved, err := svc.loadState(context.Background())
	require.NoError(t, err)
	require.True(t, saved.ReconciliationRequired)
}

func TestLiandongInventorySQLAggregationResultAccounting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	svc := &LiandongRestockService{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(liandongLocalInventorySQL)).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"batches", "allocated", "created", "unused", "used", "disabled"}).AddRow(2, 7, 6, 3, 1, 1))
	counts, err := svc.localLiandongInventory(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, counts.MissingCodes)
	require.Equal(t, 1, counts.OtherCodes)
	require.Equal(t, 3, counts.UnusedCodes)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiandongInventoryMissingMerchantTotalRemainsUnknown(t *testing.T) {
	for _, data := range []string{`{}`, `{"total":null}`, `{"total":-1}`, `{"total":"3"}`} {
		t.Run(data, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"code":1,"data":` + data + `}`))
			}))
			defer server.Close()
			svc, _, _ := newLiandongTestService(server.URL)
			report, err := svc.Inventory(context.Background())
			require.NoError(t, err)
			require.Nil(t, report.Rows[0].MerchantUnsold)
			require.Nil(t, report.Rows[0].QuantityDelta)
			require.Equal(t, "unknown", report.Rows[0].Comparison)
			require.False(t, report.Rows[0].IdentityVerified)
		})
	}
}

func TestLiandongInventoryPostUploadMismatchBlocksRegenerationAndRetry(t *testing.T) {
	for _, after := range []string{`{"total":0}`, `{"total":1}`, `{"total":3}`, `{}`} {
		t.Run(after, func(t *testing.T) {
			var uploads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/merchantApi/GoodsCardStorage/add" {
					uploads.Add(1)
					_, _ = w.Write([]byte(`{"code":1,"data":{}}`))
					return
				}
				data := `{"total":0}`
				if uploads.Load() > 0 {
					data = after
				}
				_, _ = w.Write([]byte(`{"code":1,"data":` + data + `}`))
			}))
			defer server.Close()
			svc, settings, redeem := newLiandongTestService(server.URL)
			svc.products[0].TargetStock = 2
			err := svc.RunOnce(context.Background(), true)
			require.ErrorIs(t, err, ErrLiandongNeedsReconciliation)
			require.Len(t, redeem.codes, 2)
			batches, err := svc.loadBatchStatuses(context.Background(), 20)
			require.NoError(t, err)
			require.Equal(t, "needs_reconciliation", batches[0].Status)
			require.Equal(t, 1, batches[0].SegmentsUploaded)
			require.ErrorIs(t, svc.RunOnce(context.Background(), true), ErrLiandongNeedsReconciliation)
			require.Equal(t, int32(1), uploads.Load())
			require.Len(t, redeem.codes, 2)
			restarted, _, _ := newLiandongTestService(server.URL)
			restarted.settingRepo = settings
			restarted.redeem = redeem
			restarted.products = svc.products
			require.ErrorIs(t, restarted.RunOnce(context.Background(), true), ErrLiandongNeedsReconciliation)
			require.Equal(t, int32(1), uploads.Load())
		})
	}
}

func TestLiandongInventoryMatchingUploadQuantityDoesNotProveIdentity(t *testing.T) {
	var uploaded atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/merchantApi/GoodsCardStorage/add" {
			uploaded.Store(true)
			_, _ = w.Write([]byte(`{"code":1,"data":{}}`))
			return
		}
		stock := 0
		if uploaded.Load() {
			stock = 2
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "data": map[string]int{"total": stock}})
	}))
	defer server.Close()
	svc, _, _ := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 2
	require.NoError(t, svc.RunOnce(context.Background(), true))
	report, err := svc.Inventory(context.Background())
	require.NoError(t, err)
	require.Equal(t, "quantity_match_identity_unknown", report.Rows[0].Comparison)
	require.False(t, report.Rows[0].IdentityVerified)
	require.Equal(t, 0, *report.Rows[0].QuantityDelta)
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	require.NotContains(t, string(raw), strings.Repeat("s", 32))
}

func TestLiandongInventoryLocalReadFailureDoesNotInventZeroCounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	svc, _, _ := newLiandongTestService("https://example.invalid")
	svc.db = db
	svc.token = ""
	mock.ExpectQuery(regexp.QuoteMeta(liandongLocalInventorySQL)).WithArgs(int64(42)).WillReturnError(errors.New("private driver detail"))
	report, err := svc.Inventory(context.Background())
	require.NoError(t, err)
	require.Nil(t, report.Rows[0].Local)
	require.Nil(t, report.Rows[0].QuantityDelta)
	require.Equal(t, "local_inventory_unavailable", report.Rows[0].LocalError)
	require.NoError(t, mock.ExpectationsWereMet())
}
