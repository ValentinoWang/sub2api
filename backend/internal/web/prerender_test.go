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
	// The literal file URL must resolve to the same injected document, not the raw file.
	if got := s.prerenderedPath("codex-cli/index.html"); got != "codex-cli/index.html" {
		t.Errorf("codex-cli/index.html should map to the prerendered page, got %q", got)
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

func TestSubstitutePrerenderPlaceholdersUsesConfiguredNames(t *testing.T) {
	html := []byte("<title>x｜" + prerenderSiteNamePlaceholder + "</title><p>" + prerenderStoreNamePlaceholder + "</p>")

	t.Run("admin values win", func(t *testing.T) {
		got := string(substitutePrerenderPlaceholders(html, []byte(`{"site_name":"My Gateway","xianyu_store_name":"My Store"}`)))
		if !strings.Contains(got, "My Gateway") || !strings.Contains(got, "My Store") {
			t.Errorf("configured names must appear, got %q", got)
		}
		if strings.Contains(got, prerenderSiteNamePlaceholder) || strings.Contains(got, prerenderStoreNamePlaceholder) {
			t.Errorf("no placeholder may survive, got %q", got)
		}
	})

	t.Run("unset and upstream-default site names fall back to the fork brand", func(t *testing.T) {
		for _, settings := range []string{`{}`, `{"site_name":""}`, `{"site_name":"Sub2API"}`} {
			got := string(substitutePrerenderPlaceholders(html, []byte(settings)))
			if !strings.Contains(got, prerenderDefaultSiteName) {
				t.Errorf("settings %s must fall back to %q, got %q", settings, prerenderDefaultSiteName, got)
			}
			if !strings.Contains(got, prerenderDefaultStoreName) {
				t.Errorf("settings %s must fall back to the default store name, got %q", settings, got)
			}
		}
	})

	t.Run("names are HTML escaped", func(t *testing.T) {
		got := string(substitutePrerenderPlaceholders(html, []byte(`{"xianyu_store_name":"<script>alert(1)</script>"}`)))
		if strings.Contains(got, "<script>") {
			t.Errorf("store name must be escaped, got %q", got)
		}
	})

	t.Run("nil settings still clear the placeholders", func(t *testing.T) {
		got := string(substitutePrerenderPlaceholders(html, nil))
		if strings.Contains(got, "__SUB2API_") {
			t.Errorf("placeholders must never reach a visitor, got %q", got)
		}
	})
}

// A prerendered page's own title is the point of prerendering; the SPA shell's site-name
// branding must not overwrite it.
func TestPrerenderedTitleSurvivesBranding(t *testing.T) {
	page := []byte("<head><title>Codex CLI｜" + prerenderSiteNamePlaceholder + "</title></head><body></body>")
	settings := []byte(`{"site_name":"My Gateway"}`)

	branded := string(substitutePrerenderPlaceholders(injectSettingsInto(page, settings, false), settings))
	if !strings.Contains(branded, "<title>Codex CLI｜My Gateway</title>") {
		t.Errorf("prerendered title must keep its page name and take the configured site name, got %q", branded)
	}

	shell := string(injectSettingsInto(page, settings, true))
	if strings.Contains(shell, "Codex CLI") {
		t.Errorf("the SPA shell path should still rebrand the title wholesale, got %q", shell)
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
