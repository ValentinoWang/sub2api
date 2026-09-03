package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var ErrOpenAIContinuityReplayTooLarge = errors.New("openai continuity replay exceeds configured durable size")

const codexLocalCompactionSummaryPrefix = "Another language model started to solve this problem and produced a summary of its thinking process. You also have access to the state of the tools that were used by that language model. Use this to build on the work that has already been done and avoid duplicating work. Here is the summary produced by the other language model, use the information in this summary to assist with your own analysis:"

const openAICodexWindowIDContextKey = "openai_codex_window_id"

// OpenAIContinuitySnapshot is the latest committed replay state for one
// Sub2API user, API key, and logical Codex task.
type OpenAIContinuitySnapshot struct {
	ContinuityID       string
	UserID             int64
	APIKeyID           int64
	SessionHash        string
	Sequence           int64
	RequestID          string
	ReplayInput        []byte
	ReplaySHA256       string
	UpstreamAccountID  int64
	UpstreamResponseID string
	ClientWindowID     string
	ExpiresAt          time.Time
}

type OpenAIContinuityCommit struct {
	ContinuityID       string
	UserID             int64
	APIKeyID           int64
	SessionHash        string
	RequestID          string
	ReplayInput        []byte
	ReplaySHA256       string
	UpstreamAccountID  int64
	UpstreamResponseID string
	ClientWindowID     string
	ExpiresAt          time.Time
}

type OpenAIContinuityRepository interface {
	LoadLatestCompleted(ctx context.Context, continuityID string, userID, apiKeyID int64, sessionHash string) (*OpenAIContinuitySnapshot, error)
	CommitCompleted(ctx context.Context, commit OpenAIContinuityCommit) error
}

func (s *OpenAIGatewayService) SetOpenAIContinuityRepository(repo OpenAIContinuityRepository) {
	if s != nil {
		s.openAIContinuityRepo = repo
	}
}

func (s *OpenAIGatewayService) openAIContinuityIdentity(c *gin.Context, sessionHash string) (string, int64, int64, bool) {
	if s == nil || s.cfg == nil || !s.cfg.Gateway.OpenAIContinuity.Enabled || s.openAIContinuityRepo == nil {
		return "", 0, 0, false
	}
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil || apiKey.ID <= 0 || apiKey.UserID <= 0 || strings.TrimSpace(sessionHash) == "" {
		return "", 0, 0, false
	}
	allowlist := s.cfg.Gateway.OpenAIContinuity.APIKeyIDs
	if len(allowlist) > 0 {
		allowed := false
		for _, id := range allowlist {
			if id == apiKey.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", 0, 0, false
		}
	}

	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(s.cfg.Gateway.OpenAIContinuity.Secret)))
	_, _ = fmt.Fprintf(mac, "v1\x00%d\x00%d\x00%s", apiKey.UserID, apiKey.ID, strings.TrimSpace(sessionHash))
	return hex.EncodeToString(mac.Sum(nil)), apiKey.UserID, apiKey.ID, true
}

func (s *OpenAIGatewayService) loadOpenAIContinuityReplay(ctx context.Context, c *gin.Context, sessionHash string) (*OpenAIContinuitySnapshot, bool, error) {
	continuityID, userID, apiKeyID, enabled := s.openAIContinuityIdentity(c, sessionHash)
	if !enabled {
		return nil, false, nil
	}
	snapshot, err := s.openAIContinuityRepo.LoadLatestCompleted(ctx, continuityID, userID, apiKeyID, sessionHash)
	if err != nil {
		return nil, true, fmt.Errorf("load OpenAI continuity snapshot: %w", err)
	}
	return snapshot, true, nil
}

