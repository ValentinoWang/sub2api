package costing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

const LedgerLimit = 10000

var ErrConflict = errors.New("idempotency key or business reference conflicts")
var ErrUnavailable = errors.New("cost ledger storage unavailable")
var ErrInvalid = errors.New("invalid ledger command")
var ErrLimit = errors.New("ledger prototype event limit reached")
var ErrNotFound = errors.New("ledger record not found")
var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,119}$`)
var moneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$`)

type Purchase struct {
	Reference    string `json:"reference"`
	Supplier     string `json:"supplier"`
	Asset        string `json:"asset"`
	Tier         string `json:"tier"`
	Kind         string `json:"kind"`
	Currency     string `json:"currency"`
	Amount       string `json:"amount"`
	PaidAt       string `json:"paid_at"`
	ServiceStart string `json:"service_start"`
	ServiceEnd   string `json:"service_end"`
	Evidence     string `json:"evidence"`
	EvidenceRef  string `json:"evidence_ref"`
}

// Delivery is an explicitly bounded aggregate, not an individual inference request.
type Delivery struct {
	Reference   string `json:"reference"`
	Asset       string `json:"asset"`
	Tier        string `json:"tier"`
	Unit        string `json:"unit"`
	Model       string `json:"model"`
	Quantity    string `json:"quantity"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Evidence    string `json:"evidence"`
}
type AllocationPart struct {
	Tier   string `json:"tier"`
	Weight string `json:"weight"`
}
type Allocation struct {
	PurchaseID string           `json:"purchase_id"`
	Parts      []AllocationPart `json:"parts"`
}
type Void struct {
	TargetID     string `json:"target_id"`
	ExpectedHash string `json:"expected_hash"`
	Reason       string `json:"reason"`
}
type LedgerCommand struct {
	Kind            string           `json:"kind"`
	Purchase        *Purchase        `json:"purchase,omitempty"`
	Delivery        *Delivery        `json:"delivery,omitempty"`
	Allocation      *Allocation      `json:"allocation,omitempty"`
	Void            *Void            `json:"void,omitempty"`
	CreditLot       *CreditLot       `json:"credit_lot,omitempty"`
	CreditUse       *CreditUse       `json:"credit_use,omitempty"`
	AccountInterval *AccountInterval `json:"account_interval,omitempty"`
	Traffic         *TrafficInput    `json:"traffic,omitempty"`
	Replay          *ReplayInput     `json:"replay,omitempty"`
	Forecast        *ForecastInput   `json:"forecast,omitempty"`
}
type LedgerEvent struct {
	ID          string        `json:"id"`
	Sequence    int64         `json:"sequence"`
	Key         string        `json:"idempotency_key"`
	RequestHash string        `json:"request_hash"`
	Actor       int64         `json:"actor_id"`
	RecordedAt  string        `json:"recorded_at"`
	Command     LedgerCommand `json:"command"`
}

// Store executes each append callback against one consistent snapshot under an exclusive lock.
// Nil returned event means replay; it MUST NOT create another row or advance the sequence.
type LedgerStore interface {
	Snapshot(context.Context) ([]LedgerEvent, error)
	Transact(context.Context, func([]LedgerEvent) (*LedgerEvent, error)) error
}
type Ledger struct {
	Store LedgerStore
	Clock func() time.Time
}
type AppendResult struct {
	Event    LedgerEvent `json:"event"`
	Replayed bool        `json:"replayed"`
}

func tierOK(s string, shared bool) bool {
	return s == "plus" || s == "pro5x" || s == "pro20x" || (shared && s == "unallocated")
}
func currencyOK(s string) bool {
	return s == "USD" || s == "CNY" || s == "HKD" || s == "SGD" || s == "EUR"
}
func evidenceOK(s string) bool { return s == "invoice" || s == "manual" || s == "synthetic" }
func refOK(s string) bool      { return refPattern.MatchString(s) }
func textOK(s string, max int) bool {
	return len(strings.TrimSpace(s)) > 0 && len(s) <= max && !strings.ContainsAny(s, "\x00\r\n")
}
func rat(s string) *big.Rat       { r, _ := new(big.Rat).SetString(s); return r }
func validateMoney(s string) bool { return moneyPattern.MatchString(s) && rat(s).Sign() > 0 }
func validPeriod(a, b string) bool {
	x, e := parseTime(a)
	y, f := parseTime(b)
	return e == nil && f == nil && y.After(x) && y.Sub(x) <= 3660*24*time.Hour
}
func invalid(s string) error { return fmt.Errorf("%w: %s", ErrInvalid, s) }
func hashCommand(c LedgerCommand) string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (c LedgerCommand) Validate() error {
	count := 0
	if c.Purchase != nil {
		count++
	}
	if c.Delivery != nil {
		count++
	}
	if c.Allocation != nil {
		count++
	}
	if c.Void != nil {
		count++
	}
	count += c.extensionCount()
	if count != 1 {
		return invalid("exactly one payload is required")
	}
	switch c.Kind {
	case "purchase":
		p := c.Purchase
		if p == nil {
			return invalid("purchase payload required")
		}
		if !refOK(p.Reference) || !refOK(p.Supplier) || !refOK(p.Asset) || !tierOK(p.Tier, true) || !currencyOK(p.Currency) || !validateMoney(p.Amount) || !evidenceOK(p.Evidence) || !refOK(p.EvidenceRef) {
			return invalid("purchase reference, tier, currency, amount or evidence")
		}
		if p.Kind != "subscription" && p.Kind != "server" && p.Kind != "traffic" && p.Kind != "labor" && p.Kind != "other" {
			return invalid("unsupported purchase kind; prepaid credit lots are not period expenses")
		}
		if p.Kind == "subscription" && !tierOK(p.Tier, false) {
			return invalid("subscription requires a concrete SKU")
		}
		if _, err := parseTime(p.PaidAt); err != nil || !validPeriod(p.ServiceStart, p.ServiceEnd) {
			return invalid("explicit UTC-offset paid time and service interval required")
		}
	case "delivery":
		d := c.Delivery
		if d == nil {
			return invalid("delivery payload required")
		}
		if !refOK(d.Reference) || !refOK(d.Asset) || !tierOK(d.Tier, false) || !textOK(d.Model, 100) || !textOK(d.Unit, 100) || !validateMoney(d.Quantity) || !evidenceOK(d.Evidence) || !validPeriod(d.PeriodStart, d.PeriodEnd) {
			return invalid("delivery identity, quantity or interval")
		}
	case "allocate":
		a := c.Allocation
		if a == nil || !refOK(a.PurchaseID) || len(a.Parts) < 1 || len(a.Parts) > 3 {
			return invalid("allocation target or parts")
		}
		sum := new(big.Rat)
		seen := map[string]bool{}
		for _, p := range a.Parts {
			if !tierOK(p.Tier, false) || seen[p.Tier] || !decimalPattern.MatchString(p.Weight) || rat(p.Weight) == nil || rat(p.Weight).Sign() <= 0 || rat(p.Weight).Cmp(big.NewRat(1, 1)) > 0 {
				return invalid("allocation weight or repeated SKU")
			}
			seen[p.Tier] = true
			sum.Add(sum, rat(p.Weight))
		}
		if sum.Cmp(big.NewRat(1, 1)) != 0 {
			return invalid("allocation weights must sum exactly to 1")
		}
	case "void":
		v := c.Void
		if v == nil || !refOK(v.TargetID) || len(v.ExpectedHash) != 64 || !textOK(v.Reason, 240) {
			return invalid("void requires target, expected hash and reason")
		}
	default:
		return c.validateExtension()
	}
	return nil
}
func activeEvents(events []LedgerEvent, cutoff time.Time) map[string]LedgerEvent {
	active := map[string]LedgerEvent{}
	for _, e := range events {
		at, err := parseTime(e.RecordedAt)
		if err != nil || at.After(cutoff) {
			continue
		}
		if e.Command.Kind == "void" {
			delete(active, e.Command.Void.TargetID)
		} else {
			active[e.ID] = e
		}
	}
	return active
}
func validateAgainst(events []LedgerEvent, c LedgerCommand, now time.Time) error {
	active := activeEvents(events, now)
	if err := validateExtensionAgainst(active, c, now); err != nil {
		return err
	}
	switch c.Kind {
	case "purchase":
		for _, e := range active {
			if p := e.Command.Purchase; p != nil && p.Reference == c.Purchase.Reference && p.Supplier == c.Purchase.Supplier {
				return ErrConflict
			}
		}
	case "delivery":
		d := c.Delivery
		a, _ := parseTime(d.PeriodStart)
		b, _ := parseTime(d.PeriodEnd)
		for _, e := range active {
			old := e.Command.Delivery
			if old == nil {
				continue
			}
			if old.Reference == d.Reference {
				return ErrConflict
			}
			x, _ := parseTime(old.PeriodStart)
			y, _ := parseTime(old.PeriodEnd)
			if old.Asset == d.Asset && old.Model == d.Model && old.Unit == d.Unit && a.Before(y) && x.Before(b) {
				return invalid("overlapping aggregate delivery scopes; split or void the earlier aggregate")
			}
		}
	case "allocate":
		target, ok := active[c.Allocation.PurchaseID]
		if !ok || target.Command.Purchase == nil {
			return ErrNotFound
		}
		if target.Command.Purchase.Tier != "unallocated" {
			return invalid("only unallocated purchases may be distributed")
		}
		for _, e := range active {
			if e.Command.Allocation != nil && e.Command.Allocation.PurchaseID == target.ID {
				return ErrConflict
			}
		}
	case "void":
		v := c.Void
		target, ok := active[v.TargetID]
		if !ok {
			return ErrNotFound
		}
		if target.RequestHash != v.ExpectedHash {
			return ErrConflict
		}
		if target.Command.Purchase != nil {
			for _, e := range active {
				if e.Command.Allocation != nil && e.Command.Allocation.PurchaseID == target.ID {
					return invalid("void the allocation before voiding its source purchase")
				}
			}
		}
	}
	return nil
}
func (l *Ledger) Append(ctx context.Context, actor int64, key string, c LedgerCommand) (AppendResult, error) {
	var out AppendResult
	if l == nil || l.Store == nil {
		return out, ErrUnavailable
	}
	if actor <= 0 || !refOK(key) {
		return out, invalid("verified actor and idempotency key required")
	}
	if err := c.Validate(); err != nil {
		return out, err
	}
	digest := hashCommand(c)
	err := l.Store.Transact(ctx, func(events []LedgerEvent) (*LedgerEvent, error) {
		for _, e := range events {
			if e.Key == key {
				if e.Actor != actor || e.RequestHash != digest {
					return nil, ErrConflict
				}
				out = AppendResult{e, true}
				return nil, nil
			}
		}
		if len(events) >= LedgerLimit {
			return nil, ErrLimit
		}
		now := time.Now().UTC()
		if l.Clock != nil {
			now = l.Clock().UTC()
		}
		if len(events) > 0 {
			last, _ := parseTime(events[len(events)-1].RecordedAt)
			if now.Before(last) {
				return nil, invalid("server clock moved backwards")
			}
		}
		if c.Purchase != nil {
			paid, _ := parseTime(c.Purchase.PaidAt)
			if paid.After(now) {
				return nil, invalid("future payment is a budget, not recorded cash")
			}
		}
		if c.Delivery != nil {
			end, _ := parseTime(c.Delivery.PeriodEnd)
			if end.After(now) {
				return nil, invalid("future delivery is a forecast, not observed output")
			}
		}
		if err := validateAgainst(events, c, now); err != nil {
			return nil, err
		}
		seq := int64(len(events) + 1)
		e := LedgerEvent{ID: fmt.Sprintf("cost-%012d", seq), Sequence: seq, Key: key, RequestHash: digest, Actor: actor, RecordedAt: now.Format(time.RFC3339Nano), Command: c}
		out = AppendResult{e, false}
		return &e, nil
	})
	return out, err
}

type SummaryQuery struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	AsOf     string `json:"as_of"`
	Currency string `json:"currency"`
	Model    string `json:"model"`
	Unit     string `json:"unit"`
}
type TierSummary struct {
	Tier                  string  `json:"tier"`
	CashPaid              string  `json:"cash_paid"`
	PeriodExpense         string  `json:"period_expense"`
	RecognizedExpense     string  `json:"recognized_expense"`
	RemainingServiceValue string  `json:"remaining_service_value"`
	Delivered             string  `json:"delivered"`
	UnitCost              *string `json:"recorded_scope_unit_cost"`
}
type LedgerSummary struct {
	Currency   string        `json:"currency"`
	Start      string        `json:"start"`
	End        string        `json:"end"`
	AsOf       string        `json:"as_of"`
	Rows       []TierSummary `json:"rows"`
	EventCount int           `json:"event_count"`
	Warnings   []string      `json:"warnings"`
	Status     string        `json:"status"`
	Prepaid    *CreditReport `json:"prepaid"`
}

func seconds(d time.Duration) *big.Rat { return new(big.Rat).SetInt64(int64(d)) }
func overlapAmount(p *Purchase, a, b time.Time) *big.Rat {
	s, _ := parseTime(p.ServiceStart)
	e, _ := parseTime(p.ServiceEnd)
	x, y := s, e
	if x.Before(a) {
		x = a
	}
	if y.After(b) {
		y = b
	}
	if !y.After(x) {
		return new(big.Rat)
	}
	return new(big.Rat).Mul(rat(p.Amount), new(big.Rat).Quo(seconds(y.Sub(x)), seconds(e.Sub(s))))
}
func (l *Ledger) Summary(ctx context.Context, q SummaryQuery) (LedgerSummary, error) {
	out := LedgerSummary{Currency: q.Currency, Start: q.Start, End: q.End, AsOf: q.AsOf, Rows: []TierSummary{}, Warnings: []string{}, Status: "RECORDED_SCOPE_ONLY"}
	if l == nil || l.Store == nil {
		return out, ErrUnavailable
	}
	if !validPeriod(q.Start, q.End) || !currencyOK(q.Currency) || !textOK(q.Model, 100) || !textOK(q.Unit, 100) {
		return out, invalid("query scope required")
	}
	a, _ := parseTime(q.Start)
	b, _ := parseTime(q.End)
	at, e := parseTime(q.AsOf)
	if e != nil {
		return out, invalid("as_of required")
	}
	now := time.Now()
	if l.Clock != nil {
		now = l.Clock()
	}
	if at.After(now.Add(time.Second)) {
		return out, invalid("future knowledge cutoff is not an actual ledger report")
	}
	events, err := l.Store.Snapshot(ctx)
	if err != nil {
		return out, err
	}
	active := activeEvents(events, at)
	credit, err := creditReport(active, q)
	if err != nil {
		return out, err
	}
	out.Prepaid = &credit
	cutoff := b
	if at.Before(cutoff) {
		cutoff = at
		out.Status = "IN_PROGRESS_PERIOD"
	}
	type totals struct{ cash, expense, recognized, remaining, delivered *big.Rat }
	m := map[string]*totals{}
	for _, t := range []string{"plus", "pro5x", "pro20x", "unallocated"} {
		m[t] = &totals{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	}
	allocated := map[string][]AllocationPart{}
	for _, e := range active {
		if e.Command.Allocation != nil {
			allocated[e.Command.Allocation.PurchaseID] = e.Command.Allocation.Parts
		}
	}
	warnings := map[string]bool{}
	out.EventCount = len(active)
	for _, e := range active {
		if p := e.Command.Purchase; p != nil {
			if p.Currency != q.Currency {
				if overlapAmount(p, a, cutoff).Sign() > 0 {
					warnings["OTHER_CURRENCIES_EXCLUDED"] = true
				}
				continue
			}
			if p.Evidence != "invoice" {
				warnings["COST_EVIDENCE_NOT_FULLY_INVOICED"] = true
			}
			shares := allocated[e.ID]
			if len(shares) == 0 {
				shares = []AllocationPart{{p.Tier, "1"}}
			}
			expense := overlapAmount(p, a, b)
			paid, _ := parseTime(p.PaidAt)
			cash := new(big.Rat)
			if !paid.Before(a) && paid.Before(b) && !paid.After(at) {
				cash = rat(p.Amount)
			}
			remaining := new(big.Rat)
			end, _ := parseTime(p.ServiceEnd)
			if !paid.After(cutoff) {
				remaining = overlapAmount(p, cutoff, end)
			}
			for _, s := range shares {
				v := m[s.Tier]
				v.cash.Add(v.cash, new(big.Rat).Mul(cash, rat(s.Weight)))
				v.expense.Add(v.expense, new(big.Rat).Mul(expense, rat(s.Weight)))
				v.recognized.Add(v.recognized, new(big.Rat).Mul(overlapAmount(p, a, cutoff), rat(s.Weight)))
				v.remaining.Add(v.remaining, new(big.Rat).Mul(remaining, rat(s.Weight)))
			}
		}
		if d := e.Command.Delivery; d != nil {
			x, _ := parseTime(d.PeriodStart)
			y, _ := parseTime(d.PeriodEnd)
			if !x.Before(cutoff) || !a.Before(y) {
				continue
			}
			if d.Model != q.Model || d.Unit != q.Unit {
				warnings["OTHER_WORKLOAD_DELIVERIES_EXCLUDED"] = true
				continue
			}
			if !x.Before(a) && !y.After(cutoff) {
				m[d.Tier].delivered.Add(m[d.Tier].delivered, rat(d.Quantity))
			} else if x.Before(b) && a.Before(y) {
				warnings["CROSS_BOUNDARY_DELIVERY_NOT_PRORATED"] = true
			}
		}
	}
	for tier, cost := range credit.exactCosts {
		m[tier].recognized.Add(m[tier].recognized, cost)
		m[tier].expense.Add(m[tier].expense, cost)
	}
	for _, warning := range credit.warnings {
		warnings[warning] = true
	}

	for _, tier := range []string{"plus", "pro5x", "pro20x", "unallocated"} {
		v := m[tier]

		row := TierSummary{Tier: tier, CashPaid: v.cash.FloatString(6), PeriodExpense: v.expense.FloatString(6), RecognizedExpense: v.recognized.FloatString(6), RemainingServiceValue: v.remaining.FloatString(6), Delivered: v.delivered.FloatString(6)}
		if v.delivered.Sign() > 0 && v.recognized.Sign() > 0 && !warnings["OTHER_CURRENCIES_EXCLUDED"] && !warnings["OTHER_WORKLOAD_DELIVERIES_EXCLUDED"] && !warnings["CROSS_BOUNDARY_DELIVERY_NOT_PRORATED"] && m["unallocated"].recognized.Sign() == 0 {
			row.UnitCost = str(new(big.Rat).Quo(v.recognized, v.delivered), 6)
		}
		out.Rows = append(out.Rows, row)
	}
	warnings["MANUAL_SCOPE_NOT_COMPLETE_COST_OR_MODEL_COST_ALLOCATION"] = true
	for w := range warnings {
		out.Warnings = append(out.Warnings, w)
	}
	sort.Strings(out.Warnings)
	return out, nil
}

// ValidateEventStream rejects malformed persisted data instead of silently ignoring it.
func ValidateEventStream(events []LedgerEvent) error {
	if len(events) > LedgerLimit {
		return ErrLimit
	}
	seen := map[string]bool{}
	var last time.Time
	for i, e := range events {
		t, err := parseTime(e.RecordedAt)
		if err != nil || t.Before(last) || e.Sequence != int64(i+1) || e.ID != fmt.Sprintf("cost-%012d", i+1) || e.Actor <= 0 || !refOK(e.Key) || seen[e.Key] || e.RequestHash != hashCommand(e.Command) {
			return errors.New("cost ledger integrity error")
		}
		if e.Command.Validate() != nil {
			return errors.New("cost ledger payload integrity error")
		}
		if err := validateAgainst(events[:i], e.Command, t); err != nil {
			return errors.New("cost ledger event sequence integrity error")
		}
		seen[e.Key] = true
		last = t
	}
	return nil
}
