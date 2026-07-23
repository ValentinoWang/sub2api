#!/usr/bin/env python3
"""Run a disposable end-to-end merge and restore acceptance scenario."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
import tempfile
from pathlib import Path


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_session(path: Path, session_id: str, message: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    rows = [
        {"type": "session_meta", "payload": {"id": session_id}},
        {"type": "response_item", "payload": {"message": message}},
    ]
    path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")


def run(command: list[str]) -> dict:
    completed = subprocess.run(command, check=True, capture_output=True, text=True)
    return json.loads(completed.stdout)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", required=True, type=Path)
    args = parser.parse_args()
    command_prefix = [sys.executable, str(Path(__file__).with_name("codex_memory_unifier.py"))]
    with tempfile.TemporaryDirectory(prefix="codex-memory-acceptance-") as temporary:
        root = Path(temporary)
        first = root / "oauth-home"
        second = root / "sub2api-home"
        target = root / "unified-home"
        for home in (first, second, target):
            home.mkdir()
        first_auth = first / "auth.json"
        second_auth = second / "auth.json"
        first_auth.write_text('{"token":"oauth-secret"}\n', encoding="utf-8")
        second_auth.write_text('{"token":"provider-secret"}\n', encoding="utf-8")
        write_session(first / "sessions" / "task.jsonl", "shared-task", "oauth history")
        write_session(second / "sessions" / "task.jsonl", "shared-task", "provider history")
        write_session(target / "sessions" / "original.jsonl", "target-task", "before merge")
        memory = first / "memories" / "MEMORY.md"
        memory.parent.mkdir()
        memory.write_text("provider-independent preference\n", encoding="utf-8")
        config = target / "config.toml"
        config.write_text(
            'model_provider = "sub2api"\n[features]\nmemories = true\n[memories]\nenabled = true\n',
            encoding="utf-8",
        )
        protected = {str(path): digest(path) for path in (first_auth, second_auth, config)}
        plan_path = root / "plan.json"
        planned = run([
            *command_prefix, "plan",
            "--source", str(first),
            "--source", str(second),
            "--source", str(target),
            "--target", str(target),
            "--output", str(plan_path),
        ])
        merged = run([
            *command_prefix, "merge",
            "--plan", str(plan_path),
            "--confirm",
            "--confirm-no-active-requests",
        ])
        backup = Path(merged["backup"])
        backup_manifest = json.loads((backup / "backup-manifest.json").read_text(encoding="utf-8"))
        backup_files = set(backup_manifest["files"])
        if "config.toml" in backup_files or (backup / "config.toml").exists():
            raise RuntimeError("backup copied config.toml instead of preserving it by hash")
        if any(Path(relative).parts[0] not in {"memories", "sessions", "archived_sessions"} for relative in backup_files):
            raise RuntimeError("backup contains a file outside the declared state directories")
        conflicts = list((target / "sessions").glob("task__from_*.jsonl"))
        if len(conflicts) != 1 or not (target / "sessions" / "task.jsonl").is_file():
            raise RuntimeError("conflicting task histories were not both preserved")
        if any(digest(Path(path)) != expected for path, expected in protected.items()):
            raise RuntimeError("source credentials or target config changed during merge")
        restored = run([
            *command_prefix, "restore",
            "--backup", merged["backup"],
            "--target", str(target),
            "--confirm",
            "--confirm-no-active-requests",
        ])
        if not (target / "sessions" / "original.jsonl").is_file() or (target / "sessions" / "task.jsonl").exists():
            raise RuntimeError("restore did not reproduce the original target state")
        if any(digest(Path(path)) != expected for path, expected in protected.items()):
            raise RuntimeError("protected files changed during restore")
        report = {
            "status": "passed",
            "planned": planned,
            "merged": {key: value for key, value in merged.items() if key != "backup"},
            "restored": restored["status"],
            "conflicts_preserved": 2,
            "credentials_unchanged": True,
            "config_unchanged": True,
            "backup_excludes_config": True,
            "backup_inventory_scoped": True,
            "provider_labels": ["oauth", "sub2api", "custom"],
            "temporary_state_removed_on_exit": True,
        }
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