func (s *OpenAIGatewayService) commitOpenAIContinuityReplay(
	ctx context.Context,
	c *gin.Context,
	sessionHash string,
	accountID int64,
	responseID string,
	replayInput []json.RawMessage,
	clientWindowID string,
) error {
	continuityID, userID, apiKeyID, enabled := s.openAIContinuityIdentity(c, sessionHash)
	if !enabled {
		return nil
	}
	if strings.TrimSpace(responseID) == "" {
		return errors.New("cannot commit OpenAI continuity without terminal response ID")
	}
	replayJSON, err := json.Marshal(replayInput)
	if err != nil {
		return fmt.Errorf("marshal OpenAI continuity replay: %w", err)
	}
	if maxBytes := s.cfg.Gateway.OpenAIContinuity.MaxReplayBytes; maxBytes > 0 && int64(len(replayJSON)) > maxBytes {
		return fmt.Errorf("%w: bytes=%d max=%d", ErrOpenAIContinuityReplayTooLarge, len(replayJSON), maxBytes)
	}
	digest := sha256.Sum256(replayJSON)
	retention := time.Duration(s.cfg.Gateway.OpenAIContinuity.RetentionDays) * 24 * time.Hour
	return s.openAIContinuityRepo.CommitCompleted(ctx, OpenAIContinuityCommit{
		ContinuityID:       continuityID,
		UserID:             userID,
		APIKeyID:           apiKeyID,
		SessionHash:        strings.TrimSpace(sessionHash),
		RequestID:          strings.TrimSpace(responseID),
		ReplayInput:        replayJSON,
		ReplaySHA256:       hex.EncodeToString(digest[:]),
		UpstreamAccountID:  accountID,
		UpstreamResponseID: strings.TrimSpace(responseID),
		ClientWindowID:     strings.TrimSpace(clientWindowID),
		ExpiresAt:          time.Now().Add(retention),
	})
}

func decodeOpenAIContinuityReplay(snapshot *OpenAIContinuitySnapshot) ([]json.RawMessage, error) {
	if snapshot == nil || len(snapshot.ReplayInput) == 0 {
		return nil, nil
	}
	digest := sha256.Sum256(snapshot.ReplayInput)
	if !hmac.Equal([]byte(hex.EncodeToString(digest[:])), []byte(strings.TrimSpace(snapshot.ReplaySHA256))) {
		return nil, errors.New("OpenAI continuity replay hash mismatch")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(snapshot.ReplayInput, &items); err != nil {
		return nil, fmt.Errorf("decode OpenAI continuity replay: %w", err)
	}
	return items, nil
}

func applyOpenAIContinuityRecovery(payload []byte, persisted []json.RawMessage) ([]byte, []json.RawMessage, error) {
	fullInput, fullInputExists, err := buildOpenAIWSReplayInputSequence(persisted, len(persisted) > 0, payload, true)
	if err != nil {
		return nil, nil, err
	}
	withoutPrevious, _, err := dropPreviousResponseIDFromRawPayload(payload)
	if err != nil {
		return nil, nil, err
	}
	recovered, err := setOpenAIWSPayloadInputSequence(withoutPrevious, fullInput, fullInputExists)
	if err != nil {
		return nil, nil, err
	}
	return recovered, fullInput, nil
}

func appendOpenAIContinuityOutput(input, output []json.RawMessage) []json.RawMessage {
	merged := make([]json.RawMessage, 0, len(input)+len(output))
	merged = append(merged, cloneOpenAIWSRawMessages(input)...)
	merged = append(merged, cloneOpenAIWSRawMessages(output)...)
	return merged
}

// appendOpenAIContinuityOutputForWSRequest rebases the durable ledger when a
// WSv2 compaction response is returned on the normal /responses wire.
func appendOpenAIContinuityOutputForWSRequest(input, canonicalOutput, replayOutput []json.RawMessage) []json.RawMessage {
	output := canonicalOutput
	if len(output) == 0 {
		output = replayOutput
	}
	if openAIContinuityInputContainsCompaction(output) {
		return cloneOpenAIWSRawMessages(output)
	}
	return appendOpenAIContinuityOutput(input, output)
}

func openAIContinuityInputContainsCompaction(items []json.RawMessage) bool {
	for _, item := range items {
		switch gjson.GetBytes(item, "type").String() {
		case "compaction", "compaction_summary":
			return true
		}
	}
	return false
}

func openAICodexWindowID(c *gin.Context, payload []byte) string {
	if c != nil && c.Request != nil {
		if windowID := strings.TrimSpace(c.GetHeader("x-codex-window-id")); windowID != "" {
			return windowID
		}
	}
	if len(payload) == 0 {
		return ""
	}
	if windowID := strings.TrimSpace(gjson.GetBytes(payload, "client_metadata.x-codex-window-id").String()); windowID != "" {
		return windowID
	}
	turnMetadata := strings.TrimSpace(gjson.GetBytes(payload, "client_metadata.x-codex-turn-metadata").String())
	if turnMetadata == "" {
		return ""
	}
	return strings.TrimSpace(gjson.Get(turnMetadata, "window_id").String())
}

func rememberOpenAICodexWindowID(c *gin.Context, windowID string) {
	if c != nil {
		c.Set(openAICodexWindowIDContextKey, strings.TrimSpace(windowID))
	}
}

func rememberedOpenAICodexWindowID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, exists := c.Get(openAICodexWindowIDContextKey); exists {
		windowID, _ := value.(string)
		return strings.TrimSpace(windowID)
	}
	return openAICodexWindowID(c, nil)
}

