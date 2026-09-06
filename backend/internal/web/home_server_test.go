//go:build embed

package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderHomeTemplateEscapesPublicValuesAndKeepsNonceDynamic(t *testing.T) {
	template := []byte(`<!doctype html><html><head><title>__SUB2API_SITE_NAME__</title><script nonce="__SUB2API_CSP_NONCE__">window.__HOME_BOOTSTRAP__=true;</script></head><body><div id="app"><main data-home-prerender="default"><img src="__SUB2API_SITE_LOGO_URL__" alt="__SUB2API_SITE_NAME__"><p>__SUB2API_SITE_SUBTITLE__</p><code>__SUB2API_API_BASE_URL__</code><a href="__SUB2API_DOC_URL__">Docs</a></main></div></body></html>`)
	settingsJSON, err := json.Marshal(map[string]any{
		"site_name":     `Gateway </title><script>alert(1)</script>`,
		"site_logo":     `https://cdn.example.test/logo.svg?x=1&y=2`,
		"site_subtitle": `<b>public subtitle</b>`,
		"api_base_url":  `https://api.example.test/v1`,
		"doc_url":       `https://docs.example.test/guide`,
	})
	if err != nil {
		t.Fatalf("marshal public settings: %v", err)
	}

	rendered, err := renderHomeTemplate(template, settingsJSON, homePublicSettings{
		SiteName:     `Gateway </title><script>alert(1)</script>`,
		SiteLogo:     `https://cdn.example.test/logo.svg?x=1&y=2`,
		SiteSubtitle: `<b>public subtitle</b>`,
		APIBaseURL:   `https://api.example.test/v1`,
		DocURL:       `https://docs.example.test/guide`,
	})
	if err != nil {
		t.Fatalf("render home template: %v", err)
	}
	body := string(rendered)

	for _, placeholder := range append(homeDynamicPlaceholders(), HomeNoncePlaceholder) {
		if strings.Contains(body, placeholder) {
			t.Errorf("response leaked unresolved placeholder %q", placeholder)
		}
	}
	if !strings.Contains(body, `nonce="`+NonceHTMLPlaceholder+`"`) {
		t.Error("cached home HTML must retain a request-scoped nonce placeholder")
	}
	if strings.Contains(body, `Gateway </title><script>alert(1)</script>`) || strings.Contains(body, `<b>public subtitle</b>`) {
		t.Errorf("text substitutions must be HTML escaped, got %s", body)
	}
	if strings.Contains(body, `window.__APP_CONFIG__={"site_name":"Gateway </title>`) || strings.Contains(body, `</script></script>`) {
		t.Errorf("public JSON must not be able to terminate the injected script, got %s", body)
	}
	if !strings.Contains(body, `window.__APP_CONFIG__=`) || !strings.Contains(body, `Gateway \u003c/title\u003e`) {
		t.Errorf("the injected app configuration must use the exact, HTML-safe snapshot, got %s", body)
	}
}

func TestPreferredHomeLanguageHonorsQualityAndDefaultsToEnglish(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{header: "zh-CN,zh;q=0.9,en;q=0.8", want: "zh"},
		{header: "en-US,en;q=0.9,zh;q=0.8", want: "en"},
		{header: "zh-CN;q=0.4,en-US;q=0.8", want: "en"},
		{header: "fr-CA, de;q=0.8", want: "en"},
		{header: "", want: "en"},
	}
	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			if got := preferredHomeLanguage(test.header); got != test.want {
				t.Errorf("preferredHomeLanguage(%q) = %q, want %q", test.header, got, test.want)
			}
		})
	}
}

func TestSelectHomeTemplateCoversDefaultAndCompactModes(t *testing.T) {
	tests := []struct {
		compact bool
		locale  string
		want    string
	}{
		{compact: false, locale: "zh", want: homeDefaultChineseTemplate},
		{compact: false, locale: "en", want: homeDefaultEnglishTemplate},
		{compact: true, locale: "zh", want: homeCompactChineseTemplate},
		{compact: true, locale: "en", want: homeCompactEnglishTemplate},
	}
	for _, test := range tests {
		if got := selectHomeTemplate(test.compact, test.locale); got != test.want {
			t.Errorf("selectHomeTemplate(%t, %q) = %q, want %q", test.compact, test.locale, got, test.want)
		}
	}
}
