package costing

import (
	"fmt"
	"math/big"
	"sort"
	"time"
)

// CreditLot records prepaid cash separately from service-period purchases.
// Pool and unit identify fungible credit; currency must never be converted implicitly.
type CreditLot struct {
	Reference string `json:"reference"`
	Pool      string `json:"pool"`
	Unit      string `json:"unit"`
	Currency  string `json:"currency"`
	Amount    string `json:"amount"`
	Quantity  string `json:"quantity"`
	PaidAt    string `json:"paid_at"`
	ExpiresAt string `json:"expires_at"`
	Evidence  string `json:"evidence"`
}
type CreditUse struct {
	Reference  string `json:"reference"`
	Pool       string `json:"pool"`
	Unit       string `json:"unit"`
	Currency   string `json:"currency"`
	Tier       string `json:"tier"`
	Model      string `json:"model"`
	Quantity   string `json:"quantity"`
	OccurredAt string `json:"occurred_at"`
	Evidence   string `json:"evidence"`
}

// AccountInterval is an explicit historical mapping, never inferred from an account name.
type AccountInterval struct {
	Asset       string `json:"asset"`
	AccountID   int64  `json:"account_id"`
	Tier        string `json:"tier"`
	Start       string `json:"start"`
	End         string `json:"end"`
	EvidenceRef string `json:"evidence_ref"`
}
type CreditPart struct {
	LotID    string `json:"lot_id"`
	Quantity string `json:"quantity"`
	Cost     string `json:"cost"`
}
type CreditConsumption struct {
	EventID string       `json:"event_id"`
	Tier    string       `json:"tier"`
	Cost    string       `json:"cost"`
	Parts   []CreditPart `json:"parts"`
}
type CreditBalance struct {
	EventID           string `json:"event_id"`
	Pool              string `json:"pool"`
	Unit              string `json:"unit"`
	RemainingQuantity string `json:"remaining_quantity"`
	RemainingValue    string `json:"remaining_value"`
	ExpiredQuantity   string `json:"expired_quantity"`
	ExpiredValue      string `json:"expired_value"`
}
type CreditReport struct {
	CashPaid    string `json:"cash_paid"`
	ExpiredLoss string `json:"expired_loss"`
	exactCosts  map[string]*big.Rat
	warnings    []string
	Uses        []CreditConsumption `json:"uses"`
	Balances    []CreditBalance     `json:"balances"`
}

