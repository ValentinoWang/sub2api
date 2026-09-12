package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestLiandongRestockTargetDefaultsAndLegacyCompatibility(t *testing.T) {
	products, err := validateLiandongConfiguration("token", strings.Repeat("s", 32), []LiandongRestockProduct{{
		CNYAmount: 20, USDCredit: 2.78, GoodsID: 42, Threshold: 500, RestockCount: 50,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if products[0].TargetStock != liandongDefaultTargetStock {
		t.Fatalf("new normalized target = %d, want %d", products[0].TargetStock, liandongDefaultTargetStock)
	}
	legacy := normalizeStoredLiandongProducts([]LiandongRestockProduct{{
		CNYAmount: 20, USDCredit: 2.78, GoodsID: 42, Threshold: 500, RestockCount: 50,
	}})
	if legacy[0].TargetStock != 500 {
		t.Fatalf("legacy target = %d, want threshold safety floor 500", legacy[0].TargetStock)
	}
}

func TestLiandongRestockPlanUsesExactTargetGap(t *testing.T) {
	for _, test := range []struct {
		name    string
		current int
		want    int
	}{
		{name: "empty", current: 0, want: 50000},
		{name: "partial", current: 12000, want: 38000},
		{name: "at target", current: 50000, want: 0},
		{name: "above target", current: 60000, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := liandongPlanAddition(test.current, liandongDefaultTargetStock)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("planned = %d, want %d", got, test.want)
			}
		})
	}
}

func TestLiandongRestockGeneratesUniqueTwentyCharacterCodes(t *testing.T) {
	svc := &LiandongRestockService{codeSecret: []byte(strings.Repeat("s", 32))}
	codes, err := svc.deriveCodesChecked(&liandongRestockPendingBatch{BatchID: "batch-50000", Count: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 50000 {
		t.Fatalf("generated %d codes, want 50000", len(codes))
	}
	if err := validateLiandongCodeSet(codes); err != nil {
		t.Fatal(err)
	}
	for _, code := range codes {
		if len(code) != liandongCodeLength || strings.IndexFunc(code, func(r rune) bool {
			return !strings.ContainsRune(liandongCodeAlphabet, r)
		}) >= 0 {
			t.Fatalf("invalid code %q", code)
		}
		if !strings.ContainsAny(code, liandongCodeDigits) || !strings.ContainsAny(code, liandongCodeLower) || !strings.ContainsAny(code, liandongCodeUpper) {
			t.Fatalf("code %q does not contain every required character category", code)
		}
	}
}

func TestLiandongRestockRetryKeepsCodeSetIdentical(t *testing.T) {
	svc := &LiandongRestockService{codeSecret: []byte(strings.Repeat("s", 32))}
	batch := &liandongRestockPendingBatch{BatchID: "stable-batch", Count: 2101}
	first, err := svc.deriveCodesChecked(batch)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.deriveCodesChecked(batch)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("retry changed deterministic code set")
	}
	if liandongCodesDigest(first) != liandongCodesDigest(second) {
		t.Fatal("retry changed deterministic code digest")
	}
}

func TestLiandongRestockSegmentsAccountForEveryCode(t *testing.T) {
	ranges := liandongSegmentRanges(2501)
	if len(ranges) != 3 {
		t.Fatalf("segment count = %d, want 3", len(ranges))
	}
	total := 0
	for i, bounds := range ranges {
		if bounds[0] != total {
			t.Fatalf("segment %d starts at %d, want %d", i, bounds[0], total)
		}
		if bounds[1] <= 0 || bounds[1] > liandongSegmentSize {
			t.Fatalf("segment %d count = %d, outside 1..%d", i, bounds[1], liandongSegmentSize)
		}
		total += bounds[1]
	}
	if total != 2501 {
		t.Fatalf("segment total = %d, want 2501", total)
	}
}

func TestLiandongRestockUploadFailureNeedsReconciliation(t *testing.T) {
	t.Run("application rejection", func(t *testing.T) {
		var uploadCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/merchantApi/goodsCardStorage/list":
				_, _ = io.WriteString(w, `{"code":1,"data":{"total":0}}`)
			case "/merchantApi/GoodsCardStorage/add":
				if uploadCount.Add(1) == 1 {
					_, _ = io.WriteString(w, `{"code":0,"msg":"rejected"}`)
					return
				}
				_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		svc, _, _ := newLiandongTestService(server.URL)
		if err := svc.RunOnce(context.Background(), true); !errors.Is(err, ErrLiandongNeedsReconciliation) {
			t.Fatalf("application rejection error = %v, want reconciliation", err)
		}
		batches, err := svc.loadBatchStatuses(context.Background(), 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || batches[0].Status != liandongBatchStatusNeedsReconciliation {
			t.Fatalf("failure status = %+v, want needs_reconciliation", batches)
		}
		if err := svc.RunOnce(context.Background(), true); !errors.Is(err, ErrLiandongNeedsReconciliation) {
			t.Fatalf("retry error = %v, want reconciliation gate", err)
		}
		if uploadCount.Load() != 1 {
			t.Fatalf("upload count = %d, want no blind retry", uploadCount.Load())
		}
	})

	t.Run("unknown outcome", func(t *testing.T) {
		var uploadCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/merchantApi/goodsCardStorage/list" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"code":1,"data":{"total":0}}`)
				return
			}
			if r.URL.Path == "/merchantApi/GoodsCardStorage/add" {
				uploadCount.Add(1)
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					t.Error("test server does not support hijacking")
					return
				}
				connection, _, err := hijacker.Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = connection.Close()
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()
		svc, _, _ := newLiandongTestService(server.URL)
		if err := svc.RunOnce(context.Background(), true); err == nil {
			t.Fatal("expected unknown remote outcome")
		}
		batches, err := svc.loadBatchStatuses(context.Background(), 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || batches[0].Status != liandongBatchStatusNeedsReconciliation {
			t.Fatalf("unknown-outcome status = %+v, want needs_reconciliation", batches)
		}
		if err := svc.RunOnce(context.Background(), true); !errors.Is(err, ErrLiandongNeedsReconciliation) {
			t.Fatalf("retry error = %v, want reconciliation gate", err)
		}
		if uploadCount.Load() != 1 {
			t.Fatalf("upload count = %d, want no blind retry", uploadCount.Load())
		}
	})
}

func TestLiandongRestockPreviewAndManualJobRemainOperationallySeparate(t *testing.T) {
	var uploadCount atomic.Int32
	stock := &liandongMerchantStockFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			stock.writeTotal(t, w, r)
		case "/merchantApi/GoodsCardStorage/add":
			stock.add(t, r)
			uploadCount.Add(1)
			_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	svc, _, redeem := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 4
	preview, err := svc.Preview(context.Background(), []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Products) != 1 || preview.Products[0].CurrentStock == nil || preview.Products[0].Planned != 4 || preview.Products[0].TargetStock != 4 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if uploadCount.Load() != 0 || len(redeem.codes) != 0 || len(svc.memoryBatches) != 0 {
		t.Fatal("preview performed a write")
	}
	job, err := svc.StartManualJob(context.Background(), []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != LiandongRestockJobQueued {
		t.Fatalf("manual job returned %q, want queued", job.Status)
	}
	svc.waitForLiandongManualJobs()
	readBack, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if readBack.Status != LiandongRestockJobCompleted || readBack.TotalPlanned != 4 || readBack.TotalUploaded != 4 {
		t.Fatalf("unexpected completed job: %+v", readBack)
	}
	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.CurrentJob == nil || status.CurrentJob.JobID != job.JobID || len(status.Jobs) != 1 {
		t.Fatalf("status did not expose durable job summary: %+v", status)
	}
	export, err := svc.ExportJob(context.Background(), readBack.JobID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = export.Reader.Close() }()
	content, err := io.ReadAll(export.Reader)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if export.CodeCount != 4 || len(lines) != 4 {
		t.Fatalf("export count = %d and lines = %d, want 4", export.CodeCount, len(lines))
	}
	if err := validateLiandongCodeSet(lines); err != nil {
		t.Fatal(err)
	}
	svc.configMu.Lock()
	svc.codeSecret = []byte(strings.Repeat("rotated", 8))
	svc.configMu.Unlock()
	if _, err := svc.ExportJob(context.Background(), readBack.JobID); err == nil {
		t.Fatal("historical export must be refused after code-secret rotation")
	}
}

func TestLiandongManualJobOutlivesRequestContext(t *testing.T) {
	stock := &liandongMerchantStockFixture{}
	requestStarted := make(chan struct{})
	var requestStartedOnce sync.Once
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			requestStartedOnce.Do(func() { close(requestStarted) })
			select {
			case <-releaseRequest:
				stock.writeTotal(t, w, r)
			case <-r.Context().Done():
			}
		case "/merchantApi/GoodsCardStorage/add":
			stock.add(t, r)
			_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 1
	requestContext, cancel := context.WithCancel(context.Background())
	job, err := svc.StartManualJob(requestContext, []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != LiandongRestockJobQueued {
		t.Fatalf("manual job status = %q, want queued", job.Status)
	}
	cancel()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("background inventory request did not start")
	}
	close(releaseRequest)
	svc.waitForLiandongManualJobs()
	completed, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != LiandongRestockJobCompleted || completed.TotalUploaded != 1 {
		t.Fatalf("request cancellation affected manual job: %+v", completed)
	}
}

func TestLiandongManualJobRejectsResumeWhileRunning(t *testing.T) {
	requestStarted := make(chan struct{})
	var requestStartedOnce sync.Once
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			requestStartedOnce.Do(func() { close(requestStarted) })
			select {
			case <-releaseRequest:
				_, _ = io.WriteString(w, `{"code":1,"data":{"total":0}}`)
			case <-r.Context().Done():
			}
		case "/merchantApi/GoodsCardStorage/add":
			_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 1
	job, err := svc.StartManualJob(context.Background(), []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("background inventory request did not start")
	}
	resumed, err := svc.ResumeJob(context.Background(), job.JobID)
	if !errors.Is(err, ErrLiandongRunBusy) {
		t.Fatalf("resume error = %v, want busy", err)
	}
	if resumed == nil || resumed.Status != LiandongRestockJobRunning {
		t.Fatalf("resume changed active job: %+v", resumed)
	}
	readBack, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if readBack.Status != LiandongRestockJobRunning {
		t.Fatalf("active job state = %q, want running", readBack.Status)
	}
	close(releaseRequest)
	svc.waitForLiandongManualJobs()
}

func TestLiandongManualJobDoesNotResumeNeedsReconciliation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":1,"data":{"total":0}}`)
		case "/merchantApi/GoodsCardStorage/add":
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("test server does not support hijacking")
				return
			}
			connection, _, err := hijacker.Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 1
	job, err := svc.StartManualJob(context.Background(), []int64{42})
	if err != nil {
		t.Fatal(err)
	}
	svc.waitForLiandongManualJobs()
	needsReconciliation, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if needsReconciliation.Status != LiandongRestockJobNeedsReconciliation {
		t.Fatalf("job state = %q, want needs_reconciliation", needsReconciliation.Status)
	}
	resumed, err := svc.ResumeJob(context.Background(), job.JobID)
	if !errors.Is(err, ErrLiandongNeedsReconciliation) {
		t.Fatalf("resume error = %v, want reconciliation gate", err)
	}
	if resumed == nil || resumed.Status != LiandongRestockJobNeedsReconciliation {
		t.Fatalf("resume changed reconciliation state: %+v", resumed)
	}
}

func TestLiandongRestockBatchSnapshotKeepsMappingVersionAndTarget(t *testing.T) {
	svc, _, _ := newLiandongTestService("https://ldxp.cn")
	product := svc.products[0]
	product.GrantType = "balance"
	product.Version = 7
	product.ExternalURL = "https://ldxp.cn/goods/42"
	product.TargetStock = 50000
	batch := newLiandongPendingBatch(product, 12000, 38000, "job-1", "2026-09-06T00:00:00Z")
	if batch.MappingKey != liandongMappingKey(product) || batch.Version != 7 || batch.TargetStock != 50000 || batch.Count != 38000 {
		t.Fatalf("batch snapshot lost mapping fields: %+v", batch)
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "merchant") || strings.Contains(string(raw), "secret") {
		t.Fatal("batch snapshot unexpectedly contains credentials")
	}
}

func TestLiandongCanceledUploadPersistsReconciliationBeforeReturning(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var requestStartedOnce sync.Once
	var uploadCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchantApi/GoodsCardStorage/add" {
			http.NotFound(w, r)
			return
		}
		uploadCount.Add(1)
		requestStartedOnce.Do(func() { close(requestStarted) })
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	product := svc.products[0]
	product.TargetStock = 1
	batch := newLiandongPendingBatch(product, 0, 1, "", "2026-09-06T00:00:00Z")
	batch.BatchID = "cancelled-upload"
	batch.CodeSecretDigest = svc.currentLiandongCodeSecretDigest()
	state := &LiandongRestockState{PendingBatch: batch}
	requestContext, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- svc.fulfillPendingBatch(requestContext, state) }()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("upload request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrLiandongNeedsReconciliation) {
			t.Fatalf("cancelled upload error = %v, want reconciliation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled upload did not return")
	}
	close(releaseHandler)
	if !state.ReconciliationRequired {
		t.Fatal("cancelled upload did not latch durable recovery state")
	}
	batches, err := svc.loadBatchStatuses(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].Status != liandongBatchStatusNeedsReconciliation {
		t.Fatalf("batch status = %+v, want needs_reconciliation", batches)
	}
	segments, err := svc.loadLiandongSegmentStatuses(context.Background(), batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].Status != liandongSegmentStatusNeedsReconciliation {
		t.Fatalf("segment status = %+v, want needs_reconciliation", segments)
	}
	if err := svc.fulfillPendingBatch(context.Background(), state); !errors.Is(err, ErrLiandongNeedsReconciliation) {
		t.Fatalf("latched retry error = %v, want reconciliation", err)
	}
	if uploadCount.Load() != 1 {
		t.Fatalf("cancelled upload was replayed %d times", uploadCount.Load())
	}
}

