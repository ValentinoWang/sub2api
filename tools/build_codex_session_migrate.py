#!/usr/bin/env python3
"""Build the standalone offline Codex session migration download."""

from __future__ import annotations

import hashlib
import json
import shutil
import sys
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
PUBLIC = ROOT / "frontend" / "public"
VERSION = "1.0.0"
FILES = [
    "codex_session_migrate.py",
    "codex_migration_inspect.py",
    "codex_migration_transaction.py",
    "codex-session-migrate.sh",
    "codex-session-migrate.ps1",
]
PROMPT = ROOT / "frontend" / "src" / "content" / "codexSessionMigrationPrompt.txt"


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def main() -> int:
    destination = PUBLIC / f"codex-session-migrate-{VERSION}.zip"
    PUBLIC.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(destination, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for name in FILES:
            archive.write(TOOLS / name, f"codex-session-migrate/{name}")
        archive.writestr(
            "codex-session-migrate/README.txt",
            "Run inspect first. Review a plan, close Codex, then use apply with its exact digest.\n"
            "The tool never uploads local history or changes config.toml/authentication files.\n",
        )
    manifest = {
        "schema_version": 1,
        "tool_version": VERSION,
        "file": destination.name,
        "size": destination.stat().st_size,
        "sha256": digest(destination),
        "minimum_python": "3.11",
        "platforms": ["macOS", "Linux", "Windows"],
        "client_support": "Supported JSONL session metadata and SQLite indexes with session_id/thread_id/id plus model_provider/provider columns.",
    }
    shutil.copyfile(PROMPT, PUBLIC / "codex-session-migrate-prompt.txt")
    (PUBLIC / "codex-session-migrate-manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(manifest, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
