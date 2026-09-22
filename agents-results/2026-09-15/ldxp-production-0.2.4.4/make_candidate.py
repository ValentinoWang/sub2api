"""Create an explicit candidate from a committed snapshot and a built image ID."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

from release_common import candidate, require


def git(*args):
    return subprocess.check_output(['git', *args], stderr=subprocess.PIPE).decode()


def migrations(commit):
    paths = git('ls-tree', '-r', '--name-only', commit, 'backend/migrations').splitlines()
    return {Path(path).name: hashlib.sha256(git('show', f'{commit}:{path}').strip().encode()).hexdigest()
            for path in paths if path.endswith('.sql')}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--commit', required=True)
    parser.add_argument('--base-commit', required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--image-id', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    commit = git('rev-parse', '--verify', args.commit + '^{commit}').strip()
    base = git('rev-parse', '--verify', args.base_commit + '^{commit}').strip()
    subprocess.run(['git', 'merge-base', '--is-ancestor', base, commit], check=True,
                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    require(git('show', commit + ':backend/cmd/server/VERSION').strip() == args.version,
            'Version does not match committed candidate')
    before, after = migrations(base), migrations(commit)
    require(all(after.get(name) == checksum for name, checksum in before.items()),
            'Existing migration was changed or deleted')
    value = {'schema_version': 1, 'commit': commit, 'base_commit': base,
             'version': args.version, 'image_id': args.image_id, 'platform': 'linux/amd64',
             'image': f'sub2api-local:{args.version}-{commit[:12]}',
             'base_migrations': before, 'added_migrations': {
                 name: checksum for name, checksum in after.items() if name not in before}}
    # A candidate is created once; release evidence must not be silently rebound.
    with args.output.open('x') as output:
        json.dump(value, output, indent=2)
        output.write('\n')
    candidate(args.output)
    print(json.dumps({'status': 'CANDIDATE_CREATED', 'commit': commit,
                      'version': args.version, 'added_migrations': sorted(value['added_migrations'])}))


if __name__ == '__main__':
    main()
