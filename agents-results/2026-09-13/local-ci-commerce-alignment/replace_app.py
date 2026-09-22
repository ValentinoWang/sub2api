"""One-shot app replacement, preserving the live environment and rollback image."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

deploy_directory, release_image, expected_old_image_id, backup_directory = sys.argv[1:]
os.umask(0o077)
os.chdir(deploy_directory)
backup = Path(backup_directory)
assert (backup / 'manifest.json').is_file(), 'Verified backup is required'
run = lambda args: subprocess.check_output(args, stderr=subprocess.PIPE, text=True)
before = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
assert before['Image'] == expected_old_image_id, 'Current image changed since inventory'
candidate = json.loads(run(['docker', 'image', 'inspect', release_image]))[0]
labels = candidate['Config']['Labels']
assert labels['org.opencontainers.image.revision'] == '8365f45ea179b99e627cfbbf751b604a45ad2eee'
assert labels['org.opencontainers.image.version'] == '0.2.4.1'
assert candidate['Architecture'] == 'amd64' and candidate['Os'] == 'linux'
env_path = Path('.env')
original = env_path.read_bytes()
text = original.decode()
pattern = r'(?m)^SUB2API_IMAGE=.*$'
assert len(re.findall(pattern, text)) <= 1, 'Ambiguous image configuration'
updated = re.sub(pattern, 'SUB2API_IMAGE=' + release_image, text) if re.search(pattern, text) else text.rstrip('\n') + '\nSUB2API_IMAGE=' + release_image + '\n'
env_path.write_text(updated)
env_path.chmod(0o600)
command = ['docker', 'compose', '-f', 'docker-compose.local.yml']
started = False
try:
    config = json.loads(run(command + ['config', '--format', 'json']))['services']['sub2api']
    assert config['image'] == release_image, 'Compose image override prevents deployment'
    previous_env = dict(item.split('=', 1) for item in before['Config']['Env'] if '=' in item)
    assert all(previous_env.get(key) == str(value) for key, value in config['environment'].items()), 'Compose environment drift: review privately before cutover'
    started = True
    output = run(command + ['up', '-d', '--no-deps', '--pull', 'never', '--force-recreate', 'sub2api'])
    after = json.loads(run(['docker', 'inspect', 'sub2api']))[0]
    assert after['Image'] == candidate['Id'], 'Container did not use the verified image'
    print(json.dumps({'status': 'RECREATED', 'image_id': after['Image'], 'state': after['State']['Status'], 'health': after['State'].get('Health', {}).get('Status')}))
except Exception:
    env_path.write_bytes(original)
    if started:
        rollback_env = dict(os.environ, SUB2API_IMAGE=before['Config']['Image'])
        result = subprocess.run(command + ['up', '-d', '--no-deps', '--pull', 'never', '--force-recreate', 'sub2api'], env=rollback_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        print(json.dumps({'status': 'FAILED', 'rollback_command_exit': result.returncode}))
    raise
