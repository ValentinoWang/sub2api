#!/usr/bin/env python3
"""Check source boundaries and the pinned pnpm path without running application CI."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class LocalCITest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='sub2api-ci-guard-')
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.repo = self.base / 'repo'
        self.repo.mkdir()
        script = self.repo / 'tools/quality/run_local_ci.sh'
        script.parent.mkdir(parents=True)
        shutil.copy2(ROOT / 'tools/quality/run_local_ci.sh', script)
        for name, content in {
            'backend/go.mod': 'module example.invalid/fixture\n\ngo 1.27.0\n',
            'source.txt': 'committed\n',
            'acceptance/human-acceptance-log.json': '{}\n',
            'acceptance/human-acceptance-log.md': 'committed evidence\n',
        }.items():
            target = self.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(content)
        self.git('init', '-q')
        self.git('add', '.')
        self.git('-c', 'user.name=CI Fixture', '-c', 'user.email=fixture@example.invalid',
                 '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
        self.bin = self.base / 'bin'
        self.bin.mkdir()
        # Fail at toolchain by default: no application command or network runs.
        self.stub('node', 'printf "0\\n"')
        self.stub('pnpm', 'printf "9.15.9\\n"')
        self.stub('go', 'printf "go0.0.0\\n"')
        for name in ('golangci-lint', 'govulncheck', 'docker', 'corepack'):
            self.stub(name, 'printf "%s\\n" "$0" >> "$CI_UNEXPECTED_CALLS"\nexit 97')

    def git(self, *args):
        subprocess.run(['git', '-C', str(self.repo), *args], check=True, capture_output=True)

    def stub(self, name, body):
        target = self.bin / name
        target.write_text('#!/bin/sh\nset -eu\n' + body + '\n')
        target.chmod(0o755)

    def run_ci(self, **environment):
        env = dict(os.environ, PATH=f'{self.bin}:{os.environ["PATH"]}',
                   CI_NODE_BIN_DIR=str(self.bin), CI_TEST_LOG=str(self.base / 'corepack.log'),
                   CI_UNEXPECTED_CALLS=str(self.base / 'unexpected-calls.log'))
        env.update(environment)
        result = subprocess.run(['bash', 'tools/quality/run_local_ci.sh', 'agents-results/evidence'],
                                cwd=self.repo, env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.base / 'unexpected-calls.log').exists(),
                         'Toolchain validation fell through to external tools')
        summary = json.loads((self.repo / 'agents-results/evidence/summary.json').read_text())
        self.assertEqual(summary['exit_code'], result.returncode)
        return {stage['stage']: stage for stage in summary['stages']}

    def test_only_declared_existing_evidence_is_exempt(self):
        for name in ('acceptance/human-acceptance-log.json', 'acceptance/human-acceptance-log.md',
                     'acceptance/human/2026-W37/pending.md'):
            target = self.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text('updated evidence\n')
        stages = self.run_ci()
        self.assertEqual(stages['source-preflight']['status'], 'PASS')
        self.assertEqual(stages['snapshot']['status'], 'PASS')
        self.assertEqual(stages['toolchain']['status'], 'FAIL')
        self.assertEqual(stages['frozen-install']['status'], 'NOT_RUN')
        self.assertIn('requires Node 24',
                      (self.repo / 'agents-results/evidence/toolchain.log').read_text())

    def test_dirty_source_is_rejected_before_install(self):
        (self.repo / 'source.txt').write_text('dirty\n')
        stages = self.run_ci()
        self.assertEqual(stages['source-preflight']['status'], 'FAIL')
        self.assertEqual(stages['snapshot']['status'], 'NOT_RUN')

    def test_unrelated_acceptance_file_is_not_exempt(self):
        (self.repo / 'acceptance/README.md').write_text('uncommitted policy\n')
        stages = self.run_ci()
        self.assertEqual(stages['source-preflight']['status'], 'FAIL')

    def test_each_toolchain_version_mismatch_stops_before_external_tools(self):
        cases = (
            ('node', '0', 'requires Node 24'),
            ('pnpm', '11.19.0', 'requires pnpm 9.15.9'),
            ('go', 'go0.0.0', 'requires Go 1.27.0'),
            ('golangci-lint', 'golangci-lint has version 2.12.0', 'requires golangci-lint 2.13.x'),
        )
        for tool, version, diagnostic in cases:
            with self.subTest(tool=tool):
                fixture = LocalCITest()
                fixture.setUp()
                self.addCleanup(fixture.doCleanups)
                for name, valid in (('node', '24'), ('pnpm', '9.15.9'),
                                    ('go', 'go1.27.0'),
                                    ('golangci-lint', 'golangci-lint has version 2.13.0')):
                    fixture.stub(name, f'printf "%s\\n" "{valid}"')
                fixture.stub(tool, f'printf "%s\\n" "{version}"')
                if tool == 'pnpm':
                    # Verify the preflight rejects an unsuccessful pinned shim too.
                    fixture.stub('corepack', 'printf "8.0.0\\n"')
                stages = fixture.run_ci()
                self.assertEqual(stages['toolchain']['status'], 'FAIL')
                self.assertEqual(stages['frozen-install']['status'], 'NOT_RUN')
                self.assertIn(diagnostic,
                              (fixture.repo / 'agents-results/evidence/toolchain.log').read_text())

    def test_inherited_go_test_filters_are_removed(self):
        self.stub('node', 'printf "24\\n"')
        self.stub('go', 'printf "%s\\n" "$GOFLAGS" > "$CI_TEST_LOG"\n'
                  'printf "go0.0.0\\n"')
        stages = self.run_ci(GOFLAGS='-run=^$ -skip=.* -short')
        self.assertEqual(stages['toolchain']['status'], 'FAIL')
        self.assertEqual((self.base / 'corepack.log').read_text().strip(), '-p=1')

    def test_wrong_pnpm_is_replaced_by_exact_private_shim(self):
        self.stub('node', 'printf "24\\n"')
        self.stub('pnpm', 'printf "11.19.0\\n"')
        self.stub('corepack', 'printf "%s\\n" "$@" >> "$CI_TEST_LOG"\nprintf "9.15.9\\n"')
        stages = self.run_ci()
        self.assertEqual(stages['source-preflight']['status'], 'PASS')
        self.assertEqual(stages['toolchain']['status'], 'FAIL')
        args = (self.base / 'corepack.log').read_text().splitlines()
        self.assertEqual(args, ['pnpm@9.15.9', '--version', 'pnpm@9.15.9', '--version'])


if __name__ == '__main__':
    unittest.main(verbosity=2)
