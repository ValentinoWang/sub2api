//go:build costpg || integration

package costing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Explicit opt-in test. Missing configuration FAILS rather than becoming a skipped PASS.
func TestPostgresLedgerIntegration(t *testing.T) {
	dsn := os.Getenv("COST_LEDGER_TEST_DSN")
	if dsn == "" {
		name := fmt.Sprintf("sub2api-cost-test-%d", time.Now().UnixNano())
		run := exec.Command("docker", "run", "--rm", "-d", "--name", name, "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_DB=cost_test", "-p", "127.0.0.1::5432", "postgres:16-alpine")
		if err := run.Run(); err != nil {
			t.Fatal("isolated PostgreSQL could not start", err)
		}
		t.Cleanup(func() {
			if err := exec.Command("docker", "rm", "-f", name).Run(); err != nil {
				t.Error("test container cleanup failed", err)
			}
		})
		port, err := exec.Command("docker", "port", name, "5432").Output()
		if err != nil {
			t.Fatal(err)
		}
		dsn = "postgres://postgres@" + strings.TrimSpace(string(port)) + "/cost_test?sslmode=disable"
	} else if os.Getenv("COST_LEDGER_TEST_ALLOW_ISOLATED_SCHEMA") != "yes" {
		t.Fatal("explicit isolated test opt-in required")
	}
	u, e := url.Parse(dsn)
	if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("test DSN must be a PostgreSQL URL")
	}
	root, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal("test database configuration invalid")
	}
	defer root.Close()
	deadline := time.Now().Add(60 * time.Second)
	for root.Ping() != nil {
		if time.Now().After(deadline) {
			t.Fatal("isolated PostgreSQL readiness timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	schema := fmt.Sprintf("cost_acceptance_%d", time.Now().UnixNano())
	if _, e = root.ExecContext(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal("could not create isolated test schema")
	}
	defer root.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	params := u.Query()
	params.Set("search_path", schema)
	u.RawQuery = params.Encode()
	db, e := sql.Open("postgres", u.String())
	if e != nil {
		t.Fatal("isolated connection unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	migrations, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range migrations {
		ddl, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecContext(ctx, string(ddl)); err != nil {
			t.Fatal("isolated migration", path, err)
		}
	}
	store := &SQLLedgerStore{Open: func() (*sql.DB, func(), error) { return db, func() {}, nil }}
	l := &Ledger{Store: store}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	cmd := LedgerCommand{Kind: "purchase", Purchase: &Purchase{Reference: "integration", Supplier: "synthetic", Asset: "synthetic", Tier: "pro5x", Kind: "subscription", Currency: "USD", Amount: "100", PaidAt: past, ServiceStart: past, ServiceEnd: future, Evidence: "synthetic", EvidenceRef: "test-only"}}
	if _, e = l.Append(ctx, 1, "integration-key", cmd); e != nil {
		t.Fatal(e)
	}
	if r, e := l.Append(ctx, 1, "integration-key", cmd); e != nil || !r.Replayed {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, `DELETE FROM cost_center_events`); e == nil {
		t.Fatal("append-only delete guard missing")
	}
	events, e := store.Snapshot(ctx)
	if e != nil || len(events) != 1 {
		t.Fatal(e)
	}
	if !strings.HasPrefix(events[0].ID, "cost-") {
		t.Fatal("invalid persisted identity")
	}
	t.Run("parallel_idempotency", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, err := l.Append(ctx, 1, "integration-key", cmd)
				if err != nil || !r.Replayed {
					t.Errorf("concurrent retry: %v", err)
				}
			}()
		}
		wg.Wait()
	})
	t.Run("FIFO_and_readonly_reconciliation", func(t *testing.T) {
		l.Clock = func() time.Time { return instant("2026-09-16T00:00:00Z") }
		// An independent event store inside the same disposable schema avoids clock mixing.
		if _, err := db.ExecContext(ctx, `CREATE TABLE accounts(id bigint,platform text,type text,status text,deleted_at timestamptz,credentials jsonb DEFAULT '{}');
  INSERT INTO accounts(id,platform,type,status,deleted_at) VALUES(7,'openai','oauth','active',NULL);
  CREATE TABLE usage_logs(account_id bigint,model text,created_at timestamptz,input_tokens bigint,output_tokens bigint,cache_read_tokens bigint,cache_creation_tokens bigint,total_cost numeric);
  INSERT INTO usage_logs VALUES(7,'m','2026-09-02T00:00:00Z',12,7,3,1,0.25),(7,'m','2026-09-09T00:00:00Z',20,8,4,0,0.50);`); err != nil {
			t.Fatal(err)
		}
		source := &SQLSource{Open: store.Open}
		accounts, err := source.Accounts(ctx)
		if err != nil || len(accounts) != 1 || accounts[0].ID != 7 {
			t.Fatal(accounts, err)
		}
		unboundQuery := SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-09-16T00:00:00Z", AsOf: time.Now().UTC().Format(time.RFC3339Nano)}
		unbound, err := source.Reconcile(ctx, l, unboundQuery)
		if err != nil || len(unbound.Rows) != 1 || unbound.Rows[0].Requests != 2 || unbound.Rows[0].Tier != "unassigned" {
			t.Fatal("unbound source usage must remain readable", unbound, err)
		}
		snapshots := &SourceSnapshotStore{Open: store.Open}
		saved, err := snapshots.Save(ctx, "test-source", 1, unboundQuery, unbound)
		if err != nil || saved.RequestCount != 2 || saved.StoredRequestCount != 2 || saved.AccountCount != 1 {
			t.Fatal("source snapshot readback", saved, err)
		}
		replayed, err := snapshots.Save(ctx, "test-source", 1, unboundQuery, unbound)
		if err != nil || replayed.ID != saved.ID {
			t.Fatal("source snapshot duplicated", err)
		}

		if _, err := db.ExecContext(ctx, `ALTER TABLE cost_center_source_snapshots DISABLE TRIGGER cost_center_source_no_rewrite`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE cost_center_source_snapshots SET payload=jsonb_set(payload,'{rows,0,requests}','999'::jsonb) WHERE snapshot_id=$1`, saved.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE cost_center_source_snapshots ENABLE TRIGGER cost_center_source_no_rewrite`); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshots.Save(ctx, "test-source", 1, unboundQuery, unbound); !errors.Is(err, ErrConflict) {
			t.Fatal("corrupted stored payload passed reconciliation", err)
		}
		sampleStore := &TrafficSampleStore{Open: store.Open}
		sample := NetworkSample{Source: "test", Interface: "eth0", Adapter: "linux_kernel", Scope: "container_interface", ObservedAt: "2026-09-01T00:00:00Z", Epoch: strings.Repeat("a", 64), RXBytes: "100", TXBytes: "200"}
		if err := sampleStore.Record(ctx, sample); err != nil {
			t.Fatal(err)
		}
		if err := sampleStore.Record(ctx, sample); err != nil {
			t.Fatal(err)
		}
		sample.RXBytes = "101"
		if err := sampleStore.Record(ctx, sample); !errors.Is(err, ErrConflict) {
			t.Fatal("same sample timestamp changed without conflict", err)
		}
		storedSamples, err := sampleStore.Latest(ctx)
		if err != nil || len(storedSamples) != 1 || storedSamples[0].RXBytes != "100" {
			t.Fatal(storedSamples, err)
		}

		if err = source.read(ctx, func(tx *sql.Tx) error { _, err := tx.ExecContext(ctx, `DELETE FROM accounts`); return err }); err == nil {
			t.Fatal("source transaction allowed writes")
		}
		// Use present clock and past business timestamps for all persisted extensions.
		l.Clock = nil
		add(t, l, "lot1", lot("lot1", "10", "100", "2026-09-01T00:00:00Z", "2026-10-01T00:00:00Z"))
		add(t, l, "lot2", lot("lot2", "40", "200", "2026-09-02T00:00:00Z", "2026-10-01T00:00:00Z"))
		add(t, l, "use", use("use", "150", "2026-09-03T00:00:00Z"))
		for _, p := range []AccountInterval{{"own", 7, "plus", "2026-09-01T00:00:00Z", "2026-09-08T00:00:00Z", "receipt-1"}, {"own", 7, "pro5x", "2026-09-08T00:00:00Z", "2026-10-01T00:00:00Z", "receipt-2"}} {
			add(t, l, p.Tier, LedgerCommand{Kind: "account_interval", AccountInterval: &p})
		}
		q := SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-09-16T00:00:00Z", AsOf: time.Now().UTC().Format(time.RFC3339Nano), Currency: "USD", Model: "m", Unit: "request"}
		report, err := source.Reconcile(ctx, l, q)
		if err != nil || len(report.Rows) != 2 || report.Rows[0].InputTokens != "12" || report.Rows[1].InputTokens != "20" {
			t.Fatal(report, err)
		}
		summary, err := l.Summary(ctx, q)
		if err != nil || summary.Prepaid.Uses[0].Cost != "20.000000" {
			t.Fatal(summary, err)
		}
		second := &Ledger{Store: &SQLLedgerStore{Open: store.Open}}
		again, err := second.Summary(ctx, q)
		if err != nil || again.Prepaid.Uses[0].Cost != summary.Prepaid.Uses[0].Cost {
			t.Fatal("reopened store lost FIFO", err)
		}

		radar := &RadarService{Open: store.Open, Client: &http.Client{Transport: radarRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(radarFixture))}, nil
		})}}
		if _, err := radar.Get(ctx); err != nil {
			t.Fatal("public reference did not persist", err)
		}
		var references int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM cost_center_public_references`).Scan(&references); err != nil || references != 1 {
			t.Fatal("public reference readback", err)
		}
		qs := &QuotaStore{Open: store.Open}
		obs := QuotaObservation{AccountID: 7, ObservedAt: past, Kind: "snapshot", Windows: []ObservedQuotaWindow{{Pool: "codex", Window: "primary", UsedPercent: 80, DurationSeconds: 18000, ResetAt: time.Now().Add(time.Hour).Unix()}}}
		if err := qs.Record(ctx, obs); err != nil {
			t.Fatal(err)
		}
		if err := qs.Record(ctx, obs); err != nil {
			t.Fatal(err)
		}
		items, err := qs.List(ctx, 7, time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339), future)
		if err != nil || len(items) != 1 {
			t.Fatal("quota dedup", items, err)
		}
		obs.Kind = "after_reset"
		obs.Windows[0].DurationSeconds = 0
		if err := qs.Record(ctx, obs); err != nil {
			t.Fatal("unknown duration observation rejected", err)
		}
		items, err = qs.List(ctx, 7, time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339), future)
		unknown := false
		for _, item := range items {
			if len(item.QualityFlags) > 0 && item.QualityFlags[0] == "WINDOW_DURATION_UNKNOWN" {
				unknown = true
			}
		}
		if err != nil || !unknown {
			t.Fatal("unknown duration lost its quality flag", err)
		}

	})

}
