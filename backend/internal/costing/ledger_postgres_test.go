package costing

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// This is a SQL adapter contract double, NOT a real PostgreSQL server or syntax validator.
type contractDB struct {
	mu         sync.Mutex
	events     []LedgerEvent
	failCommit bool
	calls      []string
}
type contractDriver struct{ state *contractDB }
type contractConn struct {
	state   *contractDB
	inTx    bool
	locked  bool
	pending []LedgerEvent
}
type contractTx struct{ c *contractConn }
type contractRows struct {
	cols   []string
	values [][]driver.Value
	n      int
}

func (d contractDriver) Open(string) (driver.Conn, error) { return &contractConn{state: d.state}, nil }
func (c *contractConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *contractConn) Close() error { return nil }
func (c *contractConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *contractConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	c.state.mu.Lock()
	c.inTx = true
	c.pending = append([]LedgerEvent{}, c.state.events...)
	c.state.calls = append(c.state.calls, "begin")
	return &contractTx{c}, nil
}
func (tx *contractTx) Commit() error {
	c := tx.c
	c.state.calls = append(c.state.calls, "commit")
	defer c.state.mu.Unlock()
	c.inTx = false
	if c.state.failCommit {
		c.state.failCommit = false
		return errors.New("injected commit failure")
	}
	c.state.events = c.pending
	return nil
}
func (tx *contractTx) Rollback() error {
	c := tx.c
	if c.inTx {
		c.state.calls = append(c.state.calls, "rollback")
		c.inTx = false
		c.state.mu.Unlock()
	}
	return nil
}
func (c *contractConn) QueryContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if strings.Contains(q, "FOR UPDATE") {
		if !c.inTx {
			return nil, errors.New("lock outside transaction")
		}
		c.locked = true
		c.state.calls = append(c.state.calls, "lock")
		return &contractRows{cols: []string{"schema_version"}, values: [][]driver.Value{{int64(1)}}}, nil
	}
	if !strings.Contains(q, "SELECT payload FROM cost_center_events") {
		return nil, errors.New("unexpected query")
	}
	if !c.inTx {
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
	} else if !c.locked {
		return nil, errors.New("read before lock")
	}
	c.state.calls = append(c.state.calls, "read")
	evs := c.state.events
	if c.inTx {
		evs = c.pending
	}
	out := &contractRows{cols: []string{"payload"}}
	for _, e := range evs {
		raw, _ := json.Marshal(e)
		out.values = append(out.values, []driver.Value{raw})
	}
	return out, nil
}
func (c *contractConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !c.inTx || !c.locked || !strings.HasPrefix(q, "INSERT INTO cost_center_events") || len(args) != 8 {
		return nil, errors.New("unexpected insert")
	}
	var e LedgerEvent
	if err := json.Unmarshal([]byte(args[7].Value.(string)), &e); err != nil {
		return nil, err
	}
	c.pending = append(c.pending, e)
	c.state.calls = append(c.state.calls, "insert")
	return driver.RowsAffected(1), nil
}
func (r *contractRows) Columns() []string { return r.cols }
func (r *contractRows) Close() error      { return nil }
func (r *contractRows) Next(dest []driver.Value) error {
	if r.n >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.n])
	r.n++
	return nil
}
func TestSQLLedgerTransactionContract(t *testing.T) {
	state := &contractDB{}
	name := "cost-contract-" + strings.ReplaceAll(t.Name(), "/", "-")
	sql.Register(name, contractDriver{state})
	db, e := sql.Open(name, "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	store := &SQLLedgerStore{Open: func() (*sql.DB, func(), error) { return db, func() {}, nil }}
	l := &Ledger{Store: store, Clock: func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }}
	c := LedgerCommand{Kind: "purchase", Purchase: &Purchase{Reference: "synthetic", Supplier: "test", Asset: "test", Tier: "plus", Kind: "subscription", Currency: "USD", Amount: "20", PaidAt: "2026-09-01T00:00:00Z", ServiceStart: "2026-09-01T00:00:00Z", ServiceEnd: "2026-10-01T00:00:00Z", Evidence: "synthetic", EvidenceRef: "fixture"}}
	first, e := l.Append(context.Background(), 1, "key", c)
	if e != nil || first.Replayed {
		t.Fatal(e)
	}
	if strings.Join(state.calls, ",") != "begin,lock,read,insert,commit" {
		t.Fatal(state.calls)
	}
	state.calls = nil
	again, e := l.Append(context.Background(), 1, "key", c)
	if e != nil || !again.Replayed || strings.Contains(strings.Join(state.calls, ","), "insert") {
		t.Fatal(again, e, state.calls)
	}
	state.failCommit = true
	c.Purchase.Reference = "new"
	if _, e = l.Append(context.Background(), 1, "new-key", c); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	evs, e := store.Snapshot(context.Background())
	if e != nil || len(evs) != 1 {
		t.Fatal(evs, e)
	}
}
func TestSQLStoreNotConfigured(t *testing.T) {
	s := &SQLLedgerStore{}
	if _, e := s.Snapshot(context.Background()); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
}
