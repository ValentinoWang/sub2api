package costing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }
func example() CompareRequest {
	yes := true
	return CompareRequest{Currency: "USD", Unit: "qualified-task-v1", Model: "synthetic-common-workload",
		PeriodStart: "2026-09-01T00:00:00Z", PeriodEnd: "2026-10-01T00:00:00Z", AsOf: "2026-09-16T01:00:00+08:00", Purpose: "existing", Demand: "300",
		Rows: []InputRow{{"plus", ptr("20"), ptr("100"), ptr("1"), &yes, "synthetic"},
			{"pro5x", ptr("100"), ptr("500"), ptr("1"), &yes, "synthetic"},
			{"pro20x", ptr("200"), ptr("2000"), ptr("1"), &yes, "synthetic"}}}
}
func TestThreeTiers(t *testing.T) {
	for _, tc := range []struct{ demand, purpose, want string }{{"50", "existing", "plus"}, {"300", "existing", "pro5x"}, {"900", "existing", "pro20x"}, {"900", "new_purchase", ""}, {"0", "existing", ""}} {
		t.Run(tc.demand+tc.purpose, func(t *testing.T) {
			q := example()
			q.Demand = tc.demand
			q.Purpose = tc.purpose
			r, e := Compare(q)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Join(r.LowestCostFeasibleTiers, ",") != tc.want {
				t.Fatalf("got %+v", r)
			}
		})
	}
}
func TestInvalid(t *testing.T) {
	cases := map[string]func(*CompareRequest){"ambiguous": func(q *CompareRequest) { q.Rows[1].Tier = "pro" },
		"missing tier": func(q *CompareRequest) { q.Rows = q.Rows[:2] }, "duplicate": func(q *CompareRequest) { q.Rows[1].Tier = "plus" },
		"nan": func(q *CompareRequest) { q.Rows[0].PeriodCost = ptr("NaN") }, "fraction": func(q *CompareRequest) { q.Rows[0].UsefulFraction = ptr("2") },
		"negative": func(q *CompareRequest) { q.Demand = "-1" }, "timestamp": func(q *CompareRequest) { q.AsOf = "2026-09-16" },
		"unknown evidence": func(q *CompareRequest) { q.Rows[0].Evidence = "real" }, "missing boolean": func(q *CompareRequest) { q.Rows[0].WorkloadVerified = nil }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			q := example()
			change(&q)
			if _, e := Compare(q); e == nil {
				t.Fatal("expected error")
			}
		})
	}
}
func TestUnknownCapacity(t *testing.T) {
	q := example()
	q.Rows[1].PeriodCapacity = nil
	r, e := Compare(q)
	if e != nil {
		t.Fatal(e)
	}
	if r.Rows[1].Delivered != nil || r.Rows[1].Eligible {
		t.Fatal("missing is not zero")
	}
}
func TestZeroEfficiency(t *testing.T) {
	q := example()
	q.Rows[0].UsefulFraction = ptr("0")
	r, e := Compare(q)
	if e != nil || r.Rows[0].UnitCost != nil {
		t.Fatal("zero delivered cost undefined")
	}
}
func TestExactDecimal(t *testing.T) {
	q := example()
	q.Demand = "3"
	q.Rows[0].PeriodCost = ptr("0.1")
	r, e := Compare(q)
	if e != nil || *r.Rows[0].UnitCost != "0.033333" {
		t.Fatal(r, e)
	}
}
func TestTie(t *testing.T) {
	q := example()
	q.Demand = "50"
	q.Rows[1].PeriodCost = ptr("20")
	r, _ := Compare(q)
	if len(r.LowestCostFeasibleTiers) != 2 {
		t.Fatal(r)
	}
}
func TestExpiredAcquisitionEvidence(t *testing.T) {
	q := example()
	q.AsOf = "2026-10-01T00:00:00Z"
	q.Purpose = "new_purchase"
	r, _ := Compare(q)
	if len(r.LowestCostFeasibleTiers) != 0 {
		t.Fatal(r)
	}
}
func TestCatalogContainsNoCapacity(t *testing.T) {
	c, e := PlanCatalog()
	if e != nil || len(c.Plans) != 3 {
		t.Fatal(e)
	}
	for _, p := range c.Plans {
		if p.MeasuredWeeklyCapacity != nil {
			t.Fatal("invented capacity")
		}
	}
}
func TestHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, path, method, body, content string
		authorized                        bool
		want                              int
	}{
		{"catalog", "catalog", "GET", "", "", true, 200},
		{"unauthenticated", "catalog", "GET", "", "", false, 401},
		{"unknown", "oops", "GET", "", "", true, 404},
		{"method", "compare", "GET", "", "", true, 405},
		{"type", "compare", "POST", "{}", "text/plain", true, 415},
		{"unknown field", "compare", "POST", "{\"extra\":1}", "application/json", true, 400},
		{"too large", "compare", "POST", strings.Repeat(" ", 65537) + "{}", "application/json", true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/v1/admin/cost-center/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.content)
			rec := httptest.NewRecorder()
			h := HTTPHandler{Authorize: func(*http.Request) bool { return tc.authorized }}
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	raw, _ := json.Marshal(example())
	req := httptest.NewRequest("POST", "/api/v1/admin/cost-center/compare", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HTTPHandler{Authorize: func(*http.Request) bool { return true }}.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "pro5x") {
		t.Fatal(rec.Body.String())
	}
}
func TestNilAuthorizationDenies(t *testing.T) {
	rec := httptest.NewRecorder()
	HTTPHandler{}.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/admin/cost-center/catalog", nil))
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
}

func TestSharedGoldenContract(t *testing.T) {
	raw, e := os.ReadFile("testdata/comparison_cases.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Name      string         `json:"name"`
		Request   CompareRequest `json:"request"`
		Winners   []string       `json:"winners"`
		UnitCosts []string       `json:"unit_costs"`
	}
	if e = json.Unmarshal(raw, &cases); e != nil {
		t.Fatal(e)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r, e := Compare(tc.Request)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Join(r.LowestCostFeasibleTiers, ",") != strings.Join(tc.Winners, ",") {
				t.Fatal(r)
			}
			for i, row := range r.Rows {
				if row.UnitCost == nil || *row.UnitCost != tc.UnitCosts[i] {
					t.Fatalf("cost mismatch %+v", row)
				}
			}
		})
	}
}
