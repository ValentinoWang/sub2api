#!/usr/bin/env python3
"""Backup-gated provider migration transactions for supported Codex records."""

from __future__ import annotations

import json
import os
import shutil
import sqlite3
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from codex_migration_inspect import MigrationError, SESSION_META_TYPE, sha256_file, utc_now, write_json


class BackupError(MigrationError):
    """A migration must stop before any original data changes."""


class DriftError(MigrationError):
    """The selected input changed after planning or during recovery."""


@dataclass(frozen=True)
class BackupResult:
    root: Path
    manifest: dict[str, Any]


def _atomic_write(path: Path, data: bytes) -> None:
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=str(path.parent))
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except Exception:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def _selected_file_entries(plan: dict[str, Any]) -> dict[Path, list[dict[str, Any]]]:
    grouped: dict[Path, list[dict[str, Any]]] = {}
    for session in plan["sessions"]:
        grouped.setdefault(Path(session["path"]), []).append(session)
    return grouped


def _selected_index_entries(plan: dict[str, Any]) -> dict[Path, list[dict[str, Any]]]:
    grouped: dict[Path, list[dict[str, Any]]] = {}
    for index in plan["indexes"]:
        grouped.setdefault(Path(index["database"]), []).append(index)
    return grouped


def _validate_plan_shape(plan: dict[str, Any]) -> None:
    if plan.get("schema_version") != 1 or not isinstance(plan.get("target_provider"), str):
        raise MigrationError("unsupported or malformed migration plan")
    if not isinstance(plan.get("sessions"), list) or not isinstance(plan.get("indexes"), list):
        raise MigrationError("migration plan must contain sessions and indexes")


def verify_plan_inputs(plan: dict[str, Any]) -> None:
    _validate_plan_shape(plan)
    for path, entries in _selected_file_entries(plan).items():
        if not path.is_file() or sha256_file(path) != entries[0]["file_sha256"]:
            raise DriftError(f"session file changed since planning: {path}")
    for path, entries in _selected_index_entries(plan).items():
        if not path.is_file() or sha256_file(path) != entries[0]["database_sha256"]:
            raise DriftError(f"SQLite index changed since planning: {path}")


def create_verified_backup(plan: dict[str, Any], backup_root: Path) -> BackupResult:
    """Create and verify every backup before callers are allowed to write originals."""
    _validate_plan_shape(plan)
    root = backup_root.resolve()
    if root.exists():
        raise BackupError(f"backup destination already exists: {root}")
    try:
        root.mkdir(parents=True, mode=0o700)
        if os.name != "nt":
            os.chmod(root, 0o700)
        manifest: dict[str, Any] = {"schema_version": 1, "created_at": utc_now(), "files": [], "databases": []}
        for index, (source, entries) in enumerate(_selected_file_entries(plan).items()):
            target = root / "files" / f"{index}-{source.name}"
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, target)
            if os.name != "nt":
                os.chmod(target, 0o600)
            source_sha = sha256_file(source)
            if sha256_file(target) != source_sha:
                raise BackupError(f"backup readback mismatch: {source}")
            manifest["files"].append({"source": str(source), "backup": str(target), "sha256": source_sha, "sessions": entries})
        for index, (source, entries) in enumerate(_selected_index_entries(plan).items()):
            target = root / "databases" / f"{index}-{source.name}"
            target.parent.mkdir(parents=True, exist_ok=True)
            # sqlite backup API captures a consistent view without mutating the source database.
            read = sqlite3.connect(f"file:{source}?mode=ro", uri=True)
            write = sqlite3.connect(target)
            try:
                read.backup(write)
                write.execute("PRAGMA integrity_check").fetchone()
            finally:
                write.close()
                read.close()
            if os.name != "nt":
                os.chmod(target, 0o600)
            manifest["databases"].append({"source": str(source), "backup": str(target), "sha256": sha256_file(source), "indexes": entries})
        write_json(root / "backup-manifest.json", manifest)
        return BackupResult(root, manifest)
    except Exception as error:
        shutil.rmtree(root, ignore_errors=True)
        if isinstance(error, BackupError):
            raise
        raise BackupError(f"unable to create verified backup: {error}") from error


