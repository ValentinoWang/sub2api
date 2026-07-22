package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type fakeOpenAIContinuityRepository struct {
	latest  *OpenAIContinuitySnapshot
	commits []OpenAIContinuityCommit
	err     error
}

func (r *fakeOpenAIContinuityRepository) LoadLatestCompleted(_ context.Context, _ string, _, _ int64, _ string) (*OpenAIContinuitySnapshot, error) {
	return r.latest, r.err
}

func (r *fakeOpenAIContinuityRepository) CommitCompleted(_ context.Context, commit OpenAIContinuityCommit) error {
	r.commits = append(r.commits, commit)
	return r.err
}

func continuityTestService(t *testing.T, apiKeyIDs []int64) (*OpenAIGatewayService, *fakeOpenAIContinuityRepository) {
	t.Helper()
	repo := &fakeOpenAIContinuityRepository{}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{OpenAIContinuity: config.GatewayOpenAIContinuityConfig{
			Enabled:        true,
			Secret:         "0123456789abcdef0123456789abcdef",
			APIKeyIDs:      apiKeyIDs,
			RetentionDays:  30,
			MaxReplayBytes: 1024 * 1024,
		}}},
		openAIContinuityRepo: repo,
	}
	return svc, repo
}

func continuityTestContext(userID, apiKeyID int64) *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set("api_key", &APIKey{ID: apiKeyID, UserID: userID})
	return c
}

func TestOpenAIContinuityIdentityIsTenantAndCredentialIsolated(t *testing.T) {
	svc, _ := continuityTestService(t, nil)

	first, _, _, enabled := svc.openAIContinuityIdentity(continuityTestContext(10, 20), "session-hash")
	require.True(t, enabled)
	same, _, _, enabled := svc.openAIContinuityIdentity(continuityTestContext(10, 20), "session-hash")
	require.True(t, enabled)
	otherUser, _, _, _ := svc.openAIContinuityIdentity(continuityTestContext(11, 20), "session-hash")
	otherKey, _, _, _ := svc.openAIContinuityIdentity(continuityTestContext(10, 21), "session-hash")

	require.Equal(t, first, same)
	require.NotEqual(t, first, otherUser)
	require.NotEqual(t, first, otherKey)
}

func TestOpenAIContinuityIdentityHonorsAPIKeyAllowlist(t *testing.T) {
	svc, _ := continuityTestService(t, []int64{20})
	_, _, _, allowed := svc.openAIContinuityIdentity(continuityTestContext(10, 20), "session-hash")
	_, _, _, denied := svc.openAIContinuityIdentity(continuityTestContext(10, 21), "session-hash")
	require.True(t, allowed)
	require.False(t, denied)
}

func TestApplyOpenAIContinuityRecoveryDropsUpstreamAnchorAndPreservesOrder(t *testing.T) {
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"first"}`),
		json.RawMessage(`{"type":"message","role":"assistant","content":"answer"}`),
	}
	payload := []byte(`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"next"}]}`)

	recovered, fullInput, err := applyOpenAIContinuityRecovery(payload, persisted)
	require.NoError(t, err)
	require.Len(t, fullInput, 3)
	require.Empty(t, openAIWSPayloadStringFromRaw(recovered, "previous_response_id"))
	items, exists, err := openAIWSExtractNormalizedInputSequence(recovered)
	require.NoError(t, err)
	require.True(t, exists)
	require.Len(t, items, 3)
}

func TestCommitOpenAIContinuityReplayPersistsCompletedSnapshot(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	replay := []json.RawMessage{json.RawMessage(`{"type":"message","role":"user","content":"hello"}`)}

	err := svc.commitOpenAIContinuityReplay(context.Background(), c, "session-hash", 99, "resp_1", replay)
	require.NoError(t, err)
	require.Len(t, repo.commits, 1)
	require.Equal(t, int64(10), repo.commits[0].UserID)
	require.Equal(t, int64(20), repo.commits[0].APIKeyID)
	require.Equal(t, int64(99), repo.commits[0].UpstreamAccountID)
	require.Equal(t, "resp_1", repo.commits[0].UpstreamResponseID)
	require.True(t, repo.commits[0].ExpiresAt.After(time.Now().Add(29*24*time.Hour)))
}

func TestCommitOpenAIContinuityReplayFailsAboveDurableBound(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	svc.cfg.Gateway.OpenAIContinuity.MaxReplayBytes = 8
	err := svc.commitOpenAIContinuityReplay(
		context.Background(),
		continuityTestContext(10, 20),
		"session-hash",
		99,
		"resp_1",
		[]json.RawMessage{json.RawMessage(`{"content":"larger than eight bytes"}`)},
	)
	require.ErrorIs(t, err, ErrOpenAIContinuityReplayTooLarge)
	require.Empty(t, repo.commits)
}

func TestPrepareOpenAIContinuityHTTPRequestRecoversAcrossAccounts(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"first"}`),
		json.RawMessage(`{"type":"message","role":"assistant","content":"answer"}`),
	}
	replayJSON, err := json.Marshal(persisted)
	require.NoError(t, err)
	digest := sha256.Sum256(replayJSON)
	sessionHash := svc.GenerateExplicitSessionHash(c, []byte(`{"prompt_cache_key":"task-1"}`))
	repo.latest = &OpenAIContinuitySnapshot{
		SessionHash: sessionHash, ReplayInput: replayJSON,
		ReplaySHA256: hex.EncodeToString(digest[:]), UpstreamAccountID: 100,
	}
	body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"task-1","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"next"}]}`)

	recovered, replay, gotSessionHash, enabled, err := svc.PrepareOpenAIContinuityHTTPRequest(context.Background(), c, body)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, sessionHash, gotSessionHash)
	require.Len(t, replay, 3)
	require.Empty(t, gjson.GetBytes(recovered, "previous_response_id").String())
	require.Len(t, gjson.GetBytes(recovered, "input").Array(), 3)
}

func TestCommitOpenAIContinuityHTTPResponseRequiresCanonicalCompletedOutput(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	result := &OpenAIForwardResult{
		ResponseID: "resp_2",
		httpCanonicalOutput: []json.RawMessage{
			json.RawMessage(`{"type":"message","role":"assistant","content":"done"}`),
		},
	}
	err := svc.CommitOpenAIContinuityHTTPResponse(
		context.Background(), c, "session-hash", 200,
		[]json.RawMessage{json.RawMessage(`{"type":"message","role":"user","content":"next"}`)},
		result,
	)
	require.NoError(t, err)
	require.Len(t, repo.commits, 1)
	var replay []json.RawMessage
	require.NoError(t, json.Unmarshal(repo.commits[0].ReplayInput, &replay))
	require.Len(t, replay, 2)

	result.httpCanonicalOutput = nil
	require.Error(t, svc.CommitOpenAIContinuityHTTPResponse(context.Background(), c, "session-hash", 200, nil, result))
	require.Len(t, repo.commits, 1)
}
