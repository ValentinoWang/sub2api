import importlib.util
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('space_guard', Path(__file__).with_name('run_with_space_guard.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class SpaceGuardTests(unittest.TestCase):
    def test_low_space_never_starts_build(self):
        with patch.object(guard.shutil, 'disk_usage', return_value=SimpleNamespace(free=2 * guard.GIB)), patch.object(guard.subprocess, 'Popen') as spawn:
            self.assertEqual(guard.run(['go', 'build'], '/', 4, 3, None), 73)
            spawn.assert_not_called()

    def test_pressure_stops_only_child_process_group(self):
        with patch.object(guard.shutil, 'disk_usage', side_effect=[SimpleNamespace(free=5 * guard.GIB), SimpleNamespace(free=int(3.8 * guard.GIB)), SimpleNamespace(free=4 * guard.GIB)]), patch.object(guard.subprocess, 'Popen') as spawn, patch.object(guard.os, 'killpg') as kill:
            process = spawn.return_value
            process.pid = 321
            process.poll.side_effect = [None, 0]
            self.assertEqual(guard.run(['go', 'build'], '/', 4, 3, None), 73)
            kill.assert_called_once_with(321, guard.signal.SIGTERM)
            spawn.assert_called_once_with(['go', 'build'], start_new_session=True)

    def test_real_command_writes_evidence(self):
        import json
        with tempfile.TemporaryDirectory() as folder:
            target = Path(folder) / 'space.json'
            self.assertEqual(guard.run(['/usr/bin/true'], folder, .000002, .000001, target), 0)
            self.assertTrue(json.loads(target.read_text())['reserve_preserved'])


if __name__ == '__main__':
    unittest.main()
