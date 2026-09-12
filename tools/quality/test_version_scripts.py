#!/usr/bin/env python3
"""Verify release identity and committed-only Docker context without building images."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class VersionScriptsTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='sub2api-version-test-')
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        for source in ('backend/scripts/resolve-version.sh', 'deploy/build_image.sh'):
            target = self.repo / source
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / source, target)
        self.version = self.repo / 'backend/cmd/server/VERSION'
        self.version.parent.mkdir(parents=True)
        self.version.write_text('0.2.4.1\n')
        (self.repo / 'Dockerfile').write_text('FROM scratch\n')
        (self.repo / 'source.txt').write_text('committed\n')
        self.git('init', '-q')
        self.git('add', '.')
        self.commit()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True).strip()

    def commit(self):
        self.git('-c', 'user.name=Version Fixture', '-c', 'user.email=fixture@example.invalid',
                 '-c', 'commit.gpgsign=false', 'commit', '-qm', 'version fixture')

    def resolve(self):
        return subprocess.run(['sh', str(self.repo / 'backend/scripts/resolve-version.sh')],
                              capture_output=True, text=True, cwd=self.repo)

    def test_four_part_file_and_release_tag(self):
        self.assertEqual(self.resolve().stdout.strip(), '0.2.4.1')
        self.git('tag', 'v0.2.4.2')
        self.assertEqual(self.resolve().stdout.strip(), '0.2.4.2')

    def test_prior_three_part_release_tag(self):
        self.git('tag', 'v0.2.4')
        self.assertEqual(self.resolve().stdout.strip(), '0.2.4')

    def test_nonrelease_tag_does_not_replace_version(self):
        self.git('tag', 'v0.2.4-preview')
        self.assertEqual(self.resolve().stdout.strip(), '0.2.4.1')

    def test_invalid_file_is_rejected(self):
        for version in ('0.2', '0.2.4.1.1', '01.2.4', '0.2.4;echo unsafe', ''):
            with self.subTest(version=version):
                self.version.write_text(version + '\n')
                result = self.resolve()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Invalid technical version', result.stderr)

    def build(self):
        shim = self.repo / 'test-bin'
        shim.mkdir(exist_ok=True)
        docker = shim / 'docker'
        docker.write_text('''#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$@" >> "$DOCKER_TEST_LOG"
if [[ "$1" == build ]]; then
  for context; do :; done
  test "$(cat "$context/source.txt")" = committed
  test ! -e "$context/dirty-untracked.txt"
fi
''')
        docker.chmod(0o755)
        env = dict(os.environ, PATH=f'{shim}:{os.environ["PATH"]}',
                   DOCKER_TEST_LOG=str(self.repo / 'docker-test.log'))
        return subprocess.run(['bash', str(self.repo / 'deploy/build_image.sh')],
                              env=env, capture_output=True, text=True, cwd=self.repo)

    def test_image_build_keeps_commit_provenance_and_excludes_dirty_source(self):
        commit = self.git('rev-parse', 'HEAD')
        self.version.write_text('9.9.9\n')
        (self.repo / 'source.txt').write_text('dirty\n')
        (self.repo / 'dirty-untracked.txt').write_text('untracked\n')
        result = self.build()
        self.assertEqual(result.returncode, 0, result.stderr)
        args = (self.repo / 'docker-test.log').read_text()
        self.assertIn(f'sub2api-local:0.2.4.1-{commit[:12]}', args)
        self.assertIn(f'COMMIT={commit}', args)
        self.assertIn(f'org.opencontainers.image.revision={commit}', args)
        self.assertIn('VERSION=0.2.4.1', args)

    def test_image_build_accepts_prior_three_part_version(self):
        self.version.write_text('0.2.4\n')
        self.git('add', str(self.version))
        self.commit()
        result = self.build()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('VERSION=0.2.4\n', (self.repo / 'docker-test.log').read_text())

    def test_image_build_rejects_invalid_committed_version_before_docker(self):
        self.version.write_text('0.2.4.1.1\n')
        self.git('add', str(self.version))
        self.commit()
        result = self.build()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Invalid technical version', result.stderr)
        self.assertFalse((self.repo / 'docker-test.log').exists())


if __name__ == '__main__':
    unittest.main(verbosity=2)
