#!/usr/bin/env python3
"""Read-only discovery for supported Codex session provider migrations."""

from __future__ import annotations

import hashlib
import json
import os
import sqlite3
import sys
import tomllib
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

PLAN_SCHEMA_VERSION = 1
TOOL_VERSION = "1.0.0"
SESSION_META_TYPE = "session_meta"
SUPPORTED_INDEX_PROVIDER_COLUMNS = ("model_provider", "provider")
SUPPORTED_INDEX_SESSION_COLUMNS = ("session_id", "thread_id", "id")


class MigrationError(RuntimeError):
    """An actionable error that must not be ignored by a migration caller."""


class UnsupportedData(MigrationError):
    """A local data shape cannot be safely changed by the current adapter."""


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def utc_now() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def resolve_codex_home(explicit: str | None) -> Path:
    if explicit:
        return Path(explicit).expanduser().resolve()
    configured = os.environ.get("CODEX_HOME")
    if configured:
        return Path(configured).expanduser().resolve()
    return (Path.home() / ".codex").resolve()


def locate_config(codex_home: Path) -> Path:
    config = codex_home / "config.toml"
    if not config.is_file():
        raise MigrationError(f"missing Codex config: {config}")
    return config


def load_config(config_path: Path) -> dict[str, Any]:
    try:
        with config_path.open("rb") as handle:
            loaded = tomllib.load(handle)
    except tomllib.TOMLDecodeError as error:
        raise MigrationError(f"invalid TOML in {config_path}: {error}") from error
    if not isinstance(loaded, dict):
        raise MigrationError(f"config root must be a table: {config_path}")
    return loaded


def configured_provider_ids(config: dict[str, Any]) -> set[str]:
    providers = config.get("model_providers", {})
    if not isinstance(providers, dict):
        raise MigrationError("model_providers must be a TOML table when present")
    names = {name for name, definition in providers.items() if isinstance(name, str) and isinstance(definition, dict)}
    # Codex has built-in OpenAI support. It does not require a custom provider table.
    names.add("openai")
    return names


def config_fingerprint(config_path: Path) -> str:
    return sha256_file(config_path)


@dataclass(frozen=True)
class SessionRecord:
    session_id: str
    provider: str
    path: str
    line_number: int
    archived: bool
    file_sha256: str


@dataclass(frozen=True)
class IndexRecord:
    database: str
    table: str
    rowid: int
    session_column: str
    provider_column: str
    session_id: str
    provider: str
    database_sha256: str


def _session_meta_from_line(raw: bytes, path: Path, line_number: int) -> tuple[str, str] | None:
    try:
        item = json.loads(raw)
    except json.JSONDecodeError as error:
        raise UnsupportedData(f"invalid JSONL in {path}:{line_number}: {error.msg}") from error
    if not isinstance(item, dict) or item.get("type") != SESSION_META_TYPE:
        return None
    payload = item.get("payload")
    if not isinstance(payload, dict):
        raise UnsupportedData(f"session metadata payload is not an object in {path}:{line_number}")
    session_id = payload.get("id") or payload.get("session_id")
    provider = payload.get("model_provider")
    if not isinstance(session_id, str) or not session_id:
        raise UnsupportedData(f"session metadata has no stable id in {path}:{line_number}")
    if not isinstance(provider, str) or not provider:
        return None
    return session_id, provider


def discover_session_records(codex_home: Path) -> tuple[list[SessionRecord], list[str]]:
    records: list[SessionRecord] = []
    errors: list[str] = []
    for dirname, archived in (("sessions", False), ("archived_sessions", True)):
        root = codex_home / dirname
        if not root.exists():
            continue
        if not root.is_dir():
            errors.append(f"{root} exists but is not a directory")
            continue
        for path in sorted(root.rglob("*.jsonl")):
            if path.is_symlink():
                errors.append(f"symlinked session file is not supported: {path}")
                continue
            try:
                raw = path.read_bytes()
                file_sha = sha256_bytes(raw)
                for line_number, line in enumerate(raw.splitlines(keepends=True), start=1):
                    found = _session_meta_from_line(line, path, line_number)
                    if found is None:
                        continue
                    session_id, provider = found
                    records.append(SessionRecord(session_id, provider, str(path), line_number, archived, file_sha))
            except (OSError, UnsupportedData) as error:
                errors.append(str(error))
    return records, errors


def discover_sqlite_records(codex_home: Path) -> tuple[list[IndexRecord], list[str]]:
    records: list[IndexRecord] = []
    errors: list[str] = []
    ignored_names = {"backups", "sessions", "archived_sessions"}
    for path in sorted(codex_home.rglob("*")):
        if any(part in ignored_names for part in path.parts):
            continue
        if not path.is_file() or path.is_symlink() or path.suffix.lower() not in {".sqlite", ".db"}:
            continue
        try:
            # mode=ro keeps discovery from creating a journal, WAL, or shared-memory file.
            connection = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
            try:
                tables = connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'"
                ).fetchall()
                digest = sha256_file(path)
                for (table,) in tables:
                    safe_table = str(table).replace('"', '""')
                    columns = [row[1] for row in connection.execute(f'PRAGMA table_info("{safe_table}")')]
                    session_column = next((name for name in SUPPORTED_INDEX_SESSION_COLUMNS if name in columns), None)
                    provider_column = next((name for name in SUPPORTED_INDEX_PROVIDER_COLUMNS if name in columns), None)
                    if not session_column or not provider_column:
                        continue
                    query = (
                        f'SELECT rowid, "{session_column}", "{provider_column}" FROM "{safe_table}" '
                        f'WHERE "{session_column}" IS NOT NULL AND "{provider_column}" IS NOT NULL'
                    )
                    for rowid, session_id, provider in connection.execute(query):
                        if isinstance(session_id, str) and isinstance(provider, str):
                            records.append(IndexRecord(str(path), str(table), int(rowid), session_column, provider_column, session_id, provider, digest))
            finally:
                connection.close()
        except (sqlite3.Error, OSError) as error:
            errors.append(f"cannot inspect SQLite index {path}: {error}")
    return records, errors


def inspect(codex_home: Path, target_provider: str | None = None, source_provider: str | None = None) -> dict[str, Any]:
    config_path = locate_config(codex_home)
    config = load_config(config_path)
    providers = configured_provider_ids(config)
    session_records, session_errors = discover_session_records(codex_home)
    index_records, index_errors = discover_sqlite_records(codex_home)
    if target_provider is not None and target_provider not in providers:
        raise MigrationError(
            f"target provider {target_provider!r} is not declared by the effective configuration; "
            "repair the active config before planning a migration"
        )
    selected_sessions = [record for record in session_records if source_provider is None or record.provider == source_provider]
    if target_provider is not None:
        selected_sessions = [record for record in selected_sessions if record.provider != target_provider]
    selected_ids = {record.session_id for record in selected_sessions}
    selected_indexes = [
        record for record in index_records
        if record.session_id in selected_ids and (source_provider is None or record.provider == source_provider)
    ]
    return {
        "schema_version": PLAN_SCHEMA_VERSION,
        "tool_version": TOOL_VERSION,
        "generated_at": utc_now(),
        "codex_home": str(codex_home),
        "config": {"path": str(config_path), "sha256": config_fingerprint(config_path), "provider_ids": sorted(providers)},
        "sessions": [asdict(record) for record in selected_sessions],
        "indexes": [asdict(record) for record in selected_indexes],
        "all_session_count": len(session_records),
        "selected_session_count": len(selected_sessions),
        "selected_index_count": len(selected_indexes),
        "diagnostics": session_errors + index_errors,
    }


def write_json(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
