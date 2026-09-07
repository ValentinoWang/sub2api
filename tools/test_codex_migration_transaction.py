#!/usr/bin/env python3
"""Focused interruption and recovery checks for the migration transaction."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parent))

from codex_migration_transaction import DriftError, apply_plan, recover
from test_codex_session_migrate import CodexSessionMigrationTests


class MigrationTransactionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.fixture = CodexSessionMigrationTests()
        self.fixture.setUp()

    def tearDown(self) -> None:
        self.fixture.tearDown()

    def test_interrupted_index_write_records_recovery_without_claiming_success(self) -> None:
        backup = Path(self.fixture.temp.name) / "interrupted-backup"
        with patch("codex_migration_transaction._update_index", side_effect=OSError("interrupted")):
            with self.assertRaises(OSError):
                apply_plan(self.fixture.ready_plan(), backup)

        self.assertIn(b'"model_provider":"sub2api"', self.fixture.session.read_bytes())
        with self.assertRaisesRegex(DriftError, "requires rollback"):
            recover(backup)


if __name__ == "__main__":
    unittest.main()
