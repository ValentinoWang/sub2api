//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
)

func TestCostQuotaObserverKeepsPoolsAndActualObservationTime(t *testing.T) {
	var got costing.QuotaObservation
	service := NewOpenAIQuotaService(nil, nil, nil, nil, nil)
	service.costQuotaObserver = func(_ context.Context, q costing.QuotaObservation) error { got = q; return nil }
	at := time.Now().Add(-time.Minute).Unix()
	usage := &OpenAIQuotaUsage{FetchedAt: at, RateLimit: &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 80, LimitWindowSeconds: 18000, ResetAfterSeconds: 3600}}, AdditionalRateLimits: []OpenAIAdditionalRateLimit{{MeteredFeature: "spark", RateLimit: &OpenAIRateLimit{SecondaryWindow: &OpenAIRateLimitWindow{UsedPercent: 10, LimitWindowSeconds: 604800, ResetAt: at + 20000}}}}}
	service.observeCostQuota(context.Background(), 7, usage, "after_reset")
	if got.AccountID != 7 || got.Kind != "after_reset" || len(got.Windows) != 2 || got.Windows[0].Pool != "codex" || got.Windows[1].Pool != "spark" || got.Windows[0].ResetAt != at+3600 || got.ObservedAt != time.Unix(at, 0).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("incorrect observation: %+v", got)
	}
}
func TestCostQuotaObserverMissingWindowsAndSinkFailure(t *testing.T) {
	service := NewOpenAIQuotaService(nil, nil, nil, nil, nil)
	calls := 0
	service.costQuotaObserver = func(context.Context, costing.QuotaObservation) error { calls++; return errors.New("sink offline") }
	service.observeCostQuota(context.Background(), 1, &OpenAIQuotaUsage{}, "snapshot")
	if calls != 0 {
		t.Fatal("empty quota became observed capacity")
	}
	service.observeCostQuota(context.Background(), 1, &OpenAIQuotaUsage{FetchedAt: time.Now().Unix(), RateLimit: &OpenAIRateLimit{PrimaryWindow: &OpenAIRateLimitWindow{UsedPercent: 80, LimitWindowSeconds: 18000, ResetAfterSeconds: 600}}}, "snapshot")
	if calls != 1 {
		t.Fatal("observation sink not invoked")
	}
}

func TestCostQuotaHooksRunThroughQueryAndSuccessfulCache(t *testing.T) {
	account := &Account{ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"chatgpt_account_id": "fixture-account"}}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{100: account}}
	cache := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fixture-token"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("observer test unexpectedly issued a mutating upstream request")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/backend-api/wham/usage":
			_, _ = w.Write([]byte(`{"rate_limit":{"allowed":true,"primary_window":{"used_percent":80,"limit_window_seconds":18000,"reset_after_seconds":3600},"secondary_window":{"used_percent":20,"limit_window_seconds":604800,"reset_after_seconds":86400}},"rate_limit_reset_credits":{"available_count":0,"credits":[]}}`))
		case "/backend-api/wham/rate-limit-reset-credits":
			_, _ = w.Write([]byte(`{"available_count":0,"credits":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	svc := NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, cache, nil), newQuotaRedirectingFactory(server), nil)
	records := []costing.QuotaObservation{}
	svc.costQuotaObserver = func(_ context.Context, q costing.QuotaObservation) error { records = append(records, q); return nil }
	usage, err := svc.QueryUsage(context.Background(), 100)
	if err != nil || len(records) != 1 || records[0].Kind != "snapshot" {
		t.Fatal("query did not invoke observer", err, records)
	}
	if err := svc.CachePostResetSnapshot(context.Background(), 100, usage); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1].Kind != "after_reset" || repo.extraUpdateCalls != 1 {
		t.Fatal("successful cache did not invoke reset observer", records)
	}
	repo.extraUpdateErr = errors.New("cache unavailable")
	if err := svc.CachePostResetSnapshot(context.Background(), 100, usage); err == nil {
		t.Fatal("cache failure lost")
	}
	if len(records) != 2 {
		t.Fatal("failed cache was published as a successful reset observation")
	}
}
