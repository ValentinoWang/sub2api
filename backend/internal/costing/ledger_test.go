//go:build linux || darwin

package costing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testLedger(t *testing.T) *Ledger {
	t.Helper()
	d := t.TempDir()
	if e := os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	return &Ledger{Store: &FileLedgerStore{filepath.Join(d, "ledger.json")}, Clock: func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }}
}
func purchase(tier, amount, ref string) LedgerCommand {
	return LedgerCommand{Kind: "purchase", Purchase: &Purchase{Reference: ref, Supplier: "synthetic-supplier", Asset: "synthetic-" + tier, Tier: tier, Kind: "subscription", Currency: "USD", Amount: amount, PaidAt: "2026-09-01T00:00:00Z", ServiceStart: "2026-09-01T00:00:00Z", ServiceEnd: "2026-10-01T00:00:00Z", Evidence: "synthetic", EvidenceRef: "fixture-only"}}
}
func delivery(tier, amount, ref string) LedgerCommand {
	return LedgerCommand{Kind: "delivery", Delivery: &Delivery{Reference: ref, Asset: "synthetic-" + tier, Tier: tier, Unit: "qualified-task-v1", Model: "synthetic-model", Quantity: amount, PeriodStart: "2026-09-01T00:00:00Z", PeriodEnd: "2026-10-01T00:00:00Z", Evidence: "synthetic"}}
}
func query() SummaryQuery {
	return SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-10-01T00:00:00Z", AsOf: "2026-10-02T00:00:00Z", Currency: "USD", Model: "synthetic-model", Unit: "qualified-task-v1"}
}
func appendOK(t *testing.T, l *Ledger, key string, c LedgerCommand) AppendResult {
	t.Helper()
	v, e := l.Append(context.Background(), 7, key, c)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestLedgerThreeTiersAndAllocation(t *testing.T) {
	l := testLedger(t)
	for i, tier := range tiers {
		appendOK(t, l, "p-"+tier, purchase(tier, []string{"20", "100", "200"}[i], tier))
		appendOK(t, l, "d-"+tier, delivery(tier, []string{"80", "400", "1600"}[i], tier))
	}
	common := purchase("unallocated", "60", "shared")
	common.Purchase.Kind = "server"
	source := appendOK(t, l, "shared", common)
	appendOK(t, l, "allocate", LedgerCommand{Kind: "allocate", Allocation: &Allocation{PurchaseID: source.Event.ID, Parts: []AllocationPart{{"plus", ".1"}, {"pro5x", ".3"}, {"pro20x", ".6"}}}})
	summary, e := l.Summary(context.Background(), query())
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"26.000000", "118.000000", "236.000000", "0.000000"} {
		if summary.Rows[i].RecognizedExpense != want {
			t.Fatal(summary)
		}
	}
	for i, want := range []string{"0.325000", "0.295000", "0.147500"} {
		if summary.Rows[i].UnitCost == nil || *summary.Rows[i].UnitCost != want {
			t.Fatal(summary.Rows[i])
		}
	}
}
func TestLedgerRestartAndReplay(t *testing.T) {
	l := testLedger(t)
	first := appendOK(t, l, "once", purchase("plus", "20", "p"))
	second := &Ledger{Store: &FileLedgerStore{l.Store.(*FileLedgerStore).Path}, Clock: l.Clock}
	repeat := appendOK(t, second, "once", purchase("plus", "20", "p"))
	if !repeat.Replayed || repeat.Event.ID != first.Event.ID {
		t.Fatal(repeat)
	}
	events, e := second.Store.Snapshot(context.Background())
	if e != nil || len(events) != 1 {
		t.Fatal(events, e)
	}
}
func TestLedgerConcurrentIdempotency(t *testing.T) {
	l := testLedger(t)
	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := &Ledger{Store: &FileLedgerStore{l.Store.(*FileLedgerStore).Path}, Clock: l.Clock}
			_, e := other.Append(context.Background(), 7, "same", purchase("plus", "20", "p"))
			errCh <- e
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			t.Fatal(e)
		}
	}
	events, _ := l.Store.Snapshot(context.Background())
	if len(events) != 1 {
		t.Fatal(len(events))
	}
}
func TestLedgerKeyMismatchAndActor(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "once", purchase("plus", "20", "p"))
	for _, c := range []struct {
		actor  int64
		amount string
	}{{7, "21"}, {8, "20"}} {
		_, e := l.Append(context.Background(), c.actor, "once", purchase("plus", c.amount, "p"))
		if !errors.Is(e, ErrConflict) {
			t.Fatal(e)
		}
	}
}
func TestLedgerDuplicateBusinessReference(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "once", purchase("plus", "20", "p"))
	_, e := l.Append(context.Background(), 7, "another", purchase("plus", "20", "p"))
	if !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}
