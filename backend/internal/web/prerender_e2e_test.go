//go:build embed

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type configuredSettings struct{ site, store string }

func (c configuredSettings) GetPublicSettingsForInjection(context.Context) (any, error) {
	return map[string]any{"site_name": c.site, "xianyu_store_name": c.store}, nil
}

// End-to-end: a crawler fetching the verification page must read the administrator's configured
// store name, and every page must keep its own title.
func TestE2EPrerenderedPagesReflectAdminConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, err := NewFrontendServer(configuredSettings{site: "我的网关", store: "我的闲鱼小店"})
	if err != nil {
		t.Skipf("embedded frontend unavailable: %v", err)
	}
	if !server.fileExists("verify/xianyu/index.html") {
		t.Skip("dist has no prerendered pages; run the frontend build first")
	}

	router := gin.New()
	router.Use(server.Middleware())

	fetch := func(path string) string {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
		return rec.Body.String()
	}

	verify := fetch("/verify/xianyu")
	if !strings.Contains(verify, "我的闲鱼小店") {
		t.Error("verification page must show the configured store name")
	}
	if strings.Contains(verify, "Rest2Build AI 接入实验室") {
		t.Error("verification page must not fall back to the built-in store name when one is configured")
	}
	if strings.Contains(verify, "__SUB2API_") {
		t.Error("no placeholder may reach a visitor")
	}

	codex := fetch("/codex-cli")
	if !strings.Contains(codex, "Codex CLI") {
		t.Error("each page must keep its own title")
	}
	if !strings.Contains(codex, "我的网关") {
		t.Error("each page must pick up the configured site name")
	}
	if strings.Contains(codex, "<title>我的网关 - AI API Gateway</title>") {
		t.Error("the SPA shell title must not overwrite a prerendered page title")
	}
}
