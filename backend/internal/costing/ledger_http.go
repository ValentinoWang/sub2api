package costing

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
)

func ledgerReply(w http.ResponseWriter, status int, data any, reason string) {
	w.WriteHeader(status)
	code := status
	if status == 200 || status == 201 {
		code = 0
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": reason, "reason": reason, "data": data})
}
func ledgerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		ledgerReply(w, 422, nil, err.Error())
	case errors.Is(err, ErrConflict):
		ledgerReply(w, 409, nil, "COST_LEDGER_CONFLICT")
	case errors.Is(err, ErrNotFound):
		ledgerReply(w, 404, nil, "COST_RECORD_NOT_FOUND")
	case errors.Is(err, ErrLimit):
		ledgerReply(w, 422, nil, "COST_LEDGER_LIMIT_REQUIRES_MIGRATION")
	default:
		ledgerReply(w, 503, nil, "COST_LEDGER_UNAVAILABLE")
	}
}
func (h HTTPHandler) serveLedger(w http.ResponseWriter, r *http.Request) {
	if h.Ledger == nil || h.Ledger.Store == nil {
		ledgerReply(w, 503, nil, "COST_LEDGER_NOT_CONFIGURED")
		return
	}
	expected := "GET"
	if r.URL.Path == "/api/v1/admin/cost-center/ledger/commands" {
		expected = "POST"
	}
	if r.Method != expected {
		w.Header().Set("Allow", expected)
		ledgerReply(w, 405, nil, "METHOD_NOT_ALLOWED")
		return
	}
	switch r.URL.Path {
	case "/api/v1/admin/cost-center/ledger/commands":
		if h.Actor == nil || h.Actor(r) <= 0 {
			ledgerReply(w, 403, nil, "VERIFIED_ADMIN_ACTOR_REQUIRED")
			return
		}
		typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || typ != "application/json" {
			ledgerReply(w, 415, nil, "JSON_REQUIRED")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var c LedgerCommand
		if e = decoder.Decode(&c); e != nil || decoder.Decode(new(any)) != io.EOF {
			ledgerReply(w, 400, nil, "INVALID_OR_OVERSIZED_COMMAND")
			return
		}
		result, e := h.Ledger.Append(r.Context(), h.Actor(r), r.Header.Get("Idempotency-Key"), c)
		if e != nil {
			ledgerError(w, e)
			return
		}
		status := 201
		if result.Replayed {
			status = 200
		}
		ledgerReply(w, status, result, "success")
	case "/api/v1/admin/cost-center/ledger/events":
		after := 0
		limit := 50
		var e error
		if s := r.URL.Query().Get("after"); s != "" {
			after, e = strconv.Atoi(s)
			if e != nil || after < 0 {
				ledgerReply(w, 400, nil, "INVALID_CURSOR")
				return
			}
		}
		if s := r.URL.Query().Get("limit"); s != "" {
			limit, e = strconv.Atoi(s)
			if e != nil || limit < 1 || limit > 200 {
				ledgerReply(w, 400, nil, "INVALID_LIMIT")
				return
			}
		}
		events, e := h.Ledger.Store.Snapshot(r.Context())
		if e != nil {
			ledgerError(w, e)
			return
		}
		items := []LedgerEvent{}
		var next *int64
		for _, ev := range events {
			if ev.Sequence <= int64(after) {
				continue
			}
			if len(items) == limit {
				v := items[len(items)-1].Sequence
				next = &v
				break
			}
			items = append(items, ev)
		}
		ledgerReply(w, 200, map[string]any{"items": items, "next_after": next, "total": len(events)}, "success")
	case "/api/v1/admin/cost-center/ledger/summary":
		q := r.URL.Query()
		cutoff := q.Get("as_of")
		if cutoff == "" {
			cutoff = time.Now().UTC().Format(time.RFC3339Nano)
		}
		result, e := h.Ledger.Summary(r.Context(), SummaryQuery{Start: q.Get("start"), End: q.Get("end"), AsOf: cutoff, Currency: q.Get("currency"), Model: q.Get("model"), Unit: q.Get("unit")})
		if e != nil {
			ledgerError(w, e)
			return
		}
		ledgerReply(w, 200, result, "success")
	case "/api/v1/admin/cost-center/ledger/health":
		events, e := h.Ledger.Store.Snapshot(r.Context())
		if e != nil {
			ledgerError(w, e)
			return
		}
		ledgerReply(w, 200, map[string]any{"status": "READABLE", "event_count": len(events), "event_limit": LedgerLimit, "schema_version": 1}, "success")
	default:
		ledgerReply(w, 404, nil, "NOT_FOUND")
	}
}
