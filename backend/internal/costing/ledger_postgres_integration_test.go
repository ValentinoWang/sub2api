//go:build costpg

package costing

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Explicit opt-in test. Missing configuration FAILS rather than becoming a skipped PASS.
func TestPostgresLedgerIntegration(t *testing.T) {
	dsn := os.Getenv("COST_LEDGER_TEST_DSN")
	if dsn == "" || os.Getenv("COST_LEDGER_TEST_ALLOW_ISOLATED_SCHEMA") != "yes" {
		t.Fatal("requires an isolated test PostgreSQL DSN and explicit schema-test opt-in")
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	ddl, e := os.ReadFile("migrations/001_cost_center.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, string(ddl)); e != nil {
		t.Fatalf("isolated migration failed: %T", e)
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
}
