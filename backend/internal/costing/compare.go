// Package costing contains side-effect-free plan comparison; no credentials, accounts or purchases.
package costing

import (
	_ "embed"
	"encoding/json"
	"errors"
	"math/big"
	"regexp"
	"time"
)

//go:embed catalog.json
var catalogJSON []byte

var tiers = []string{"plus", "pro5x", "pro20x"}
var decimalPattern = regexp.MustCompile(`^(?:[0-9]{1,24}(?:\.[0-9]{1,18})?|\.[0-9]{1,18})$`)

type Plan struct {
	ID                        string  `json:"id"`
	Label                     string  `json:"label"`
	ReferenceMonthlyUSD       string  `json:"reference_monthly_usd"`
	AdvertisedUsageMultiplier string  `json:"advertised_usage_multiplier"`
	NewPurchaseStatus         string  `json:"new_purchase_status"`
	RenewalStatus             string  `json:"renewal_status"`
	MeasuredWeeklyCapacity    *string `json:"measured_weekly_capacity"`
	ModelCapabilities         string  `json:"model_capabilities"`
	PauseEffective            string  `json:"pause_effective,omitempty"`
}
type Catalog struct {
	Version    string              `json:"version"`
	CheckedAt  string              `json:"checked_at"`
	ValidUntil string              `json:"valid_until"`
	Scope      string              `json:"scope"`
	Evidence   []map[string]string `json:"evidence"`
	Plans      []Plan              `json:"plans"`
}

func PlanCatalog() (Catalog, error) {
	var c Catalog
	err := json.Unmarshal(catalogJSON, &c)
	return c, err
}

type InputRow struct {
	Tier             string  `json:"tier"`
	PeriodCost       *string `json:"period_cost"`
	PeriodCapacity   *string `json:"period_capacity"`
	UsefulFraction   *string `json:"useful_fraction"`
	WorkloadVerified *bool   `json:"workload_verified"`
	Evidence         string  `json:"evidence"`
}
type CompareRequest struct {
	Currency    string     `json:"currency"`
	Unit        string     `json:"unit"`
	Model       string     `json:"model"`
	PeriodStart string     `json:"period_start"`
	PeriodEnd   string     `json:"period_end"`
	AsOf        string     `json:"as_of"`
	Purpose     string     `json:"purpose"`
	Demand      string     `json:"demand"`
	Rows        []InputRow `json:"rows"`
}
type OutputRow struct {
	Tier              string   `json:"tier"`
	Label             string   `json:"label"`
	PeriodCost        *string  `json:"period_cost"`
	PeriodCapacity    *string  `json:"period_capacity"`
	EffectiveCapacity *string  `json:"effective_capacity"`
	Delivered         *string  `json:"delivered"`
	Unmet             *string  `json:"unmet"`
	UnitCost          *string  `json:"unit_cost"`
	UnusedRawCapacity *string  `json:"unused_raw_capacity"`
	Eligible          bool     `json:"eligible"`
	Reasons           []string `json:"reasons"`
	Evidence          string   `json:"evidence"`
}
type CompareResult struct {
	Status                  string      `json:"status"`
	CatalogVersion          string      `json:"catalog_version"`
	Currency                string      `json:"currency"`
	Unit                    string      `json:"unit"`
	Model                   string      `json:"model"`
	Purpose                 string      `json:"purpose"`
	Demand                  string      `json:"demand"`
	Rows                    []OutputRow `json:"rows"`
	LowestCostFeasibleTiers []string    `json:"lowest_cost_feasible_tiers"`
	Boundaries              []string    `json:"boundaries"`
}

