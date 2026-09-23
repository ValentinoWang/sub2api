package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type codexHarvestHandlerProxyRepo struct {
	service.ProxyRepository
	proxies map[int64]service.Proxy
}

func (r *codexHarvestHandlerProxyRepo) ListByIDs(_ context.Context, ids []int64) ([]service.Proxy, error) {
	out := []service.Proxy{}
	for _, id := range ids {
		if p, ok := r.proxies[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// codexHarvestHandlerSettings records writes; the shared stub panics on Set.
type codexHarvestHandlerSettings struct {
	*settingHandlerRepoStub
}

func (s *codexHarvestHandlerSettings) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func newCodexHarvestTestHandler(t *testing.T) (*CodexHarvestHandler, *codexHarvestHandlerSettings) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	settings := &codexHarvestHandlerSettings{settingHandlerRepoStub: &settingHandlerRepoStub{values: map[string]string{}}}
	proxies := &codexHarvestHandlerProxyRepo{proxies: map[int64]service.Proxy{
		3: {ID: 3, Name: "residential-3", Protocol: "http", Host: "res.example", Port: 8080, Username: "u", Password: "pool-secret", Status: service.StatusActive},
	}}
	harvest := service.NewCodexHarvestService(nil, nil, settings, proxies)
	gateway := &service.OpenAIGatewayService{}
	gateway.SetCodexHarvestService(harvest)
	return NewCodexHarvestHandler(gateway, harvest), settings
}

func serveCodexHarvest(h *CodexHarvestHandler, method, path string, body any) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/codex-harvest", h.Get)
	router.PUT("/codex-harvest/controls", h.UpdateControls)
	router.GET("/codex-harvest/nodes", h.ListNodes)
	router.POST("/codex-harvest/nodes/reset", h.ResetNodes)
	router.POST("/codex-harvest/accounts/:id/manual", h.StartManual)
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCodexHarvestHandler_SnapshotAndControls(t *testing.T) {
	h, settings := newCodexHarvestTestHandler(t)
	rec := serveCodexHarvest(h, http.MethodGet, "/codex-harvest", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"preset":"standard"`)
	require.Contains(t, rec.Body.String(), `"proxy_ids":[]`)

	controls := service.DefaultCodexHarvestControls()
	controls.ProxyIDs = []int64{3}
	rec = serveCodexHarvest(h, http.MethodPut, "/codex-harvest/controls", controls)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, settings.values["openai_codex_harvest_controls_v1"], `"proxy_ids":[3]`)

	rec = serveCodexHarvest(h, http.MethodGet, "/codex-harvest", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"residential-3"`)
	require.NotContains(t, rec.Body.String(), "pool-secret")

	for name, body := range map[string]any{
		"unknown proxy": map[string]any{"version": 1, "preset": "standard", "speed": controls.Speed, "proxy_ids": []int64{99}},
		"bad preset":    map[string]any{"version": 1, "preset": "warp", "speed": controls.Speed, "proxy_ids": []int64{}},
		"malformed":     "not-json-object",
	} {
		rec = serveCodexHarvest(h, http.MethodPut, "/codex-harvest/controls", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	require.Contains(t, settings.values["openai_codex_harvest_controls_v1"], `"proxy_ids":[3]`, "rejected saves keep the previous pool")
}

func TestCodexHarvestHandler_NodesAndManualValidation(t *testing.T) {
	h, _ := newCodexHarvestTestHandler(t)
	rec := serveCodexHarvest(h, http.MethodGet, "/codex-harvest/nodes?page=1&page_size=20", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"items":[]`)

	rec = serveCodexHarvest(h, http.MethodPost, "/codex-harvest/nodes/reset", map[string]any{"record_id": -1})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = serveCodexHarvest(h, http.MethodPost, "/codex-harvest/accounts/abc/manual", map[string]any{})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Harvesting is off in this handler (no settings service, default config), so a manual run is refused.
	rec = serveCodexHarvest(h, http.MethodPost, "/codex-harvest/accounts/7/manual", map[string]any{})
	require.NotEqual(t, http.StatusAccepted, rec.Code, rec.Body.String())
}