func (c LedgerCommand) extensionCount() int {
	n := 0
	if c.CreditLot != nil {
		n++
	}
	if c.CreditUse != nil {
		n++
	}
	if c.AccountInterval != nil {
		n++
	}
	if c.Traffic != nil {
		n++
	}
	if c.Replay != nil {
		n++
	}
	if c.Forecast != nil {
		n++
	}
	return n
}
func nonnegativeMoney(v string) bool { return moneyPattern.MatchString(v) }
func (c LedgerCommand) validateExtension() error {
	switch c.Kind {
	case "credit_lot":
		p := c.CreditLot
		if p == nil || !refOK(p.Reference) || !refOK(p.Pool) || !textOK(p.Unit, 100) || !currencyOK(p.Currency) || !nonnegativeMoney(p.Amount) || !validateMoney(p.Quantity) || !validPeriod(p.PaidAt, p.ExpiresAt) || !evidenceOK(p.Evidence) {
			return invalid("prepaid lot identity, amount, credit or expiry")
		}
	case "credit_use":
		p := c.CreditUse
		if p == nil || !refOK(p.Reference) || !refOK(p.Pool) || !textOK(p.Unit, 100) || !currencyOK(p.Currency) || !tierOK(p.Tier, false) || !textOK(p.Model, 100) || !validateMoney(p.Quantity) || !evidenceOK(p.Evidence) {
			return invalid("credit consumption scope or quantity")
		}
		if _, e := parseTime(p.OccurredAt); e != nil {
			return invalid("credit consumption timestamp")
		}
	case "account_interval":
		p := c.AccountInterval
		if p == nil || !refOK(p.Asset) || p.AccountID <= 0 || !tierOK(p.Tier, false) || !validPeriod(p.Start, p.End) || !refOK(p.EvidenceRef) {
			return invalid("account interval identity, tier or dates")
		}
	case "traffic":
		if c.Traffic == nil {
			return invalid("traffic payload required")
		}
		_, err := AnalyzeTraffic(*c.Traffic)
		return err
	case "replay":
		if c.Replay == nil {
			return invalid("replay payload required")
		}
		_, err := ReplayWindows(*c.Replay)
		return err
	case "forecast":
		if c.Forecast == nil {
			return invalid("forecast payload required")
		}
		return c.Forecast.Validate()
	default:
		return invalid("unknown command kind")
	}
	return nil
}
func instant(v string) time.Time { t, _ := parseTime(v); return t }
func sameCredit(pool, unit, currency string, p *CreditUse) bool {
	return pool == p.Pool && unit == p.Unit && currency == p.Currency
}
func validateExtensionAgainst(active map[string]LedgerEvent, c LedgerCommand, now time.Time) error {
	for _, e := range active {
		if p := c.AccountInterval; p != nil {
			old := e.Command.AccountInterval
			if old != nil && (old.AccountID == p.AccountID || old.Asset == p.Asset) && instant(p.Start).Before(instant(old.End)) && instant(old.Start).Before(instant(p.End)) {
				return invalid("overlapping account tier intervals")
			}
		}
		if p := c.CreditLot; p != nil {
			if old := e.Command.CreditLot; old != nil && p.Pool == old.Pool && p.Reference == old.Reference {
				return ErrConflict
			}
			if u := e.Command.CreditUse; u != nil && sameCredit(p.Pool, p.Unit, p.Currency, u) && !instant(p.PaidAt).After(instant(u.OccurredAt)) {
				return invalid("backdated lot would reprice existing FIFO consumption")
			}
		}
		if p := c.CreditUse; p != nil {
			if old := e.Command.CreditUse; old != nil {
				if old.Reference == p.Reference {
					return ErrConflict
				}
				if sameCredit(p.Pool, p.Unit, p.Currency, old) && instant(p.OccurredAt).Before(instant(old.OccurredAt)) {
					return invalid("backdated use would reprice FIFO; correct dependent uses first")
				}
			}
		}
	}
	if p := c.CreditLot; p != nil && instant(p.PaidAt).After(now) {
		return invalid("future prepaid payment")
	}
	if p := c.CreditUse; p != nil && instant(p.OccurredAt).After(now) {
		return invalid("future credit consumption")
	}
	if p := c.AccountInterval; p != nil && instant(p.Start).After(now) {
		return invalid("future account binding is not observed history")
	}
	if p := c.Traffic; p != nil && instant(p.End).After(now) {
		return invalid("future traffic observation")
	}
	if c.Void != nil {
		target, ok := active[c.Void.TargetID]
		if !ok {
			return nil
		}
		lot, use := target.Command.CreditLot, target.Command.CreditUse
		for _, e := range active {
			u := e.Command.CreditUse
			if u == nil || e.ID == target.ID {
				continue
			}
			if lot != nil && sameCredit(lot.Pool, lot.Unit, lot.Currency, u) && !instant(u.OccurredAt).Before(instant(lot.PaidAt)) {
				return invalid("void dependent credit uses before their lot")
			}
			if use != nil && sameCredit(use.Pool, use.Unit, use.Currency, u) && e.Sequence > target.Sequence {
				return invalid("void later credit uses first")
			}
		}
	}
	if c.CreditUse != nil {
		events := make(map[string]LedgerEvent, len(active)+1)
		var seq int64
		for id, e := range active {
			events[id] = e
			if e.Sequence > seq {
				seq = e.Sequence
			}
		}
		events["candidate"] = LedgerEvent{ID: "candidate", Sequence: seq + 1, Command: c}
		_, err := fifo(events, now)
		return err
	}
	return nil
}

type fifoLot struct {
	event     LedgerEvent
	remaining *big.Rat
}
type fifoUse struct {
	event LedgerEvent
	cost  *big.Rat
	parts []CreditPart
}
type fifoState struct {
	lots []*fifoLot
	uses []fifoUse
}

