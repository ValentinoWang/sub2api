#!/usr/bin/env python3
from __future__ import annotations

import json
import sqlite3
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from codex_migration_inspect import inspect, sha256_file
from codex_migration_transaction import BackupError, DriftError, apply_plan, rollback


class CodexSessionMigrationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / ".codex"
        (self.root / "sessions" / "2026").mkdir(parents=True)
        (self.root / "archived_sessions").mkdir()
        (self.root / "config.toml").write_text(
            'model_provider = "sub2api"\n[model_providers.sub2api]\nbase_url = "https://example.test"\n', encoding="utf-8"
        )
        self.session = self.root / "sessions" / "2026" / "session.jsonl"
        self.message = b'{"type":"message","payload":{"text":"provider remote_sub2api must remain inside content"}}\n'
        self.session.write_bytes(
            b'{"type":"session_meta","payload":{"id":"task-1","model_provider":"remote_sub2api","title":"keep"}}\n'
            + self.message
        )
        archived = self.root / "archived_sessions" / "archived.jsonl"
        archived.write_text('{"type":"session_meta","payload":{"id":"task-2","model_provider":"remote_sub2api"}}\n', encoding="utf-8")
        self.db = self.root / "state.sqlite"
        connection = sqlite3.connect(self.db)
        connection.execute("CREATE TABLE session_index (session_id TEXT, model_provider TEXT, note TEXT)")
        connection.execute("INSERT INTO session_index VALUES ('task-1', 'remote_sub2api', 'leave alone')")
        connection.execute("INSERT INTO session_index VALUES ('other', 'remote_sub2api', 'leave alone')")
        connection.commit()
        connection.close()

    def tearDown(self) -> None:
        self.temp.cleanup()

    def ready_plan(self) -> dict:
        plan = inspect(self.root, "sub2api", "remote_sub2api")
        plan.update({"target_provider": "sub2api", "source_provider": "remote_sub2api", "status": "READY"})
        return plan

    def test_inspect_is_read_only_and_matches_active_and_archived_sessions(self) -> None:
        session_before = sha256_file(self.session)
        db_before = sha256_file(self.db)
        result = inspect(self.root, "sub2api", "remote_sub2api")
        self.assertEqual(result["selected_session_count"], 2)
        self.assertEqual(result["selected_index_count"], 1)
        self.assertEqual(session_before, sha256_file(self.session))
        self.assertEqual(db_before, sha256_file(self.db))
        self.assertEqual([], result["diagnostics"])

    def test_target_must_be_declared_and_builtin_openai_is_supported(self) -> None:
        with self.assertRaisesRegex(Exception, "not declared"):
            inspect(self.root, "missing", "remote_sub2api")
        self.assertEqual(2, inspect(self.root, "openai", "remote_sub2api")["selected_session_count"])

    def test_apply_preserves_message_and_unrelated_rows_then_rollback_restores_selected_data(self) -> None:
        plan = self.ready_plan()
        backup = Path(self.temp.name) / "backup"
        journal = apply_plan(plan, backup)
        self.assertEqual("VERIFIED", journal["status"])
        content = self.session.read_bytes()
        self.assertIn(self.message, content)
        self.assertIn(b'"model_provider":"sub2api"', content)
        connection = sqlite3.connect(self.db)
        self.assertEqual("sub2api", connection.execute("SELECT model_provider FROM session_index WHERE session_id='task-1'").fetchone()[0])
        self.assertEqual("remote_sub2api", connection.execute("SELECT model_provider FROM session_index WHERE session_id='other'").fetchone()[0])
        connection.close()
        restored = rollback(backup)
        self.assertEqual("ROLLED_BACK", restored["status"])
        self.assertIn(b'"model_provider":"remote_sub2api"', self.session.read_bytes())

    def test_backup_failure_prevents_writes(self) -> None:
        plan = self.ready_plan()
        before = sha256_file(self.session)
        occupied = Path(self.temp.name) / "occupied"
        occupied.write_text("not a directory", encoding="utf-8")
        with self.assertRaises(BackupError):
            apply_plan(plan, occupied)
        self.assertEqual(before, sha256_file(self.session))

    def test_drift_blocks_apply_before_backup_and_rollback_refuses_new_messages(self) -> None:
        plan = self.ready_plan()
        self.session.write_bytes(self.session.read_bytes() + b'{"type":"message","payload":{"text":"new"}}\n')
        with self.assertRaises(DriftError):
            apply_plan(plan, Path(self.temp.name) / "backup")
        plan = self.ready_plan()
        backup = Path(self.temp.name) / "backup-2"
        apply_plan(plan, backup)
        self.session.write_bytes(self.session.read_bytes() + b'{"type":"message","payload":{"text":"new"}}\n')
        with self.assertRaises(DriftError):
            rollback(backup)


if __name__ == "__main__":
    unittest.main()
