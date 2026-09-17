package costing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"
)

type memoryLedger struct {
	mu     sync.Mutex
	events []LedgerEvent
}

func (s *memoryLedger) Snapshot(context.Context) ([]LedgerEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LedgerEvent{}, s.events...), nil
}
func (s *memoryLedger) Transact(_ context.Context, fn func([]LedgerEvent) (*LedgerEvent, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := fn(s.events)
	if err == nil && next != nil {
		s.events = append(s.events, *next)
	}
	return err
}
func extensionLedger() *Ledger {
	return &Ledger{Store: &memoryLedger{}, Clock: func() time.Time { return instant("2026-09-16T00:00:00Z") }}
}
func lot(ref, amount, quantity, paid, expires string) LedgerCommand {
	return LedgerCommand{Kind: "credit_lot", CreditLot: &CreditLot{Reference: ref, Pool: "pool-a", Unit: "credit", Currency: "USD", Amount: amount, Quantity: quantity, PaidAt: paid, ExpiresAt: expires, Evidence: "synthetic"}}
}
func use(ref, quantity, at string) LedgerCommand {
	return LedgerCommand{Kind: "credit_use", CreditUse: &CreditUse{Reference: ref, Pool: "pool-a", Unit: "credit", Currency: "USD", Tier: "plus", Model: "m", Quantity: quantity, OccurredAt: at, Evidence: "synthetic"}}
}
func add(t *testing.T, l *Ledger, key string, c LedgerCommand) LedgerEvent {
	t.Helper()
	out, err := l.Append(context.Background(), 1, key, c)
	if err != nil {
		t.Fatal(err)
	}
	return out.Event
}
func TestPrepaidFIFOActualLotPricesAndExpiry(t *testing.T) {
	l := extensionLedger()
	ctx := context.Background()
	old := add(t, l, "old", lot("old", "10", "100", "2026-09-01T00:00:00Z", "2026-09-03T00:00:00Z"))
	add(t, l, "second", lot("second", "40", "200", "2026-09-02T00:00:00Z", "2026-10-01T00:00:00Z"))
	add(t, l, "use1", use("use1", "150", "2026-09-02T12:00:00Z"))
	// 100 at 0.1 + 50 at 0.2 = 20; later topups cannot reprice history.
	q := SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-09-16T00:00:00Z", AsOf: "2026-09-16T00:00:00Z", Currency: "USD", Model: "m", Unit: "task"}
	s, err := l.Summary(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if s.Prepaid.CashPaid != "50.000000" || s.Prepaid.Uses[0].Cost != "20.000000" || s.Prepaid.Balances[1].RemainingValue != "30.000000" {
		t.Fatalf("wrong FIFO report: %+v", s.Prepaid)
	}
	if s.Rows[0].RecognizedExpense != "20.000000" {
		t.Fatal(s.Rows[0])
	}
	if _, err = l.Append(ctx, 1, "late-lot", lot("late", "1", "100", "2026-09-01T01:00:00Z", "2026-10-01T00:00:00Z")); !errors.Is(err, ErrInvalid) {
		t.Fatal("backdated lot repriced history")
	}
	if _, err = l.Append(ctx, 1, "void-lot", LedgerCommand{Kind: "void", Void: &Void{TargetID: old.ID, ExpectedHash: old.RequestHash, Reason: "correction"}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("dependent lot removed")
	}
	if _, err = l.Append(ctx, 1, "overdraft", use("too-much", "151", "2026-09-04T00:00:00Z")); !errors.Is(err, ErrInvalid) {
		t.Fatal("overdraft accepted")
	}
	if out, err := l.Append(ctx, 1, "use1", use("use1", "150", "2026-09-02T12:00:00Z")); err != nil || !out.Replayed {
		t.Fatal("retry was not idempotent")
	}
}
func TestPrepaidExpiredAndGiftLots(t *testing.T) {
	l := extensionLedger()
	add(t, l, "expired", lot("expired", "10", "100", "2026-09-01T00:00:00Z", "2026-09-03T00:00:00Z"))
	if _, err := l.Append(context.Background(), 1, "use", use("use", "1", "2026-09-03T00:00:00Z")); !errors.Is(err, ErrInvalid) {
		t.Fatal("expiry boundary accepted")
	}
	add(t, l, "gift", lot("gift", "0", "10", "2026-09-03T00:00:00Z", "2026-10-01T00:00:00Z"))
	add(t, l, "use-gift", use("gift-use", "10", "2026-09-04T00:00:00Z"))
	events, _ := l.Store.Snapshot(context.Background())
	q := SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-09-16T00:00:00Z", AsOf: "2026-09-16T00:00:00Z", Currency: "USD"}
	r, err := creditReport(activeEvents(events, instant(q.AsOf)), q)
	if err != nil || r.Uses[0].Cost != "0.000000" || r.Balances[0].ExpiredValue != "10.000000" {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestAccountUpgradeIntervalsCannotOverlap(t *testing.T) {
	l := extensionLedger()
	cmd := LedgerCommand{Kind: "account_interval", AccountInterval: &AccountInterval{AccountID: 7, Asset: "own-account", Tier: "plus", Start: "2026-09-01T00:00:00Z", End: "2026-09-08T00:00:00Z", EvidenceRef: "invoice-1"}}
	add(t, l, "plus", cmd)
	next := *cmd.AccountInterval
	next.Tier = "pro5x"
	next.Start = next.End
	next.End = "2026-10-01T00:00:00Z"
	add(t, l, "upgrade", LedgerCommand{Kind: "account_interval", AccountInterval: &next})
	next.Start = "2026-09-07T23:59:59Z"
	if _, err := l.Append(context.Background(), 1, "overlap", LedgerCommand{Kind: "account_interval", AccountInterval: &next}); !errors.Is(err, ErrInvalid) {
		t.Fatal("overlap accepted")
	}
}
func TestTrafficCounterIntegrityAndExactLargeBytes(t *testing.T) {
	before := json.RawMessage(`{"jsonversion":"2","interfaces":[{"name":"eth0","created":{"timestamp":1},"updated":{"timestamp":1788220800},"traffic":{"total":{"rx":100,"tx":9007199254740993}}}]}`)
	after := json.RawMessage(`{"jsonversion":"2","interfaces":[{"name":"eth0","created":{"timestamp":1},"updated":{"timestamp":1788307200},"traffic":{"total":{"rx":1000000100,"tx":9007201254740993}}}]}`)
	q := TrafficInput{Interface: "eth0", Start: "2026-09-01T00:00:00Z", End: "2026-09-02T00:00:00Z", Before: before, After: after, Mode: "egress", IncludedBytes: "1000000000", GBBytes: "1000000000", Rate: "0.1", Currency: "USD", CoverageComplete: true}
	r, err := AnalyzeTraffic(q)
	if err != nil || r.Charge == nil || *r.Charge != "0.100000" {
		t.Fatalf("%+v %v", r, err)
	}
	q.Mode = "both"
	r, err = AnalyzeTraffic(q)
	if err != nil || *r.Charge != "0.200000" {
		t.Fatalf("%+v %v", r, err)
	}
	q.Before, q.After = q.After, q.Before
	q.Start, q.End = "2026-09-02T00:00:00Z", "2026-09-03T00:00:00Z"
	var data map[string]any
	_ = json.Unmarshal(q.After, &data)
	data["interfaces"].([]any)[0].(map[string]any)["updated"] = map[string]any{"timestamp": 1788393600}
	q.After, _ = json.Marshal(data)
	r, err = AnalyzeTraffic(q)
	if err != nil || r.Status != "COUNTER_RESET" || r.Charge != nil {
		t.Fatalf("counter reset %+v %v", r, err)
	}
}
func replayFixture() ReplayInput {
	return ReplayInput{Pool: "codex", Model: "m", PriceVersion: "v1", Start: "2026-09-01T00:00:00Z", End: "2026-09-02T00:00:00Z", CoverageComplete: true, Windows: []QuotaWindow{{ID: "5h", Capacity: "10", Remaining: "10", DurationSeconds: 18000, ResetAt: "2026-09-01T05:00:00Z"}, {ID: "7d", Capacity: "100", Remaining: "100", DurationSeconds: 604800, ResetAt: "2026-09-08T00:00:00Z"}}, Actions: []QuotaAction{{ID: "a", At: "2026-09-01T01:00:00Z", Kind: "demand", Amount: "12"}, {ID: "b", At: "2026-09-01T05:00:00Z", Kind: "demand", Amount: "10"}}}
}
func TestDualWindowNaturalResetAndIncompleteCoverage(t *testing.T) {
	q := replayFixture()
	r, err := ReplayWindows(q)
	if err != nil || r.Delivered == nil || *r.Delivered != "20.000000" || *r.Unmet != "2.000000" {
		t.Fatalf("%+v %v", r, err)
	}
	if r.Windows[1].Remaining != "80.000000" {
		t.Fatal("weekly reset or double debit", r.Windows)
	}
	q.Actions = append(q.Actions, QuotaAction{ID: "reset", At: "2026-09-01T05:00:00Z", Kind: "refill", Window: "7d", Amount: "100"})
	r, err = ReplayWindows(q)
	if err != nil || r.Steps[2].NetRefill != "20.000000" {
		t.Fatal("refill not capped at actual headroom", r, err)
	}
	q.CoverageComplete = false
	r, err = ReplayWindows(q)
	if err != nil || r.Delivered != nil {
		t.Fatal("missing coverage became capacity")
	}
}
func TestForecastDeterminismAndColdStart(t *testing.T) {
	q := ForecastInput{OwnExposureComplete: true, WeeklyReferenceCapacity: 1000, BillingDays: 30, CashAndAllocatedCost: 200, PriorEventShape: 1, PriorExposureWeeks: 1, OwnObservedWeeks: 4, OwnEventCount: 4, VerifiedNetRefillFractions: []float64{.2, .4, .6, .8}, UsefulUtilization: .5, Seed: 7}
	a, err := Forecast(q)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Forecast(q)
	if err != nil || !reflect.DeepEqual(a, b) || a.P10 > a.P50 || a.P50 > a.P90 || math.Abs(a.Rate-1) > 1e-12 {
		t.Fatal(a, b, err)
	}
	q.VerifiedNetRefillFractions = []float64{0, 0, 0, 0}
	a, err = Forecast(q)
	if err != nil || math.Abs(a.P50-200.0/(1000.0*30.0/7.0*.5)) > 1e-12 || a.P10 != a.P90 {
		t.Fatal("zero confirmed gains were invented", a, err)
	}
	q.OwnExposureComplete = false
	if _, err = Forecast(q); !errors.Is(err, ErrInvalid) {
		t.Fatal("incomplete exposure accepted")
	}
}

func TestPrepaidRoundingOccursAfterSummingAndExpiryIsLoss(t *testing.T) {
	l := extensionLedger()
	add(t, l, "fraction", lot("fraction", "1", "3", "2026-09-01T00:00:00Z", "2026-10-01T00:00:00Z"))
	for _, ref := range []string{"a", "b", "c"} {
		add(t, l, ref, use(ref, "1", "2026-09-02T00:00:00Z"))
	}
	add(t, l, "expiry", lot("expiry", "10", "100", "2026-09-03T00:00:00Z", "2026-09-04T00:00:00Z"))
	q := SummaryQuery{Start: "2026-09-01T00:00:00Z", End: "2026-09-16T00:00:00Z", AsOf: "2026-09-16T00:00:00Z", Currency: "USD", Model: "m", Unit: "request"}
	summary, err := l.Summary(context.Background(), q)
	if err != nil || summary.Rows[0].RecognizedExpense != "1.000000" || summary.Rows[3].RecognizedExpense != "10.000000" || summary.Prepaid.ExpiredLoss != "10.000000" {
		t.Fatalf("%+v %v", summary, err)
	}
}

func TestReconciliationRequiresCompleteMatchingDeliveryScope(t *testing.T) {
	row := UsageRow{Asset: "own", Tier: "plus", Model: "m", Requests: 7, Start: "2026-09-01T00:00:00Z", End: "2026-09-03T00:00:00Z"}
	d := &Delivery{Asset: "own", Tier: "plus", Model: "m", Unit: "request", Quantity: "8", PeriodStart: row.Start, PeriodEnd: row.End}
	active := map[string]LedgerEvent{"d": {Command: LedgerCommand{Kind: "delivery", Delivery: d}}}
	compareRecordedRequests(active, &row)
	if row.ReconciliationStatus != "MISMATCH" || row.RequestDifference == nil || *row.RequestDifference != "1.000000" {
		t.Fatal(row)
	}
	d.PeriodEnd = "2026-09-02T00:00:00Z"
	row.RecordedRequests = nil
	row.RequestDifference = nil
	compareRecordedRequests(active, &row)
	if row.ReconciliationStatus != "RECORDED_SCOPE_INCOMPLETE" || row.RequestDifference != nil {
		t.Fatal(row)
	}
}