func fifo(active map[string]LedgerEvent, cutoff time.Time) (fifoState, error) {
	out := fifoState{}
	uses := []LedgerEvent{}
	for _, e := range active {
		if p := e.Command.CreditLot; p != nil && !instant(p.PaidAt).After(cutoff) {
			out.lots = append(out.lots, &fifoLot{e, rat(p.Quantity)})
		}
		if p := e.Command.CreditUse; p != nil && !instant(p.OccurredAt).After(cutoff) {
			uses = append(uses, e)
		}
	}
	sort.Slice(out.lots, func(i, j int) bool {
		a, b := out.lots[i].event, out.lots[j].event
		x, y := instant(a.Command.CreditLot.PaidAt), instant(b.Command.CreditLot.PaidAt)
		if x.Equal(y) {
			return a.Sequence < b.Sequence
		}
		return x.Before(y)
	})
	sort.Slice(uses, func(i, j int) bool {
		a, b := uses[i], uses[j]
		x, y := instant(a.Command.CreditUse.OccurredAt), instant(b.Command.CreditUse.OccurredAt)
		if x.Equal(y) {
			return a.Sequence < b.Sequence
		}
		return x.Before(y)
	})
	for _, e := range uses {
		p := e.Command.CreditUse
		at := instant(p.OccurredAt)
		need := rat(p.Quantity)
		use := fifoUse{event: e, cost: new(big.Rat), parts: []CreditPart{}}
		for _, l := range out.lots {
			lot := l.event.Command.CreditLot
			if !sameCredit(lot.Pool, lot.Unit, lot.Currency, p) || instant(lot.PaidAt).After(at) || !instant(lot.ExpiresAt).After(at) || l.remaining.Sign() == 0 {
				continue
			}
			quantity := new(big.Rat).Set(need)
			if quantity.Cmp(l.remaining) > 0 {
				quantity.Set(l.remaining)
			}
			cost := new(big.Rat).Mul(quantity, new(big.Rat).Quo(rat(lot.Amount), rat(lot.Quantity)))
			use.cost.Add(use.cost, cost)
			need.Sub(need, quantity)
			l.remaining.Sub(l.remaining, quantity)
			use.parts = append(use.parts, CreditPart{l.event.ID, quantity.FloatString(6), cost.FloatString(6)})
			if need.Sign() == 0 {
				break
			}
		}
		if need.Sign() != 0 {
			return out, invalid(fmt.Sprintf("insufficient unexpired prepaid credit for %s", p.Reference))
		}
		out.uses = append(out.uses, use)
	}
	return out, nil
}
func creditReport(active map[string]LedgerEvent, q SummaryQuery) (CreditReport, error) {
	out := CreditReport{CashPaid: "0.000000", ExpiredLoss: "0.000000", exactCosts: map[string]*big.Rat{}, Uses: []CreditConsumption{}, Balances: []CreditBalance{}}
	end := instant(q.End)
	if at := instant(q.AsOf); at.Before(end) {
		end = at
	}
	// Uses at the end of a half-open reporting interval belong to the next period.
	fifoCutoff := instant(q.End).Add(-time.Nanosecond)
	if at := instant(q.AsOf); at.Before(instant(q.End)) {
		fifoCutoff = at
	}
	state, err := fifo(active, fifoCutoff)
	if err != nil {
		return out, err
	}
	cash := new(big.Rat)
	expiredLoss := new(big.Rat)
	for _, l := range state.lots {
		p := l.event.Command.CreditLot
		if p.Currency != q.Currency {
			continue
		}
		if !instant(p.PaidAt).Before(instant(q.Start)) && instant(p.PaidAt).Before(end) {
			cash.Add(cash, rat(p.Amount))
		}
		value := new(big.Rat).Mul(l.remaining, new(big.Rat).Quo(rat(p.Amount), rat(p.Quantity)))
		b := CreditBalance{EventID: l.event.ID, Pool: p.Pool, Unit: p.Unit, RemainingQuantity: l.remaining.FloatString(6), RemainingValue: value.FloatString(6), ExpiredQuantity: "0.000000", ExpiredValue: "0.000000"}
		if !instant(p.ExpiresAt).After(end) {
			b.ExpiredQuantity = b.RemainingQuantity
			b.ExpiredValue = b.RemainingValue
			if !instant(p.ExpiresAt).Before(instant(q.Start)) && instant(p.ExpiresAt).Before(end) {
				expiredLoss.Add(expiredLoss, value)
			}
			b.RemainingQuantity = "0.000000"
			b.RemainingValue = "0.000000"
		}
		out.Balances = append(out.Balances, b)
	}
	for _, u := range state.uses {
		p := u.event.Command.CreditUse
		inScope := !instant(p.OccurredAt).Before(instant(q.Start)) && instant(p.OccurredAt).Before(instant(q.End)) && !instant(p.OccurredAt).After(instant(q.AsOf))
		if inScope && p.Currency != q.Currency {
			out.warnings = append(out.warnings, "OTHER_CURRENCIES_EXCLUDED")
		}
		if p.Currency == q.Currency && inScope {
			out.Uses = append(out.Uses, CreditConsumption{u.event.ID, p.Tier, u.cost.FloatString(6), u.parts})
			if out.exactCosts[p.Tier] == nil {
				out.exactCosts[p.Tier] = new(big.Rat)
			}
			out.exactCosts[p.Tier].Add(out.exactCosts[p.Tier], u.cost)
			if p.Model != q.Model {
				out.warnings = append(out.warnings, "OTHER_WORKLOAD_DELIVERIES_EXCLUDED")
			}
		}
	}
	out.CashPaid = cash.FloatString(6)
	out.ExpiredLoss = expiredLoss.FloatString(6)
	out.exactCosts["unallocated"] = expiredLoss
	return out, nil
}
