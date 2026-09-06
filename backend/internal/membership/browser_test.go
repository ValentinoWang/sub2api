package membership

import (
	"errors"
	"strings"
	"testing"
)

func TestNewHTTPBrowserRestrictsSidecarToLoopback(t *testing.T) {
	token := strings.Repeat("x", 32)
	tests := []struct {
		name     string
		endpoint string
		wantURL  string
		valid    bool
	}{
		{name: "ipv4 loopback", endpoint: "http://127.0.0.1:8080", wantURL: "http://127.0.0.1:8080", valid: true},
		{name: "localhost", endpoint: "http://localhost:8080", wantURL: "http://localhost:8080", valid: true},
		{name: "ipv6 loopback", endpoint: "http://[::1]:8080", wantURL: "http://[::1]:8080", valid: true},
		{name: "root slash", endpoint: "http://localhost:8080/", wantURL: "http://localhost:8080", valid: true},
		{name: "remote https", endpoint: "https://example.com:443"},
		{name: "remote http", endpoint: "http://example.com:8080"},
		{name: "loopback https", endpoint: "https://127.0.0.1:8080"},
		{name: "missing port", endpoint: "http://127.0.0.1"},
		{name: "zero port", endpoint: "http://localhost:0"},
		{name: "out of range port", endpoint: "http://localhost:65536"},
		{name: "nonnumeric port", endpoint: "http://localhost:sidecar"},
		{name: "nonroot path", endpoint: "http://localhost:8080/api"},
		{name: "query", endpoint: "http://localhost:8080?target=remote"},
		{name: "empty query", endpoint: "http://localhost:8080?"},
		{name: "fragment", endpoint: "http://localhost:8080#remote"},
		{name: "empty fragment", endpoint: "http://localhost:8080#"},
		{name: "userinfo", endpoint: "http://user:pass@localhost:8080"},
		{name: "userinfo host confusion", endpoint: "http://127.0.0.1:8080@evil.example"},
		{name: "suffix host confusion", endpoint: "http://localhost.evil.example:8080"},
		{name: "ipv4 shorthand", endpoint: "http://127.1:8080"},
		{name: "ipv6 zone", endpoint: "http://[::1%25lo0]:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser, err := NewHTTPBrowser(tt.endpoint, token)
			if tt.valid {
				if err != nil {
					t.Fatalf("NewHTTPBrowser(%q) error = %v", tt.endpoint, err)
				}
				if browser.URL != tt.wantURL {
					t.Fatalf("browser URL = %q, want %q", browser.URL, tt.wantURL)
				}
				return
			}
			if browser != nil || !errors.Is(err, ErrInvalid) {
				t.Fatalf("NewHTTPBrowser(%q) = (%#v, %v), want (nil, ErrInvalid)", tt.endpoint, browser, err)
			}
		})
	}
}