func openAIContinuityLocalCompactionSummaries(items []json.RawMessage) map[string]struct{} {
	summaries := make(map[string]struct{})
	for _, item := range items {
		if gjson.GetBytes(item, "type").String() != "message" || gjson.GetBytes(item, "role").String() != "user" {
			continue
		}
		content := gjson.GetBytes(item, "content")
		if content.Type == gjson.String {
			if text := content.String(); strings.HasPrefix(text, codexLocalCompactionSummaryPrefix) {
				summaries[text] = struct{}{}
			}
			continue
		}
		for _, part := range content.Array() {
			if text := part.Get("text").String(); strings.HasPrefix(text, codexLocalCompactionSummaryPrefix) {
				summaries[text] = struct{}{}
			}
		}
	}
	return summaries
}

func openAIContinuityCompactionRebaseReason(
	current, persisted []json.RawMessage,
	currentWindowID, persistedWindowID string,
) string {
	if openAIContinuityInputContainsCompaction(current) {
		return "compaction_item"
	}
	currentWindowID = strings.TrimSpace(currentWindowID)
	persistedWindowID = strings.TrimSpace(persistedWindowID)
	if currentWindowID != "" && persistedWindowID != "" && currentWindowID != persistedWindowID {
		return "window_changed"
	}
	persistedSummaries := openAIContinuityLocalCompactionSummaries(persisted)
	for summary := range openAIContinuityLocalCompactionSummaries(current) {
		if _, ok := persistedSummaries[summary]; !ok {
			return "local_compaction_summary"
		}
	}
	return ""
}

func prepareOpenAIWSContinuityCompactionRebase(
	payload []byte,
	persisted []json.RawMessage,
	persistedWindowID string,
	currentWindowID string,
) ([]byte, []json.RawMessage, bool, string, error) {
	currentInput, currentInputExists, err := openAIWSExtractNormalizedInputSequence(payload)
	if err != nil {
		return nil, nil, false, "", err
	}
	rebaseReason := openAIContinuityCompactionRebaseReason(
		currentInput,
		persisted,
		currentWindowID,
		persistedWindowID,
	)
	if rebaseReason == "" {
		return payload, cloneOpenAIWSRawMessages(persisted), len(persisted) > 0, "", nil
	}
	rebasedPayload, _, err := dropPreviousResponseIDFromRawPayload(payload)
	if err != nil {
		return nil, nil, false, "", err
	}
	return rebasedPayload, cloneOpenAIWSRawMessages(currentInput), currentInputExists, rebaseReason, nil
}

// OpenAIContinuityEnabledForHTTPRequest reports whether this authenticated
// request has an explicit task identity and is inside the continuity rollout.
func (s *OpenAIGatewayService) OpenAIContinuityEnabledForHTTPRequest(c *gin.Context, body []byte) bool {
	sessionHash := s.GenerateExplicitSessionHash(c, body)
	_, _, _, enabled := s.openAIContinuityIdentity(c, sessionHash)
	return enabled
}

