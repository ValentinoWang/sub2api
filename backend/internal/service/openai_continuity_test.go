package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func TestOpenAICodexWindowIDUsesCanonicalHeaderAndMetadataFallbacks(t *testing.T) {
	c := continuityTestContext(10, 20)
	c.Request.Header.Set("x-codex-window-id", "window-header")
	payload := []byte(`{"client_metadata":{"x-codex-window-id":"window-flat","x-codex-turn-metadata":"{\"window_id\":\"window-nested\"}"}}`)
	require.Equal(t, "window-header", openAICodexWindowID(c, payload))

	c.Request.Header.Del("x-codex-window-id")
	require.Equal(t, "window-flat", openAICodexWindowID(c, payload))

	payload = []byte(`{"client_metadata":{"x-codex-turn-metadata":"{\"window_id\":\"window-nested\"}"}}`)
	require.Equal(t, "window-nested", openAICodexWindowID(c, payload))
}

func TestApplyOpenAIContinuityRecoveryDropsUpstreamAnchorAndPreservesOrder(t *testing.T) {
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"first"}`),
		json.RawMessage(`{"type":"message","role":"assistant","content":"answer"}`),
	}
	payload := []byte(`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"next"}]}`)

	recovered, fullInput, rebaseReason, err := applyOpenAIContinuityRecovery(payload, persisted)
	require.NoError(t, err)
	require.Empty(t, rebaseReason)
	require.Len(t, fullInput, 3)
	require.Empty(t, openAIWSPayloadStringFromRaw(recovered, "previous_response_id"))
	items, exists, err := openAIWSExtractNormalizedInputSequence(recovered)
	require.NoError(t, err)
	require.True(t, exists)
	require.Len(t, items, 3)
}

func TestOpenAIContinuityRecoveryRebasesOnlyOnLongTailOverlap(t *testing.T) {
	persisted := make([]json.RawMessage, 0, 8)
	for i := 0; i < 8; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		persisted = append(persisted, json.RawMessage(fmt.Sprintf(`{"type":"message","role":"%s","content":"turn-%d"}`, role, i)))
	}

	longTailCurrent := append([]json.RawMessage{
		json.RawMessage(`{"type":"message","role":"developer","content":"current instructions"}`),
	}, persisted...)
	payload, err := json.Marshal(map[string]any{
		"type":  "response.create",
		"input": longTailCurrent,
	})
	require.NoError(t, err)
	recovered, replay, reason, err := applyOpenAIContinuityRecovery(payload, persisted)
	require.NoError(t, err)
	require.Equal(t, "persisted_tail_overlap", reason)
	require.Len(t, replay, 9)
	require.Len(t, gjson.GetBytes(recovered, "input").Array(), 9)
	require.Equal(t, "developer", gjson.GetBytes(recovered, "input.0.role").String())

	shortCurrent := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"developer","content":"current instructions"}`),
		persisted[6],
		persisted[7],
	}
	shortPayload, err := json.Marshal(map[string]any{
		"type":  "response.create",
		"input": shortCurrent,
	})
	require.NoError(t, err)
	_, shortReplay, shortReason, err := applyOpenAIContinuityRecovery(shortPayload, persisted)
	require.NoError(t, err)
	require.Empty(t, shortReason)
	require.Len(t, shortReplay, 11)
}

