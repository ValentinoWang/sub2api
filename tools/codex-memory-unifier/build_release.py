#!/usr/bin/env python3
"""Build deterministic Codex Memory CLI release archives and one manifest."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import zipfile
from pathlib import Path

EPOCH = (2020, 1, 1, 0, 0, 0)
TARGETS = {
    "macos": ("codex_memory_unifier.py", "codex-memory"),
    "linux": ("codex_memory_unifier.py", "codex-memory"),
    "windows": ("codex_memory_unifier.py", "codex-memory.ps1"),
}
VERSION_PATTERN = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(?:[-.][0-9A-Za-z]+)*$")
REPOSITORY_PATTERN = re.compile(r"^[0-9A-Za-z_.-]+/[0-9A-Za-z_.-]+$")


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


def add_file(archive: zipfile.ZipFile, source: Path, name: str, executable: bool) -> None:
    info = zipfile.ZipInfo(name, EPOCH)
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = ((0o755 if executable else 0o644) & 0xFFFF) << 16
    archive.writestr(info, source.read_bytes())


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--repository", required=True, help="GitHub owner/repository")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    version = args.version.removeprefix("v")
    if not VERSION_PATTERN.fullmatch(version):
        parser.error("version must be a safe semantic version")
    if not REPOSITORY_PATTERN.fullmatch(args.repository):
        parser.error("repository must be a GitHub owner/name pair")
    output = args.output.resolve()
    if output.exists() and (not output.is_dir() or any(output.iterdir())):
        parser.error("output must not exist or must be an empty directory")
    output.mkdir(parents=True, exist_ok=True)
    tag = f"codex-memory-v{version}"
    assets = []
    for platform_name, files in TARGETS.items():
        filename = f"codex-memory_{version}_{platform_name}.zip"
        archive_path = output / filename
        with zipfile.ZipFile(archive_path, "w") as archive:
            for source_name in files:
                add_file(archive, root / source_name, source_name, source_name == "codex-memory")
            add_file(archive, root / "README.md", "README.md", False)
        assets.append({
            "platform": platform_name,
            "filename": filename,
            "url": f"https://github.com/{args.repository}/releases/download/{tag}/{filename}",
            "sha256": digest(archive_path),
            "size": archive_path.stat().st_size,
        })
    checksums = output / "codex-memory-checksums.txt"
    checksums.write_text("".join(f"{item['sha256']}  {item['filename']}\n" for item in assets), encoding="ascii")
    manifest = {
        "schema_version": 1,
        "version": version,
        "tag": tag,
        "repository": args.repository,
        "python_minimum": "3.11",
        "released_at": None,
        "assets": assets,
        "checksums": {
            "filename": checksums.name,
            "url": f"https://github.com/{args.repository}/releases/download/{tag}/{checksums.name}",
            "sha256": digest(checksums),
        },
    }
    (output / "codex-memory-release-manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(json.dumps({"status": "built", "version": version, "assets": len(assets), "output": str(output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