func decimal(v *string, boundedOne bool) (*big.Rat, error) {
	if v == nil {
		return nil, nil
	}
	if !decimalPattern.MatchString(*v) {
		return nil, errors.New("use a nonnegative decimal string, not exponent, NaN or float")
	}
	n, ok := new(big.Rat).SetString(*v)
	if !ok || (boundedOne && n.Cmp(big.NewRat(1, 1)) > 0) {
		return nil, errors.New("decimal out of range")
	}
	return n, nil
}
func str(n *big.Rat, digits int) *string {
	if n == nil {
		return nil
	}
	s := n.FloatString(digits)
	return &s
}
func parseTime(s string) (time.Time, error) {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e == nil && (t.Year() < 2000 || t.Year() > 2100) {
		e = errors.New("time outside supported range")
	}
	return t, e
}
func Compare(q CompareRequest) (CompareResult, error) {
	bad := func(message string) (CompareResult, error) { return CompareResult{}, errors.New(message) }
	if q.Currency != "USD" && q.Currency != "CNY" && q.Currency != "HKD" && q.Currency != "SGD" && q.Currency != "EUR" {
		return bad("one supported currency required")
	}
	if len(q.Unit) == 0 || len(q.Unit) > 100 || len(q.Model) == 0 || len(q.Model) > 100 {
		return bad("explicit workload unit and model required")
	}
	if q.Purpose != "existing" && q.Purpose != "new_purchase" {
		return bad("purpose must be existing or new_purchase")
	}
	start, e := parseTime(q.PeriodStart)
	if e != nil {
		return bad("invalid period start")
	}
	end, e := parseTime(q.PeriodEnd)
	if e != nil || !end.After(start) {
		return bad("invalid half-open reporting period")
	}
	at, e := parseTime(q.AsOf)
	if e != nil {
		return bad("invalid as_of")
	}
	demand, e := decimal(&q.Demand, false)
	if e != nil {
		return bad("invalid qualified demand")
	}
	if len(q.Rows) != 3 {
		return bad("all three SKUs required exactly once")
	}
	c, e := PlanCatalog()
	if e != nil {
		return bad("catalog unavailable")
	}
	checked, e := parseTime(c.CheckedAt)
	if e != nil {
		return bad("catalog date invalid")
	}
	until, e := parseTime(c.ValidUntil)
	if e != nil {
		return bad("catalog validity invalid")
	}
	rows := map[string]InputRow{}
	for _, r := range q.Rows {
		if r.Tier != "plus" && r.Tier != "pro5x" && r.Tier != "pro20x" {
			return bad("explicit SKU required; Pro is ambiguous")
		}
		if _, ok := rows[r.Tier]; ok {
			return bad("duplicate SKU")
		}
		if r.WorkloadVerified == nil {
			return bad("workload_verified must be explicit")
		}
		if r.Evidence != "synthetic" && r.Evidence != "observed" && r.Evidence != "assumption" {
			return bad("explicit evidence status required")
		}
		rows[r.Tier] = r
	}
	result := CompareResult{Status: "CONDITIONAL_COMPARISON", CatalogVersion: c.Version, Currency: q.Currency,
		Unit: q.Unit, Model: q.Model, Purpose: q.Purpose, Demand: q.Demand, Rows: []OutputRow{}, LowestCostFeasibleTiers: []string{},
		Boundaries: []string{"Qualified-delivery target; capacities are independent inputs, not multiplied reference quotas.",
			"No buying, plan changes, model calls or pricing writes.", "Scalar capacity is not a concurrency or latency guarantee.",
			"Declared costs have not been reconciled to invoices."}}
	var cheapest *big.Rat
	for _, id := range tiers {
		r := rows[id]
		var plan Plan
		for _, p := range c.Plans {
			if p.ID == id {
				plan = p
			}
		}
		cost, er := decimal(r.PeriodCost, false)
		if er != nil {
			return bad("invalid period_cost")
		}
		capacity, er := decimal(r.PeriodCapacity, false)
		if er != nil {
			return bad("invalid period_capacity")
		}
		useful, er := decimal(r.UsefulFraction, true)
		if er != nil {
			return bad("invalid useful_fraction")
		}
		out := OutputRow{Tier: id, Label: plan.Label, PeriodCost: str(cost, 6), PeriodCapacity: str(capacity, 18), Reasons: []string{}, Evidence: r.Evidence}
		if cost == nil || capacity == nil || useful == nil {
			out.Reasons = append(out.Reasons, "MISSING_COST_OR_CAPACITY")
		}
		if !*r.WorkloadVerified {
			out.Reasons = append(out.Reasons, "WORKLOAD_NOT_VERIFIED")
		}
		if q.Purpose == "new_purchase" {
			if at.Before(checked) || !at.Before(until) {
				out.Reasons = append(out.Reasons, "ACQUISITION_FACTS_REQUIRE_REFRESH")
			} else if plan.NewPurchaseStatus == "paused" {
				out.Reasons = append(out.Reasons, "NEW_PURCHASE_PAUSED")
			}
		}
		if capacity != nil && useful != nil {
			effective := new(big.Rat).Mul(capacity, useful)
			delivered := new(big.Rat).Set(effective)
			if delivered.Cmp(demand) > 0 {
				delivered.Set(demand)
			}
			unmet := new(big.Rat).Sub(demand, delivered)
			out.EffectiveCapacity = str(effective, 18)
			out.Delivered = str(delivered, 18)
			out.Unmet = str(unmet, 18)
			if unmet.Sign() > 0 {
				out.Reasons = append(out.Reasons, "INSUFFICIENT_USEFUL_CAPACITY")
			}
			if delivered.Sign() > 0 && cost != nil {
				out.UnitCost = str(new(big.Rat).Quo(cost, delivered), 6)
			}
			if useful.Sign() > 0 {
				unused := new(big.Rat).Sub(capacity, new(big.Rat).Quo(delivered, useful))
				out.UnusedRawCapacity = str(unused, 18)
			}
		}
		if demand.Sign() == 0 {
			out.Reasons = append(out.Reasons, "NO_DEMAND")
		}
		out.Eligible = len(out.Reasons) == 0
		if out.Eligible {
			if cheapest == nil || cost.Cmp(cheapest) < 0 {
				cheapest = new(big.Rat).Set(cost)
				result.LowestCostFeasibleTiers = []string{id}
			} else if cost.Cmp(cheapest) == 0 {
				result.LowestCostFeasibleTiers = append(result.LowestCostFeasibleTiers, id)
			}
		}
		result.Rows = append(result.Rows, out)
	}
	return result, nil
}
