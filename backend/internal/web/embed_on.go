//go:build embed

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

const (
	// NonceHTMLPlaceholder is the placeholder for nonce in HTML script tags
	NonceHTMLPlaceholder = "__CSP_NONCE_VALUE__"

	// HomeNoncePlaceholder is emitted by the build-time home documents. It remains a
	// placeholder in cached HTML and is replaced with the request-scoped CSP nonce
	// immediately before the response is written.
	HomeNoncePlaceholder = "__SUB2API_CSP_NONCE__"
)

//go:embed all:dist
var frontendFS embed.FS

// PublicSettingsProvider is an interface to fetch public settings
type PublicSettingsProvider interface {
	GetPublicSettingsForInjection(ctx context.Context) (any, error)
}

// FrontendServer serves the embedded frontend with settings injection
type FrontendServer struct {
	distFS      fs.FS
	fileServer  http.Handler
	baseHTML    []byte
	cache       *HTMLCache
	settings    PublicSettingsProvider
	overrideDir string // local file override directory

	// Prerendered public pages (dist/<route>/index.html, produced at build time). Served with the
	// same settings injection as index.html and cached per settings snapshot.
	prerenderMu    sync.Mutex
	prerendered    map[string]*prerenderedPage
	lastSettingsJS []byte
	homeTemplates  map[string][]byte
	homeRendered   map[string]*homeRenderedPage
}

// prerenderedPage caches one build-time page plus its rendered form for the current settings.
type prerenderedPage struct {
	base        []byte
	settingsKey string
	rendered    []byte
}

// homeRenderedPage holds a settings-specific, nonce-free response. The nonce is
// deliberately substituted only at write time so caching never leaks it across requests.
type homeRenderedPage struct {
	etag    string
	content []byte
}

const (
	homeDefaultChineseTemplate = "home/default.zh.html"
	homeDefaultEnglishTemplate = "home/default.en.html"
	homeCompactChineseTemplate = "home/compact.zh.html"
	homeCompactEnglishTemplate = "home/compact.en.html"
	homePreviewTemplate        = "home/index.html"
	homeStaticCSS              = "home/static.css"
	homeTemplateSentinel       = "SUB2API_HOME_TEMPLATE"
)

var homeTemplatePaths = []string{
	homeDefaultChineseTemplate,
	homeDefaultEnglishTemplate,
	homeCompactChineseTemplate,
	homeCompactEnglishTemplate,
}

// NewFrontendServer creates a new frontend server with settings injection
func NewFrontendServer(settingsProvider PublicSettingsProvider) (*FrontendServer, error) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil, err
	}

	// Read base HTML once
	file, err := distFS.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	baseHTML, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	cache := NewHTMLCache()
	cache.SetBaseHTML(baseHTML)
	homeTemplates, err := loadHomeTemplates(distFS)
	if err != nil {
		return nil, err
	}

	return &FrontendServer{
		distFS:        distFS,
		fileServer:    http.FileServer(http.FS(distFS)),
		baseHTML:      baseHTML,
		cache:         cache,
		settings:      settingsProvider,
		overrideDir:   filepath.Join("data", "public"),
		homeTemplates: homeTemplates,
	}, nil
}

// InvalidateCache invalidates the HTML cache (call when settings change)
func (s *FrontendServer) InvalidateCache() {
	if s == nil {
		return
	}
	if s.cache != nil {
		s.cache.Invalidate()
	}
	s.prerenderMu.Lock()
	s.prerendered = nil
	s.lastSettingsJS = nil
	s.homeRendered = nil
	s.prerenderMu.Unlock()
}

