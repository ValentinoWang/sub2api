package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const codexHarvestProbeBodyLimit = 1 << 20

type codexHarvestProbeResult struct {
	State      string
	Status     int
	RetryAfter time.Duration
	Err        error
	Kind       string
	Sent       bool
}

// requestCodexHarvestProbe sends one synthetic Responses request through the
// chosen harvest proxy. reserve is called immediately before sending so budget
// is only charged for requests that actually leave the process.
func (s *OpenAIGatewayService) requestCodexHarvestProbe(ctx context.Context, account *Account, token, model, proxyURL string, reserve func() bool) (out codexHarvestProbeResult) {
	body := []byte(`{"model":` + jsonString(model) + `,"store":false,"stream":true,"instructions":"Reply with exactly: pong","input":[{"role":"user","content":[{"type":"input_text","text":"ping"}]}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatgptCodexURL, bytes.NewReader(body))
	if err != nil {
		out.Err = err
		return
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAIHarvest))
	req.Close = true
	req.Host = "chatgpt.com"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	// Every probe starts a new upstream conversation; reusing a session makes a
	// refreshed ticket look like a continuation of the previous one.
	req.Header.Set("session_id", uuid.NewString())
	if err := resolveAndSetOpenAIChatGPTAccountHeaders(ctx, s.accountRepo, req.Header, account); err != nil {
		out.Err = err
		return
	}
	applyOpenAICodexTicketHarvestIdentity(req.Header, model)
	if ctx.Err() != nil {
		out.Err = ctx.Err()
		return
	}
	if reserve != nil && !reserve() {
		out.Err = errors.New("harvest request no longer admitted")
		return
	}
	out.Sent = true
	// Synthetic probes use the dedicated no-reuse transport even when the
	// account is bound to a plugin.
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		out.Err = err
		return
	}
	if resp == nil {
		out.Err = errors.New("nil upstream response")
		return
	}
	out.Status = resp.StatusCode
	out.State = extractOpenAICodexTurnState(resp.Header)
	out.RetryAfter = codexHarvestRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	if resp.Body == nil {
		out.Err = errors.New("probe response body missing")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	response, err := io.ReadAll(io.LimitReader(resp.Body, codexHarvestProbeBodyLimit+1))
	if err != nil || len(response) > codexHarvestProbeBodyLimit {
		out.Err = errors.New("probe response incomplete")
		return
	}
	if out.Status == http.StatusOK {
		out.Err = validateCodexProbeResponse(response)
	}
	return
}

// classifyCodexHarvestProbe maps a probe to a learning outcome. Only a
// completed 200 response with a correctly shaped state is a success.
func classifyCodexHarvestProbe(ctx context.Context, targetLength int, result codexHarvestProbeResult) string {
	switch {
	case ctx.Err() != nil && !result.Sent:
		return "cancelled"
	case !result.Sent:
		return "not_sent"
	case result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden:
		return "account_error"
	case result.Status == http.StatusTooManyRequests:
		return "rate_limited"
	case result.Status == 0:
		if ctx.Err() != nil {
			return "cancelled"
		}
		return "network_error"
	case result.Status != http.StatusOK:
		return "upstream_error"
	case result.Err != nil:
		return "response_incomplete_or_error"
	case len(result.State) != targetLength || !strings.HasPrefix(result.State, openAICodexTicketStatePrefix):
		return "invalid_state"
	}
	return "success"
}

func codexHarvestRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
		return time.Duration(min(seconds, 86400)) * time.Second
	}
	if date, err := http.ParseTime(raw); err == nil && date.After(now) {
		return min(date.Sub(now), 24*time.Hour)
	}
	return 0
}

// validateCodexProbeResponse requires an explicit response.completed event.
// A clean EOF or [DONE] is not evidence of a successful generation.
func validateCodexProbeResponse(body []byte) error {
	completed := false
	inspect := func(data []byte) error {
		if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			return nil
		}
		var event struct {
			Type     string          `json:"type"`
			Status   string          `json:"status"`
			Error    json.RawMessage `json:"error"`
			Response *struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if json.Unmarshal(data, &event) != nil {
			return errors.New("invalid probe event")
		}
		if event.Type == "error" || event.Type == "response.failed" || event.Type == "response.incomplete" ||
			event.Status == "failed" || event.Status == "incomplete" || (len(event.Error) > 0 && string(event.Error) != "null") {
			return errors.New("probe response failed")
		}
		if event.Response != nil && (event.Response.Status == "failed" || event.Response.Status == "incomplete") {
			return errors.New("probe response failed")
		}
		if event.Type == "response.completed" {
			if event.Response == nil || event.Response.Status != "completed" {
				return errors.New("invalid probe completion")
			}
			completed = true
		}
		if event.Type == "" && event.Status == "completed" {
			completed = true
		}
		return nil
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if err := inspect(trimmed); err != nil {
			return err
		}
	} else {
		scanner := bufio.NewScanner(bytes.NewReader(body))
		scanner.Buffer(make([]byte, 4096), codexHarvestProbeBodyLimit)
		var data []string
		flush := func() error {
			if len(data) == 0 {
				return nil
			}
			err := inspect([]byte(strings.Join(data, "\n")))
			data = nil
			return err
		}
		for scanner.Scan() {
			line := strings.TrimSuffix(scanner.Text(), "\r")
			if line == "" {
				if err := flush(); err != nil {
					return err
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if scanner.Err() != nil {
			return errors.New("invalid probe stream")
		}
		// A trailing unterminated event may be truncated, even if its JSON parses.
		if len(data) > 0 {
			return errors.New("unterminated probe event")
		}
	}
	if !completed {
		return errors.New("probe did not complete successfully")
	}
	return nil
}
