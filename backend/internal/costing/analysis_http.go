package costing

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
)

func (h HTTPHandler) serveAnalysis(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/api/v1/admin/cost-center/source/traffic" {
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
			return
		}
		samples, err := h.Traffic.Latest(r.Context())
		if err != nil {
			ledgerError(w, err)
			return
		}
		ledgerReply(w, 200, map[string]any{"items": samples, "status": "OBSERVED_COUNTERS_NOT_BILLED_COST"}, "success")
		return
	}
	if path == "/api/v1/admin/cost-center/source/sync" {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
			return
		}
		if h.Actor == nil || h.Actor(r) <= 0 {
			ledgerReply(w, 403, nil, "VERIFIED_ADMIN_ACTOR_REQUIRED")
			return
		}
		typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || typ != "application/json" {
			ledgerReply(w, 415, nil, "JSON_REQUIRED")
			return
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		var query SummaryQuery
		if decoder.Decode(&query) != nil || decoder.Decode(new(any)) != io.EOF {
			ledgerReply(w, 400, nil, "INVALID_QUERY")
			return
		}
		if query.AsOf == "" {
			query.AsOf = time.Now().UTC().Format(time.RFC3339Nano)
		}
		report, err := h.Source.Reconcile(r.Context(), h.Ledger, query)
		if err != nil {
			ledgerError(w, err)
			return
		}
		source := h.Source.Origin
		if source == "" {
			source = "configured-source"
		}
		result, err := h.Snapshots.Save(r.Context(), source, h.Actor(r), query, report)
		if err != nil {
			ledgerError(w, err)
			return
		}
		ledgerReply(w, 200, result, "success")
		return
	}
	if path == "/api/v1/admin/cost-center/source/quota" {
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
			return
		}
		q := r.URL.Query()
		id, err := strconv.ParseInt(q.Get("account_id"), 10, 64)
		if err != nil {
			ledgerError(w, invalid("account required"))
			return
		}
		items, err := h.Quota.List(r.Context(), id, q.Get("start"), q.Get("end"))
		if err != nil {
			ledgerError(w, err)
			return
		}
		ledgerReply(w, 200, map[string]any{"items": items, "status": "OBSERVATIONS_NOT_VERIFIED_RESET_GAINS", "coverage_complete": false}, "success")
		return
	}
	if path == "/api/v1/admin/cost-center/source/accounts" || path == "/api/v1/admin/cost-center/source/reconciliation" {
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
			return
		}
		if path == "/api/v1/admin/cost-center/source/accounts" {
			rows, err := h.Source.Accounts(r.Context())
			if err != nil {
				ledgerError(w, err)
				return
			}
			ledgerReply(w, 200, map[string]any{"items": rows, "status": "READ_ONLY_SOURCE"}, "success")
			return
		}
		q := r.URL.Query()
		cutoff := q.Get("as_of")
		if cutoff == "" {
			cutoff = time.Now().UTC().Format(time.RFC3339Nano)
		}
		result, err := h.Source.Reconcile(r.Context(), h.Ledger, SummaryQuery{Start: q.Get("start"), End: q.Get("end"), AsOf: cutoff, Model: q.Get("model")})
		if err != nil {
			ledgerError(w, err)
			return
		}
		ledgerReply(w, 200, result, "success")
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
		return
	}
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "application/json" {
		ledgerReply(w, 415, nil, "JSON_REQUIRED")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 262144))
	decoder.DisallowUnknownFields()
	var input any
	switch path {
	case "/api/v1/admin/cost-center/analysis/traffic":
		input = &TrafficInput{}
	case "/api/v1/admin/cost-center/analysis/replay":
		input = &ReplayInput{}
	case "/api/v1/admin/cost-center/analysis/forecast":
		input = &ForecastInput{}
	default:
		ledgerReply(w, 404, nil, "NOT_FOUND")
		return
	}
	if decoder.Decode(input) != nil || decoder.Decode(new(any)) != io.EOF {
		ledgerReply(w, 400, nil, "INVALID_ANALYSIS_INPUT")
		return
	}
	var result any
	switch q := input.(type) {
	case *TrafficInput:
		result, err = AnalyzeTraffic(*q)
	case *ReplayInput:
		result, err = ReplayWindows(*q)
	case *ForecastInput:
		result, err = Forecast(*q)
	}
	if err != nil {
		ledgerError(w, err)
		return
	}
	ledgerReply(w, 200, result, "success")
}