// Middleware returns the Gin middleware handler
func (s *FrontendServer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// /home is the only SPA route whose first response is a real, styled home
		// document. Keep the literal file URL on the same path so it can never
		// bypass dynamic settings, language selection, or CSP nonce replacement.
		if cleanPath == "home" || cleanPath == "home/index.html" {
			s.serveHome(c)
			return
		}

		// Prerendered public pages win over the directory redirect the file server would
		// otherwise issue for an extension-less route such as /codex-cli.
		if page := s.prerenderedPath(cleanPath); page != "" {
			s.servePrerendered(c, page)
			return
		}

		// For index.html or SPA routes, serve with injected settings
		if cleanPath == "index.html" || !s.fileExists(cleanPath) {
			s.serveIndexHTML(c)
			return
		}

		// Try local override first
		if s.tryServeOverride(c, cleanPath) {
			return
		}

		// Serve static files normally (hashed assets get long-lived cache headers)
		applyStaticAssetCacheHeaders(c.Writer.Header(), cleanPath)
		s.fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func (s *FrontendServer) fileExists(path string) bool {
	file, err := s.distFS.Open(path)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

// tryServeOverride checks if a local override file exists and serves it.
// Files in overrideDir take precedence over embedded files.
func (s *FrontendServer) tryServeOverride(c *gin.Context, cleanPath string) bool {
	if s.overrideDir == "" {
		return false
	}
	filePath := filepath.Join(s.overrideDir, filepath.Clean("/"+cleanPath))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}
	c.File(filePath)
	c.Abort()
	return true
}

func (s *FrontendServer) serveIndexHTML(c *gin.Context) {
	// Get nonce from context (generated by SecurityHeaders middleware)
	nonce := middleware.GetNonceFromContext(c)

	// Check cache first
	cached := s.cache.Get()
	if cached != nil {
		// Check If-None-Match for 304 response
		if match := c.GetHeader("If-None-Match"); match == cached.ETag {
			c.Status(http.StatusNotModified)
			c.Abort()
			return
		}

		// Replace nonce placeholder with actual nonce before serving
		content := replaceNoncePlaceholder(cached.Content, nonce)

		c.Header("ETag", cached.ETag)
		c.Header("Cache-Control", "no-cache") // Must revalidate
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
		c.Abort()
		return
	}

	// Cache miss - fetch settings and render
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err != nil {
		// Fallback: serve without injection
		c.Data(http.StatusOK, "text/html; charset=utf-8", s.baseHTML)
		c.Abort()
		return
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		// Fallback: serve without injection
		c.Data(http.StatusOK, "text/html; charset=utf-8", s.baseHTML)
		c.Abort()
		return
	}

	rendered := s.injectSettings(settingsJSON)
	s.cache.Set(rendered, settingsJSON)
	s.rememberSettingsJSON(settingsJSON)

	// Replace nonce placeholder with actual nonce before serving
	content := replaceNoncePlaceholder(rendered, nonce)

	cached = s.cache.Get()
	if cached != nil {
		c.Header("ETag", cached.ETag)
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func (s *FrontendServer) injectSettings(settingsJSON []byte) []byte {
	return injectSettingsInto(s.baseHTML, settingsJSON, true)
}

// loadHomeTemplates is intentionally strict. The visible first paint is a release
// artifact, not an optional crawler seed: starting a binary with a partial frontend
// build must fail rather than quietly falling back to index.html.
func loadHomeTemplates(fsys fs.FS) (map[string][]byte, error) {
	cssInfo, err := fs.Stat(fsys, homeStaticCSS)
	if err != nil {
		return nil, fmt.Errorf("home prerender CSS %q is required: %w", homeStaticCSS, err)
	}
	if cssInfo.IsDir() || cssInfo.Size() == 0 {
		return nil, fmt.Errorf("home prerender CSS %q must be a non-empty file", homeStaticCSS)
	}

	templates := make(map[string][]byte, len(homeTemplatePaths))
	for _, templatePath := range homeTemplatePaths {
		content, err := fs.ReadFile(fsys, templatePath)
		if err != nil {
			return nil, fmt.Errorf("home prerender template %q is required: %w", templatePath, err)
		}
		if err := validateHomeTemplate(templatePath, content); err != nil {
			return nil, err
		}
		templates[templatePath] = content
	}

	// This is deliberately a separately-openable preview artifact. It is not used
	// to select a mode because it cannot represent all settings combinations.
	preview, err := fs.ReadFile(fsys, homePreviewTemplate)
	if err != nil {
		return nil, fmt.Errorf("home preview %q is required: %w", homePreviewTemplate, err)
	}
	if err := validateHomeTemplate(homePreviewTemplate, preview); err != nil {
		return nil, err
	}
	return templates, nil
}

func validateHomeTemplate(templatePath string, content []byte) error {
	if !bytes.Contains(content, []byte(homeTemplateSentinel)) {
		return fmt.Errorf("home prerender template %q is missing %q", templatePath, homeTemplateSentinel)
	}
	for _, placeholder := range homeDynamicPlaceholders() {
		if !bytes.Contains(content, []byte(placeholder)) {
			return fmt.Errorf("home prerender template %q is missing placeholder %q", templatePath, placeholder)
		}
	}
	if !bytes.Contains(content, []byte(HomeNoncePlaceholder)) {
		return fmt.Errorf("home prerender template %q is missing CSP nonce placeholder %q", templatePath, HomeNoncePlaceholder)
	}
	return nil
}

func homeDynamicPlaceholders() []string {
	return []string{
		prerenderSiteNamePlaceholder,
		"__SUB2API_SITE_LOGO_URL__",
		"__SUB2API_SITE_SUBTITLE__",
		"__SUB2API_API_BASE_URL__",
		"__SUB2API_DOC_URL__",
	}
}

type homePublicSettings struct {
	SiteName           string `json:"site_name"`
	SiteLogo           string `json:"site_logo"`
	SiteSubtitle       string `json:"site_subtitle"`
	APIBaseURL         string `json:"api_base_url"`
	DocURL             string `json:"doc_url"`
	HomeContent        string `json:"home_content"`
	CompactHomeEnabled bool   `json:"compact_home_enabled"`
}

func parseHomePublicSettings(settingsJSON []byte) homePublicSettings {
	var settings homePublicSettings
	_ = json.Unmarshal(settingsJSON, &settings)
	return settings
}

func (s *FrontendServer) serveHome(c *gin.Context) {
	settingsJSON := s.homeSettingsJSON(c)
	settings := parseHomePublicSettings(settingsJSON)
	if strings.TrimSpace(settings.HomeContent) != "" {
		// Existing custom-home content is administrator-controlled. It replaces only
		// #app through the parsed DOM, retaining the app shell, CSP, and the same
		// public settings snapshot Vue receives during takeover.
		s.serveCustomHome(c, settingsJSON, settings)
		return
	}

	templatePath := selectHomeTemplate(settings.CompactHomeEnabled, preferredHomeLanguage(c.GetHeader("Accept-Language")))
	cacheKey := templatePath + "\x00" + string(settingsJSON)

	s.prerenderMu.Lock()
	if s.homeRendered == nil {
		s.homeRendered = make(map[string]*homeRenderedPage)
	}
	entry := s.homeRendered[cacheKey]
	if entry == nil {
		base := s.homeTemplates[templatePath]
		rendered, err := renderHomeTemplate(base, settingsJSON, settings)
		if err != nil {
			s.prerenderMu.Unlock()
			c.String(http.StatusInternalServerError, "Home template render failed")
			c.Abort()
			return
		}
		entry = &homeRenderedPage{content: rendered, etag: homeETag(templatePath, settingsJSON)}
		s.homeRendered[cacheKey] = entry
	}
	content := append([]byte(nil), entry.content...)
	etag := entry.etag
	s.prerenderMu.Unlock()

	s.writeHomeResponse(c, content, etag)
}

func (s *FrontendServer) homeSettingsJSON(c *gin.Context) []byte {
	s.prerenderMu.Lock()
	cached := append([]byte(nil), s.lastSettingsJS...)
	s.prerenderMu.Unlock()
	if len(cached) > 0 {
		return cached
	}
	if s.settings == nil {
		return []byte("{}")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err != nil {
		return []byte("{}")
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil || !json.Valid(settingsJSON) {
		return []byte("{}")
	}
	s.rememberSettingsJSON(settingsJSON)
	return settingsJSON
}

func (s *FrontendServer) writeHomeResponse(c *gin.Context, content []byte, etag string) {
	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.Status(http.StatusNotModified)
		c.Abort()
		return
	}
	nonce := middleware.GetNonceFromContext(c)
	content = replaceNoncePlaceholder(content, nonce)
	c.Header("ETag", etag)
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func selectHomeTemplate(compact bool, language string) string {
	if compact {
		if language == "zh" {
			return homeCompactChineseTemplate
		}
		return homeCompactEnglishTemplate
	}
	if language == "zh" {
		return homeDefaultChineseTemplate
	}
	return homeDefaultEnglishTemplate
}

type acceptLanguage struct {
	value string
	q     float64
	order int
}

func preferredHomeLanguage(header string) string {
	parts := strings.Split(header, ",")
	accepted := make([]acceptLanguage, 0, len(parts))
	for order, part := range parts {
		sections := strings.Split(part, ";")
		language := strings.ToLower(strings.TrimSpace(sections[0]))
		if language == "" {
			continue
		}
		q := 1.0
		for _, parameter := range sections[1:] {
			parameter = strings.TrimSpace(parameter)
			if strings.HasPrefix(parameter, "q=") {
				if parsed, err := strconv.ParseFloat(strings.TrimPrefix(parameter, "q="), 64); err == nil {
					q = parsed
				}
			}
		}
		if q > 0 {
			accepted = append(accepted, acceptLanguage{value: language, q: q, order: order})
		}
	}
	sort.SliceStable(accepted, func(i, j int) bool {
		if accepted[i].q == accepted[j].q {
			return accepted[i].order < accepted[j].order
		}
		return accepted[i].q > accepted[j].q
	})
	for _, candidate := range accepted {
		if candidate.value == "zh" || strings.HasPrefix(candidate.value, "zh-") {
			return "zh"
		}
		if candidate.value == "en" || strings.HasPrefix(candidate.value, "en-") {
			return "en"
		}
	}
	return "en"
}

func homeETag(templatePath string, settingsJSON []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(templatePath))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(settingsJSON)
	sum := digest.Sum(nil)
	return `"home-` + hex.EncodeToString(sum[:8]) + `"`
}

func renderHomeTemplate(base, settingsJSON []byte, settings homePublicSettings) ([]byte, error) {
	return renderHomeDocument(base, settingsJSON, settings, true, nil)
}

// renderHomeDocument uses an HTML parser instead of raw replacement. Dynamic strings
// are assigned to text/attribute nodes, and html.Render supplies the appropriate
// escaping for that context. The only raw script payload is json.Marshal output,
// which escapes '<', '>', and '&' so public values cannot terminate the script tag.
func renderHomeDocument(base, settingsJSON []byte, settings homePublicSettings, requirePlaceholders bool, replaceApp func(*html.Node) error) ([]byte, error) {
	document, err := html.Parse(bytes.NewReader(base))
	if err != nil {
		return nil, fmt.Errorf("parse home document: %w", err)
	}

	values := homePlaceholderValues(settings)
	seen := make(map[string]bool, len(values))
	var head, app *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "head" {
				head = node
			}
			if node.Data == "div" && nodeAttribute(node, "id") == "app" {
				app = node
			}
			for index := range node.Attr {
				node.Attr[index].Val = substituteHomeText(node.Attr[index].Val, values, seen)
			}
		}
		if node.Type == html.TextNode {
			node.Data = substituteHomeText(node.Data, values, seen)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	if requirePlaceholders {
		for _, placeholder := range homeDynamicPlaceholders() {
			if !seen[placeholder] {
				return nil, fmt.Errorf("home template did not expose placeholder %q", placeholder)
			}
		}
	}
	if head == nil || app == nil {
		return nil, fmt.Errorf("home document must include <head> and <div id=\"app\">")
	}
	if replaceApp != nil {
		if err := replaceApp(app); err != nil {
			return nil, err
		}
	}
	injectPublicConfigScript(head, settingsJSON)

	var rendered bytes.Buffer
	if err := html.Render(&rendered, document); err != nil {
		return nil, fmt.Errorf("render home document: %w", err)
	}
	content := rendered.Bytes()
	if requirePlaceholders {
		for _, placeholder := range homeDynamicPlaceholders() {
			if bytes.Contains(content, []byte(placeholder)) {
				return nil, fmt.Errorf("home template left placeholder %q after render", placeholder)
			}
		}
	}
	return content, nil
}

func homePlaceholderValues(settings homePublicSettings) map[string]string {
	siteName := strings.TrimSpace(settings.SiteName)
	if siteName == "" || siteName == upstreamDefaultSiteName {
		siteName = prerenderDefaultSiteName
	}
	logoURL := safeImageURL(settings.SiteLogo)
	if logoURL == "" {
		logoURL = "/logo.svg"
	}
	subtitle := strings.TrimSpace(settings.SiteSubtitle)
	if subtitle == "" {
		subtitle = "AI API Gateway Platform"
	}
	apiBaseURL := safeNavigationURL(settings.APIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = "https://ai.rest2build.lol"
	}
	docURL := safeNavigationURL(settings.DocURL)
	if docURL == "" {
		docURL = "/"
	}
	return map[string]string{
		prerenderSiteNamePlaceholder: siteName,
		"__SUB2API_SITE_LOGO_URL__":  logoURL,
		"__SUB2API_SITE_SUBTITLE__":  subtitle,
		"__SUB2API_API_BASE_URL__":   apiBaseURL,
		"__SUB2API_DOC_URL__":        docURL,
		HomeNoncePlaceholder:         NonceHTMLPlaceholder,
	}
}

func substituteHomeText(value string, replacements map[string]string, seen map[string]bool) string {
	for placeholder, replacement := range replacements {
		if strings.Contains(value, placeholder) {
			seen[placeholder] = true
			value = strings.ReplaceAll(value, placeholder, replacement)
		}
	}
	return value
}

func nodeAttribute(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func injectPublicConfigScript(head *html.Node, settingsJSON []byte) {
	// Templates must not ship a competing configuration object. Remove a stale
	// generated one defensively before appending the canonical snapshot.
	for child := head.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && child.Data == "script" && strings.Contains(childText(child), "window.__APP_CONFIG__") {
			head.RemoveChild(child)
		}
		child = next
	}
	script := &html.Node{Type: html.ElementNode, Data: "script", Attr: []html.Attribute{{Key: "nonce", Val: NonceHTMLPlaceholder}}}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: "window.__APP_CONFIG__=" + string(settingsJSON) + ";"})
	head.AppendChild(script)
}

func childText(node *html.Node) string {
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	}
	return text.String()
}

func safeNavigationURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return trimmed
}

