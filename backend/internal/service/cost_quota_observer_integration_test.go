//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/costing"
	_ "github.com/lib/pq"
)

func TestCostQuotaQueryAndResetCachePersistToPostgres(t *testing.T) {
	name := fmt.Sprintf("sub2api-cost-hook-%d", time.Now().UnixNano())
	if err := exec.Command("docker", "run", "--rm", "-d", "--name", name, "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_DB=cost_hook", "-p", "127.0.0.1::5432", "postgres:16-alpine").Run(); err != nil {
		t.Fatal("isolated PostgreSQL unavailable", err)
	}
	t.Cleanup(func() {
		if err := exec.Command("docker", "rm", "-f", name).Run(); err != nil {
			t.Error("isolated container cleanup", err)
		}
	})
	port, err := exec.Command("docker", "port", name, "5432").Output()
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", "postgres://postgres@"+strings.TrimSpace(string(port))+"/cost_hook?sslmode=disable&connect_timeout=3")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for db.PingContext(ctx) != nil {
		select {
		case <-ctx.Done():
			t.Fatal("isolated PostgreSQL readiness timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
	paths, err := filepath.Glob("../costing/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		ddl, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecContext(ctx, string(ddl)); err != nil {
			t.Fatal(err)
		}
	}
	account := &Account{ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"chatgpt_account_id": "fixture-account"}}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{100: account}}
	tokenCache := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fixture-token"}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("unexpected upstream mutation")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/backend-api/wham/usage" {
			_, _ = w.Write([]byte(`{"rate_limit":{"primary_window":{"used_percent":80,"limit_window_seconds":18000,"reset_after_seconds":3600}},"rate_limit_reset_credits":{"available_count":0,"credits":[]}}`))
		} else {
			_, _ = w.Write([]byte(`{"available_count":0,"credits":[]}`))
		}
	}))
	defer upstream.Close()
	svc := NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, tokenCache, nil), newQuotaRedirectingFactory(upstream), nil)
	store := &costing.QuotaStore{Open: func() (*sql.DB, func(), error) { return db, func() {}, nil }}
	svc.costQuotaObserver = store.Record
	usage, err := svc.QueryUsage(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CachePostResetSnapshot(ctx, 100, usage); err != nil {
		t.Fatal(err)
	}
	rows, err := store.List(ctx, 100, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, row := range rows {
		kinds[row.Kind] = true
		if row.AccountID != 100 || len(row.Windows) != 1 || row.Windows[0].UsedPercent != 80 {
			t.Fatal(row)
		}
	}
	if len(rows) != 2 || !kinds["snapshot"] || !kinds["after_reset"] {
		t.Fatal("normal entrypoints did not persist both observation kinds", rows)
	}
}
