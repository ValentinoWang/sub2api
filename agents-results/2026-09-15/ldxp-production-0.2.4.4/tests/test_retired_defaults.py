import copy
import sys
import unittest
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import replace_app

class RetiredDefaults(unittest.TestCase):
    def app(self):
        return {'Config': {'Env': ['SECRET=preserved', 'LIANDONG_TOOLKIT_DATA_DIR=/app/data',
                  'LIANDONG_TOOLKIT_ASSET_PATH=/app/ldxp-toolkit-assets/ldxp-toolkit',
                  'LIANDONG_TOOLKIT_ASSET_MANIFEST_PATH=/app/ldxp-toolkit-assets/ldxp-toolkit-release.json']},
                'Mounts': [], 'HostConfig': {'PortBindings': {'8080/tcp': [{'HostPort': '8080'}]}}}

    def test_only_exact_retired_defaults_removed(self):
        before = self.app()
        expected = replace_app.expected_runtime(before, {'Config': {'Env': []}}, {})
        self.assertEqual(expected['env'], ['SECRET=preserved'])
        self.assertEqual(expected['ports'], before['HostConfig']['PortBindings'])
        self.assertEqual(len(before['Config']['Env']), 4)

    def test_explicit_config_is_preserved(self):
        expected = replace_app.expected_runtime(self.app(), {'Config': {}}, {'LIANDONG_TOOLKIT_DATA_DIR': '/app/data'})
        self.assertIn('LIANDONG_TOOLKIT_DATA_DIR=/app/data', expected['env'])

    def test_custom_value_is_not_silently_discarded(self):
        before = self.app()
        before['Config']['Env'][1] = 'LIANDONG_TOOLKIT_DATA_DIR=/custom/data'
        self.assertIn('LIANDONG_TOOLKIT_DATA_DIR=/custom/data', replace_app.expected_runtime(before, {'Config': {}}, {})['env'])

    def test_still_present_image_default_is_preserved(self):
        before = self.app()
        self.assertEqual(replace_app.expected_runtime(before, before, {}), replace_app.invariants(before))

if __name__ == '__main__':
    unittest.main()