func (s *FrontendServer) serveCustomHome(c *gin.Context, settingsJSON []byte, settings homePublicSettings) {
	cacheKey := "custom\x00" + string(settingsJSON)
	s.prerenderMu.Lock()
	if s.homeRendered == nil {
		s.homeRendered = make(map[string]*homeRenderedPage)
	}
	entry := s.homeRendered[cacheKey]
	if entry == nil {
		rendered, err := renderHomeDocument(s.baseHTML, settingsJSON, settings, false, func(app *html.Node) error {
			return replaceCustomHomeApp(app, settings.HomeContent)
		})
		if err != nil {
			s.prerenderMu.Unlock()
			c.String(http.StatusInternalServerError, "Custom home render failed")
			c.Abort()
			return
		}
		entry = &homeRenderedPage{content: rendered, etag: homeETag("custom", settingsJSON)}
		s.homeRendered[cacheKey] = entry
	}
	content := append([]byte(nil), entry.content...)
	etag := entry.etag
	s.prerenderMu.Unlock()
	s.writeHomeResponse(c, content, etag)
}

func replaceCustomHomeApp(app *html.Node, homeContent string) error {
	for child := app.FirstChild; child != nil; {
		next := child.NextSibling
		app.RemoveChild(child)
		child = next
	}
	content := strings.TrimSpace(homeContent)
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		wrapper := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "class", Val: "min-h-screen"}}}
		iframe := &html.Node{Type: html.ElementNode, Data: "iframe", Attr: []html.Attribute{
			{Key: "src", Val: content},
			{Key: "class", Val: "h-screen w-full border-0"},
			{Key: "allowfullscreen", Val: ""},
		}}
		wrapper.AppendChild(iframe)
		app.AppendChild(wrapper)
		return nil
	}

	fragment, err := html.ParseFragment(strings.NewReader(homeContent), app)
	if err != nil {
		return fmt.Errorf("parse administrator home HTML: %w", err)
	}
	for _, node := range fragment {
		app.AppendChild(node)
	}
	return nil
}

