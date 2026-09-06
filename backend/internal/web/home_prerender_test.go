//go:build embed

package web

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

type mutableHomeSettings struct {
	value map[string]any
}

func (s *mutableHomeSettings) GetPublicSettingsForInjection(context.Context) (any, error) {
	return s.value, nil
}

func requireHomePrerenderArtifact(t *testing.T, server *FrontendServer) string {
	t.Helper()
	const homePage = "home/index.html"
	if !server.fileExists(homePage) {
		t.Fatalf("frontend build must emit %s; do not silently fall back to the legacy index seed", homePage)
	}

	file, err := server.distFS.Open(homePage)
	if err != nil {
		t.Fatalf("open %s: %v", homePage, err)
	}
	defer func() { _ = file.Close() }()

	contents, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read %s: %v", homePage, err)
	}
	return string(contents)
}

func formalHomeMain(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "<main")
	if start == -1 {
		t.Fatal("home response has no formal prerendered main element")
	}
	end := strings.Index(body[start:], "</main>")
	if end == -1 {
		t.Fatal("home response has an unterminated formal prerendered main element")
	}
	main := body[start : start+end+len("</main>")]
	if !strings.Contains(main, `data-home-prerender=`) {
		t.Fatal("home response main element is not a prerendered landing page")
	}
	return main
}

func TestHomePrerenderArtifactContainsTheFormalLandingPage(t *testing.T) {
	server, err := NewFrontendServer(prerenderStubSettings{})
	if err != nil {
		t.Fatalf("embedded frontend must contain a complete home prerender artifact: %v", err)
	}
	body := requireHomePrerenderArtifact(t, server)

	for _, marker := range []string{
		`data-home-prerender="default"`,
		`data-home-hero`,
		`data-home-primary-cta`,
		`href="/login"`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("home prerender must contain %q, got no formal first-paint landing page", marker)
		}
	}
	if strings.Contains(body, `max-width: 720px; margin: 48px auto`) {
		t.Error("home prerender must not retain the old narrow plain-text SEO seed")
	}
	if strings.Contains(body, `Static seed for crawlers`) {
		t.Error("home prerender must not describe itself as a crawler-only seed")
	}
}

func TestHomePrerenderArtifactsKeepFirstPaintStylesAndLocaleMetadata(t *testing.T) {
	server, err := NewFrontendServer(prerenderStubSettings{})
	if err != nil {
		t.Fatalf("embedded frontend must contain complete home prerender artifacts: %v", err)
	}

	for _, test := range []struct {
		path        string
		languageTag string
		description string
	}{
		{path: homeDefaultChineseTemplate, languageTag: `lang="zh-CN"`, description: "一个入口接入多个 AI 模型"},
		{path: homeDefaultEnglishTemplate, languageTag: `lang="en"`, description: "One entry point for multiple AI models"},
	} {
		t.Run(test.path, func(t *testing.T) {
			content, err := fs.ReadFile(server.distFS, test.path)
			if err != nil {
				t.Fatalf("read %s: %v", test.path, err)
			}
			body := string(content)
			headEnd := strings.Index(body, "</head>")
			styleLink := strings.Index(body, `href="/home/static.css"`)
			if headEnd == -1 || styleLink == -1 || styleLink > headEnd {
				t.Error("the static home stylesheet must load from head before the body can paint")
			}
			if !strings.Contains(body, test.languageTag) || !strings.Contains(body, test.description) {
				t.Errorf("%s must preserve its language and readable metadata without JavaScript", test.path)
			}
		})
	}
}

func TestHomeRouteServesThePrerenderedPageWithPublicConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := &mutableHomeSettings{value: map[string]any{
		"site_name":     "Gateway One",
		"site_subtitle": "A readable public subtitle",
		"site_logo":     "/uploads/gateway-one.svg",
	}}
	server, err := NewFrontendServer(settings)
	if err != nil {
		t.Fatalf("embedded frontend must contain a complete home prerender artifact: %v", err)
	}
	requireHomePrerenderArtifact(t, server)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.CSPNonceKey, "home-prerender-test-nonce")
	})
	router.Use(server.Middleware())

	fetch := func(acceptLanguage string) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/home", nil)
		if acceptLanguage != "" {
			req.Header.Set("Accept-Language", acceptLanguage)
		}
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /home: expected 200, got %d", rec.Code)
		}
		return rec.Body.String()
	}

	body := fetch("en-US,en;q=0.9")
	for _, marker := range []string{
		`data-home-prerender="default"`,
		`data-home-hero`,
		`data-home-primary-cta`,
		`href="/login"`,
		`window.__APP_CONFIG__=`,
		`nonce="home-prerender-test-nonce"`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("GET /home must retain %q before application JavaScript runs", marker)
		}
	}
	if strings.Contains(body, NonceHTMLPlaceholder) {
		t.Error("GET /home must replace the CSP nonce placeholder")
	}
	if !strings.Contains(body, `data-home-locale="en"`) {
		t.Error("GET /home must use the English default template for an English Accept-Language request")
	}
	if strings.Contains(body, `max-width: 720px; margin: 48px auto`) {
		t.Error("GET /home must not fall back to the old plain-text SEO seed")
	}
	if strings.Contains(body, "__SUB2API_") {
		t.Error("GET /home must substitute all public configuration placeholders")
	}
	if !strings.Contains(body, "Gateway One") || !strings.Contains(body, "A readable public subtitle") {
		t.Error("GET /home must use the public settings snapshot that is injected for Vue")
	}
	main := formalHomeMain(t, body)
	if !strings.Contains(main, "Gateway One") || !strings.Contains(main, "A readable public subtitle") {
		t.Error("GET /home must render the same public branding inside the no-JavaScript landing page")
	}

	settings.value = map[string]any{
		"site_name":            "Gateway Two",
		"site_subtitle":        "Replacement public subtitle",
		"compact_home_enabled": true,
	}
	server.InvalidateCache()
	body = fetch("zh-CN,zh;q=0.9,en;q=0.5")
	if !strings.Contains(body, `data-home-prerender="compact"`) || !strings.Contains(body, `data-home-locale="zh"`) {
		t.Error("GET /home must select the compact Chinese template from public settings and Accept-Language")
	}
	if !strings.Contains(body, "Gateway Two") || !strings.Contains(body, "Replacement public subtitle") {
		t.Error("GET /home must refresh its public configuration after cache invalidation")
	}
	if strings.Contains(body, "Gateway One") {
		t.Error("GET /home must not serve stale public configuration after cache invalidation")
	}
	main = formalHomeMain(t, body)
	if !strings.Contains(main, "Gateway Two") || !strings.Contains(main, "Replacement public subtitle") {
		t.Error("GET /home must refresh the no-JavaScript landing page after cache invalidation")
	}

	settings.value = map[string]any{
		"compact_home_enabled": true,
		"home_content":         `<section id="custom-home">Administrator home</section>`,
	}
	server.InvalidateCache()
	body = fetch("en")
	if !strings.Contains(body, `id="custom-home"`) || !strings.Contains(body, "Administrator home") {
		t.Error("administrator custom HTML must win over the compact prerendered home")
	}
	if strings.Contains(body, `data-home-prerender=`) {
		t.Error("custom HTML must not be wrapped in the default or compact prerendered home")
	}

	settings.value = map[string]any{
		"home_content": " https://example.com/custom-home ",
	}
	server.InvalidateCache()
	body = fetch("en")
	if !strings.Contains(body, `<iframe src="https://example.com/custom-home"`) {
		t.Error("an HTTPS administrator home URL must preserve the existing iframe home behavior")
	}
}
