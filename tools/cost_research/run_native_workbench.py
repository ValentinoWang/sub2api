#!/usr/bin/env python3
"""Build the isolated standard-library native acceptance server. Does not alter the project's go.mod."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def build(root: Path, temporary: Path) -> Path:
    module = temporary / 'src/github.com/Wei-Shaw/sub2api'
    shutil.copytree(root / 'backend/internal/costing', module / 'internal/costing')
    shutil.copytree(root / 'backend/cmd/cost-workbench', module / 'cmd/cost-workbench')
    binary = temporary / 'native-cost-workbench'
    env = dict(os.environ, GO111MODULE='off', GOPATH=str(temporary), GOTOOLCHAIN='local')
    subprocess.run(['go', 'build', '-o', str(binary), 'github.com/Wei-Shaw/sub2api/cmd/cost-workbench'], env=env, check=True)
    return binary


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ledger', required=True, type=Path)
    parser.add_argument('--addr', default='127.0.0.1:0')
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    ledger = args.ledger.expanduser().resolve()
    if ledger.is_relative_to(root):
        parser.error('Keep runtime records outside the source checkout.')
    with tempfile.TemporaryDirectory(prefix='native-cost-') as directory:
        binary = build(root, Path(directory))
        subprocess.run([str(binary), '--ledger', str(ledger), '--addr', args.addr], check=True)