def _replace_session_lines(path: Path, entries: list[dict[str, Any]], target_provider: str) -> str:
    before = path.read_bytes()
    if sha256_file(path) != entries[0]["file_sha256"]:
        raise DriftError(f"session file changed before replacement: {path}")
    lines = before.splitlines(keepends=True)
    expected = {(entry["session_id"], int(entry["line_number"])) for entry in entries}
    replaced: set[tuple[str, int]] = set()
    output: list[bytes] = []
    for line_number, raw in enumerate(lines, start=1):
        try:
            item = json.loads(raw)
        except json.JSONDecodeError as error:
            raise DriftError(f"session line is no longer valid JSON: {path}:{line_number}") from error
        if isinstance(item, dict) and item.get("type") == SESSION_META_TYPE and isinstance(item.get("payload"), dict):
            payload = item["payload"]
            session_id = payload.get("id") or payload.get("session_id")
            key = (session_id, line_number)
            if key in expected:
                if payload.get("model_provider") != entries[0]["provider"]:
                    raise DriftError(f"session provider changed before replacement: {path}:{line_number}")
                payload["model_provider"] = target_provider
                ending = b"\r\n" if raw.endswith(b"\r\n") else b"\n" if raw.endswith(b"\n") else b""
                output.append(json.dumps(item, ensure_ascii=False, separators=(",", ":")).encode("utf-8") + ending)
                replaced.add(key)
                continue
        output.append(raw)
    if replaced != expected:
        raise DriftError(f"not all planned session metadata records were found in {path}")
    _atomic_write(path, b"".join(output))
    return sha256_file(path)


def _update_index(path: Path, entries: list[dict[str, Any]], target_provider: str) -> str:
    if sha256_file(path) != entries[0]["database_sha256"]:
        raise DriftError(f"SQLite index changed before update: {path}")
    connection = sqlite3.connect(path)
    try:
        connection.execute("BEGIN IMMEDIATE")
        for entry in entries:
            table = str(entry["table"]).replace('"', '""')
            provider_column = str(entry["provider_column"]).replace('"', '""')
            session_column = str(entry["session_column"]).replace('"', '""')
            cursor = connection.execute(
                f'UPDATE "{table}" SET "{provider_column}" = ? WHERE rowid = ? '
                f'AND "{session_column}" = ? AND "{provider_column}" = ?',
                (target_provider, int(entry["rowid"]), entry["session_id"], entry["provider"]),
            )
            if cursor.rowcount != 1:
                raise DriftError(f"planned index row no longer matches in {path}:{table}:{entry['rowid']}")
        connection.commit()
    except Exception:
        connection.rollback()
        raise
    finally:
        connection.close()
    return sha256_file(path)


def apply_plan(plan: dict[str, Any], backup_root: Path) -> dict[str, Any]:
    """Apply one plan after a complete verified backup, with a durable journal."""
    verify_plan_inputs(plan)
    backup = create_verified_backup(plan, backup_root)
    journal_path = backup.root / "journal.json"
    journal: dict[str, Any] = {"schema_version": 1, "status": "BACKUP_VERIFIED", "plan": plan, "files": [], "databases": []}
    write_json(journal_path, journal)
    try:
        for path, entries in _selected_file_entries(plan).items():
            journal["files"].append({"path": str(path), "before": entries[0]["file_sha256"], "after": _replace_session_lines(path, entries, plan["target_provider"])})
            journal["status"] = "FILES_APPLIED"
            write_json(journal_path, journal)
        for path, entries in _selected_index_entries(plan).items():
            journal["databases"].append({"path": str(path), "before": entries[0]["database_sha256"], "after": _update_index(path, entries, plan["target_provider"])})
            journal["status"] = "INDEX_COMMITTED"
            write_json(journal_path, journal)
        journal["status"] = "VERIFIED"
        journal["verified_at"] = utc_now()
        write_json(journal_path, journal)
        return journal
    except Exception:
        journal["status"] = "RECOVERY_REQUIRED"
        journal["failed_at"] = utc_now()
        write_json(journal_path, journal)
        raise