// injectSettingsInto applies the public-settings script and branding to any index-style page,
// so prerendered public pages get the same treatment as index.html.
//
// brandTitle rewrites the document title from the configured site name. That is right for the SPA
// shell, whose title is a generic placeholder, but wrong for a prerendered page: those carry a
// page-specific title that is the whole point of prerendering, and they pick up the site name
// through substitutePrerenderPlaceholders instead.
func injectSettingsInto(base, settingsJSON []byte, brandTitle bool) []byte {
	// Create the script tag to inject with nonce placeholder
	// The placeholder will be replaced with actual nonce at request time
	script := []byte(`<script nonce="` + NonceHTMLPlaceholder + `">window.__APP_CONFIG__=` + string(settingsJSON) + `;</script>`)

	// Inject before </head>
	headClose := []byte("</head>")
	result := bytes.Replace(base, headClose, append(script, headClose...), 1)

	// Apply custom branding before the browser paints the static defaults.
	if brandTitle {
		result = injectSiteTitle(result, settingsJSON)
	}
	result = injectSiteFavicon(result, settingsJSON)

	return result
}

// injectSiteFavicon replaces the static favicon with a configured, browser-safe image URL.
func injectSiteFavicon(html, settingsJSON []byte) []byte {
	var cfg struct {
		SiteLogo string `json:"site_logo"`
	}
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil {
		return html
	}

	logoURL := safeImageURL(cfg.SiteLogo)
	if logoURL == "" {
		return html
	}

	linkStart := bytes.Index(html, []byte(`<link rel="icon"`))
	if linkStart == -1 {
		return html
	}
	linkEndOffset := bytes.IndexByte(html[linkStart:], '>')
	if linkEndOffset == -1 {
		return html
	}
	linkEnd := linkStart + linkEndOffset + 1
	replacement := []byte(`<link rel="icon" href="` + htmlpkg.EscapeString(logoURL) + `" />`)

	var buf bytes.Buffer
	buf.Write(html[:linkStart])
	buf.Write(replacement)
	buf.Write(html[linkEnd:])
	return buf.Bytes()
}

func safeImageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		return trimmed
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "data:image/") {
		return trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return trimmed
}

// injectSiteTitle replaces the static <title> in HTML with the configured site name.
// This ensures the browser tab shows the correct title before JS executes.
func injectSiteTitle(html, settingsJSON []byte) []byte {
	var cfg struct {
		SiteName string `json:"site_name"`
	}
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil || cfg.SiteName == "" {
		return html
	}

	// Find and replace the existing <title>...</title>
	titleStart := bytes.Index(html, []byte("<title>"))
	titleEnd := bytes.Index(html, []byte("</title>"))
	if titleStart == -1 || titleEnd == -1 || titleEnd <= titleStart {
		return html
	}

	newTitle := []byte("<title>" + htmlpkg.EscapeString(cfg.SiteName) + " - AI API Gateway</title>")
	var buf bytes.Buffer
	buf.Write(html[:titleStart])
	buf.Write(newTitle)
	buf.Write(html[titleEnd+len("</title>"):])
	return buf.Bytes()
}

// replaceNoncePlaceholder replaces the nonce placeholder with actual nonce value
func replaceNoncePlaceholder(html []byte, nonce string) []byte {
	result := bytes.ReplaceAll(html, []byte(NonceHTMLPlaceholder), []byte(nonce))
	// Home documents carry their bootstrap script before the server has injected
	// __APP_CONFIG__. Vite also uses index.html as the base for other prerendered
	// public pages, so both build-time nonce aliases must be resolved here.
	return bytes.ReplaceAll(result, []byte(HomeNoncePlaceholder), []byte(nonce))
}

