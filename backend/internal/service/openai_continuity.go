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

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

var ErrOpenAIContinuityReplayTooLarge = errors.New("openai continuity replay exceeds configured durable size")

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
	fullInput, fullInputExists, err := buildOpenAIWSReplayInputSequence(
		persisted,
		len(persisted) > 0,
		body,
		snapshot != nil,
	)
	if err != nil {
		return nil, nil, sessionHash, true, fmt.Errorf("build HTTP continuity replay: %w", err)
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
	return s.commitOpenAIContinuityReplay(
		ctx,
		c,
		sessionHash,
		accountID,
		result.ResponseID,
		appendOpenAIContinuityOutput(replayInput, result.httpCanonicalOutput),
	)
}
