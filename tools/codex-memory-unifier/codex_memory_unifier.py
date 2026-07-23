#!/usr/bin/env python3
"""Safely merge provider-independent Codex state into one CODEX_HOME."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import tomllib
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable

STATE_DIRS = ("memories", "sessions", "archived_sessions")
SENSITIVE_NAMES = {
    "auth.json",
    "credentials.json",
    ".env",
    "id_rsa",
    "id_ed25519",
}
SENSITIVE_PARTS = ("oauth", "token", "api_key", "apikey", "credential", "secret")
PLAN_SCHEMA = 1
SHA256_PATTERN = re.compile(r"^[a-f0-9]{64}$")


class UnifierError(RuntimeError):
    pass


@dataclass(frozen=True)
class StateFile:
    source: str
    source_label: str
    state_dir: str
    relative_path: str
    identity: str
    sha256: str
    size: int
    mtime_ns: int


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def canonical(path: Path) -> Path:
    return path.expanduser().resolve(strict=False)


def is_sensitive(relative: Path) -> bool:
    lowered = [part.lower() for part in relative.parts]
    if relative.name.lower() in SENSITIVE_NAMES:
        return True
    return any(marker in part for part in lowered for marker in SENSITIVE_PARTS)


def safe_files(root: Path) -> Iterable[Path]:
    if not root.exists():
        return []
    if root.is_symlink() or not root.is_dir():
        raise UnifierError(f"state path must be a real directory: {root}")
    files: list[Path] = []
    for current, directories, names in os.walk(root, followlinks=False):
        current_path = Path(current)
        for name in list(directories):
            candidate = current_path / name
            if candidate.is_symlink():
                raise UnifierError(f"symbolic link is not allowed: {candidate}")
        for name in names:
            candidate = current_path / name
            relative = candidate.relative_to(root)
            if is_sensitive(relative):
                raise UnifierError(f"credential-like file found in state directory: {relative}")
            info = candidate.lstat()
            if stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode):
                raise UnifierError(f"only regular files are allowed: {candidate}")
            files.append(candidate)
    return files


def nested_session_id(value: Any) -> str | None:
    if not isinstance(value, dict):
        return None
    if value.get("type") == "session_meta":
        payload = value.get("payload")
        if isinstance(payload, dict) and isinstance(payload.get("id"), str):
            return payload["id"]
    for key in ("session_id", "thread_id", "task_id"):
        if isinstance(value.get(key), str) and value[key]:
            return value[key]
    return None


def validate_jsonl(path: Path) -> str | None:
    identity: str | None = None
    with path.open("r", encoding="utf-8") as handle:
        for line_number, raw in enumerate(handle, 1):
            if not raw.strip():
                continue
            try:
                item = json.loads(raw)
            except (json.JSONDecodeError, UnicodeDecodeError) as exc:
                raise UnifierError(f"invalid JSONL at {path}:{line_number}: {exc}") from exc
            identity = identity or nested_session_id(item)
    return identity


def source_label(path: Path, index: int) -> str:
    base = re.sub(r"[^A-Za-z0-9._-]+", "-", path.name).strip("-._") or f"source-{index}"
    return f"{base}-{hashlib.sha256(str(path).encode()).hexdigest()[:8]}"


def inventory(home: Path, index: int) -> list[StateFile]:
    label = source_label(home, index)
    result: list[StateFile] = []
    for state_dir in STATE_DIRS:
        root = home / state_dir
        for path in safe_files(root):
            relative = path.relative_to(root)
            digest = sha256_file(path)
            identity = relative.as_posix()
            if state_dir in ("sessions", "archived_sessions") and path.suffix.lower() == ".jsonl":
                identity = validate_jsonl(path) or identity
            info = path.stat()
            result.append(StateFile(
                source=str(home),
                source_label=label,
                state_dir=state_dir,
                relative_path=relative.as_posix(),
                identity=identity,
                sha256=digest,
                size=info.st_size,
                mtime_ns=info.st_mtime_ns,
            ))
    return result


def validate_config(home: Path) -> str | None:
    config = home / "config.toml"
    if not config.exists():
        return None
    if config.is_symlink() or not config.is_file():
        raise UnifierError(f"config.toml must be a regular file: {config}")
    with config.open("rb") as handle:
        try:
            tomllib.load(handle)
        except tomllib.TOMLDecodeError as exc:
            raise UnifierError(f"invalid config.toml: {exc}") from exc
    return sha256_file(config)


def conflict_name(relative: Path, label: str, digest: str) -> Path:
    suffix = "".join(relative.suffixes)
    stem = relative.name[:-len(suffix)] if suffix else relative.name
    name = f"{stem}__from_{label}_{digest[:8]}{suffix}"
    return relative.with_name(name)


def safe_relative(value: Any, field: str) -> Path:
    if not isinstance(value, str) or not value:
        raise UnifierError(f"{field} must be a non-empty relative path")
    path = Path(value)
    if path.is_absolute() or any(part in ("", ".", "..") for part in path.parts):
        raise UnifierError(f"{field} must stay inside its state directory: {value}")
    return path


def validate_plan(plan: Any) -> dict[str, Any]:
    if not isinstance(plan, dict) or plan.get("schema_version") != PLAN_SCHEMA:
        raise UnifierError("unsupported plan schema")
    sources_value = plan.get("sources")
    operations = plan.get("operations")
    summary = plan.get("summary")
    if not isinstance(sources_value, list) or not sources_value or not all(isinstance(item, str) for item in sources_value):
        raise UnifierError("plan sources must be a non-empty list")
    sources = {str(canonical(Path(item))) for item in sources_value}
    if sources != set(sources_value):
        raise UnifierError("plan sources must use canonical absolute paths")
    target_value = plan.get("target")
    if not isinstance(target_value, str) or str(canonical(Path(target_value))) != target_value:
        raise UnifierError("plan target must use a canonical absolute path")
    config_hash = plan.get("target_config_sha256")
    if config_hash is not None and (not isinstance(config_hash, str) or not SHA256_PATTERN.fullmatch(config_hash)):
        raise UnifierError("plan target config hash is invalid")
    if not isinstance(operations, list) or not isinstance(summary, dict):
        raise UnifierError("plan operations or summary are invalid")
    destinations: set[tuple[str, str]] = set()
    total_bytes = 0
    for operation in operations:
        if not isinstance(operation, dict) or operation.get("state_dir") not in STATE_DIRS:
            raise UnifierError("plan operation state directory is invalid")
        state_dir = operation["state_dir"]
        destination = safe_relative(operation.get("destination"), "operation destination")
        destination_key = (state_dir, destination.as_posix())
        if destination_key in destinations:
            raise UnifierError(f"duplicate plan destination: {destination.as_posix()}")
        destinations.add(destination_key)
        digest = operation.get("sha256")
        size = operation.get("size")
        if not isinstance(digest, str) or not SHA256_PATTERN.fullmatch(digest):
            raise UnifierError("plan operation hash is invalid")
        if not isinstance(size, int) or isinstance(size, bool) or size < 0:
            raise UnifierError("plan operation size is invalid")
        if not isinstance(operation.get("identity"), str) or not operation["identity"]:
            raise UnifierError("plan operation identity is invalid")
        source_specs = operation.get("sources")
        if not isinstance(source_specs, list) or not source_specs:
            raise UnifierError("plan operation must have at least one source")
        for source_spec in source_specs:
            if not isinstance(source_spec, dict) or source_spec.get("home") not in sources:
                raise UnifierError("plan operation references an undeclared source")
            safe_relative(source_spec.get("path"), "operation source path")
        total_bytes += size
    expected_summary = {
        "source_files": sum(len(item["sources"]) for item in operations),
        "output_files": len(operations),
        "exact_duplicates": sum(len(item["sources"]) - 1 for item in operations),
        "bytes": total_bytes,
    }
    for key, expected in expected_summary.items():
        if summary.get(key) != expected:
            raise UnifierError(f"plan summary {key} does not match operations")
    conflicts = summary.get("conflicting_identities")
    if not isinstance(conflicts, int) or isinstance(conflicts, bool) or conflicts < 0:
        raise UnifierError("plan conflict count is invalid")
    return plan


def build_plan(sources: list[Path], target: Path) -> dict[str, Any]:
    if len(sources) < 1:
        raise UnifierError("at least one source CODEX_HOME is required")
    sources = [canonical(source) for source in sources]
    target = canonical(target)
    target_config_hash = validate_config(target)
    entries: list[StateFile] = []
    for index, source in enumerate(sources, 1):
        entries.extend(inventory(source, index))

    operations: list[dict[str, Any]] = []
    exact: dict[tuple[str, str, str], dict[str, Any]] = {}
    destinations: dict[tuple[str, str], str] = {}
    for entry in sorted(entries, key=lambda item: (item.state_dir, item.identity, item.source, item.relative_path)):
        exact_key = (entry.state_dir, entry.identity, entry.sha256)
        if exact_key in exact:
            exact[exact_key]["sources"].append({"home": entry.source, "path": entry.relative_path})
            continue
        relative = Path(entry.relative_path)
        destination = relative
        destination_key = (entry.state_dir, destination.as_posix())
        if destination_key in destinations and destinations[destination_key] != entry.sha256:
            destination = conflict_name(relative, entry.source_label, entry.sha256)
            destination_key = (entry.state_dir, destination.as_posix())
        while destination_key in destinations and destinations[destination_key] != entry.sha256:
            destination = conflict_name(destination, entry.source_label, entry.sha256 + uuid.uuid4().hex)
            destination_key = (entry.state_dir, destination.as_posix())
        destinations[destination_key] = entry.sha256
        operation = {
            "state_dir": entry.state_dir,
            "identity": entry.identity,
            "sha256": entry.sha256,
            "size": entry.size,
            "destination": destination.as_posix(),
            "sources": [{"home": entry.source, "path": entry.relative_path}],
        }
        operations.append(operation)
        exact[exact_key] = operation

    conflicts: dict[tuple[str, str], set[str]] = {}
    for operation in operations:
        key = (operation["state_dir"], operation["identity"])
        conflicts.setdefault(key, set()).add(operation["sha256"])
    conflict_count = sum(1 for values in conflicts.values() if len(values) > 1)
    return {
        "schema_version": PLAN_SCHEMA,
        "created_at": int(time.time()),
        "sources": [str(path) for path in sources],
        "target": str(target),
        "target_config_sha256": target_config_hash,
        "operations": operations,
        "summary": {
            "source_files": len(entries),
            "output_files": len(operations),
            "exact_duplicates": len(entries) - len(operations),
            "conflicting_identities": conflict_count,
            "bytes": sum(item["size"] for item in operations),
        },
    }


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(temporary_name, path)
    finally:
        Path(temporary_name).unlink(missing_ok=True)


def load_plan(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        plan = json.load(handle)
    return validate_plan(plan)


def copy_verified(source: Path, destination: Path, expected: str) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = destination.with_name(f".{destination.name}.{uuid.uuid4().hex}.tmp")
    shutil.copy2(source, temporary)
    if sha256_file(temporary) != expected:
        temporary.unlink(missing_ok=True)
        raise UnifierError(f"hash mismatch while copying {source}")
    os.replace(temporary, destination)


def backup_state(target: Path, reason: str) -> Path:
    stamp = time.strftime("%Y%m%d-%H%M%S") + f"-{uuid.uuid4().hex[:8]}"
    backup = target.parent / f".{target.name}-memory-backups" / stamp
    backup.mkdir(parents=True, exist_ok=False)
    try:
        copied: dict[str, str] = {}
        for state_dir in STATE_DIRS:
            source = target / state_dir
            if source.exists():
                list(safe_files(source))
                shutil.copytree(source, backup / state_dir, symlinks=False)
                for path in safe_files(backup / state_dir):
                    copied[path.relative_to(backup).as_posix()] = sha256_file(path)
        write_json_atomic(backup / "backup-manifest.json", {
            "schema_version": 1,
            "created_at": int(time.time()),
            "reason": reason,
            "target": str(target),
            "target_config_sha256": validate_config(target),
            "files": copied,
        })
    except Exception:
        shutil.rmtree(backup, ignore_errors=True)
        raise
    return backup


def swap_state_directories(
    target: Path,
    stages: dict[str, Path],
    replace: Callable[[Path, Path], None] = os.replace,
) -> dict[str, Path | None]:
    rollbacks: dict[str, Path | None] = {}
    try:
        for state_dir in STATE_DIRS:
            destination = target / state_dir
            rollback = target.parent / f".{target.name}-{state_dir}-rollback-{uuid.uuid4().hex}"
            if destination.exists():
                replace(destination, rollback)
                rollbacks[state_dir] = rollback
            else:
                rollbacks[state_dir] = None
            replace(stages[state_dir], destination)
    except Exception:
        rollback_state_directories(target, rollbacks, replace)
        raise
    return rollbacks


def rollback_state_directories(
    target: Path,
    rollbacks: dict[str, Path | None],
    replace: Callable[[Path, Path], None] = os.replace,
) -> None:
    for state_dir in reversed(STATE_DIRS):
        if state_dir not in rollbacks:
            continue
        destination = target / state_dir
        if destination.exists():
            shutil.rmtree(destination)
        rollback = rollbacks[state_dir]
        if rollback is not None and rollback.exists():
            replace(rollback, destination)


def discard_rollbacks(rollbacks: dict[str, Path | None]) -> None:
    for rollback in rollbacks.values():
        if rollback is not None and rollback.exists():
            shutil.rmtree(rollback)


def apply_plan(
    plan: dict[str, Any],
    confirm: bool,
    no_active: bool,
    replace: Callable[[Path, Path], None] = os.replace,
) -> dict[str, Any]:
    if not confirm or not no_active:
        raise UnifierError("merge requires --confirm and --confirm-no-active-requests")
    plan = validate_plan(plan)
    target = canonical(Path(plan["target"]))
    target.mkdir(parents=True, exist_ok=True)
    before_config = validate_config(target)
    if before_config != plan.get("target_config_sha256"):
        raise UnifierError("target config.toml changed after plan creation; create a new plan")
    existing_bytes = sum(path.stat().st_size for state_dir in STATE_DIRS for path in safe_files(target / state_dir))
    free = shutil.disk_usage(target).free
    required = existing_bytes * 2 + plan["summary"]["bytes"] + 16 * 1024 * 1024
    if free < required:
        raise UnifierError(f"insufficient free space: need at least {required} bytes")

    backup = backup_state(target, "pre-merge")
    stages: dict[str, Path] = {}
    rollbacks: dict[str, Path | None] = {}
    try:
        for state_dir in STATE_DIRS:
            stage = target.parent / f".{target.name}-{state_dir}-merge-{uuid.uuid4().hex}"
            existing = target / state_dir
            if existing.exists():
                shutil.copytree(existing, stage, symlinks=False)
            else:
                stage.mkdir(parents=True)
            stages[state_dir] = stage
        provenance: list[dict[str, Any]] = []
        for operation in plan["operations"]:
            state_dir = operation["state_dir"]
            destination = stages[state_dir] / safe_relative(operation["destination"], "operation destination")
            existing_digest = sha256_file(destination) if destination.exists() else None
            if existing_digest not in (None, operation["sha256"]):
                destination = stages[state_dir] / conflict_name(
                    Path(operation["destination"]), "target", operation["sha256"]
                )
            source_spec = operation["sources"][0]
            source_root = canonical(Path(source_spec["home"])) / state_dir
            raw_source = source_root / safe_relative(
                source_spec["path"], "operation source path"
            )
            source = raw_source.resolve(strict=False)
            if raw_source.is_symlink() or not source.is_relative_to(source_root) or not source.is_file():
                raise UnifierError(f"planned source is no longer a safe regular file: {raw_source}")
            if sha256_file(source) != operation["sha256"]:
                raise UnifierError(f"planned source changed: {source}")
            if not destination.exists():
                copy_verified(source, destination, operation["sha256"])
            provenance.append({
                "state_dir": state_dir,
                "destination": destination.relative_to(stages[state_dir]).as_posix(),
                "identity": operation["identity"],
                "sha256": operation["sha256"],
                "sources": operation["sources"],
            })

        try:
            rollbacks = swap_state_directories(target, stages, replace)
            metadata = target / ".codex-memory-unifier"
            write_json_atomic(metadata / "provenance.json", {
                "schema_version": 1,
                "merged_at": int(time.time()),
                "backup": str(backup),
                "entries": provenance,
            })
            if validate_config(target) != before_config:
                raise UnifierError("config.toml changed during merge")
        except Exception:
            rollback_state_directories(target, rollbacks, replace)
            raise
        discard_rollbacks(rollbacks)
        return {"status": "merged", "target": str(target), "backup": str(backup), **plan["summary"]}
    finally:
        for stage in stages.values():
            if stage.exists():
                shutil.rmtree(stage)


def restore_backup(
    backup: Path,
    target: Path,
    confirm: bool,
    no_active: bool,
    replace: Callable[[Path, Path], None] = os.replace,
) -> dict[str, Any]:
    if not confirm or not no_active:
        raise UnifierError("restore requires --confirm and --confirm-no-active-requests")
    backup = canonical(backup)
    target = canonical(target)
    manifest_path = backup / "backup-manifest.json"
    with manifest_path.open("r", encoding="utf-8") as handle:
        manifest = json.load(handle)
    if canonical(Path(manifest["target"])) != target:
        raise UnifierError("backup target does not match requested target")
    files = manifest.get("files")
    if not isinstance(files, dict):
        raise UnifierError("backup manifest files are invalid")
    actual_files: dict[str, str] = {}
    for state_dir in STATE_DIRS:
        for path in safe_files(backup / state_dir):
            actual_files[path.relative_to(backup).as_posix()] = sha256_file(path)
    if set(actual_files) != set(files):
        raise UnifierError("backup manifest does not exactly match backup state files")
    for relative, digest in files.items():
        path = backup / safe_relative(relative, "backup manifest path")
        if not isinstance(digest, str) or not SHA256_PATTERN.fullmatch(digest):
            raise UnifierError(f"backup digest is invalid: {relative}")
        if not path.is_file() or sha256_file(path) != digest:
            raise UnifierError(f"backup verification failed: {relative}")
    before_config = validate_config(target)
    safety = backup_state(target, "pre-restore")
    stages: dict[str, Path] = {}
    rollbacks: dict[str, Path | None] = {}
    try:
        for state_dir in STATE_DIRS:
            stage = target.parent / f".{target.name}-{state_dir}-restore-{uuid.uuid4().hex}"
            source = backup / state_dir
            if source.exists():
                shutil.copytree(source, stage, symlinks=False)
            else:
                stage.mkdir(parents=True)
            stages[state_dir] = stage
        try:
            rollbacks = swap_state_directories(target, stages, replace)
            if validate_config(target) != before_config:
                raise UnifierError("config.toml changed during restore")
        except Exception:
            rollback_state_directories(target, rollbacks, replace)
            raise
        discard_rollbacks(rollbacks)
    finally:
        for stage in stages.values():
            if stage.exists():
                shutil.rmtree(stage)
    return {"status": "restored", "target": str(target), "from": str(backup), "safety_backup": str(safety)}


def activate_home(target: Path, confirm: bool, runner=subprocess.run) -> dict[str, Any]:
    if not confirm:
        raise UnifierError("activation requires --confirm")
    system = platform.system().lower()
    config_dir = Path.home() / ".config" / "codex-memory-unifier"
    config_dir.mkdir(parents=True, exist_ok=True)
    shell_file = config_dir / "env.sh"
    powershell_file = config_dir / "env.ps1"
    shell_file.write_text(f"export CODEX_HOME={json.dumps(str(target))}\n", encoding="utf-8")
    powershell_file.write_text(f"$env:CODEX_HOME = {json.dumps(str(target))}\n", encoding="utf-8")
    action = "files-only"
    if system == "darwin":
        runner(["launchctl", "setenv", "CODEX_HOME", str(target)], check=True)
        action = "launchctl"
    elif system == "windows":
        runner(["setx", "CODEX_HOME", str(target)], check=True)
        action = "setx"
    elif system == "linux":
        env_dir = Path.home() / ".config" / "environment.d"
        env_dir.mkdir(parents=True, exist_ok=True)
        (env_dir / "90-codex-home.conf").write_text(f"CODEX_HOME={target}\n", encoding="utf-8")
        action = "environment.d"
    else:
        raise UnifierError(f"unsupported platform: {platform.system()}")
    return {
        "status": "activated",
        "target": str(target),
        "method": action,
        "restart_required": True,
        "shell_env": str(shell_file),
        "powershell_env": str(powershell_file),
    }


def print_result(value: Any) -> None:
    print(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True))


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(prog="codex-memory", description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)
    plan = commands.add_parser("plan", help="audit sources and write a merge preview")
    plan.add_argument("--source", action="append", required=True, type=Path)
    plan.add_argument("--target", required=True, type=Path)
    plan.add_argument("--output", required=True, type=Path)
    merge = commands.add_parser("merge", help="apply a verified merge plan")
    merge.add_argument("--plan", required=True, type=Path)
    merge.add_argument("--confirm", action="store_true")
    merge.add_argument("--confirm-no-active-requests", action="store_true")
    merge.add_argument("--activate", action="store_true")
    restore = commands.add_parser("restore", help="restore a pre-merge backup")
    restore.add_argument("--backup", required=True, type=Path)
    restore.add_argument("--target", required=True, type=Path)
    restore.add_argument("--confirm", action="store_true")
    restore.add_argument("--confirm-no-active-requests", action="store_true")
    activate = commands.add_parser("activate", help="set the user CODEX_HOME for the current platform")
    activate.add_argument("--target", required=True, type=Path)
    activate.add_argument("--confirm", action="store_true")
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        if args.command == "plan":
            sources = [canonical(path) for path in args.source]
            target = canonical(args.target)
            plan = build_plan(sources, target)
            write_json_atomic(canonical(args.output), plan)
            print_result({"status": "planned", "plan": str(canonical(args.output)), **plan["summary"]})
        elif args.command == "merge":
            plan = load_plan(canonical(args.plan))
            result = apply_plan(plan, args.confirm, args.confirm_no_active_requests)
            if args.activate:
                try:
                    result["activation"] = activate_home(canonical(Path(plan["target"])), True)
                except (OSError, ValueError, UnifierError) as exc:
                    result["status"] = "merged_activation_failed"
                    result["activation_error"] = str(exc)
                    print_result(result)
                    return 3
            print_result(result)
        elif args.command == "restore":
            print_result(restore_backup(canonical(args.backup), canonical(args.target), args.confirm, args.confirm_no_active_requests))
        elif args.command == "activate":
            print_result(activate_home(canonical(args.target), args.confirm))
    except (OSError, ValueError, UnifierError) as exc:
        print(json.dumps({"status": "error", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
