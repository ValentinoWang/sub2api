package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const (
	jobsPreviewPath = "/api/v1/admin/tools/ldxp/jobs/preview"
	jobsRunPath     = "/api/v1/admin/tools/ldxp/jobs/run"
)

type sub2APIClient struct {
	api *apiClient
}

type jobRequest struct {
	SelectedGoods []int64 `json:"selected_goods"`
}

func newSub2APIClient(cfg *Config) (*sub2APIClient, error) {
	api, err := newAPIClient("Sub2API", cfg.Sub2API.BaseURL, cfg.Sub2API.AdminToken, "Authorization", sub2APIResponseLimit, true)
	if err != nil {
		return nil, err
	}
	return &sub2APIClient{api: api}, nil
}

func (c *sub2APIClient) preview(ctx context.Context, request jobRequest) (*apiEnvelope, error) {
	envelope, _, err := c.api.callJSON(ctx, http.MethodPost, jobsPreviewPath, request)
	return envelope, err
}

func (c *sub2APIClient) run(ctx context.Context, request jobRequest) (*apiEnvelope, error) {
	envelope, _, err := c.api.callJSON(ctx, http.MethodPost, jobsRunPath, request)
	return envelope, err
}

func (c *sub2APIClient) status(ctx context.Context, jobID string) (*apiEnvelope, error) {
	path, err := jobPath(jobID, "")
	if err != nil {
		return nil, err
	}
	envelope, _, err := c.api.callJSON(ctx, http.MethodGet, path, nil)
	return envelope, err
}

func (c *sub2APIClient) resume(ctx context.Context, jobID string) (*apiEnvelope, error) {
	path, err := jobPath(jobID, "/resume")
	if err != nil {
		return nil, err
	}
	envelope, _, err := c.api.callJSON(ctx, http.MethodPost, path, map[string]any{})
	return envelope, err
}

func (c *sub2APIClient) export(ctx context.Context, jobID string) (*apiEnvelope, *http.Response, []byte, error) {
	path, err := jobPath(jobID, "/export")
	if err != nil {
		return nil, nil, nil, err
	}
	return c.api.callExport(ctx, path)
}

func jobPath(jobID, suffix string) (string, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return "", errors.New("job id is required")
	}
	for _, char := range jobID {
		if char == '/' || char == '\\' || char == '?' || char == '#' || char == '%' || char <= ' ' {
			return "", errors.New("job id contains an unsafe path character")
		}
	}
	return "/api/v1/admin/tools/ldxp/jobs/" + jobID + suffix, nil
}

func requestForConfig(cfg *Config, plan []RestockPlanItem) jobRequest {
	selected := make([]int64, 0, len(cfg.ProductMappings))
	if plan != nil {
		selected = make([]int64, 0, len(plan))
		for _, item := range plan {
			selected = append(selected, item.GoodsID)
		}
	} else {
		for _, mapping := range cfg.ProductMappings {
			if mapping.Enabled {
				selected = append(selected, mapping.GoodsID)
			}
		}
	}
	return jobRequest{SelectedGoods: selected}
}

func jobIDFromData(raw json.RawMessage) string {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return ""
	}
	return findStringField(value, "job_id", "jobId", "id")
}

func findStringField(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if candidate, ok := typed[key]; ok {
				if value, err := stringifyValue(candidate); err == nil {
					return strings.TrimSpace(value)
				}
			}
		}
		for _, nested := range typed {
			if found := findStringField(nested, keys...); found != "" {
				return found
			}
		}
	case []any:
		for _, nested := range typed {
			if found := findStringField(nested, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}

func redactedJSON(raw json.RawMessage, secrets ...string) any {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return redactText(string(raw), secrets...)
	}
	return redactValue(value, secrets...)
}

func redactValue(value any, secrets ...string) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if sensitiveKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactValue(nested, secrets...)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, nested := range typed {
			out[index] = redactValue(nested, secrets...)
		}
		return out
	case string:
		return redactText(typed, secrets...)
	default:
		return value
	}
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "password") {
		return true
	}
	if normalized == "authorization" || normalized == "api_key" || normalized == "apikey" || normalized == "code" || normalized == "codes" || normalized == "code_list" || normalized == "redeem_code" || normalized == "redemption_code" || normalized == "content" {
		return true
	}
	return strings.Contains(normalized, "redemption") || strings.Contains(normalized, "redeem")
}

func envelopeForOutput(envelope *apiEnvelope, secrets ...string) map[string]any {
	result := map[string]any{}
	if envelope == nil {
		return result
	}
	result["code"] = envelope.Code
	if strings.TrimSpace(envelope.Msg) != "" {
		result["message"] = redactText(envelope.Msg, secrets...)
	}
	result["data"] = redactedJSON(envelope.Data, secrets...)
	return result
}

func marshalSummary(command, endpoint, jobID string, envelope *apiEnvelope, secrets ...string) ([]byte, error) {
	value := map[string]any{
		"command":     command,
		"endpoint":    redactText(endpoint, secrets...),
		"recorded_at": nowUTC(),
		"response":    envelopeForOutput(envelope, secrets...),
	}
	if jobID != "" {
		value["job_id"] = redactText(jobID, secrets...)
	}
	return json.MarshalIndent(value, "", "  ")
}

func validateExportData(raw json.RawMessage) bool {
	return len(raw) > 0 && strings.TrimSpace(string(raw)) != "null"
}

func exportDataBytes(raw json.RawMessage) ([]byte, string, error) {
	if !validateExportData(raw) {
		return nil, "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []byte(text), ".txt", nil
	}
	return append([]byte(nil), raw...), ".json", nil
}

func nowUTC() string {
	return timeNowUTC().Format("2006-01-02T15:04:05Z07:00")
}
