#!/usr/bin/env python3
"""Focused read-only planning checks for the Codex session migration tool."""

from __future__ import annotations

import argparse
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from codex_migration_inspect import inspect, sha256_file
from codex_session_migrate import plan_for
from test_codex_session_migrate import CodexSessionMigrationTests


class MigrationInspectionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.fixture = CodexSessionMigrationTests()
        self.fixture.setUp()

    def tearDown(self) -> None:
        self.fixture.tearDown()

    def test_selected_plan_keeps_other_history_out_of_scope(self) -> None:
        before = sha256_file(self.fixture.session)
        plan = plan_for(
            argparse.Namespace(
                codex_home=str(self.fixture.root),
                target_provider="sub2api",
                source_provider="remote_sub2api",
                all_history=False,
                session_id=["task-1"],
            )
        )

        self.assertEqual("READY", plan["status"])
        self.assertEqual(["task-1"], [record["session_id"] for record in plan["sessions"]])
        self.assertEqual(["task-1"], [record["session_id"] for record in plan["indexes"]])
        self.assertEqual(before, sha256_file(self.fixture.session))

    def test_invalid_jsonl_is_reported_without_mutating_the_history(self) -> None:
        self.fixture.session.write_bytes(self.fixture.session.read_bytes() + b"{invalid}\\n")
        before = sha256_file(self.fixture.session)

        result = inspect(self.fixture.root, "sub2api", "remote_sub2api")

        self.assertTrue(result["diagnostics"])
        self.assertIn("invalid JSONL", result["diagnostics"][0])
        self.assertEqual(before, sha256_file(self.fixture.session))


if __name__ == "__main__":
    unittest.main()
