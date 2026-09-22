package costing

import (
	"encoding/json"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"time"
)

// TrafficInput compares two cumulative vnStat v2 snapshots of one billing interface.
// Tariffs are explicit assumptions; derived charges never become invoice purchases.
type TrafficInput struct {
	Interface        string          `json:"interface"`
	Start            string          `json:"start"`
	End              string          `json:"end"`
	Before           json.RawMessage `json:"before"`
	After            json.RawMessage `json:"after"`
	Mode             string          `json:"mode"`
	IncludedBytes    string          `json:"included_bytes"`
	GBBytes          string          `json:"gb_bytes"`
	Rate             string          `json:"rate"`
	Currency         string          `json:"currency"`
	CoverageComplete bool            `json:"coverage_complete"`
}
type TrafficResult struct {
	Status   string  `json:"status"`
	RXBytes  *string `json:"rx_bytes"`
	TXBytes  *string `json:"tx_bytes"`
	Charge   *string `json:"estimated_charge"`
	Currency string  `json:"currency"`
}
type vnstatCounter struct {
	RX json.Number `json:"rx"`
	TX json.Number `json:"tx"`
}
type vnstatInterface struct {
	Name    string          `json:"name"`
	Created json.RawMessage `json:"created"`
	Updated struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"updated"`
	Traffic struct {
		Total vnstatCounter `json:"total"`
	} `json:"traffic"`
}

func integer(v string) (*big.Int, bool) {
	n, ok := new(big.Int).SetString(v, 10)
	return n, ok && len(v) <= 24 && n.Sign() >= 0 && n.String() == v
}
func counter(raw json.RawMessage, iface, at string) (vnstatInterface, error) {
	var data struct {
		Version    json.RawMessage   `json:"jsonversion"`
		Interfaces []vnstatInterface `json:"interfaces"`
	}
	if len(raw) > 65536 || json.Unmarshal(raw, &data) != nil || (string(data.Version) != "2" && string(data.Version) != "\"2\"") {
		return vnstatInterface{}, invalid("vnStat JSON v2 required")
	}
	matches := []vnstatInterface{}
	for _, i := range data.Interfaces {
		if i.Name == iface {
			matches = append(matches, i)
		}
	}
	if len(matches) != 1 {
		return vnstatInterface{}, invalid("one explicit billing interface required")
	}
	i := matches[0]
	if _, ok := integer(i.Traffic.Total.RX.String()); !ok {
		return i, invalid("missing or invalid vnStat RX total")
	}
	if _, ok := integer(i.Traffic.Total.TX.String()); !ok {
		return i, invalid("missing or invalid vnStat TX total")
	}
	if len(i.Created) == 0 || string(i.Created) == "null" || i.Updated.Timestamp != instant(at).Unix() {
		return i, invalid("vnStat database identity and exact updated timestamp required")
	}
	return i, nil
}
func AnalyzeTraffic(q TrafficInput) (TrafficResult, error) {
	out := TrafficResult{Status: "INCOMPLETE_COVERAGE", Currency: q.Currency}
	included, ok := integer(q.IncludedBytes)
	gb, gbOK := integer(q.GBBytes)
	if !textOK(q.Interface, 100) || !validPeriod(q.Start, q.End) || !currencyOK(q.Currency) || !nonnegativeMoney(q.Rate) || !ok || !gbOK || gb.Sign() == 0 || (q.Mode != "egress" && q.Mode != "both") {
		return out, invalid("traffic interval or explicit volume tariff required")
	}
	before, err := counter(q.Before, q.Interface, q.Start)
	if err != nil {
		return out, err
	}
	after, err := counter(q.After, q.Interface, q.End)
	if err != nil {
		return out, err
	}
	var a, b any
	if json.Unmarshal(before.Created, &a) != nil || json.Unmarshal(after.Created, &b) != nil {
		return out, invalid("invalid vnStat database identity")
	}
	ac, _ := json.Marshal(a)
	bc, _ := json.Marshal(b)
	rx0, _ := integer(before.Traffic.Total.RX.String())
	rx1, _ := integer(after.Traffic.Total.RX.String())
	tx0, _ := integer(before.Traffic.Total.TX.String())
	tx1, _ := integer(after.Traffic.Total.TX.String())
	if string(ac) != string(bc) || rx1.Cmp(rx0) < 0 || tx1.Cmp(tx0) < 0 {
		out.Status = "COUNTER_RESET"
		return out, nil
	}
	if !q.CoverageComplete {
		return out, nil
	}
	rx := new(big.Int).Sub(rx1, rx0)
	tx := new(big.Int).Sub(tx1, tx0)
	rs, ts := rx.String(), tx.String()
	out.RXBytes = &rs
	out.TXBytes = &ts
	used := new(big.Int).Set(tx)
	if q.Mode == "both" {
		used.Add(used, rx)
	}
	used.Sub(used, included)
	if used.Sign() < 0 {
		used.SetInt64(0)
	}
	charge := new(big.Rat).Mul(new(big.Rat).SetFrac(used, gb), rat(q.Rate)).FloatString(6)
	out.Charge = &charge
	out.Status = "TARIFF_ESTIMATE_NOT_INVOICE"
	return out, nil
}

type QuotaWindow struct {
	ID              string `json:"id"`
	Capacity        string `json:"capacity"`
	Remaining       string `json:"remaining"`
	DurationSeconds int64  `json:"duration_seconds"`
	ResetAt         string `json:"reset_at"`
}
type QuotaAction struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Kind   string `json:"kind"`
	Window string `json:"window,omitempty"`
	Amount string `json:"amount"`
}
type ReplayInput struct {
	Pool             string        `json:"pool"`
	Model            string        `json:"model"`
	PriceVersion     string        `json:"price_version"`
	Start            string        `json:"start"`
	End              string        `json:"end"`
	CoverageComplete bool          `json:"coverage_complete"`
	Windows          []QuotaWindow `json:"windows"`
	Actions          []QuotaAction `json:"actions"`
}
type ReplayStep struct {
	ID        string `json:"id"`
	Delivered string `json:"delivered"`
	Unmet     string `json:"unmet"`
	NetRefill string `json:"net_refill"`
}
type ReplayResult struct {
	Status    string        `json:"status"`
	Delivered *string       `json:"delivered"`
	Unmet     *string       `json:"unmet"`
	Steps     []ReplayStep  `json:"steps"`
	Windows   []QuotaWindow `json:"windows"`
}

func ReplayWindows(q ReplayInput) (ReplayResult, error) {
	out := ReplayResult{Status: "INCOMPLETE_WINDOW_COVERAGE", Steps: []ReplayStep{}, Windows: []QuotaWindow{}}
	if !refOK(q.Pool) || !textOK(q.Model, 100) || !refOK(q.PriceVersion) || !validPeriod(q.Start, q.End) || len(q.Actions) > 10000 || len(q.Windows) > 8 {
		return out, invalid("quota replay scope")
	}
	type state struct {
		spec           QuotaWindow
		cap, remaining *big.Rat
		reset          time.Time
	}
	windows := map[string]*state{}
	for _, w := range q.Windows {
		if !refOK(w.ID) || windows[w.ID] != nil || !validateMoney(w.Capacity) || !nonnegativeMoney(w.Remaining) || rat(w.Remaining).Cmp(rat(w.Capacity)) > 0 || w.DurationSeconds < 1 || w.DurationSeconds > 366*86400 {
			return out, invalid("quota window capacity or duration")
		}
		reset, err := parseTime(w.ResetAt)
		if err != nil || !reset.After(instant(q.Start)) || reset.Sub(instant(q.Start)) > time.Duration(w.DurationSeconds)*time.Second {
			return out, invalid("explicit next natural reset required")
		}
		windows[w.ID] = &state{w, rat(w.Capacity), rat(w.Remaining), reset}
	}
	if !q.CoverageComplete || len(windows) < 2 {
		return out, nil
	}
	actions := append([]QuotaAction{}, q.Actions...)
	seen := map[string]bool{}
	for _, a := range actions {
		at, err := parseTime(a.At)
		if !refOK(a.ID) || seen[a.ID] || err != nil || at.Before(instant(q.Start)) || !at.Before(instant(q.End)) || !nonnegativeMoney(a.Amount) || (a.Kind != "demand" && a.Kind != "refill") {
			return out, invalid("quota action identity, timestamp or amount")
		}
		if a.Kind == "refill" && windows[a.Window] == nil {
			return out, invalid("refill must identify one known budget window")
		}
		if a.Kind == "demand" && a.Window != "" {
			return out, invalid("demand consumes all active windows")
		}
		seen[a.ID] = true
	}
	sort.SliceStable(actions, func(i, j int) bool { return instant(actions[i].At).Before(instant(actions[j].At)) })
	delivered, unmet := new(big.Rat), new(big.Rat)
	advance := func(at time.Time) {
		for _, w := range windows {
			if !at.Before(w.reset) {
				duration := time.Duration(w.spec.DurationSeconds) * time.Second
				cycles := at.Sub(w.reset)/duration + 1
				w.reset = w.reset.Add(cycles * duration)
				w.remaining.Set(w.cap)
			}
		}
	}
	for _, a := range actions {
		advance(instant(a.At))
		step := ReplayStep{ID: a.ID, Delivered: "0.000000", Unmet: "0.000000", NetRefill: "0.000000"}
		amount := rat(a.Amount)
		if a.Kind == "refill" {
			w := windows[a.Window]
			gain := new(big.Rat).Sub(w.cap, w.remaining)
			if gain.Cmp(amount) > 0 {
				gain.Set(amount)
			}
			w.remaining.Add(w.remaining, gain)
			step.NetRefill = gain.FloatString(6)
		} else {
			use := new(big.Rat).Set(amount)
			for _, w := range windows {
				if use.Cmp(w.remaining) > 0 {
					use.Set(w.remaining)
				}
			}
			for _, w := range windows {
				w.remaining.Sub(w.remaining, use)
			}
			missing := new(big.Rat).Sub(amount, use)
			delivered.Add(delivered, use)
			unmet.Add(unmet, missing)
			step.Delivered = use.FloatString(6)
			step.Unmet = missing.FloatString(6)
		}
		out.Steps = append(out.Steps, step)
	}
	advance(instant(q.End).Add(-time.Nanosecond))
	for _, spec := range q.Windows {
		w := windows[spec.ID]
		spec.Remaining = w.remaining.FloatString(6)
		spec.ResetAt = w.reset.UTC().Format(time.RFC3339)
		out.Windows = append(out.Windows, spec)
	}
	d, u := delivered.FloatString(6), unmet.FloatString(6)
	out.Delivered = &d
	out.Unmet = &u
	out.Status = "DECLARED_WINDOW_SCENARIO"
	return out, nil
}

// ForecastInput mirrors the research Gamma-Poisson and Bayesian-bootstrap model.
// The interval conditions on calibrated capacity and utilization; it is not a bill.
type ForecastInput struct {
	OwnExposureComplete        bool      `json:"own_exposure_complete"`
	WeeklyReferenceCapacity    float64   `json:"weekly_reference_capacity"`
	BillingDays                float64   `json:"billing_days"`
	CashAndAllocatedCost       float64   `json:"cash_and_allocated_cost"`
	PriorEventShape            float64   `json:"prior_event_shape"`
	PriorExposureWeeks         float64   `json:"prior_exposure_weeks"`
	OwnObservedWeeks           float64   `json:"own_observed_weeks"`
	OwnEventCount              int       `json:"own_event_count"`
	VerifiedNetRefillFractions []float64 `json:"verified_net_refill_fractions"`
	UsefulUtilization          float64   `json:"useful_utilization"`
	Seed                       int64     `json:"seed"`
}
type ForecastResult struct {
	Status      string   `json:"status"`
	P10         float64  `json:"cost_p10"`
	P50         float64  `json:"cost_p50"`
	P90         float64  `json:"cost_p90"`
	Rate        float64  `json:"event_rate_posterior_mean_per_week"`
	Seed        int64    `json:"seed"`
	Samples     int      `json:"samples"`
	Assumptions []string `json:"assumptions"`
}

func positiveFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }
func (q ForecastInput) Validate() error {
	if !q.OwnExposureComplete {
		return invalid("complete own event exposure required")
	}
	for _, v := range []float64{q.WeeklyReferenceCapacity, q.BillingDays, q.CashAndAllocatedCost, q.PriorEventShape, q.PriorExposureWeeks, q.OwnObservedWeeks, q.UsefulUtilization} {
		if !positiveFinite(v) || v > 1e12 {
			return invalid("positive finite forecast inputs required")
		}
	}
	if q.UsefulUtilization > 1 || q.BillingDays > 366 || q.OwnEventCount < 1 || q.OwnEventCount > 10000 || len(q.VerifiedNetRefillFractions) != q.OwnEventCount {
		return invalid("observed event gains including zero gains required")
	}
	for _, d := range q.VerifiedNetRefillFractions {
		if math.IsNaN(d) || math.IsInf(d, 0) || d < 0 || d > 1 {
			return invalid("net refill fraction outside [0,1]")
		}
	}
	if (q.PriorEventShape+float64(q.OwnEventCount))/(q.PriorExposureWeeks+q.OwnObservedWeeks)*q.BillingDays/7 > 1000 {
		return invalid("forecast event workload exceeds interactive limit")
	}
	return nil
}
func gamma(r *rand.Rand, shape float64) float64 {
	if shape < 1 {
		return gamma(r, shape+1) * math.Pow(r.Float64(), 1/shape)
	}
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := r.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
func Forecast(q ForecastInput) (ForecastResult, error) {
	out := ForecastResult{Status: "CONDITIONAL_SCENARIO", Seed: q.Seed, Samples: 4000, Assumptions: []string{"FIXED_CALIBRATED_BASE_CAPACITY", "FIXED_USEFUL_UTILIZATION", "ONE_STATIONARY_EVENT_TYPE", "NO_CONCURRENCY_OR_LATENCY_MODEL", "NOT_AN_INVOICE_OR_COST_FLOOR"}}
	if err := q.Validate(); err != nil {
		return out, err
	}
	rng := rand.New(rand.NewSource(q.Seed))
	samples := make([]float64, 4000)
	out.Rate = (q.PriorEventShape + float64(q.OwnEventCount)) / (q.PriorExposureWeeks + q.OwnObservedWeeks)
	budget := 0
	for i := range samples {
		rate := gamma(rng, q.PriorEventShape+float64(q.OwnEventCount)) / (q.PriorExposureWeeks + q.OwnObservedWeeks)
		weights := make([]float64, len(q.VerifiedNetRefillFractions))
		sum := 0.0
		for j := range weights {
			sum += rng.ExpFloat64()
			weights[j] = sum
		}
		clock, gain := 0.0, 0.0
		for {
			clock += rng.ExpFloat64() / rate
			if clock >= q.BillingDays/7 {
				break
			}
			budget++
			if budget > 8000000 {
				return out, invalid("forecast sampling budget exceeded")
			}
			draw := rng.Float64() * sum
			j := sort.SearchFloat64s(weights, draw)
			if j == len(weights) {
				j--
			}
			gain += q.VerifiedNetRefillFractions[j]
		}
		delivered := q.WeeklyReferenceCapacity * (q.BillingDays/7 + gain) * q.UsefulUtilization
		samples[i] = q.CashAndAllocatedCost / delivered
		if !positiveFinite(samples[i]) {
			return out, invalid("forecast numerical range")
		}
	}
	sort.Float64s(samples)
	out.P10 = samples[399]
	out.P50 = samples[1999]
	out.P90 = samples[3599]
	return out, nil
}
