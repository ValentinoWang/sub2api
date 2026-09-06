package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJobClientsUseExactPathsAndBearerAuthentication(t *testing.T) {
	const jobID = "job-123"
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer admin-secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/admin/tools/ldxp/jobs/" + jobID:
			writeEnvelope(t, w, `{"job_id":"job-123","status":"queued"}`)
		case "POST /api/v1/admin/tools/ldxp/jobs/" + jobID + "/resume":
			writeEnvelope(t, w, `{"job_id":"job-123","status":"resumed"}`)
		case "GET /api/v1/admin/tools/ldxp/jobs/" + jobID + "/export":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="codes.csv"`)
			_, _ = w.Write([]byte("code-a\ncode-b\n"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := newSub2APIClient(&Config{Sub2API: Sub2APIConfig{BaseURL: server.URL, AdminToken: "admin-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.status(context.Background(), jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.resume(context.Background(), jobID); err != nil {
		t.Fatal(err)
	}
	envelope, _, attachment, err := client.export(context.Background(), jobID)
	if err != nil {
		t.Fatal(err)
	}
	if envelope != nil {
		t.Fatalf("attachment export unexpectedly returned an envelope: %#v", envelope)
	}
	if string(attachment) != "code-a\ncode-b\n" {
		t.Fatalf("unexpected export data: %q", attachment)
	}
	want := []string{
		"GET /api/v1/admin/tools/ldxp/jobs/" + jobID,
		"POST /api/v1/admin/tools/ldxp/jobs/" + jobID + "/resume",
		"GET /api/v1/admin/tools/ldxp/jobs/" + jobID + "/export",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected request sequence: %#v", seen)
	}
}

func TestDecodeAPIEnvelopeRejectsNonSuccessCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelopeWithCode(t, w, 0, `{}`)
	}))
	defer server.Close()
	client, err := newAPIClient("test", server.URL, "secret", "Authorization", 1<<20, false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.callJSON(context.Background(), http.MethodGet, "/health", nil)
	var responseError *apiResponseError
	if !errors.As(err, &responseError) || responseError.Code != 0 {
		t.Fatalf("expected non-success code rejection, got %v", err)
	}
}
