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

type prerenderStubSettings struct{}

func (prerenderStubSettings) GetPublicSettingsForInjection(context.Context) (any, error) {
	return map[string]any{"site_name": "rest2build"}, nil
}

func newPrerenderTestServer(t *testing.T) *FrontendServer {
	t.Helper()
	s, err := NewFrontendServer(prerenderStubSettings{})
	if err != nil {
		t.Skipf("embedded frontend unavailable: %v", err)
	}
	return s
}

func TestPrerenderedPathOnlyMatchesExtensionlessRoutes(t *testing.T) {
	s := newPrerenderTestServer(t)
	if !s.fileExists("codex-cli/index.html") {
		t.Skip("dist has no prerendered pages; run the frontend build first")
	}

	if got := s.prerenderedPath("codex-cli"); got != "codex-cli/index.html" {
		t.Errorf("codex-cli should map to its prerendered page, got %q", got)
	}
	for _, path := range []string{"", "index.html", "assets/app.js", "logo.svg", "dashboard", "../etc/passwd"} {
		if got := s.prerenderedPath(path); got != "" {
			t.Errorf("path %q must not resolve to a prerendered page, got %q", path, got)
		}
	}
}

func TestMiddlewareServesPrerenderedPageWithInjectedSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newPrerenderTestServer(t)
	if !s.fileExists("codex-cli/index.html") {
		t.Skip("dist has no prerendered pages; run the frontend build first")
	}

	router := gin.New()
	router.Use(s.Middleware())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/codex-cli", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "window.__APP_CONFIG__=") {
		t.Error("prerendered page must carry the injected public settings")
	}
	if !strings.Contains(body, "canonical") || !strings.Contains(body, "/codex-cli") {
		t.Error("prerendered page must keep its own canonical URL")
	}
	if strings.Contains(body, NonceHTMLPlaceholder) {
		t.Error("nonce placeholder must be replaced before serving")
	}
}

func TestInvalidateCacheClearsPrerenderedPages(t *testing.T) {
	s := newPrerenderTestServer(t)
	s.prerendered = map[string]*prerenderedPage{"codex-cli/index.html": {base: []byte("x")}}
	s.lastSettingsJS = []byte(`{"site_name":"x"}`)

	s.InvalidateCache()

	if s.prerendered != nil || s.lastSettingsJS != nil {
		t.Error("InvalidateCache must drop prerendered pages and the settings snapshot")
	}
}