// PrepareOpenAIContinuityHTTPRequest rebuilds an HTTP Responses request from
// the latest durable snapshot. The upstream response anchor is deliberately
// removed: anchors belong to one upstream account, while the replay ledger is
// portable across OAuth and API-key accounts.
func (s *OpenAIGatewayService) PrepareOpenAIContinuityHTTPRequest(
	ctx context.Context,
	c *gin.Context,
	body []byte,
) ([]byte, []json.RawMessage, string, bool, error) {
	sessionHash := s.GenerateExplicitSessionHash(c, body)
	snapshot, enabled, err := s.loadOpenAIContinuityReplay(ctx, c, sessionHash)
	if err != nil || !enabled {
		return body, nil, sessionHash, enabled, err
	}
	persisted, err := decodeOpenAIContinuityReplay(snapshot)
	if err != nil {
		return nil, nil, sessionHash, true, err
	}
	hasPrevious := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()) != ""
	if snapshot == nil && hasPrevious {
		return nil, nil, sessionHash, true, errors.New("durable continuity snapshot not found for previous_response_id")
	}
	currentInput, currentInputExists, err := openAIWSExtractNormalizedInputSequence(body)
	if err != nil {
		return nil, nil, sessionHash, true, fmt.Errorf("extract current HTTP continuity input: %w", err)
	}
	currentWindowID := openAICodexWindowID(c, body)
	rememberOpenAICodexWindowID(c, currentWindowID)
	persistedWindowID := ""
	if snapshot != nil {
		persistedWindowID = snapshot.ClientWindowID
	}
	compactionRebaseReason := openAIContinuityCompactionRebaseReason(
		currentInput,
		persisted,
		currentWindowID,
		persistedWindowID,
	)
	compactionRebased := compactionRebaseReason != ""
	fullInput := cloneOpenAIWSRawMessages(currentInput)
	fullInputExists := currentInputExists
	if !compactionRebased {
		fullInput, fullInputExists, err = buildOpenAIWSReplayInputSequence(
			persisted,
			len(persisted) > 0,
			body,
			snapshot != nil,
		)
		if err != nil {
			return nil, nil, sessionHash, true, fmt.Errorf("build HTTP continuity replay: %w", err)
		}
	}
	if snapshot == nil {
		return body, fullInput, sessionHash, true, nil
	}
	recovered, _, err := dropPreviousResponseIDFromRawPayload(body)
	if err != nil {
		return nil, nil, sessionHash, true, fmt.Errorf("drop HTTP continuity anchor: %w", err)
	}
	recovered, err = setOpenAIWSPayloadInputSequence(recovered, fullInput, fullInputExists)
	if err != nil {
		return nil, nil, sessionHash, true, fmt.Errorf("set HTTP continuity replay: %w", err)
	}
	recovered, removedStaleTrigger, err := DropStaleCompactionTriggers(recovered)
	if err != nil {
		return nil, nil, sessionHash, true, fmt.Errorf("drop stale compact trigger from HTTP continuity replay: %w", err)
	}
	if removedStaleTrigger {
		fullInput, fullInputExists, err = openAIWSExtractNormalizedInputSequence(recovered)
		if err != nil {
			return nil, nil, sessionHash, true, fmt.Errorf("extract normalized HTTP continuity replay: %w", err)
		}
		if !fullInputExists {
			fullInput = nil
		}
		logger.FromContext(ctx).Info("codex.http_continuity.dropped_stale_trigger",
			zap.Int("replay_input_count", len(fullInput)),
		)
	}
	if compactionRebased {
		logger.FromContext(ctx).Info("codex.http_continuity.compaction_rebased",
			zap.Int("old_replay_input_count", len(persisted)),
			zap.Int("new_replay_input_count", len(fullInput)),
			zap.String("reason", compactionRebaseReason),
		)
	}
	return recovered, fullInput, sessionHash, true, nil
}

// CommitOpenAIContinuityHTTPResponse advances the durable ledger only after a
// successfully completed HTTP Responses call.
func (s *OpenAIGatewayService) CommitOpenAIContinuityHTTPResponse(
	ctx context.Context,
	c *gin.Context,
	sessionHash string,
	accountID int64,
	replayInput []json.RawMessage,
	result *OpenAIForwardResult,
) error {
	if result == nil || strings.TrimSpace(result.ResponseID) == "" {
		return errors.New("cannot commit HTTP continuity without completed response")
	}
	if len(result.httpCanonicalOutput) == 0 {
		return errors.New("cannot commit HTTP continuity without canonical output")
	}
	replay := appendOpenAIContinuityOutput(replayInput, result.httpCanonicalOutput)
	// A compact response replaces the prior conversation boundary. Persisting
	// the pre-compaction replay alongside the compaction item would resurrect
	// the full history on the next resumed request.
	if isOpenAIResponsesCompactPath(c) {
		replay = cloneOpenAIWSRawMessages(result.httpCanonicalOutput)
	}
	return s.commitOpenAIContinuityReplay(
		ctx,
		c,
		sessionHash,
		accountID,
		result.ResponseID,
		replay,
		rememberedOpenAICodexWindowID(c),
	)
}
