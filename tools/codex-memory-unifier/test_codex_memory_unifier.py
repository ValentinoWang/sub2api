import importlib.util
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("codex_memory_unifier.py")
SPEC = importlib.util.spec_from_file_location("codex_memory_unifier", MODULE_PATH)
assert SPEC and SPEC.loader
unifier = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = unifier
SPEC.loader.exec_module(unifier)


def write_session(path: Path, session_id: str, message: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    rows = [
        {"type": "session_meta", "payload": {"id": session_id}},
        {"type": "response_item", "payload": {"message": message}},
    ]
    path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")


class CodexMemoryUnifierTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)

    def tearDown(self):
        self.temporary.cleanup()

    def home(self, name: str) -> Path:
        path = self.root / name
        path.mkdir()
        return path

    def test_exact_identity_and_digest_are_deduplicated(self):
        first = self.home("first")
        second = self.home("second")
        write_session(first / "sessions" / "a.jsonl", "thread-1", "same")
        write_session(second / "sessions" / "b.jsonl", "thread-1", "same")

        plan = unifier.build_plan([first, second], self.root / "target")

        self.assertEqual(plan["summary"]["source_files"], 2)
        self.assertEqual(plan["summary"]["output_files"], 1)
        self.assertEqual(plan["summary"]["exact_duplicates"], 1)
        self.assertEqual(len(plan["operations"][0]["sources"]), 2)

    def test_different_content_with_same_identity_is_preserved(self):
        first = self.home("first")
        second = self.home("second")
        write_session(first / "sessions" / "task.jsonl", "thread-1", "short")
        write_session(second / "sessions" / "task.jsonl", "thread-1", "detailed")

        plan = unifier.build_plan([first, second], self.root / "target")

        self.assertEqual(plan["summary"]["output_files"], 2)
        self.assertEqual(plan["summary"]["conflicting_identities"], 1)
        destinations = {item["destination"] for item in plan["operations"]}
        self.assertEqual(len(destinations), 2)
        self.assertTrue(any("__from_" in item for item in destinations))

    def test_invalid_jsonl_is_rejected(self):
        source = self.home("source")
        path = source / "sessions" / "broken.jsonl"
        path.parent.mkdir()
        path.write_text("{not-json}\n", encoding="utf-8")
        with self.assertRaises(unifier.UnifierError):
            unifier.build_plan([source], self.root / "target")

    def test_symlink_is_rejected(self):
        source = self.home("source")
        memories = source / "memories"
        memories.mkdir()
        secret = self.root / "secret"
        secret.write_text("secret", encoding="utf-8")
        try:
            (memories / "linked").symlink_to(secret)
        except OSError as exc:
            self.skipTest(f"platform cannot create test symlinks: {exc}")
        with self.assertRaises(unifier.UnifierError):
            unifier.build_plan([source], self.root / "target")

    def test_credential_like_files_are_rejected(self):
        source = self.home("source")
        memories = source / "memories"
        memories.mkdir()
        (memories / "oauth-token.txt").write_text("secret", encoding="utf-8")
        with self.assertRaises(unifier.UnifierError):
            unifier.build_plan([source], self.root / "target")

    def test_merge_preserves_config_and_creates_recoverable_backup(self):
        first = self.home("first")
        second = self.home("second")
        target = self.home("target")
        config = (
            'model_provider = "sub2api"\n'
            'profile = "work"\n'
            '[features]\nmemories = true\n'
            '[memories]\nenabled = true\n'
            '[model_providers.sub2api]\nbase_url = "https://example.invalid"\n'
            '[profiles.work]\nmodel = "gpt-test"\n'
            '[projects."/tmp/project"]\ntrust_level = "trusted"\n'
        )
        (target / "config.toml").write_text(config, encoding="utf-8")
        write_session(target / "sessions" / "existing.jsonl", "existing", "target")
        write_session(first / "sessions" / "first.jsonl", "thread-1", "one")
        write_session(second / "archived_sessions" / "second.jsonl", "thread-2", "two")
        (first / "memories").mkdir()
        (first / "memories" / "MEMORY.md").write_text("remember", encoding="utf-8")
        plan = unifier.build_plan([target, first, second], target)

        result = unifier.apply_plan(plan, confirm=True, no_active=True)

        self.assertEqual(result["status"], "merged")
        self.assertEqual((target / "config.toml").read_text(encoding="utf-8"), config)
        self.assertTrue((target / "sessions" / "existing.jsonl").exists())
        self.assertTrue((target / "sessions" / "first.jsonl").exists())
        self.assertTrue((target / "archived_sessions" / "second.jsonl").exists())
        self.assertTrue((target / "memories" / "MEMORY.md").exists())
        backup = Path(result["backup"])
        self.assertTrue((backup / "backup-manifest.json").exists())
        self.assertFalse((backup / "config.toml").exists())
        self.assertTrue((target / ".codex-memory-unifier" / "provenance.json").exists())

        restored = unifier.restore_backup(backup, target, confirm=True, no_active=True)
        self.assertEqual(restored["status"], "restored")
        self.assertTrue((target / "sessions" / "existing.jsonl").exists())
        self.assertFalse((target / "sessions" / "first.jsonl").exists())
        self.assertEqual((target / "config.toml").read_text(encoding="utf-8"), config)

    def test_merge_requires_both_confirmations(self):
        source = self.home("source")
        plan = unifier.build_plan([source], self.root / "target")
        with self.assertRaises(unifier.UnifierError):
            unifier.apply_plan(plan, confirm=False, no_active=True)
        with self.assertRaises(unifier.UnifierError):
            unifier.apply_plan(plan, confirm=True, no_active=False)

    def test_tampered_plan_cannot_escape_state_directories(self):
        source = self.home("source")
        write_session(source / "sessions" / "task.jsonl", "thread-1", "safe")
        plan = unifier.build_plan([source], self.root / "target")
        plan["operations"][0]["destination"] = "../../auth.json"

        with self.assertRaisesRegex(unifier.UnifierError, "stay inside"):
            unifier.apply_plan(plan, confirm=True, no_active=True)
        self.assertFalse((self.root / "auth.json").exists())
        self.assertFalse((self.root / ".target-memory-backups").exists())

    def test_tampered_plan_cannot_read_undeclared_sources(self):
        source = self.home("source")
        write_session(source / "sessions" / "task.jsonl", "thread-1", "safe")
        plan = unifier.build_plan([source], self.root / "target")
        plan["operations"][0]["sources"][0]["home"] = str(self.root)

        with self.assertRaisesRegex(unifier.UnifierError, "undeclared source"):
            unifier.apply_plan(plan, confirm=True, no_active=True)

    def test_source_symlink_swap_after_plan_is_rejected(self):
        source = self.home("source")
        session = source / "sessions" / "task.jsonl"
        write_session(session, "thread-1", "safe")
        plan = unifier.build_plan([source], self.root / "target")
        outside = self.root / "outside.jsonl"
        outside.write_bytes(session.read_bytes())
        session.unlink()
        session.symlink_to(outside)

        with self.assertRaisesRegex(unifier.UnifierError, "safe regular file"):
            unifier.apply_plan(plan, confirm=True, no_active=True)

    def test_insufficient_space_is_rejected_before_backup(self):
        source = self.home("source")
        target = self.home("target")
        write_session(source / "sessions" / "task.jsonl", "thread-1", "content")
        plan = unifier.build_plan([source], target)
        disk_usage = type("DiskUsage", (), {"free": 1})()

        with mock.patch.object(unifier.shutil, "disk_usage", return_value=disk_usage):
            with self.assertRaisesRegex(unifier.UnifierError, "insufficient free space"):
                unifier.apply_plan(plan, confirm=True, no_active=True)
        self.assertFalse((self.root / ".target-memory-backups").exists())

    def test_interrupted_directory_swap_restores_original_state(self):
        source = self.home("source")
        target = self.home("target")
        original = target / "sessions" / "original.jsonl"
        write_session(original, "target-thread", "original")
        write_session(source / "sessions" / "new.jsonl", "source-thread", "new")
        before = unifier.sha256_file(original)
        plan = unifier.build_plan([source, target], target)
        calls = 0

        def fail_once(source_path, destination_path):
            nonlocal calls
            calls += 1
            if calls == 3:
                raise OSError("injected swap interruption")
            unifier.os.replace(source_path, destination_path)

        with self.assertRaisesRegex(OSError, "injected swap interruption"):
            unifier.apply_plan(plan, confirm=True, no_active=True, replace=fail_once)
        self.assertEqual(unifier.sha256_file(original), before)
        self.assertFalse((target / "sessions" / "new.jsonl").exists())

    def test_large_task_record_is_merged_and_verified(self):
        source = self.home("source")
        target = self.home("target")
        message = "x" * (2 * 1024 * 1024)
        write_session(source / "sessions" / "large.jsonl", "large-thread", message)
        plan = unifier.build_plan([source], target)

        unifier.apply_plan(plan, confirm=True, no_active=True)

        merged = target / "sessions" / "large.jsonl"
        self.assertEqual(unifier.sha256_file(merged), plan["operations"][0]["sha256"])

    def test_activation_uses_platform_specific_user_scope(self):
        target = self.root / "target"
        cases = [
            ("Darwin", "launchctl", ["launchctl", "setenv", "CODEX_HOME", str(target)]),
            ("Windows", "setx", ["setx", "CODEX_HOME", str(target)]),
            ("Linux", "environment.d", None),
        ]
        for system, method, expected_command in cases:
            calls = []

            def runner(command, check):
                calls.append((command, check))

            with self.subTest(system=system), \
                 mock.patch.object(unifier.platform, "system", return_value=system), \
                 mock.patch.object(unifier.Path, "home", return_value=self.root):
                result = unifier.activate_home(target, True, runner=runner)
                self.assertEqual(result["method"], method)
                if expected_command is not None:
                    self.assertEqual(calls[0][0], expected_command)
        linux_environment = self.root / ".config" / "environment.d" / "90-codex-home.conf"
        self.assertIn(str(target), linux_environment.read_text(encoding="utf-8"))

    def test_activation_failure_reports_successful_merge_and_backup(self):
        source = self.home("source")
        target = self.home("target")
        write_session(source / "sessions" / "task.jsonl", "thread-1", "content")
        plan_path = self.root / "plan.json"
        unifier.write_json_atomic(plan_path, unifier.build_plan([source], target))
        output = StringIO()

        with mock.patch.object(unifier, "activate_home", side_effect=OSError("activation denied")), \
             redirect_stdout(output):
            exit_code = unifier.main([
                "merge", "--plan", str(plan_path), "--confirm",
                "--confirm-no-active-requests", "--activate",
            ])

        result = json.loads(output.getvalue())
        self.assertEqual(exit_code, 3)
        self.assertEqual(result["status"], "merged_activation_failed")
        self.assertTrue(Path(result["backup"]).is_dir())
        self.assertTrue((target / "sessions" / "task.jsonl").is_file())


if __name__ == "__main__":
    unittest.main()
