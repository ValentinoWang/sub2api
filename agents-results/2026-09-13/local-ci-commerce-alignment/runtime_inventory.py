"""Read-only release inventory; never emit environment values or business secrets."""
import datetime
import hashlib
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/quality'))
from commerce_release import Transport, postgres, logical_schema, SCHEMA_SQL, IDENTITY_SQL, MIGRATIONS_SQL, canonical_bytes

CONFIGS = {
    'dev': {'app_container': 'sub2api', 'db_container': 'sub2api-postgres'},
    'prod': {'app_container': 'sub2api', 'db_container': 'sub2api-postgres',
             'ssh_host': 'ubuntu@43.156.50.78',
             'ssh_key': '/Users/vsiyo/Desktop/Key/MacbookAir.pem'},
}

def collect(environment):
    config = CONFIGS[environment]
    transport = Transport()
    app = json.loads(transport.command(config, ['docker', 'inspect', 'sub2api']))[0]
    schema = logical_schema(json.loads(postgres(config, transport, 'psql', SCHEMA_SQL)))
    migrations = json.loads(postgres(config, transport, 'psql', MIGRATIONS_SQL))
    tables = ('users', 'accounts', 'api_keys', 'redeem_codes', 'groups', 'user_subscriptions',
              'payment_orders', 'liandong_restock_batches')
    counts = {}
    for table in tables:
        counts[table] = int(postgres(config, transport, 'psql', 'SELECT count(*) FROM ' + table).strip())
    environment_values = dict(item.split('=', 1) for item in app['Config']['Env'] if '=' in item)
    return {
        'observed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'environment': environment, 'image_id': app['Image'], 'image_name': app['Config']['Image'],
        'container_state': app['State']['Status'], 'health': app['State'].get('Health', {}).get('Status'),
        'environment_sha256': hashlib.sha256(canonical_bytes(environment_values)).hexdigest(),
        'secret_fingerprints': {key: hashlib.sha256(environment_values.get(key, '').encode()).hexdigest()
                                for key in ('JWT_SECRET', 'TOTP_ENCRYPTION_KEY', 'DATABASE_PASSWORD', 'REDIS_PASSWORD')},
        'mounts': [{'source': m['Source'], 'destination': m['Destination']} for m in app['Mounts']],
        'ports': app['HostConfig']['PortBindings'], 'counts': counts,
        'database_identity': hashlib.sha256(postgres(config, transport, 'psql', IDENTITY_SQL).strip().encode()).hexdigest(),
        'schema_hash': hashlib.sha256(canonical_bytes(schema)).hexdigest(),
        'schema_objects': {key: len(rows) for key, rows in schema.items()},
        'migrations': migrations,
    }

if __name__ == '__main__':
    environment, output_path = sys.argv[1:]
    result = collect(environment)
    output = Path(output_path)
    if output.exists():
        raise SystemExit('Refusing to overwrite release evidence')
    output.write_text(json.dumps(result, indent=2) + '\n')
    output.chmod(0o600)
    print(json.dumps({key: result[key] for key in ('environment', 'image_id', 'health', 'counts', 'schema_hash')}))
