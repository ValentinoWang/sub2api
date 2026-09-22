"""Candidate identity and bounded release evidence; no implicit live operations."""
import hashlib
import json
from pathlib import Path
import re
import time


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def candidate(path):
    value = json.loads(Path(path).read_text())
    require(value['schema_version'] == 1, 'Unsupported candidate schema')
    require(re.fullmatch(r'[0-9a-f]{40}', value['commit']), 'Full commit required')
    require(re.fullmatch(r'[0-9a-f]{40}', value['base_commit']), 'Full base commit required')
    require(re.fullmatch(r'\d+\.\d+\.\d+\.\d+', value['version']), 'Four-part version required')
    require(re.fullmatch(r'sha256:[0-9a-f]{64}', value['image_id']), 'Exact image ID required')
    require(value['image'] == f"sub2api-local:{value['version']}-{value['commit'][:12]}",
            'Image tag must bind version and source')
    require(value['platform'] == 'linux/amd64', 'Unsupported platform')
    require(value['base_migrations'], 'Empty base migration manifest')
    for group in ('base_migrations', 'added_migrations'):
        for name, checksum in value[group].items():
            require(re.fullmatch(r'\d+[a-z]?_[a-z0-9_]+\.sql', name), 'Invalid migration filename')
            require(re.fullmatch(r'[0-9a-f]{64}', checksum), 'Invalid migration checksum')
    require(not (value['base_migrations'].keys() & value['added_migrations'].keys()),
            'Added migrations overlap base')
    return value


def receipt(path, candidate_path, environment, status):
    value = json.loads(Path(path).read_text())
    require(value['candidate_sha256'] == sha(candidate_path), 'Receipt candidate mismatch')
    require(value['environment'] == environment, 'Receipt environment mismatch')
    require(value['status'] == status, 'Receipt status mismatch')
    require(0 <= time.time() - value['created_at'] <= 1800, 'Receipt older than 30 minutes')
    return value


def migration_map(rows):
    result = {row['version']: row['checksum'] for row in rows}
    require(len(result) == len(rows), 'Duplicate migration record')
    return result


def verify_migrations(value, before, after):
    old = migration_map(before)
    for name, checksum in value['base_migrations'].items():
        require(old.get(name) == checksum, 'Base migration mismatch')
    require(not (old.keys() & value['added_migrations'].keys()), 'Candidate already partly migrated')
    require(migration_map(after) == old | value['added_migrations'], 'Unexpected migration delta')