def recover(backup_root: Path) -> dict[str, Any]:
    journal_path = backup_root / "journal.json"
    if not journal_path.is_file():
        raise MigrationError(f"missing journal: {journal_path}")
    journal = json.loads(journal_path.read_text(encoding="utf-8"))
    if journal.get("status") == "VERIFIED":
        return journal
    # Recovery never guesses how to merge an interrupted cross-file transaction.
    raise DriftError("migration requires rollback or manual review; journal preserves the completed steps")


def rollback(backup_root: Path) -> dict[str, Any]:
    journal_path = backup_root / "journal.json"
    manifest_path = backup_root / "backup-manifest.json"
    if not journal_path.is_file() or not manifest_path.is_file():
        raise MigrationError("rollback requires the verified backup manifest and journal")
    journal = json.loads(journal_path.read_text(encoding="utf-8"))
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if journal.get("status") not in {"VERIFIED", "RECOVERY_REQUIRED"}:
        raise MigrationError(f"rollback is not valid for journal state {journal.get('status')!r}")
    for file_entry in manifest.get("files", []):
        source = Path(file_entry["source"])
        journal_entry = next((entry for entry in journal.get("files", []) if entry["path"] == str(source)), None)
        if journal_entry is None:
            continue
        if sha256_file(source) != journal_entry["after"]:
            raise DriftError(f"refusing to overwrite changed session file during rollback: {source}")
        _atomic_write(source, Path(file_entry["backup"]).read_bytes())
    for database_entry in manifest.get("databases", []):
        source = Path(database_entry["source"])
        journal_entry = next((entry for entry in journal.get("databases", []) if entry["path"] == str(source)), None)
        if journal_entry is None:
            continue
        if sha256_file(source) != journal_entry["after"]:
            raise DriftError(f"refusing to overwrite changed SQLite index during rollback: {source}")
        # Restore only declared rows from the verified snapshot, never the entire database.
        source_db = sqlite3.connect(source)
        backup_db = sqlite3.connect(f"file:{database_entry['backup']}?mode=ro", uri=True)
        try:
            source_db.execute("BEGIN IMMEDIATE")
            for index in database_entry["indexes"]:
                table = str(index["table"]).replace('"', '""')
                provider_column = str(index["provider_column"]).replace('"', '""')
                session_column = str(index["session_column"]).replace('"', '""')
                original = backup_db.execute(
                    f'SELECT "{provider_column}" FROM "{table}" WHERE rowid = ? AND "{session_column}" = ?',
                    (int(index["rowid"]), index["session_id"]),
                ).fetchone()
                if original is None:
                    raise DriftError(f"backup row missing for rollback: {source}:{table}:{index['rowid']}")
                current = source_db.execute(
                    f'SELECT "{provider_column}" FROM "{table}" WHERE rowid = ? AND "{session_column}" = ?',
                    (int(index["rowid"]), index["session_id"]),
                ).fetchone()
                if current is None or current[0] != journal["plan"]["target_provider"]:
                    raise DriftError(f"refusing to overwrite changed index row during rollback: {source}:{table}:{index['rowid']}")
                source_db.execute(
                    f'UPDATE "{table}" SET "{provider_column}" = ? WHERE rowid = ?',
                    (original[0], int(index["rowid"])),
                )
            source_db.commit()
        except Exception:
            source_db.rollback()
            raise
        finally:
            backup_db.close()
            source_db.close()
    journal["status"] = "ROLLED_BACK"
    journal["rolled_back_at"] = utc_now()
    write_json(journal_path, journal)
    return journal
