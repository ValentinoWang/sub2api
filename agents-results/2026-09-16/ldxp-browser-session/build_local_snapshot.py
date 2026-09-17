#!/usr/bin/env python3
"""Build the committed local candidate with a monitored free-space reserve."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time

PROJECT = Path('/Users/vsiyo/Desktop/Opensource_Tool/Sub2api')
REPO = Path('/Users/vsiyo/.local/share/sub2api/ldxp-verification-checkout')
OUT = PROJECT / 'agents-results/2026-09-16/ldxp-browser-session/acceptance/native-build-1'
TOOLS = Path('/Users/vsiyo/.local/share/sub2api/ldxp-recovery-toolchain/bin')
COMMIT = 'd2966e82874a32a0f4eefaa43b56b7bd252fa1c7'
BASE_IMAGE = 'sub2api-local:0.2.4.8-f0e321a7cb35'
BASE_ID = 'sha256:6519bf8ffba025428f87dd547e1bb2b1ec91905b8f33751423dad1ed4906326f'
GIB = 1024 ** 3
OUT.mkdir(parents=True, exist_ok=True)
environment = os.environ.copy()
environment['PATH'] = str(TOOLS) + os.pathsep + environment['PATH']
minimum_free = shutil.disk_usage(PROJECT).free
stages = []


def free_bytes():
    global minimum_free
    value = shutil.disk_usage(PROJECT).free
    minimum_free = min(minimum_free, value)
    return value


def run(name, args, cwd, env=None, stdout=None):
    if free_bytes() < 5 * GIB:
        raise RuntimeError('insufficient space before stage: ' + name)
    started = time.time()
    with (OUT / (name + '.log')).open('wb') as log:
        proc = subprocess.Popen(args, cwd=cwd, env=env or environment,
                                stdout=stdout or log, stderr=log, start_new_session=True)
        while proc.poll() is None:
            if free_bytes() < 4 * GIB:
                os.killpg(proc.pid, signal.SIGSTOP)
                # All deletions are restricted to the finished frontend install in this snapshot.
                dependencies = Path(cwd) / 'frontend/node_modules'
                if name != 'frontend' and dependencies.is_dir():
                    shutil.rmtree(dependencies)
                if free_bytes() < 5 * GIB:
                    os.killpg(proc.pid, signal.SIGCONT)
                    os.killpg(proc.pid, signal.SIGTERM)
                    proc.wait(timeout=30)
                    raise RuntimeError('disk reserve stopped stage before reaching 3 GiB: ' + name)
                os.killpg(proc.pid, signal.SIGCONT)
            time.sleep(1)
    stages.append({'stage': name, 'exit_code': proc.returncode,
                   'seconds': round(time.time() - started, 2), 'minimum_free_gib': round(minimum_free / GIB, 3)})
    (OUT / 'progress.json').write_text(json.dumps(stages, indent=2) + '\n')
    print(json.dumps(stages[-1]), flush=True)
    if proc.returncode:
        raise RuntimeError('stage failed: ' + name)


summary = {'commit': COMMIT, 'runtime_base_image': BASE_IMAGE, 'runtime_base_image_id': BASE_ID,
           'source': 'git archive of the exact commit; no working-tree sources'}
try:
    with tempfile.TemporaryDirectory(prefix='ldxp-browser-build-') as temp:
        root = Path(temp)
        archive = root / 'source.tar'
        with archive.open('wb') as output:
            run('archive', ['git', 'archive', '--format=tar', COMMIT], REPO, stdout=output)
        source = root / 'source'
        source.mkdir()
        run('extract', ['tar', '-xf', str(archive), '-C', str(source)], REPO)
        archive.unlink()
        version = (source / 'backend/cmd/server/VERSION').read_text().strip()
        assert version == '0.2.4.9'
        run('dependencies', [str(TOOLS / 'pnpm'), '--dir', 'frontend', 'install', '--frozen-lockfile', '--prefer-offline'], source)
        run('frontend', [str(TOOLS / 'pnpm'), '--dir', 'frontend', 'run', 'build'], source)
        shutil.rmtree(source / 'frontend/node_modules')
        payload = root / 'image'
        payload.mkdir()
        build_env = environment | {'CGO_ENABLED': '0', 'GOOS': 'linux', 'GOARCH': 'amd64', 'GOTOOLCHAIN': 'local', 'GOMAXPROCS': '4'}
        date = subprocess.check_output(['git', 'show', '-s', '--format=%cI', COMMIT], cwd=REPO, text=True).strip()
        ldflags = f'-s -w -X main.Version={version} -X main.Commit={COMMIT} -X main.Date={date} -X main.BuildType=release'
        run('go', [str(TOOLS / 'go'), 'build', '-p', '4', '-tags', 'embed', '-ldflags=' + ldflags,
                   '-trimpath', '-o', str(payload / 'sub2api'), './cmd/server'], source / 'backend', build_env)
        summary['binary_sha256'] = hashlib.sha256((payload / 'sub2api').read_bytes()).hexdigest()
        metadata = json.loads(subprocess.check_output(['docker', 'image', 'inspect', BASE_IMAGE], text=True, timeout=30))[0]
        assert metadata['Id'] == BASE_ID
        shutil.copytree(source / 'backend/resources', payload / 'resources')
        dockerfile = f'''FROM {BASE_IMAGE}
LABEL org.opencontainers.image.source="https://github.com/ValentinoWang/sub2api"
LABEL org.opencontainers.image.revision="{COMMIT}"
LABEL org.opencontainers.image.version="{version}"
LABEL org.opencontainers.image.created="{date}"
COPY --chown=sub2api:sub2api sub2api /app/sub2api
COPY --chown=sub2api:sub2api resources /app/resources
'''
        (payload / 'Dockerfile').write_text(dockerfile)
        image = f'sub2api-local:{version}-{COMMIT[:12]}'
        run('image', ['docker', 'build', '--pull=false', '--network=none', '--platform=linux/amd64', '-t', image, '.'], payload)
        final = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image], text=True, timeout=30))[0]
        summary.update(status='PASS', image=image, image_id=final['Id'], version=version)
except Exception as exc:
    summary.update(status='FAIL', error=str(exc))
finally:
    summary.update(stages=stages, minimum_free_gib=round(minimum_free / GIB, 3), required_free_gib=3)
    (OUT / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
if summary['status'] != 'PASS':
    raise SystemExit(1)
