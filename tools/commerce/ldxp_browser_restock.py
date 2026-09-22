#!/usr/bin/env python3
"""Run local restocking through one persistent, operator-accessible browser."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import signal
import subprocess
import sys
import threading
import time

import ldxp_http_restock as restock
from ldxp_browser_session import BROWSER_HOME, CONTROL_ORIGIN, BrowserControl, BrowserSession

LABEL = 'lol.rest2build.ldxp-browser-local'
OLD_LABEL = 'lol.rest2build.ldxp-http-local'


class BrowserMerchant(restock.Merchant):
    def __init__(self, session, expected_subject=None):
        self.session = session
        self.expected_subject = expected_subject

    def load(self):
        return self

    def save(self):
        # Chromium owns persistence; never overwrite the independent HTTP session file.
        pass

    def _request(self, route, payload):
        value = self.session.request(route, payload)
        if route == '/merchantApi/user/userinfo' and self.expected_subject:
            actual = hashlib.sha256(str(value.get('id')).encode()).hexdigest() if isinstance(value, dict) else ''
            restock.require(actual == self.expected_subject, 'binding_changed')
        return value


def expected_subject(directory):
    path = Path(directory) / 'dedup-capability.json'
    if not path.exists():
        return None
    try:
        data = json.loads(restock.probe.private_read(path))
        digest = data['merchant_subject_sha256']
        restock.require(data['origin'] == restock.probe.ORIGIN and
                        isinstance(digest, str) and re.fullmatch(r'[a-f0-9]{64}', digest), 'binding_changed')
        return digest
    except (ValueError, KeyError, TypeError):
        raise restock.RestockError('binding_changed') from None


def install_service(args, config):
    restock.require(sys.platform == 'darwin', 'invalid_config')
    restock.read_state(args.private_dir)
    from importlib.metadata import version
    restock.require(version('playwright') == '1.58.0', 'invalid_config')
    own_script = str(Path(__file__).resolve())
    old_script = str(Path(restock.__file__).resolve())
    agents = Path.home() / 'Library/LaunchAgents'
    agents.mkdir(parents=True, exist_ok=True)
    entries = []
    # Retire the former autostart entry, or it would return at the next user login.
    for label, script in ((OLD_LABEL, old_script), (LABEL, own_script)):
        path = agents / (label + '.plist')
        target = f'gui/{os.getuid()}/{label}'
        running = subprocess.run(['/bin/launchctl', 'print', target], capture_output=True)
        if path.exists():
            spec = plistlib.loads(path.read_bytes())
            restock.require(script in spec.get('ProgramArguments', []), 'binding_changed')
        if running.returncode == 0:
            restock.require(path.exists() and script.encode() in running.stdout, 'binding_changed')
        entries.append((path, target, running.returncode == 0))
    backup = args.private_dir / 'backups' / ('browser-switch-' + time.strftime('%Y%m%dT%H%M%S') + '-' + os.urandom(3).hex())
    for path, target, running in entries:
        if path.exists():
            backup.mkdir(parents=True, exist_ok=True, mode=0o700)
            shutil.copy2(path, backup / path.name)
        if running:
            restock.require(subprocess.run(['/bin/launchctl', 'bootout', target], capture_output=True).returncode == 0, 'backend_error')
        if path.exists():
            path.unlink()
    spec = {
        'Label': LABEL,
        'ProgramArguments': [sys.executable, '-B', own_script, 'run', '--notify',
                             '--config', str(args.config.resolve()), '--private-dir', str(args.private_dir.resolve()),
                             '--browser-home', str(args.browser_home.resolve()), '--evidence', str((args.private_dir / 'runs').resolve())],
        'WorkingDirectory': str(Path(__file__).resolve().parents[2]),
        'RunAtLoad': True, 'StartInterval': config['interval_seconds'], 'ProcessType': 'Interactive',
        'Umask': 0o077, 'StandardOutPath': str((args.private_dir / 'browser-service.log').resolve()),
        'StandardErrorPath': str((args.private_dir / 'browser-service-error.log').resolve()),
    }
    destination = agents / (LABEL + '.plist')
    with destination.open('wb') as stream:
        plistlib.dump(spec, stream)
    destination.chmod(0o600)
    restock.require(subprocess.run(['/bin/launchctl', 'bootstrap', f'gui/{os.getuid()}', str(destination)], capture_output=True).returncode == 0, 'backend_error')
    restock.require(subprocess.run(['/bin/launchctl', 'print', f'gui/{os.getuid()}/{LABEL}'], capture_output=True).returncode == 0, 'backend_error')
    return {'status': 'SCHEDULED', 'execution_mode': 'browser', 'control_url': CONTROL_ORIGIN,
            'interval_seconds': config['interval_seconds']}


def run(args, config, stopped):
    restock.read_state(args.private_dir)
    subject = expected_subject(args.private_dir)
    session = BrowserSession(args.browser_home)
    control = BrowserControl(session)
    try:
        control.start()
        try:
            session.open()
        except restock.probe.ProbeError:
            # Keep the local handoff page available even if initial navigation fails.
            pass
        return restock.run_worker(args, config, stopped,
                                  merchant_factory=lambda: BrowserMerchant(session, subject),
                                  on_tick=control.tick, execution_mode='browser')
    finally:
        control.close()
        session.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['run', 'install-service'])
    parser.add_argument('--config', type=Path, default=restock.CONFIG)
    parser.add_argument('--private-dir', type=Path, default=restock.PRIVATE)
    parser.add_argument('--browser-home', type=Path, default=BROWSER_HOME)
    parser.add_argument('--evidence', type=Path, default=restock.PRIVATE / 'runs')
    parser.add_argument('--notify', action='store_true')
    args = parser.parse_args()
    args.session = None
    os.umask(0o077)
    try:
        config = restock.load_config(args.config)
        if args.action == 'install-service':
            restock.receipt(install_service(args, config), args.evidence)
            return 0
        stopped = threading.Event()
        signal.signal(signal.SIGTERM, lambda *_: stopped.set())
        signal.signal(signal.SIGINT, lambda *_: stopped.set())
        return run(args, config, stopped)
    except (restock.probe.ProbeError, restock.RestockError) as exc:
        restock.receipt({'status': 'PAUSED', 'error': exc.kind, 'message': str(exc), 'execution_mode': 'browser'}, args.evidence)
        return 1
    except Exception:
        restock.receipt({'status': 'PAUSED', 'error': 'state_invalid', 'message': restock.MESSAGES['state_invalid'], 'execution_mode': 'browser'}, args.evidence)
        return 1


if __name__ == '__main__':
    sys.exit(main())