func TestAppendOpenAIContinuityOutputDeduplicatesToolItemsByTypeAndCallID(t *testing.T) {
	input := []json.RawMessage{
		json.RawMessage(`{"type":"function_call","id":"fc-1","call_id":"call-1","name":"lookup","arguments":"{}"}`),
		json.RawMessage(`{"type":"function_call_output","call_id":"call-1","output":"ok"}`),
	}
	output := []json.RawMessage{
		json.RawMessage(`{"type":"function_call","id":"fc-1","call_id":"call-1","name":"lookup","arguments":"{}"}`),
		json.RawMessage(`{"type":"function_call_output","call_id":"call-1","output":"ok"}`),
		json.RawMessage(`{"type":"message","role":"assistant","content":"done"}`),
	}

	merged := appendOpenAIContinuityOutput(input, output)
	require.Len(t, merged, 3)
	require.Equal(t, "function_call", gjson.GetBytes(merged[0], "type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(merged[1], "type").String())
	require.Equal(t, "message", gjson.GetBytes(merged[2], "type").String())
}

func TestCommitOpenAIContinuityReplayPersistsCompletedSnapshot(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	c.Request.Header.Set("x-codex-window-id", "019fc2d4-window")
	replay := []json.RawMessage{json.RawMessage(`{"type":"message","role":"user","content":"hello"}`)}

	err := svc.commitOpenAIContinuityReplay(context.Background(), c, "session-hash", 99, "resp_1", replay, "019fc2d4-window")
	require.NoError(t, err)
	require.Len(t, repo.commits, 1)
	require.Equal(t, int64(10), repo.commits[0].UserID)
	require.Equal(t, int64(20), repo.commits[0].APIKeyID)
	require.Equal(t, int64(99), repo.commits[0].UpstreamAccountID)
	require.Equal(t, "resp_1", repo.commits[0].UpstreamResponseID)
	require.Equal(t, "019fc2d4-window", repo.commits[0].ClientWindowID)
	require.True(t, repo.commits[0].ExpiresAt.After(time.Now().Add(29*24*time.Hour)))
}

func TestPrepareAndCommitOpenAIContinuityPersistsPayloadWindowID(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"task-payload-window","client_metadata":{"x-codex-window-id":"window-from-payload"},"input":[{"type":"message","role":"user","content":"hello"}]}`)

	_, replay, sessionHash, enabled, err := svc.PrepareOpenAIContinuityHTTPRequest(context.Background(), c, body)
	require.NoError(t, err)
	require.True(t, enabled)
	result := &OpenAIForwardResult{
		ResponseID: "resp_payload_window",
		httpCanonicalOutput: []json.RawMessage{
			json.RawMessage(`{"type":"message","role":"assistant","content":"done"}`),
		},
	}
	require.NoError(t, svc.CommitOpenAIContinuityHTTPResponse(context.Background(), c, sessionHash, 99, replay, result))
	require.Len(t, repo.commits, 1)
	require.Equal(t, "window-from-payload", repo.commits[0].ClientWindowID)
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
		"",
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

func TestPrepareOpenAIContinuityHTTPRequestDropsPersistedCompactionTriggerBeforeNextTurn(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"compaction","id":"cmp_1","encrypted_content":"opaque-state"}`),
		json.RawMessage(`{"type":"compaction_trigger"}`),
	}
	replayJSON, err := json.Marshal(persisted)
	require.NoError(t, err)
	digest := sha256.Sum256(replayJSON)
	sessionHash := svc.GenerateExplicitSessionHash(c, []byte(`{"prompt_cache_key":"task-compact"}`))
	repo.latest = &OpenAIContinuitySnapshot{
		SessionHash: sessionHash, ReplayInput: replayJSON,
		ReplaySHA256: hex.EncodeToString(digest[:]), UpstreamAccountID: 100,
	}
	body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"task-compact","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"continue"}]}`)

	recovered, replay, _, enabled, err := svc.PrepareOpenAIContinuityHTTPRequest(context.Background(), c, body)
	require.NoError(t, err)
	require.True(t, enabled)
	require.False(t, HasCompactionTriggerInInput(recovered))
	require.Len(t, replay, 2)
	require.Equal(t, "compaction", gjson.GetBytes(recovered, "input.0.type").String())
	require.Equal(t, "message", gjson.GetBytes(recovered, "input.1.type").String())
}

func TestPrepareOpenAIContinuityHTTPRequestRebasesOnCurrentCompaction(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"old user message"}`),
		json.RawMessage(`{"type":"message","role":"assistant","content":"old assistant message"}`),
		json.RawMessage(`{"type":"function_call","name":"old_tool","call_id":"call_old","arguments":"{}"}`),
	}
	replayJSON, err := json.Marshal(persisted)
	require.NoError(t, err)
	digest := sha256.Sum256(replayJSON)
	sessionHash := svc.GenerateExplicitSessionHash(c, []byte(`{"prompt_cache_key":"task-current-compaction"}`))
	repo.latest = &OpenAIContinuitySnapshot{
		SessionHash: sessionHash, ReplayInput: replayJSON,
		ReplaySHA256: hex.EncodeToString(digest[:]), UpstreamAccountID: 100,
	}
	body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"task-current-compaction","previous_response_id":"resp_old","input":[{"type":"message","role":"developer","content":"current instructions"},{"type":"compaction","id":"cmp_current","encrypted_content":"replacement-state"},{"type":"message","role":"user","content":"continue after compaction"}]}`)

	recovered, replay, _, enabled, err := svc.PrepareOpenAIContinuityHTTPRequest(context.Background(), c, body)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Len(t, replay, 3)
	require.Len(t, gjson.GetBytes(recovered, "input").Array(), 3)
	require.Empty(t, gjson.GetBytes(recovered, "previous_response_id").String())
	require.Equal(t, "developer", gjson.GetBytes(recovered, "input.0.role").String())
	require.Equal(t, "compaction", gjson.GetBytes(recovered, "input.1.type").String())
	require.Equal(t, "continue after compaction", gjson.GetBytes(recovered, "input.2.content").String())
	require.False(t, gjson.GetBytes(recovered, "input.#(content==\"old user message\")").Exists())
	require.False(t, gjson.GetBytes(recovered, "input.#(name==\"old_tool\")").Exists())
	for i := range replay {
		require.JSONEq(t, gjson.GetBytes(body, "input").Array()[i].Raw, string(replay[i]))
	}
}

