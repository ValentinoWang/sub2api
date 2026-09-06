package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestListGoodsFullyPagesDeduplicatesCardsAndFixesProxyFlag(t *testing.T) {
	var mu sync.Mutex
	var requests []struct {
		method string
		path   string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, struct {
			method string
			path   string
			body   map[string]any
		}{method: r.Method, path: r.URL.Path, body: body})
		mu.Unlock()
		if r.URL.Path != goodsListPath {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		current := int(body["current"].(float64))
		switch current {
		case 1:
			writeEnvelope(t, w, `{"records":[{"id":10,"name":"A","type":"card","current_stock":7},{"id":11,"name":"Subscription","type":"subscription","current_stock":99}],"total":3,"pageSize":2}`)
		case 2:
			writeEnvelope(t, w, `{"records":[{"id":10,"name":"A duplicate","type":"card","current_stock":8},{"id":12,"name":"B","type":1,"current_stock":3}],"total":3,"pageSize":2}`)
		default:
			t.Fatalf("unexpected page %d", current)
		}
	}))
	defer server.Close()

	client, err := newLDXPClient(&Config{LDXP: LDXPConfig{BaseURL: server.URL, MerchantToken: "merchant-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	goods, err := client.listGoods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(goods) != 2 || goods[0].ID != 10 || goods[0].Name != "A" || goods[1].ID != 12 || goods[1].Type != "1" || goods[1].CurrentStock != 3 {
		t.Fatalf("unexpected goods: %#v", goods)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("expected two pages, got %d", len(requests))
	}
	for _, request := range requests {
		if request.method != http.MethodPost || request.path != goodsListPath {
			t.Fatalf("unexpected request: %#v", request)
		}
		if proxy, ok := request.body["is_proxy"].(float64); !ok || proxy != 0 {
			t.Fatalf("is_proxy was not fixed to numeric zero: %#v", request.body["is_proxy"])
		}
	}
}

func TestRestockPreviewComputesTargetGaps(t *testing.T) {
	cases := []struct {
		name  string
		stock int
		want  int
	}{
		{name: "empty", stock: 0, want: 50000},
		{name: "partial", stock: 12000, want: 38000},
		{name: "at target", stock: 50000, want: 0},
		{name: "above target", stock: 60000, want: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Fatalf("LDXP request must be POST, got %s", r.Method)
				}
				switch r.URL.Path {
				case goodsListPath:
					writeEnvelope(t, w, `{"records":[{"id":99,"name":"Mapped","type":"card","current_stock":1},{"id":100,"name":"Unconfigured","type":"card","current_stock":2},{"id":101,"name":"Ignore","type":"subscription","current_stock":2}],"total":3,"pageSize":1000}`)
				case unsoldInventoryPath:
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatalf("decode inventory request: %v", err)
					}
					if body["status"] != "0" {
						t.Fatalf("inventory status must be string zero, got %#v", body["status"])
					}
					writeEnvelope(t, w, fmt.Sprintf(`{"total":%d}`, testCase.stock))
				default:
					t.Fatalf("unexpected LDXP path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			cfg := &Config{
				LDXP:          LDXPConfig{BaseURL: server.URL, MerchantToken: "merchant-secret"},
				TargetStock:   50000,
				UploadSegment: 1000,
				ProductMappings: []ProductMapping{{
					GoodsID: 99, CNYAmount: 20, USDCredit: 2.78, Enabled: true,
				}},
			}
			client, err := newLDXPClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := buildRestockPreview(context.Background(), cfg, client)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Items) != 1 || preview.Items[0].Needed != testCase.want {
				t.Fatalf("unexpected preview for stock %d: %#v", testCase.stock, preview.Items)
			}
			if len(preview.UnconfiguredRemoteCardGoods) != 1 || preview.UnconfiguredRemoteCardGoods[0].ID != 100 {
				t.Fatalf("only the unconfigured card good should be reported: %#v", preview.UnconfiguredRemoteCardGoods)
			}
		})
	}
}

func TestRestockPreviewDoesNotUseWriteEndpoints(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case goodsListPath:
			writeEnvelope(t, w, `{"records":[{"id":1,"name":"Card","type":"card","current_stock":0}],"total":1,"pageSize":1000}`)
		case unsoldInventoryPath:
			writeEnvelope(t, w, `{"total":0}`)
		case "/merchantApi/GoodsCardStorage/add", jobsRunPath:
			t.Fatalf("preview called a write endpoint: %s", r.URL.Path)
		default:
			t.Fatalf("preview called unexpected endpoint: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	cfg := &Config{
		LDXP:          LDXPConfig{BaseURL: server.URL, MerchantToken: "merchant-secret"},
		TargetStock:   50000,
		UploadSegment: 1000,
		ProductMappings: []ProductMapping{{
			GoodsID: 1, CNYAmount: 1, USDCredit: 1, Enabled: true,
		}},
	}
	client, err := newLDXPClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildRestockPreview(context.Background(), cfg, client); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 2 || paths[0] != "POST "+goodsListPath || paths[1] != "POST "+unsoldInventoryPath {
		t.Fatalf("unexpected preview request sequence: %#v", paths)
	}
}