func TestLedgerVoidIsAppendOnlyAndHistorical(t *testing.T) {
	l := testLedger(t)
	orig := appendOK(t, l, "p", purchase("plus", "20", "p"))
	l.Clock = func() time.Time { return time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC) }
	appendOK(t, l, "void", LedgerCommand{Kind: "void", Void: &Void{orig.Event.ID, orig.Event.RequestHash, "booking mistake, not a refund"}})
	before, e := l.Summary(context.Background(), query())
	if e != nil || before.Rows[0].PeriodExpense != "20.000000" {
		t.Fatal(before, e)
	}
	q := query()
	q.AsOf = "2026-10-03T00:00:00Z"
	after, e := l.Summary(context.Background(), q)
	if e != nil || after.Rows[0].PeriodExpense != "0.000000" {
		t.Fatal(after, e)
	}
	events, _ := l.Store.Snapshot(context.Background())
	if len(events) != 2 || events[0].Command.Purchase.Amount != "20" {
		t.Fatal(events)
	}
}
func TestLedgerVoidDependenciesAndHash(t *testing.T) {
	l := testLedger(t)
	p := purchase("unallocated", "50", "common")
	p.Purchase.Kind = "server"
	s := appendOK(t, l, "s", p)
	a := appendOK(t, l, "a", LedgerCommand{Kind: "allocate", Allocation: &Allocation{s.Event.ID, []AllocationPart{{"plus", "1"}}}})
	c := LedgerCommand{Kind: "void", Void: &Void{s.Event.ID, s.Event.RequestHash, "correct mistake"}}
	if _, e := l.Append(context.Background(), 7, "v", c); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	appendOK(t, l, "va", LedgerCommand{Kind: "void", Void: &Void{a.Event.ID, a.Event.RequestHash, "wrong allocation"}})
	c.Void.ExpectedHash = strings.Repeat("0", 64)
	if _, e := l.Append(context.Background(), 7, "v", c); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	c.Void.ExpectedHash = s.Event.RequestHash
	appendOK(t, l, "v", c)
}
func TestLedgerAccrualActualVersusScheduled(t *testing.T) {
	l := testLedger(t)
	l.Clock = func() time.Time { return time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC) }
	appendOK(t, l, "p", purchase("plus", "30", "p"))
	q := query()
	q.AsOf = "2026-09-16T00:00:00Z"
	s, e := l.Summary(context.Background(), q)
	if e != nil || s.Rows[0].RecognizedExpense != "15.000000" || s.Rows[0].PeriodExpense != "30.000000" || s.Rows[0].RemainingServiceValue != "15.000000" || s.Status != "IN_PROGRESS_PERIOD" {
		t.Fatal(s, e)
	}
}
func TestLedgerCrossBoundaryDeliveryNotProrated(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "d", delivery("plus", "100", "d"))
	q := query()
	q.Start = "2026-09-15T00:00:00Z"
	s, e := l.Summary(context.Background(), q)
	if e != nil || s.Rows[0].Delivered != "0.000000" || s.Rows[0].UnitCost != nil {
		t.Fatal(s, e)
	}
}
func TestLedgerOverlappingDeliveryRejected(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "d", delivery("plus", "100", "d"))
	d := delivery("plus", "30", "different")
	d.Delivery.PeriodStart = "2026-09-15T00:00:00Z"
	_, e := l.Append(context.Background(), 7, "d2", d)
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func TestLedgerCurrenciesAndUnallocatedDoNotHideCosts(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "p", purchase("plus", "20", "p"))
	appendOK(t, l, "d", delivery("plus", "100", "d"))
	cn := purchase("plus", "140", "cn")
	cn.Purchase.Currency = "CNY"
	appendOK(t, l, "cn", cn)
	s, e := l.Summary(context.Background(), query())
	if e != nil || s.Rows[0].PeriodExpense != "20.000000" || s.Rows[0].UnitCost != nil {
		t.Fatal(s, e)
	}
}
func TestLedgerValidation(t *testing.T) {
	cases := map[string]func(*LedgerCommand){"ambiguous-tier": func(c *LedgerCommand) { c.Purchase.Tier = "pro" }, "negative": func(c *LedgerCommand) { c.Purchase.Amount = "-1" }, "exponent": func(c *LedgerCommand) { c.Purchase.Amount = "1e3" }, "nan": func(c *LedgerCommand) { c.Purchase.Amount = "NaN" }, "excess precision": func(c *LedgerCommand) { c.Purchase.Amount = "1.0000001" }, "prepaid": func(c *LedgerCommand) { c.Purchase.Kind = "prepaid" }, "timezone": func(c *LedgerCommand) { c.Purchase.PaidAt = "2026-09-01" }, "two-payloads": func(c *LedgerCommand) { c.Delivery = delivery("plus", "10", "d").Delivery }, "no-evidence": func(c *LedgerCommand) { c.Purchase.EvidenceRef = "" }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			l := testLedger(t)
			c := purchase("plus", "20", "p")
			change(&c)
			_, e := l.Append(context.Background(), 7, "p", c)
			if !errors.Is(e, ErrInvalid) {
				t.Fatal(e)
			}
			events, _ := l.Store.Snapshot(context.Background())
			if len(events) != 0 {
				t.Fatal(events)
			}
		})
	}
}
func TestLedgerAllocationWeights(t *testing.T) {
	for _, parts := range [][]AllocationPart{{{"plus", ".3"}, {"pro5x", ".6"}}, {{"plus", ".5"}, {"plus", ".5"}}, {{"pro", "1"}}, {{"plus", "0"}, {"pro5x", "1"}}} {
		c := LedgerCommand{Kind: "allocate", Allocation: &Allocation{"cost-1", parts}}
		if !errors.Is(c.Validate(), ErrInvalid) {
			t.Fatal(parts)
		}
	}
}
func TestLedgerFutureObservationsRejected(t *testing.T) {
	l := testLedger(t)
	for _, c := range []LedgerCommand{purchase("plus", "20", "p"), delivery("plus", "10", "d")} {
		if c.Purchase != nil {
			c.Purchase.PaidAt = "2027-01-01T00:00:00Z"
		} else {
			c.Delivery.PeriodEnd = "2027-01-01T00:00:00Z"
		}
		if _, e := l.Append(context.Background(), 7, "f", c); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
}
func TestLedgerCorruptFileFailsClosed(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "p", purchase("plus", "20", "p"))
	path := l.Store.(*FileLedgerStore).Path
	if e := os.WriteFile(path, []byte(`[{"bad":"data"}]`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := l.Store.Snapshot(context.Background()); e == nil {
		t.Fatal("corruption accepted")
	}
}
func TestLedgerHTTPContracts(t *testing.T) {
	l := testLedger(t)
	body, _ := json.Marshal(purchase("plus", "20", "p"))
	h := HTTPHandler{Authorize: func(*http.Request) bool { return true }, Actor: func(*http.Request) int64 { return 7 }, Ledger: l}
	send := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/admin/cost-center/ledger/"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := send("POST", "commands", string(body), ""); w.Code != 422 {
		t.Fatal(w.Code, w.Body)
	}
	for _, status := range []int{201, 200} {
		if w := send("POST", "commands", string(body), "p"); w.Code != status {
			t.Fatal(w.Code, w.Body)
		}
	}
	for _, path := range []string{"health", "events?limit=1"} {
		if w := send("GET", path, "", ""); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	for _, body := range []string{`{"kind":"purchase","actor_id":99}`, string(body) + " {}", strings.Repeat(" ", 16385) + "{}"} {
		if w := send("POST", "commands", body, "bad"); w.Code != 400 {
			t.Fatal(w.Code, w.Body)
		}
	}
	h.Actor = nil
	if w := send("POST", "commands", string(body), "p"); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	h.Authorize = func(*http.Request) bool { return false }
	if w := send("GET", "events", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestLedgerSnapshotPagination(t *testing.T) {
	l := testLedger(t)
	for i := 0; i < 3; i++ {
		appendOK(t, l, fmt.Sprintf("p%d", i), purchase("plus", "20", fmt.Sprintf("p%d", i)))
	}
	h := HTTPHandler{Authorize: func(*http.Request) bool { return true }, Ledger: l}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/cost-center/ledger/events?limit=2", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"next_after":2`) {
		t.Fatal(w.Body.String())
	}
}

func TestLedgerCancelledWriteDoesNotCommit(t *testing.T) {
	l := testLedger(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := l.Append(ctx, 7, "cancel", purchase("plus", "20", "p")); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	events, e := l.Store.Snapshot(context.Background())
	if e != nil || len(events) != 0 {
		t.Fatal(events, e)
	}
}

func TestLedgerOutOfPeriodDataDoesNotPoisonUnitCosts(t *testing.T) {
	l := testLedger(t)
	appendOK(t, l, "p", purchase("plus", "20", "p"))
	appendOK(t, l, "d", delivery("plus", "100", "d"))
	old := purchase("plus", "140", "old")
	old.Purchase.Currency = "CNY"
	old.Purchase.ServiceStart = "2026-08-01T00:00:00Z"
	old.Purchase.ServiceEnd = "2026-09-01T00:00:00Z"
	appendOK(t, l, "old", old)
	other := delivery("plus", "90", "old-delivery")
	other.Delivery.Model = "other-model"
	other.Delivery.PeriodStart = "2026-08-01T00:00:00Z"
	other.Delivery.PeriodEnd = "2026-09-01T00:00:00Z"
	appendOK(t, l, "old-delivery", other)
	result, e := l.Summary(context.Background(), query())
	if e != nil || result.Rows[0].UnitCost == nil || *result.Rows[0].UnitCost != "0.200000" {
		t.Fatal(result, e)
	}
}
