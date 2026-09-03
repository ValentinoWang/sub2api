-- Codex advances x-codex-window-id whenever compaction replaces active history.
ALTER TABLE codex_continuity_turns
    ADD COLUMN IF NOT EXISTS client_window_id VARCHAR(128) NOT NULL DEFAULT '';
