package service

import (
	"bytes"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// HasCompactionTriggerInInput detects an input item with
// type="compaction_trigger". The handler combines this body signal with the
// request path, stream flag, and Codex beta feature header to distinguish the
// native remote compaction v2 wire from the legacy /responses/compact bridge.
func HasCompactionTriggerInInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "compaction_trigger" {
			found = true
			return false
		}
		return true
	})
	return found
}

// DropStaleCompactionTriggers removes compaction triggers that are followed by
// another input item. Codex remote compaction v2 requires a real trigger to be
// the final item; a trigger replayed before the next user message is stale.
func DropStaleCompactionTriggers(body []byte) ([]byte, bool, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, false, nil
	}

	items := input.Array()
	var rebuilt bytes.Buffer
	rebuilt.WriteByte('[')
	kept := 0
	removed := false
	for index, item := range items {
		if index != len(items)-1 && item.Get("type").String() == "compaction_trigger" {
			removed = true
			continue
		}
		if kept > 0 {
			rebuilt.WriteByte(',')
		}
		rebuilt.WriteString(item.Raw)
		kept++
	}
	rebuilt.WriteByte(']')

	if !removed {
		return body, false, nil
	}
	normalized, err := sjson.SetRawBytes(body, "input", rebuilt.Bytes())
	if err != nil {
		return nil, false, err
	}
	return normalized, true, nil
}
