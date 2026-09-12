package membership

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPBrowser struct {
	URL, Token string
	Client     *http.Client
}

func NewHTTPBrowser(endpoint, token string) (*HTTPBrowser, error) {
	if endpoint == "" && token == "" {
		return nil, nil
	}
	normalizedEndpoint, ok := normalizeLoopbackBrowserURL(endpoint)
	if !ok || len(token) < 32 {
		return nil, ErrInvalid
	}
	return &HTTPBrowser{URL: normalizedEndpoint, Token: token, Client: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func normalizeLoopbackBrowserURL(endpoint string) (string, bool) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(endpoint, "#") {
		return "", false
	}
	if u.Path != "" && u.Path != "/" {
		return "", false
	}

	host, portText, err := net.SplitHostPort(u.Host)
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") || !isValidBrowserPort(portText) {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

func isValidBrowserPort(portText string) bool {
	if portText == "" {
		return false
	}
	for _, ch := range portText {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	port, err := strconv.Atoi(portText)
	return err == nil && port >= 1 && port <= 65535
}

func (b *HTTPBrowser) Execute(ctx context.Context, input BrowserInput) (BrowserResult, error) {
	if b == nil {
		return BrowserResult{}, ErrUnavailable
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return BrowserResult{}, ErrInvalid
	}
	req, err := http.NewRequestWithContext(ctx, "POST", b.URL+"/execute", bytes.NewReader(raw))
	if err != nil {
		return BrowserResult{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+b.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := b.Client.Do(req)
	if err != nil {
		return BrowserResult{}, ErrReview
	}
	defer closeResource(res.Body)
	if res.StatusCode != http.StatusOK {
		return BrowserResult{}, ErrReview
	}
	var out BrowserResult
	if json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&out) != nil {
		return BrowserResult{}, ErrReview
	}
	return NormalizeResult(out), nil
}