func TestPrepareOpenAIContinuityHTTPRequestRebasesOnWindowChange(t *testing.T) {
	svc, repo := continuityTestService(t, nil)
	c := continuityTestContext(10, 20)
	c.Request.Header.Set("x-codex-window-id", "window-after")
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"old user message"}`),
		json.RawMessage(`{"type":"function_call","name":"old_tool","call_id":"call_old","arguments":"{}"}`),
	}
	replayJSON, err := json.Marshal(persisted)
	require.NoError(t, err)
	digest := sha256.Sum256(replayJSON)
	sessionHash := svc.GenerateExplicitSessionHash(c, []byte(`{"prompt_cache_key":"task-window-change"}`))
	repo.latest = &OpenAIContinuitySnapshot{
		SessionHash: sessionHash, ReplayInput: replayJSON,
		ReplaySHA256: hex.EncodeToString(digest[:]), UpstreamAccountID: 100,
		ClientWindowID: "window-before",
	}
	body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"task-window-change","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"replacement history"}]}`)

	recovered, replay, _, enabled, err := svc.PrepareOpenAIContinuityHTTPRequest(context.Background(), c, body)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Empty(t, gjson.GetBytes(recovered, "previous_response_id").String())
	require.Len(t, replay, 1)
	require.Equal(t, "replacement history", gjson.GetBytes(recovered, "input.0.content").String())
	require.False(t, gjson.GetBytes(recovered, "input.#(name==\"old_tool\")").Exists())
}

func TestAppendOpenAIContinuityOutputForWSRequestReplacesOnCompaction(t *testing.T) {
	for _, itemType := range []string{"compaction", "compaction_summary"} {
		t.Run(itemType, func(t *testing.T) {
			previous := []json.RawMessage{
				json.RawMessage(`{"type":"message","role":"user","content":"old"}`),
			}
			compaction := []json.RawMessage{
				json.RawMessage(fmt.Sprintf(`{"type":"%s","id":"cmp_ws","encrypted_content":"replacement-state"}`, itemType)),
			}

			replay := appendOpenAIContinuityOutputForWSRequest(previous, compaction, nil)

			require.Len(t, replay, 1)
			require.Equal(t, itemType, gjson.GetBytes(replay[0], "type").String())
		})
	}
}

func TestPrepareOpenAIWSContinuityCompactionRebaseReplacesPersistedReplay(t *testing.T) {
	for _, itemType := range []string{"compaction", "compaction_summary"} {
		t.Run(itemType, func(t *testing.T) {
			persisted := []json.RawMessage{
				json.RawMessage(`{"type":"message","role":"user","content":"old history"}`),
				json.RawMessage(`{"type":"function_call","name":"old_tool","call_id":"call_old","arguments":"{}"}`),
			}
			payload := []byte(fmt.Sprintf(
				`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"developer","content":"current instructions"},{"type":"%s","id":"cmp_current","encrypted_content":"replacement-state"},{"type":"message","role":"user","content":"continue"}]}`,
				itemType,
			))

			rebasedPayload, replay, replayExists, rebaseReason, err :=
				prepareOpenAIWSContinuityCompactionRebase(payload, persisted, "window-old", "window-old")

			require.NoError(t, err)
			require.Equal(t, "compaction_item", rebaseReason)
			require.True(t, replayExists)
			require.Empty(t, gjson.GetBytes(rebasedPayload, "previous_response_id").String())
			require.Len(t, replay, 3)
			require.Equal(t, itemType, gjson.GetBytes(replay[1], "type").String())
			require.False(t, gjson.GetBytes(rebasedPayload, "input.#(content==\"old history\")").Exists())
			require.False(t, gjson.GetBytes(rebasedPayload, "input.#(name==\"old_tool\")").Exists())
		})
	}
}

