"""Malformed security reports must not become successful empty audits."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

CHECKER = Path(__file__).resolve().parents[2] / 'check_pnpm_audit_exceptions.py'


def report(format_name='advisories', high=0):
    return {format_name: {}, 'metadata': {'vulnerabilities': {
        'info': 0, 'low': 0, 'moderate': 0, 'high': high, 'critical': 0}}}


class AuditReportTest(unittest.TestCase):
    def check(self, audit, expected, exceptions='', audit_exit_code=None):
        with tempfile.TemporaryDirectory(prefix='sub2api-audit-guard-') as directory:
            root = Path(directory)
            (root / 'audit.json').write_text(json.dumps(audit))
            (root / 'exceptions.yml').write_text('version: 1\nexceptions:\n' + exceptions)
            command = ['python3', str(CHECKER), '--audit', str(root / 'audit.json'),
                       '--exceptions', str(root / 'exceptions.yml')]
            if audit_exit_code is not None:
                command.extend(['--audit-exit-code', str(audit_exit_code)])
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            return result.stderr

    def high_report(self):
        data = report(high=1)
        data['advisories']['123'] = {'module_name': 'package', 'severity': 'high',
                                      'github_advisory_id': 'GHSA-fixture', 'title': 'Finding'}
        return data

    def test_empty_complete_report_passes(self):
        self.check(report(), 0, audit_exit_code=0)
        self.check(report('vulnerabilities'), 0, audit_exit_code=0)

    def test_metadata_only_report_is_rejected(self):
        data = report(high=1)
        del data['advisories']
        self.assertIn('findings object', self.check(data, 1))

    def test_counts_without_details_are_rejected(self):
        self.assertIn('counts do not match', self.check(report(high=1), 1))

    def test_high_vulnerability_with_empty_via_is_rejected(self):
        data = report('vulnerabilities', high=1)
        data['vulnerabilities']['package'] = {'severity': 'high', 'via': []}
        self.assertIn('missing advisory details', self.check(data, 1))

    def test_count_shape_and_total_must_be_consistent(self):
        for bad in (-1, True, '1', None):
            data = report()
            data['metadata']['vulnerabilities']['high'] = bad
            self.assertIn('count', self.check(data, 1))
        data = report()
        data['metadata']['vulnerabilities']['total'] = 1
        self.check(data, 1)

    def test_details_cannot_be_hidden_by_zero_counts(self):
        data = self.high_report()
        data['metadata']['vulnerabilities']['high'] = 0
        self.assertIn('counts do not match', self.check(data, 1))

    def test_missing_identity_and_invalid_finding_are_rejected(self):
        data = self.high_report()
        data['advisories']['123'] = {'module_name': 'package', 'severity': 'high'}
        self.check(data, 1)
        data['advisories']['123'] = None
        self.check(data, 1)

    def test_high_finding_requires_exact_exception(self):
        self.check(self.high_report(), 1, audit_exit_code=1)
        exception = '''  - package: package
    advisory: GHSA-fixture
    severity: high
    mitigation: isolated fixture
    expires_on: 2099-01-01
'''
        self.check(self.high_report(), 0, exception, audit_exit_code=1)
        self.check(self.high_report(), 1, exception, audit_exit_code=0)

    def test_npm_vulnerability_finding_requires_exception(self):
        data = report('vulnerabilities', high=1)
        data['vulnerabilities']['package'] = {'severity': 'high', 'via': [
            {'source': 123, 'title': 'Finding', 'url': 'GHSA-fixture'}]}
        self.assertIn('missing exceptions', self.check(data, 1, audit_exit_code=1))

    def test_transport_errors_and_inconsistent_exit_are_rejected(self):
        for data in ({'error': {'code': 'FETCH_FAILED'}}, [], {}, None):
            self.check(data, 1)
        self.check(report(), 1, audit_exit_code=1)
        self.check(report(), 1, audit_exit_code=2)

    def test_unknown_severity_and_unresolved_dependency_are_rejected(self):
        data = self.high_report()
        data['advisories']['123']['severity'] = 'unknown'
        self.check(data, 1)
        data = report('vulnerabilities', high=1)
        data['vulnerabilities']['package'] = {'severity': 'high', 'via': ['missing']}
        self.check(data, 1)


if __name__ == '__main__':
    unittest.main(verbosity=2)