// ServeEmbeddedFrontend returns a middleware for serving embedded frontend
// This is the legacy function for backward compatibility when no settings provider is available
func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to get dist subdirectory: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(distFS))
	overrideDir := filepath.Join("data", "public")

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if file, err := distFS.Open(cleanPath); err == nil {
			_ = file.Close()
			// Try local override first
			if tryServeOverrideFile(c, overrideDir, cleanPath) {
				return
			}
			applyStaticAssetCacheHeaders(c.Writer.Header(), cleanPath)
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		serveIndexHTML(c, distFS)
	}
}

// tryServeOverrideFile is a standalone version of tryServeOverride for legacy usage.
func tryServeOverrideFile(c *gin.Context, overrideDir, cleanPath string) bool {
	if overrideDir == "" {
		return false
	}
	filePath := filepath.Join(overrideDir, filepath.Clean("/"+cleanPath))
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}
	c.File(filePath)
	c.Abort()
	return true
}

func shouldBypassEmbeddedFrontend(path string) bool {
	trimmed := strings.TrimSpace(path)
	return strings.HasPrefix(trimmed, "/api/") ||
		strings.HasPrefix(trimmed, "/v1/") ||
		strings.HasPrefix(trimmed, "/v1beta/") ||
		strings.HasPrefix(trimmed, "/backend-api/") ||
		strings.HasPrefix(trimmed, "/antigravity/") ||
		strings.HasPrefix(trimmed, "/setup/") ||
		trimmed == "/health" ||
		trimmed == "/models" ||
		trimmed == "/responses" ||
		strings.HasPrefix(trimmed, "/responses/") ||
		trimmed == "/alpha/search" ||
		strings.HasPrefix(trimmed, "/images/") ||
		strings.HasPrefix(trimmed, "/videos/")
}