func TestLiandongSuccessfulUploadWithLocalAckFailureNeedsReconciliation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	var uploadCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchantApi/GoodsCardStorage/add" {
			http.NotFound(w, r)
			return
		}
		uploadCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	svc.db = db
	product := svc.products[0]
	product.TargetStock = 1
	batch := newLiandongPendingBatch(product, 0, 1, "", "2026-09-06T00:00:00Z")
	batch.BatchID = "ack-failure"
	batch.CodeSecretDigest = svc.currentLiandongCodeSecretDigest()
	codes, err := svc.deriveCodesChecked(batch)
	if err != nil {
		t.Fatal(err)
	}
	digest := liandongCodesDigest(codes)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_batches")).
		WithArgs(batch.BatchID, nil, batch.GoodsID, batch.CNYAmount, batch.USDCredit, len(codes), digest, sqlmock.AnyArg(), batch.CreatedAt, batch.MappingKey, batch.Version, batch.GrantType, batch.ExternalURL, batch.TargetStock, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_segments")).
		WithArgs(batch.BatchID, 0, 0, len(codes), digest).
		WillReturnResult(sqlmock.NewResult(0, 1))
	codeDigest := sha256.Sum256([]byte(codes[0]))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO liandong_restock_batch_codes")).
		WithArgs(batch.BatchID, hex.EncodeToString(codeDigest[:]), codes[0][:11], 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM liandong_restock_batches WHERE batch_id = $1")).
		WithArgs(batch.BatchID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(liandongBatchStatusPending))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT segment_no, ordinal_start, code_count, code_sha256, status, error, uploaded_at, updated_at FROM liandong_restock_segments WHERE batch_id = $1 ORDER BY segment_no")).
		WithArgs(batch.BatchID).
		WillReturnRows(sqlmock.NewRows([]string{"segment_no", "ordinal_start", "code_count", "code_sha256", "status", "error", "uploaded_at", "updated_at"}).
			AddRow(0, 0, 1, digest, liandongSegmentStatusPending, nil, nil, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_segments")).
		WithArgs(batch.BatchID, 0, liandongSegmentStatusCodesCreated, nil, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_segments")).
		WithArgs(batch.BatchID, 0, liandongSegmentStatusUploaded, nil, true).
		WillReturnError(errors.New("ack unavailable"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_segments")).
		WithArgs(batch.BatchID, 0, liandongSegmentStatusNeedsReconciliation, "Liandong restock operation failed", false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE liandong_restock_batches")).
		WithArgs(batch.BatchID, "Liandong restock operation failed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	state := &LiandongRestockState{PendingBatch: batch}
	err = svc.fulfillPendingBatch(context.Background(), state)
	if !errors.Is(err, ErrLiandongNeedsReconciliation) {
		t.Fatalf("ack failure error = %v, want reconciliation", err)
	}
	if uploadCount.Load() != 1 {
		t.Fatalf("successful remote upload was attempted %d times", uploadCount.Load())
	}
	if !state.ReconciliationRequired {
		t.Fatal("local acknowledgement failure did not latch recovery")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLiandongManualResumeContinuesSavedMultiProductPlan(t *testing.T) {
	var uploadCount atomic.Int32
	stock := &liandongMerchantStockFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			stock.writeTotal(t, w, r)
		case "/merchantApi/GoodsCardStorage/add":
			stock.add(t, r)
			uploadCount.Add(1)
			_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _, redeem := newLiandongTestService(server.URL)
	redeem.failCreateOnce = true
	svc.products[0].TargetStock = 1
	svc.products = append(svc.products, LiandongRestockProduct{
		CNYAmount: 30, USDCredit: 4.17, GoodsID: 43, RestockCount: 3, TargetStock: 1, Enabled: true,
	})
	job, err := svc.StartManualJob(context.Background(), []int64{42, 43})
	if err != nil {
		t.Fatal(err)
	}
	svc.waitForLiandongManualJobs()
	failed, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != LiandongRestockJobFailed || len(failed.Products) != 2 {
		t.Fatalf("first job state = %+v, want failed two-product plan", failed)
	}
	resumed, err := svc.ResumeJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != LiandongRestockJobQueued {
		t.Fatalf("resume state = %q, want queued", resumed.Status)
	}
	svc.waitForLiandongManualJobs()
	completed, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != LiandongRestockJobCompleted || len(completed.Products) != 2 || len(completed.Batches) != 2 || completed.TotalUploaded != 2 {
		t.Fatalf("resumed job state = %+v, want both products completed", completed)
	}
	if uploadCount.Load() != 2 {
		t.Fatalf("upload count = %d, want two confirmed uploads after the safe local retry", uploadCount.Load())
	}
}

func TestLiandongStopWorkerWaitsForAutomaticCycleAndClosesAdmission(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var requestStartedOnce sync.Once
	var releaseHandlerOnce sync.Once
	release := func() { releaseHandlerOnce.Do(func() { close(releaseHandler) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchantApi/goodsCardStorage/list" {
			http.NotFound(w, r)
			return
		}
		requestStartedOnce.Do(func() { close(requestStarted) })
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	defer func() {
		release()
		server.Close()
	}()

	svc, _, _ := newLiandongTestService(server.URL)
	if err := svc.saveState(context.Background(), &LiandongRestockState{Enabled: true, Products: cloneLiandongProducts(svc.products)}); err != nil {
		t.Fatal(err)
	}
	if !svc.scheduleAutomaticCycle() {
		t.Fatal("automatic cycle was not admitted")
	}
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("automatic cycle did not start")
	}
	stopDone := make(chan struct{})
	go func() {
		svc.StopWorker()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("StopWorker did not wait for and cancel the automatic cycle")
	}
	release()
	if svc.scheduleAutomaticCycle() {
		t.Fatal("automatic cycle was admitted after shutdown")
	}
}

func TestLiandongStaleRunningJobCanBeResumed(t *testing.T) {
	stock := &liandongMerchantStockFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			stock.writeTotal(t, w, r)
		case "/merchantApi/GoodsCardStorage/add":
			stock.add(t, r)
			_, _ = io.WriteString(w, `{"code":1,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc, _, _ := newLiandongTestService(server.URL)
	svc.products[0].TargetStock = 1
	plan := liandongPreviewItem(svc.products[0], nil, 0, "queued")
	now := time.Now().UTC()
	job := &LiandongRestockJobSummary{
		JobID: "stale-running", Status: LiandongRestockJobRunning, SelectedGoods: []int64{42},
		CodeSecretDigest: svc.currentLiandongCodeSecretDigest(), Products: []LiandongRestockPreviewItem{plan},
		CreatedAt: now.Add(-5 * time.Minute).Format(time.RFC3339), UpdatedAt: now.Add(-5 * time.Minute).Format(time.RFC3339),
	}
	svc.memoryJobs = map[string]*LiandongRestockJobSummary{job.JobID: job}
	queued, err := svc.ResumeJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if queued.Status != LiandongRestockJobQueued {
		t.Fatalf("stale resume state = %q, want queued", queued.Status)
	}
	svc.waitForLiandongManualJobs()
	completed, err := svc.GetJob(context.Background(), job.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != LiandongRestockJobCompleted {
		t.Fatalf("stale running job state = %+v, want completed", completed)
	}
}

func TestLiandongExportRejectsNonCompletedJobs(t *testing.T) {
	svc, _, _ := newLiandongTestService("https://ldxp.cn")
	for _, status := range []string{LiandongRestockJobQueued, LiandongRestockJobRunning, LiandongRestockJobFailed, LiandongRestockJobNeedsReconciliation} {
		jobID := "export-" + status
		svc.memoryJobs = map[string]*LiandongRestockJobSummary{
			jobID: {JobID: jobID, Status: status, SelectedGoods: []int64{42}},
		}
		if _, err := svc.ExportJob(context.Background(), jobID); err == nil {
			t.Fatalf("export for %s unexpectedly succeeded", status)
		}
	}
}
