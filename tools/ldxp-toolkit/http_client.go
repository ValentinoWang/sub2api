package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ldxpResponseLimit    = 8 << 20
	sub2APIResponseLimit = 64 << 20
)

type apiEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type apiClient struct {
	provider       string
	origin         *url.URL
	token          string
	authHeader     string
	responseLimit  int64
	compatProtocol bool
	httpClient     *http.Client
}

type compatibilityError struct {
	Provider string
	Method   string
	Path     string
	Status   int
	Cause    error
}

func (e *compatibilityError) Error() string {
	endpoint := fmt.Sprintf("%s %s", e.Method, e.Path)
	if e.Status != 0 {
		return fmt.Sprintf("protocol compatibility error: %s endpoint %s is unavailable (HTTP %d); deploy a compatible LDXP jobs API", e.Provider, endpoint, e.Status)
	}
	if e.Cause != nil {
		return fmt.Sprintf("protocol compatibility error: %s endpoint %s is unavailable: %s", e.Provider, endpoint, e.Cause)
	}
	return fmt.Sprintf("protocol compatibility error: %s endpoint %s is unavailable", e.Provider, endpoint)
}

func (e *compatibilityError) Unwrap() error { return e.Cause }

type httpStatusError struct {
	Provider string
	Method   string
	Path     string
	Status   int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s endpoint %s %s returned HTTP %d", e.Provider, e.Method, e.Path, e.Status)
}

type apiResponseError struct {
	Provider string
	Method   string
	Path     string
	Code     int
	Message  string
}

func (e *apiResponseError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "request rejected"
	}
	return fmt.Sprintf("%s endpoint %s %s returned code %d: %s", e.Provider, e.Method, e.Path, e.Code, message)
}

func newAPIClient(provider, rawBaseURL, token, authHeader string, responseLimit int64, compatibility bool) (*apiClient, error) {
	origin, err := parseOrigin(rawBaseURL, provider)
	if err != nil {
		return nil, err
	}
	client := &apiClient{
		provider:       provider,
		origin:         origin,
		token:          strings.TrimSpace(token),
		authHeader:     authHeader,
		responseLimit:  responseLimit,
		compatProtocol: compatibility,
	}
	client.httpClient = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !sameOrigin(req.URL, origin) {
				return fmt.Errorf("refused cross-origin redirect for %s", provider)
			}
			return nil
		},
	}
	return client, nil
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func (c *apiClient) endpointURL(path string) (*url.URL, error) {
	if path == "" || !strings.HasPrefix(path, "/") {
		return nil, errors.New("API path must be absolute")
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("API path must be a path without query or fragment")
	}
	u := *c.origin
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return &u, nil
}

func (c *apiClient) callRaw(ctx context.Context, method, path string, payload any) ([]byte, *http.Response, error) {
	endpoint, err := c.endpointURL(path)
	if err != nil {
		return nil, nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("encode %s request: %w", c.provider, err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s request: %w", c.provider, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" && c.authHeader != "" {
		value := c.token
		if strings.EqualFold(c.authHeader, "Authorization") {
			value = "Bearer " + c.token
		}
		req.Header.Set(c.authHeader, value)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.compatProtocol {
			return nil, nil, &compatibilityError{Provider: c.provider, Method: method, Path: path, Cause: redactError(err, c.token)}
		}
		return nil, nil, fmt.Errorf("request %s %s: %w", c.provider, path, redactError(err, c.token))
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, c.responseLimit+1))
	if err != nil {
		return nil, resp, fmt.Errorf("read %s response: %w", c.provider, err)
	}
	if int64(len(responseBody)) > c.responseLimit {
		return nil, resp, fmt.Errorf("%s response exceeds the %d-byte limit", c.provider, c.responseLimit)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if c.compatProtocol && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented) {
			return nil, resp, &compatibilityError{Provider: c.provider, Method: method, Path: path, Status: resp.StatusCode}
		}
		return nil, resp, &httpStatusError{Provider: c.provider, Method: method, Path: path, Status: resp.StatusCode}
	}
	return responseBody, resp, nil
}

func (c *apiClient) callJSON(ctx context.Context, method, path string, payload any) (*apiEnvelope, *http.Response, error) {
	responseBody, resp, err := c.callRaw(ctx, method, path, payload)
	if err != nil {
		return nil, resp, err
	}
	envelope, err := decodeAPIEnvelope(responseBody)
	if err != nil {
		if c.compatProtocol {
			return nil, resp, fmt.Errorf("protocol compatibility error: %s endpoint %s %s returned an incompatible response: %w", c.provider, method, path, redactError(err, c.token))
		}
		return nil, resp, fmt.Errorf("%s endpoint %s %s returned an invalid response: %w", c.provider, method, path, redactError(err, c.token))
	}
	if envelope.Code != 1 {
		return nil, resp, &apiResponseError{
			Provider: c.provider,
			Method:   method,
			Path:     path,
			Code:     envelope.Code,
			Message:  redactText(envelope.Msg, c.token),
		}
	}
	return envelope, resp, nil
}

// callExport accepts the attachment response emitted by the server export
// handler. A JSON envelope remains supported for protocol errors and older
// compatible deployments, but attachment bytes are never parsed or printed.
func (c *apiClient) callExport(ctx context.Context, path string) (*apiEnvelope, *http.Response, []byte, error) {
	responseBody, resp, err := c.callRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, resp, nil, err
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	attachment := strings.TrimSpace(resp.Header.Get("Content-Disposition")) != ""
	if strings.Contains(contentType, "json") && !attachment {
		envelope, err := decodeAPIEnvelope(responseBody)
		if err != nil {
			return nil, resp, nil, fmt.Errorf("protocol compatibility error: %s export response is neither a valid JSON envelope nor an attachment: %w", c.provider, redactError(err, c.token))
		}
		if envelope.Code != 1 {
			return nil, resp, nil, &apiResponseError{Provider: c.provider, Method: http.MethodGet, Path: path, Code: envelope.Code, Message: redactText(envelope.Msg, c.token)}
		}
		return envelope, resp, nil, nil
	}
	return nil, resp, responseBody, nil
}

func decodeAPIEnvelope(body []byte) (*apiEnvelope, error) {
	var raw struct {
		Code *int            `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.Code == nil || raw.Data == nil {
		return nil, errors.New("expected JSON envelope {code:1,data:...}")
	}
	return &apiEnvelope{Code: *raw.Code, Msg: raw.Msg, Data: raw.Data}, nil
}

func redactError(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	return errors.New(redactText(err.Error(), secrets...))
}

func redactText(value string, secrets ...string) string {
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
