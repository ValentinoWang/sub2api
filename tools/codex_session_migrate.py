#!/usr/bin/env python3
"""Offline, backup-gated repair for supported local Codex session provider references."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import uuid
from pathlib import Path
from typing import Any

from codex_migration_inspect import MigrationError, inspect, resolve_codex_home, sha256_file, utc_now, write_json
from codex_migration_transaction import BackupError, DriftError, apply_plan, recover, rollback

EXIT_OK = 0
EXIT_ARGUMENT = 2
EXIT_BLOCKED = 3
EXIT_DRIFT = 4
EXIT_BACKUP = 5
EXIT_RECOVERY = 6
EXIT_VERIFY = 7


def plan_for(args: argparse.Namespace) -> dict[str, Any]:
    codex_home = resolve_codex_home(args.codex_home)
    result = inspect(codex_home, args.target_provider, args.source_provider)
    if not args.target_provider:
        raise MigrationError("plan requires --target-provider; inspect is the read-only default")
    result.update(
        {
            "plan_id": str(uuid.uuid4()),
            "target_provider": args.target_provider,
            "source_provider": args.source_provider,
            "selection": {
                "all_history": bool(args.all_history),
                "session_ids": sorted(set(args.session_id or [])),
            },
        }
    )
    selected_ids = set(args.session_id or [])
    if selected_ids:
        result["sessions"] = [record for record in result["sessions"] if record["session_id"] in selected_ids]
        result["indexes"] = [record for record in result["indexes"] if record["session_id"] in selected_ids]
    elif not args.all_history:
        # A broad destructive default is unsafe. The caller must explicitly opt into all history
        # or name the sessions it reviewed.
        raise MigrationError("plan requires --session-id or --all-history after reviewing the inspection output")
    result["selected_session_count"] = len(result["sessions"])
    result["selected_index_count"] = len(result["indexes"])
    if not result["sessions"]:
        result["status"] = "NO_CHANGES"
    elif result["diagnostics"]:
        result["status"] = "BLOCKED"
    else:
        result["status"] = "READY"
    encoded = json.dumps(result, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    result["plan_sha256"] = hashlib.sha256(encoded).hexdigest()
    return result


def load_plan(path: Path) -> dict[str, Any]:
    try:
        plan = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise MigrationError(f"cannot load migration plan: {error}") from error
    supplied = plan.pop("plan_sha256", None)
    encoded = json.dumps(plan, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    plan["plan_sha256"] = supplied
    if not isinstance(supplied, str) or hashlib.sha256(encoded).hexdigest() != supplied:
        raise DriftError("migration plan digest does not match its content")
    return plan


def command_inspect(args: argparse.Namespace) -> dict[str, Any]:
    return inspect(resolve_codex_home(args.codex_home), args.target_provider, args.source_provider)


def command_plan(args: argparse.Namespace) -> dict[str, Any]:
    plan = plan_for(args)
    output = Path(args.output).resolve()
    if output.exists():
        raise MigrationError(f"refusing to overwrite an existing plan: {output}")
    write_json(output, plan)
    return {"status": plan["status"], "plan": str(output), "plan_sha256": plan["plan_sha256"], "selected_session_count": plan["selected_session_count"]}


def command_apply(args: argparse.Namespace) -> dict[str, Any]:
    if not args.confirm_quiescent:
        raise MigrationError("apply requires --confirm-quiescent after every Codex client using this data root is closed")
    plan = load_plan(Path(args.plan).resolve())
    if args.confirm_plan_sha256 != plan["plan_sha256"]:
        raise MigrationError("apply requires the exact --confirm-plan-sha256 shown by plan")
    if plan.get("status") == "NO_CHANGES":
        return {"status": "NO_CHANGES", "plan_sha256": plan["plan_sha256"]}
    if plan.get("status") != "READY":
        raise MigrationError(f"plan is not writable: {plan.get('status')}")
    backup_root = Path(args.backup_dir).resolve()
    journal = apply_plan(plan, backup_root)
    return {"status": journal["status"], "backup": str(backup_root), "plan_sha256": plan["plan_sha256"]}


def command_verify(args: argparse.Namespace) -> dict[str, Any]:
    plan = load_plan(Path(args.plan).resolve())
    expected = plan["target_provider"]
    result = inspect(resolve_codex_home(plan["codex_home"]), expected, None)
    remaining = [record for record in result["sessions"] if record["session_id"] in {item["session_id"] for item in plan["sessions"]} and record["provider"] != expected]
    if remaining:
        raise DriftError(f"verification found {len(remaining)} selected sessions without the target provider")
    return {"status": "VERIFIED", "plan_sha256": plan["plan_sha256"], "client_continuation": "NOT_TESTED"}


def command_recover(args: argparse.Namespace) -> dict[str, Any]:
    return recover(Path(args.backup_dir).resolve())


def command_rollback(args: argparse.Namespace) -> dict[str, Any]:
    return rollback(Path(args.backup_dir).resolve())


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    root.add_argument("--codex-home", help="explicit local Codex data root; defaults to CODEX_HOME or ~/.codex")
    subcommands = root.add_subparsers(dest="command")

    for name in ("inspect", "plan"):
        current = subcommands.add_parser(name)
        current.add_argument("--target-provider")
        current.add_argument("--source-provider")
        if name == "plan":
            current.add_argument("--output", required=True)
            current.add_argument("--session-id", action="append")
            current.add_argument("--all-history", action="store_true")

    apply = subcommands.add_parser("apply")
    apply.add_argument("--plan", required=True)
    apply.add_argument("--backup-dir", required=True)
    apply.add_argument("--confirm-plan-sha256", required=True)
    apply.add_argument("--confirm-quiescent", action="store_true")

    verify = subcommands.add_parser("verify")
    verify.add_argument("--plan", required=True)

    for name in ("recover", "rollback"):
        current = subcommands.add_parser(name)
        current.add_argument("--backup-dir", required=True)
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    if args.command is None:
        args.command = "inspect"
        args.target_provider = None
        args.source_provider = None
    handlers = {
        "inspect": command_inspect,
        "plan": command_plan,
        "apply": command_apply,
        "verify": command_verify,
        "recover": command_recover,
        "rollback": command_rollback,
    }
    try:
        result = handlers[args.command](args)
        print(json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True))
        return EXIT_OK
    except BackupError as error:
        print(json.dumps({"status": "BACKUP_FAILED", "error": str(error)}, ensure_ascii=False), file=sys.stderr)
        return EXIT_BACKUP
    except DriftError as error:
        print(json.dumps({"status": "DRIFT_OR_CONFLICT", "error": str(error)}, ensure_ascii=False), file=sys.stderr)
        return EXIT_DRIFT if args.command != "recover" else EXIT_RECOVERY
    except MigrationError as error:
        print(json.dumps({"status": "BLOCKED", "error": str(error)}, ensure_ascii=False), file=sys.stderr)
        return EXIT_BLOCKED


if __name__ == "__main__":
    raise SystemExit(main())
