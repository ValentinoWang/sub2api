//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHasCompactionTriggerInInput_DetectsCompactSignal(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.5",
		"stream":true,
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"compaction_trigger"}
		]
	}`)
	require.True(t, HasCompactionTriggerInInput(body))
}

func TestHasCompactionTriggerInInput_NoTrigger(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.5",
		"input":[
			{"type":"message","role":"user","content":"hello"}
		]
	}`)
	require.False(t, HasCompactionTriggerInInput(body))
}

func TestHasCompactionTriggerInInput_EmptyInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[]}`)
	require.False(t, HasCompactionTriggerInInput(body))
}

func TestHasCompactionTriggerInInput_NoInputField(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5"}`)
	require.False(t, HasCompactionTriggerInInput(body))
}

func TestHasCompactionTriggerInInput_EmptyBody(t *testing.T) {
	require.False(t, HasCompactionTriggerInInput(nil))
	require.False(t, HasCompactionTriggerInInput([]byte{}))
}

func TestHasCompactionTriggerInInput_StringInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":"compaction_trigger"}`)
	require.False(t, HasCompactionTriggerInInput(body))
}

func TestHasCompactionTriggerInInput_CompactTriggerOnly(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"compaction_trigger"}]}`)
	require.True(t, HasCompactionTriggerInInput(body))
}

func TestDropStaleCompactionTriggers_DropsTriggerBeforeNextTurn(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.6-sol",
		"input":[
			{"type":"compaction","encrypted_content":"opaque-state"},
			{"type":"compaction_trigger"},
			{"type":"message","role":"user","content":"continue"}
		]
	}`)

	normalized, changed, err := DropStaleCompactionTriggers(body)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, HasCompactionTriggerInInput(normalized))
	require.Equal(t, "opaque-state", gjson.GetBytes(normalized, "input.0.encrypted_content").String())
	require.Equal(t, "continue", gjson.GetBytes(normalized, "input.1.content").String())
}

func TestDropStaleCompactionTriggers_KeepsFinalTrigger(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","content":"compact now"},{"type":"compaction_trigger"}]}`)

	normalized, changed, err := DropStaleCompactionTriggers(body)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, body, normalized)
}