func TestMalformedLDXPEnvelopeRejectsGoodsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1}`))
	}))
	defer server.Close()
	client, err := newLDXPClient(&Config{LDXP: LDXPConfig{BaseURL: server.URL, MerchantToken: "merchant-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.listGoods(context.Background())
	if err == nil || !strings.Contains(err.Error(), "expected JSON envelope") {
		t.Fatalf("expected malformed envelope rejection, got %v", err)
	}
}

func TestSub2APIUnavailableReturnsCompatibilityError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client, err := newSub2APIClient(&Config{Sub2API: Sub2APIConfig{BaseURL: server.URL, AdminToken: "admin-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.run(context.Background(), jobRequest{SelectedGoods: []int64{1}})
	var compatibility *compatibilityError
	if !errors.As(err, &compatibility) || !strings.Contains(err.Error(), "protocol compatibility error") {
		t.Fatalf("expected compatibility error, got %v", err)
	}
}

func TestPrivateJobSummaryRedactsSecretsAndUses0600(t *testing.T) {
	dir := t.TempDir()
	envelope := &apiEnvelope{Code: 1, Msg: "merchant-secret accepted", Data: json.RawMessage(`{"job_id":"job-1","admin_token":"admin-secret","codes":["LD-secret"]}`)}
	data, err := marshalSummary("restock run", jobsRunPath, "job-1", envelope, "merchant-secret", "admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "merchant-secret") || strings.Contains(string(data), "admin-secret") || strings.Contains(string(data), "LD-secret") {
		t.Fatalf("summary leaked a secret: %s", data)
	}
	path, err := writePrivateFile(dir, "job-summary", ".json", data)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private summary mode is %04o, want 0600", info.Mode().Perm())
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("summary escaped data directory: %s", path)
	}
}

func TestRestockRunUsesBearerTokenAndWritesRedactedSummary(t *testing.T) {
	dataDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != jobsRunPath {
			t.Fatalf("unexpected run request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer admin-secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode run request: %v", err)
		}
		encoded, _ := json.Marshal(body)
		if strings.Contains(string(encoded), "merchant-secret") || strings.Contains(string(encoded), "admin-secret") {
			t.Fatalf("run request leaked a credential: %s", encoded)
		}
		selected, ok := body["selected_goods"].([]any)
		if !ok || len(selected) != 1 {
			t.Fatalf("run request did not select the enabled goods: %#v", body)
		}
		if selectedID, ok := selected[0].(float64); !ok || selectedID != 1 {
			t.Fatalf("run request selected the wrong goods: %#v", body)
		}
		writeEnvelope(t, w, `{"job_id":"job-123","status":"queued"}`)
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	config := fmt.Sprintf(`{"ldxp":{"base_url":"%s","merchant_token":"merchant-secret"},"sub2api":{"base_url":"%s","admin_token":"admin-secret"},"data_dir":"%s","product_mappings":[{"goods_id":1,"cny_amount":20,"usd_credit":2.78,"enabled":true}]}`, server.URL, server.URL, dataDir)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if exitCode := runCLI([]string{"--config", configPath, "restock", "run"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("restock run failed with %d: stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "admin-secret") || strings.Contains(stdout.String(), "merchant-secret") {
		t.Fatalf("CLI output leaked a credential: %s", stdout.String())
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &output); err != nil {
		t.Fatalf("decode CLI output: %v", err)
	}
	summaryPath, ok := output["summary_path"].(string)
	if !ok || filepath.Dir(summaryPath) != dataDir {
		t.Fatalf("unexpected summary path: %#v", output["summary_path"])
	}
	info, err := os.Stat(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("summary mode is %04o, want 0600", info.Mode().Perm())
	}
}

func TestExportCommandWritesAttachmentAndSummaryWithPrivateModes(t *testing.T) {
	dataDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/admin/tools/ldxp/jobs/job-1/export" {
			t.Fatalf("unexpected export request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer admin-secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="job-1.txt"`)
		_, _ = w.Write([]byte("server-owned-export\n"))
	}))
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "config.json")
	config := fmt.Sprintf(`{"ldxp":{"base_url":"%s","merchant_token":"merchant-secret"},"sub2api":{"base_url":"%s","admin_token":"admin-secret"},"data_dir":"%s","product_mappings":[{"goods_id":1,"cny_amount":20,"usd_credit":2.78,"enabled":true}]}`, server.URL, server.URL, dataDir)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if exitCode := runCLI([]string{"export", "--id", "job-1", "--config", configPath}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("export failed with %d: stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected export and summary files, got %d", len(entries))
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s has mode %04o, want 0600", entry.Name(), info.Mode().Perm())
		}
	}
	if !strings.Contains(stdout.String(), "export_path") || strings.Contains(stdout.String(), "server-owned-export") {
		t.Fatalf("export output was not a redacted path-only result: %s", stdout.String())
	}
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, data string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"code":1,"data":%s}`, data)
}

func writeEnvelopeWithCode(t *testing.T, w http.ResponseWriter, code int, data string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"code":%d,"data":%s}`, code, data)
}