func TestPrepareOpenAIWSContinuityCompactionRebaseKeepsPersistedReplayWithoutCompaction(t *testing.T) {
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"old history"}`),
	}
	payload := []byte(`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"continue"}]}`)

	rebasedPayload, replay, replayExists, rebaseReason, err :=
		prepareOpenAIWSContinuityCompactionRebase(payload, persisted, "window-1", "window-1")

	require.NoError(t, err)
	require.Empty(t, rebaseReason)
	require.True(t, replayExists)
	require.Equal(t, "resp_old", gjson.GetBytes(rebasedPayload, "previous_response_id").String())
	require.Len(t, replay, 1)
	require.Equal(t, "old history", gjson.GetBytes(replay[0], "content").String())
}

func TestPrepareOpenAIWSContinuityCompactionRebaseReplacesOnLocalSummary(t *testing.T) {
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"old history"}`),
		json.RawMessage(`{"type":"function_call","name":"old_tool","call_id":"call_old","arguments":"{}"}`),
	}
	summary := codexLocalCompactionSummaryPrefix + "\n## Current state\nThe old work is summarized."
	payload := []byte(fmt.Sprintf(
		`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"current instructions"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`,
		summary,
	))

	rebasedPayload, replay, replayExists, rebaseReason, err :=
		prepareOpenAIWSContinuityCompactionRebase(payload, persisted, "", "window-after-compact")

	require.NoError(t, err)
	require.Equal(t, "local_compaction_summary", rebaseReason)
	require.True(t, replayExists)
	require.Empty(t, gjson.GetBytes(rebasedPayload, "previous_response_id").String())
	require.Len(t, replay, 3)
	require.False(t, gjson.GetBytes(rebasedPayload, "input.#(name==\"old_tool\")").Exists())
}

func TestPrepareOpenAIWSContinuityCompactionRebaseReplacesOnWindowChange(t *testing.T) {
	persisted := []json.RawMessage{
		json.RawMessage(`{"type":"message","role":"user","content":"old history"}`),
	}
	payload := []byte(`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"replacement history"}]}`)

	rebasedPayload, replay, replayExists, rebaseReason, err :=
		prepareOpenAIWSContinuityCompactionRebase(payload, persisted, "window-before", "window-after")

	require.NoError(t, err)
	require.Equal(t, "window_changed", rebaseReason)
	require.True(t, replayExists)
	require.Empty(t, gjson.GetBytes(rebasedPayload, "previous_response_id").String())
	require.Len(t, replay, 1)
	require.Equal(t, "replacement history", gjson.GetBytes(replay[0], "content").String())
}

func TestPrepareOpenAIWSContinuityCompactionRebaseDoesNotRepeatKnownLocalSummary(t *testing.T) {
	summary := codexLocalCompactionSummaryPrefix + "\nKnown summary."
	persisted := []json.RawMessage{
		json.RawMessage(fmt.Sprintf(`{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}`, summary)),
		json.RawMessage(`{"type":"message","role":"assistant","content":"answer after compact"}`),
	}
	payload := []byte(fmt.Sprintf(
		`{"type":"response.create","previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]},{"type":"message","role":"user","content":"next"}]}`,
		summary,
	))

	rebasedPayload, replay, replayExists, rebaseReason, err :=
		prepareOpenAIWSContinuityCompactionRebase(payload, persisted, "window-current", "window-current")

	require.NoError(t, err)
	require.Empty(t, rebaseReason)
	require.True(t, replayExists)
	require.Equal(t, "resp_old", gjson.GetBytes(rebasedPayload, "previous_response_id").String())
	require.Len(t, replay, 2)
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

func TestCommitOpenAIContinuityHTTPResponseReplacesReplayOnCompaction(t *testing.T) {
	for _, itemType := range []string{"compaction", "compaction_summary"} {
		t.Run(itemType, func(t *testing.T) {
			svc, repo := continuityTestService(t, nil)
			c := continuityTestContext(10, 20)
			c.Request = httptest.NewRequest("POST", "/v1/responses/compact", nil)
			oldReplay := []json.RawMessage{
				json.RawMessage(`{"type":"message","role":"user","content":"old history"}`),
			}
			result := &OpenAIForwardResult{
				ResponseID: "cmp_resp",
				httpCanonicalOutput: []json.RawMessage{
					json.RawMessage(fmt.Sprintf(`{"type":"%s","id":"cmp_new","encrypted_content":"opaque"}`, itemType)),
				},
			}

			require.NoError(t, svc.CommitOpenAIContinuityHTTPResponse(context.Background(), c, "session-hash", 99, oldReplay, result))
			require.Len(t, repo.commits, 1)
			var replay []json.RawMessage
			require.NoError(t, json.Unmarshal(repo.commits[0].ReplayInput, &replay))
			require.Len(t, replay, 1)
			require.Equal(t, itemType, gjson.GetBytes(replay[0], "type").String())
			require.False(t, gjson.GetBytes(repo.commits[0].ReplayInput, "#(content==\"old history\")").Exists())
		})
	}
}