func serveIndexHTML(c *gin.Context, fsys fs.FS) {
	file, err := fsys.Open("index.html")
	if err != nil {
		c.String(http.StatusNotFound, "Frontend not found")
		c.Abort()
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read index.html")
		c.Abort()
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func HasEmbeddedFrontend() bool {
	_, err := frontendFS.ReadFile("dist/index.html")
	return err == nil
}

// rememberSettingsJSON stores the latest injected settings so prerendered pages can reuse them
// without an extra settings fetch per request.
func (s *FrontendServer) rememberSettingsJSON(settingsJSON []byte) {
	s.prerenderMu.Lock()
	s.lastSettingsJS = append([]byte(nil), settingsJSON...)
	s.prerenderMu.Unlock()
}

// prerenderedPath maps an extension-less SPA route ("codex-cli") to its build-time prerendered
// document ("codex-cli/index.html") when one exists in the embedded dist.
func (s *FrontendServer) prerenderedPath(cleanPath string) string {
	if cleanPath == "" || cleanPath == "index.html" || strings.Contains(cleanPath, "..") {
		return ""
	}
	// Both /codex-cli and its literal file URL /codex-cli/index.html must go through the same
	// injection path; otherwise the second one is served raw, without public settings or
	// branding, and crawlers see two different documents for one page.
	route := strings.TrimSuffix(strings.Trim(cleanPath, "/"), "/index.html")
	if route == "" || route == "index.html" || path.Ext(route) != "" {
		return ""
	}
	candidate := path.Join(route, "index.html")
	if candidate == "index.html" || !s.fileExists(candidate) {
		return ""
	}
	return candidate
}

// servePrerendered serves a prerendered public page with settings injection, mirroring
// serveIndexHTML but keyed per page. Any failure falls back to the SPA shell.
func (s *FrontendServer) servePrerendered(c *gin.Context, page string) {
	nonce := middleware.GetNonceFromContext(c)
	settingsJSON := s.currentSettingsJSON(c)

	s.prerenderMu.Lock()
	if s.prerendered == nil {
		s.prerendered = make(map[string]*prerenderedPage)
	}
	entry := s.prerendered[page]
	if entry == nil {
		file, err := s.distFS.Open(page)
		if err != nil {
			s.prerenderMu.Unlock()
			s.serveIndexHTML(c)
			return
		}
		base, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			s.prerenderMu.Unlock()
			s.serveIndexHTML(c)
			return
		}
		entry = &prerenderedPage{base: base}
		s.prerendered[page] = entry
	}
	key := string(settingsJSON)
	if entry.rendered == nil || entry.settingsKey != key {
		if len(settingsJSON) > 0 {
			entry.rendered = substitutePrerenderPlaceholders(injectSettingsInto(entry.base, settingsJSON, false), settingsJSON)
		} else {
			entry.rendered = substitutePrerenderPlaceholders(entry.base, nil)
		}
		entry.settingsKey = key
	}
	content := replaceNoncePlaceholder(entry.rendered, nonce)
	s.prerenderMu.Unlock()

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

// currentSettingsJSON reuses the snapshot captured by the last index.html render and only falls
// back to a fresh fetch when nothing has been rendered yet.
func (s *FrontendServer) currentSettingsJSON(c *gin.Context) []byte {
	s.prerenderMu.Lock()
	cached := s.lastSettingsJS
	s.prerenderMu.Unlock()
	if len(cached) > 0 {
		return cached
	}
	if s.settings == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err != nil {
		return nil
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil
	}
	s.rememberSettingsJSON(settingsJSON)
	return settingsJSON
}

// Placeholders the prerender build leaves in the static HTML for values an administrator can
// rename. Keep in sync with PRERENDER_PLACEHOLDERS in frontend/prerender.config.ts.
const (
	prerenderSiteNamePlaceholder  = "__SUB2API_SITE_NAME__"
	prerenderStoreNamePlaceholder = "__SUB2API_XIANYU_STORE_NAME__"

	// Fork defaults, mirroring resolveBrandName / resolveStoreName in frontend/src/constants/brand.ts.
	prerenderDefaultSiteName  = "rest2build"
	prerenderDefaultStoreName = "Rest2Build AI 接入实验室"
	// The upstream project's stock site name is treated as "unset" so a deployment that never
	// customised it still shows the fork brand rather than the upstream one.
	upstreamDefaultSiteName = "Sub2API"
)

// substitutePrerenderPlaceholders fills the admin-configurable values into a prerendered page so
// a crawler that never runs JavaScript reads the same names the Vue app renders.
func substitutePrerenderPlaceholders(html, settingsJSON []byte) []byte {
	var cfg struct {
		SiteName        string `json:"site_name"`
		XianyuStoreName string `json:"xianyu_store_name"`
	}
	if len(settingsJSON) > 0 {
		_ = json.Unmarshal(settingsJSON, &cfg)
	}

	siteName := strings.TrimSpace(cfg.SiteName)
	if siteName == "" || siteName == upstreamDefaultSiteName {
		siteName = prerenderDefaultSiteName
	}
	storeName := strings.TrimSpace(cfg.XianyuStoreName)
	if storeName == "" {
		storeName = prerenderDefaultStoreName
	}

	result := bytes.ReplaceAll(html, []byte(prerenderSiteNamePlaceholder), []byte(htmlpkg.EscapeString(siteName)))
	return bytes.ReplaceAll(result, []byte(prerenderStoreNamePlaceholder), []byte(htmlpkg.EscapeString(storeName)))
}
