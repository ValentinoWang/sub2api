package costing

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
)

// HTTPHandler must be mounted behind the existing admin middleware. Nil authorization denies all.
// The callback is useful for unit testing; the Gin adapter confirms prior admin middleware execution.
type HTTPHandler struct{ Authorize func(*http.Request) bool }

func (h HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	reply := func(status int, data any, message string) {
		w.WriteHeader(status)
		code := status
		if status == 200 {
			code = 0
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": message, "data": data})
	}
	if h.Authorize == nil || !h.Authorize(r) {
		reply(401, nil, "admin authorization required")
		return
	}
	switch r.URL.Path {
	case "/api/v1/admin/cost-center/catalog":
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			reply(405, nil, "GET required")
			return
		}
		c, e := PlanCatalog()
		if e != nil {
			reply(500, nil, "catalog unavailable")
			return
		}
		reply(200, c, "success")
	case "/api/v1/admin/cost-center/compare":
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			reply(405, nil, "POST required")
			return
		}
		typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || typ != "application/json" {
			reply(415, nil, "JSON required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var q CompareRequest
		if e = decoder.Decode(&q); e != nil {
			reply(400, nil, "invalid or oversized comparison request")
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			reply(400, nil, "one JSON object required")
			return
		}
		result, e := Compare(q)
		if e != nil {
			reply(400, nil, e.Error())
			return
		}
		reply(200, result, "success")
	default:
		reply(404, nil, "not found")
	}
}
