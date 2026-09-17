package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const chatGPTResetCreditHistoryURL = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/history"
const OpenAIResetRecentWindow = 10 * time.Minute

// OpenAIResetCreditCheck exposes observations, never upstream credit or user IDs.
// UsedCount is limited to the returned history window, not lifetime consumption.
type OpenAIResetCreditCheck struct {
	Status          string `json:"status"`
	CheckedAt       string `json:"checked_at"`
	AsOf            string `json:"as_of,omitempty"`
	WindowStart     string `json:"window_start,omitempty"`
	LastUsedAt      string `json:"last_used_at,omitempty"`
	UsedCount       int    `json:"used_count"`
	RecentUseCount  int    `json:"recent_use_count"`
	HistoryComplete bool   `json:"history_complete"`
	Reason          string `json:"reason,omitempty"`
	ConfirmationKey string `json:"confirmation_key"`
}

type resetCreditHistoryEvent struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurred_at"`
}

type resetCreditHistoryPage struct {
	AsOf        time.Time                  `json:"as_of"`
	WindowStart time.Time                  `json:"window_start"`
	Events      *[]resetCreditHistoryEvent `json:"events"`
	NextCursor  *string                    `json:"next_cursor"`
}

func finishResetCreditCheck(check *OpenAIResetCreditCheck, eventKeys []string) *OpenAIResetCreditCheck {
	sort.Strings(eventKeys)
	digest := sha256.Sum256([]byte(check.Status + "|" + check.Reason + "|" + strings.Join(eventKeys, "|")))
	check.ConfirmationKey = hex.EncodeToString(digest[:])
	return check
}

// CheckResetCreditHistory only reads upstream history. It never notifies the
// automatic reset worker and never writes a reset-credit snapshot.
func (s *OpenAIQuotaService) CheckResetCreditHistory(ctx context.Context, accountID int64) *OpenAIResetCreditCheck {
	callCtx, cancel := context.WithTimeout(ctx, openaiQuotaUpstreamTimeout)
	defer cancel()
	now := time.Now().UTC()
	check := &OpenAIResetCreditCheck{Status: "unknown", CheckedAt: now.Format(time.RFC3339Nano)}
	fail := func(reason string) *OpenAIResetCreditCheck {
		check.Reason = reason
		return finishResetCreditCheck(check, nil)
	}
	token, upstreamID, proxyURL, fedRAMP, err := s.prepareUpstreamCall(callCtx, accountID)
	if err != nil {
		return fail("authentication_unavailable")
	}
	client, err := s.privacyClientFactory(proxyURL)
	if err != nil {
		return fail("client_unavailable")
	}
	headers, _, err := s.buildCodexQuotaHeaders(callCtx, accountID, token, upstreamID, fedRAMP)
	if err != nil {
		return fail("authentication_unavailable")
	}
	headers["cache-control"] = "no-cache"
	var pages []resetCreditHistoryPage
	cursor := ""
	seenCursors := map[string]bool{}
	for i := 0; i < 5; i++ {
		endpoint := chatGPTResetCreditHistoryURL
		if cursor != "" {
			endpoint += "?cursor=" + url.QueryEscape(cursor)
		}
		resp, err := client.R().SetContext(callCtx).SetHeaders(headers).Get(endpoint)
		if err != nil {
			return fail("history_request_failed")
		}
		if resp.StatusCode != http.StatusOK {
			return fail("history_unavailable")
		}
		page, err := parseResetCreditHistoryPage(resp.Bytes())
		if err != nil {
			return fail("history_format_invalid")
		}
		pages = append(pages, page)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
		if seenCursors[cursor] {
			return fail("history_cursor_repeated")
		}
		seenCursors[cursor] = true
	}
	return assessResetCreditHistory(pages, now)
}

func parseResetCreditHistoryPage(body []byte) (resetCreditHistoryPage, error) {
	var page resetCreditHistoryPage
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return page, err
	}
	for _, key := range []string{"as_of", "window_start", "events", "next_cursor"} {
		if _, ok := fields[key]; !ok {
			return page, fmt.Errorf("missing history field %s", key)
		}
	}
	err := json.Unmarshal(body, &page)
	return page, err
}

func assessResetCreditHistory(pages []resetCreditHistoryPage, now time.Time) *OpenAIResetCreditCheck {
	check := &OpenAIResetCreditCheck{Status: "unknown", CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	if len(pages) == 0 {
		check.Reason = "history_unavailable"
		return finishResetCreditCheck(check, nil)
	}
	first := pages[0]
	check.AsOf = first.AsOf.UTC().Format(time.RFC3339Nano)
	check.WindowStart = first.WindowStart.UTC().Format(time.RFC3339Nano)
	complete := true
	var latest time.Time
	seen := map[string]string{}
	recentKeys := []string{}
	for _, page := range pages {
		freshClock := !page.AsOf.IsZero() && page.AsOf.Sub(now) <= 2*time.Minute && now.Sub(page.AsOf) <= 2*time.Minute
		if page.Events == nil || page.AsOf.IsZero() || page.WindowStart.IsZero() ||
			page.AsOf.Sub(now) > 2*time.Minute || now.Sub(page.AsOf) > 2*time.Minute ||
			page.WindowStart.After(page.AsOf.Add(-OpenAIResetRecentWindow)) ||
			page.AsOf.Sub(first.AsOf) > 2*time.Minute || first.AsOf.Sub(page.AsOf) > 2*time.Minute {
			complete = false
		}
		if page.Events == nil {
			continue
		}
		for _, event := range *page.Events {
			if event.Kind != "used" {
				if event.Kind != "granted" && event.Kind != "expired" {
					complete = false
				}
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
			if err != nil || event.ID == "" || at.After(page.AsOf) || at.Before(page.WindowStart) {
				complete = false
				continue
			}
			if previous, ok := seen[event.ID]; ok {
				if previous != event.OccurredAt {
					complete = false
				}
				continue
			}
			seen[event.ID] = event.OccurredAt
			check.UsedCount++
			if at.After(latest) {
				latest = at
			}
			if freshClock && page.AsOf.Sub(at) < OpenAIResetRecentWindow {
				check.RecentUseCount++
				recentKeys = append(recentKeys, event.ID+":"+event.OccurredAt)
			}
		}
	}
	last := pages[len(pages)-1]
	if last.NextCursor != nil && *last.NextCursor != "" {
		complete = false
	}
	check.HistoryComplete = complete
	if !latest.IsZero() {
		check.LastUsedAt = latest.UTC().Format(time.RFC3339Nano)
	}
	if check.RecentUseCount > 0 {
		check.Status = "recent"
	} else if complete {
		check.Status = "clear"
	} else {
		check.Reason = "history_incomplete"
	}
	return finishResetCreditCheck(check, recentKeys)
}
