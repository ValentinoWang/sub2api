#!/usr/bin/env python3
"""Check/apply two native route insertions against exact reviewed source blobs.

No network calls, database migrations or runtime operations. Stop editors before --apply.
"""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path

RULES = (
    ('backend/internal/server/routes/admin.go',
     'a18d069b4f5b7b3742ba294d121a35f6abe0e826',
     '\t\tregisterDashboardRoutes(admin, h)\n',
     '\t\tregisterDashboardRoutes(admin, h)\n\t\tregisterCostCenterRoutes(admin)\n'),
    ('frontend/src/router/index.ts',
     '53ea004b5ca6de1368c0d792034e851c6edda869',
     "  {\n    path: '/admin/ops',\n",
     "  {\n    path: '/admin/cost-center',\n    name: 'AdminCostCenter',\n"
     "    component: () => import('@/views/admin/CostCenterView.vue'),\n"
     "    meta: { requiresAuth: true, requiresAdmin: true, title: 'Cost Center' }\n"
     "  },\n  {\n    path: '/admin/ops',\n"),
)


def blob_sha(raw: bytes) -> str:
    return hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()


def transform(raw: bytes, expected: str, old: str, new: str) -> tuple[str, bytes]:
    a, b = old.encode(), new.encode()
    if raw.count(b) == 1:
        original = raw.replace(b, a, 1)
        if blob_sha(original) == expected:
            return 'ALREADY_APPLIED', raw
    if blob_sha(raw) != expected or raw.count(a) != 1:
        raise ValueError('source differs from reviewed blob; review the diff, do not force patch')
    return 'READY', raw.replace(a, b, 1)


def run(root: Path, apply: bool = False, rules=RULES) -> list[dict]:
    pending = []
    for relative, expected, old, new in rules:
        path = root / relative
        if path.is_symlink() or not path.is_file():
            raise ValueError('missing regular source file: '+relative)
        raw = path.read_bytes()
        status, edited = transform(raw, expected, old, new)
        pending.append((path, relative, raw, edited, status))
    # Preflight both files before any writes; never patch one file after the other failed validation.
    if apply:
        for path, _, original, _, _ in pending:
            if path.read_bytes() != original:
                raise ValueError('source changed during preflight')
        for path, _, _, edited, status in pending:
            if status == 'READY':
                path.write_bytes(edited)
    return [{'path': relative, 'status': 'APPLIED' if apply and status == 'READY' else status}
            for _, relative, _, _, status in pending]


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[2])
    mode = p.add_mutually_exclusive_group()
    mode.add_argument('--apply', action='store_true')
    mode.add_argument('--check', action='store_true')
    args = p.parse_args()
    try:
        print(json.dumps(run(args.root, args.apply), indent=2))
    except (OSError, ValueError) as exc:
        raise SystemExit(str(exc)) from None
