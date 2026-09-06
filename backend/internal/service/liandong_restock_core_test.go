package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
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

func TestLiandongRestockRemoteFailureIsRetryableButUnknownOutcomeIsNot(t *testing.T) {
	t.Run("remote failure", func(t *testing.T) {
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
		if err := svc.RunOnce(context.Background(), true); err == nil {
			t.Fatal("expected definite remote rejection")
		}
		batches, err := svc.loadBatchStatuses(context.Background(), 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(batches) != 1 || batches[0].Status != liandongBatchStatusFailed {
			t.Fatalf("failure status = %+v, want failed", batches)
		}
		if err := svc.RunOnce(context.Background(), true); err != nil {
			t.Fatal(err)
		}
		if uploadCount.Load() != 2 {
			t.Fatalf("upload count = %d, want retry after definite failure", uploadCount.Load())
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			_, _ = io.WriteString(w, `{"code":1,"data":{"total":0}}`)
		case "/merchantApi/GoodsCardStorage/add":
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
	defer export.Reader.Close()
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
}

func TestLiandongManualJobOutlivesRequestContext(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			close(requestStarted)
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
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/merchantApi/goodsCardStorage/list":
			close(requestStarted)
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
