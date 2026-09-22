#!/usr/bin/env python3
"""Run a build with a monitored free-space reserve, terminating only its process group."""
import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

GIB = 1024 ** 3


def run(command, volume, stop_free_gib, minimum_free_gib, evidence):
    threshold = int(stop_free_gib * GIB)
    free = shutil.disk_usage(volume).free
    report = dict(command=command, minimum_required_gib=minimum_free_gib,
                  stop_threshold_gib=stop_free_gib, lowest_free_bytes=free,
                  status='NOT_STARTED', exit_code=None)
    process = None
    try:
        if free < threshold:
            report.update(status='BLOCKED_SPACE', exit_code=73)
            print('Build not started: free space is below the reserve.', file=sys.stderr)
            return 73
        process = subprocess.Popen(command, start_new_session=True)
        while process.poll() is None:
            free = shutil.disk_usage(volume).free
            report['lowest_free_bytes'] = min(report['lowest_free_bytes'], free)
            if free < threshold:
                print('Stopping this build to preserve the disk reserve.', file=sys.stderr)
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
                report.update(status='STOPPED_FOR_SPACE', exit_code=73)
                return 73
            time.sleep(1)
        code = process.returncode
        report.update(status='PASS' if code == 0 else 'COMMAND_FAILED', exit_code=code)
        return code
    finally:
        if process is not None and process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
            process.wait(timeout=10)
        report['lowest_free_bytes'] = min(report['lowest_free_bytes'], shutil.disk_usage(volume).free)
        report['reserve_preserved'] = report['lowest_free_bytes'] >= minimum_free_gib * GIB
        if evidence:
            Path(evidence).parent.mkdir(parents=True, exist_ok=True)
            Path(evidence).write_text(json.dumps(report, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--volume', default='/System/Volumes/Data')
    parser.add_argument('--minimum-free-gib', type=float, default=3)
    parser.add_argument('--stop-free-gib', type=float, default=4)
    parser.add_argument('--evidence')
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not command or not 0 < args.minimum_free_gib < args.stop_free_gib:
        parser.error('command and a stop threshold above the minimum reserve are required')
    return run(command, args.volume, args.stop_free_gib, args.minimum_free_gib, args.evidence)


if __name__ == '__main__':
    sys.exit(main())
