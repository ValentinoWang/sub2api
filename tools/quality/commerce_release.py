#!/usr/bin/env python3
"""Collect live sanitized parity evidence and optionally enable the commerce release workflow.

This wrapper does not intercept independent admin API operations or confer human PASS.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import sys
from urllib.parse import urlsplit
from urllib.request import build_opener, HTTPRedirectHandler, Request

import check_commerce_environment_parity as parity


PARITY_ROUTE = '/api/v1/admin/liandong/restock/parity'
WORKER_ROUTE = '/api/v1/admin/liandong/restock/enable'
SETTINGS_ROUTE = '/api/v1/admin/settings'
CONFIG_KEYS = {'url', 'app_container', 'db_container', 'ssh_host', 'ssh_key',
               'admin_key_file', 'acceptance_file'}
BUSINESS_KEYS = {'business_rules', 'products', 'merchant_configured', 'code_secret_configured',
                 'code_secret_digest', 'purchase_enabled', 'restock_enabled', 'reconciliation_required',
                 'database_identity', 'purchase_url_configured'}
API_KEYS = BUSINESS_KEYS | {'current_migrations'}
RETIRED_MIGRATIONS_FILE = Path(__file__).with_name('commerce_retired_migrations.json')
MIGRATIONS_SQL = (
    "SELECT COALESCE(json_agg(json_build_object('version',filename,'checksum',checksum) "
    "ORDER BY filename),'[]'::json) FROM schema_migrations"
)
IDENTITY_SQL = (
    "SELECT json_build_array(system_identifier::text,current_database()) "
    "FROM pg_control_system()"
)
SCHEMA_SQL = r"""
SET search_path = pg_catalog;
WITH namespaces AS (
  SELECT oid, nspname FROM pg_namespace
  WHERE nspname !~ '^pg_' AND nspname <> 'information_schema'
), relations AS (
  SELECT c.*, n.nspname FROM pg_class c JOIN namespaces n ON n.oid = c.relnamespace
), objects AS (
  SELECT 'relations' AS category, jsonb_build_object(
    'schema', c.nspname, 'name', c.relname, 'kind', c.relkind,
    'persistence', c.relpersistence, 'replica_identity', c.relreplident,
    'row_security', c.relrowsecurity, 'force_row_security', c.relforcerowsecurity,
    'partition_key', CASE WHEN c.relkind = 'p' THEN pg_get_partkeydef(c.oid) END,
    'partition_bound', pg_get_expr(c.relpartbound, c.oid, false),
    'options', c.reloptions,
    'parents', (SELECT jsonb_agg(p.inhparent::regclass::text ORDER BY p.inhseqno)
                FROM pg_inherits p WHERE p.inhrelid = c.oid)) AS item
  FROM relations c WHERE c.relkind IN ('r','p','v','m','f')
  UNION ALL
  SELECT 'columns', jsonb_build_object(
    'schema', c.nspname, 'relation', c.relname, 'name', a.attname,
    'type', format_type(a.atttypid, a.atttypmod), 'not_null', a.attnotnull,
    'default', pg_get_expr(d.adbin, d.adrelid, false), 'identity', a.attidentity,
    'generated', a.attgenerated, 'dimensions', a.attndims,
    'collation', CASE WHEN a.attcollation <> 0 THEN a.attcollation::regcollation::text END,
    'storage', a.attstorage, 'compression', a.attcompression)
  FROM relations c JOIN pg_attribute a ON a.attrelid = c.oid
  LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
  WHERE c.relkind IN ('r','p','v','m','f') AND a.attnum > 0 AND NOT a.attisdropped
  UNION ALL
  SELECT 'constraints', jsonb_build_object(
    'schema', n.nspname, 'relation', r.relname, 'domain', t.typname,
    'name', c.conname, 'type', c.contype, 'definition', pg_get_constraintdef(c.oid, false),
    'validated', c.convalidated, 'deferrable', c.condeferrable,
    'deferred', c.condeferred, 'no_inherit', c.connoinherit,
    'enforced', COALESCE(to_jsonb(c)->'conenforced', 'true'::jsonb))
  FROM pg_constraint c JOIN namespaces n ON n.oid = c.connamespace
  LEFT JOIN pg_class r ON r.oid = c.conrelid LEFT JOIN pg_type t ON t.oid = c.contypid
  WHERE c.contype <> 'n'
  UNION ALL
  SELECT 'indexes', jsonb_build_object(
    'schema', c.nspname, 'relation', c.relname, 'name', x.relname,
    'definition', pg_get_indexdef(i.indexrelid, 0, false),
    'valid', i.indisvalid, 'ready', i.indisready, 'live', i.indislive,
    'clustered', i.indisclustered, 'replica_identity', i.indisreplident, 'options', x.reloptions)
  FROM relations c JOIN pg_index i ON i.indrelid = c.oid JOIN pg_class x ON x.oid = i.indexrelid
  UNION ALL
  SELECT 'sequences', jsonb_build_object(
    'schema', c.nspname, 'name', c.relname, 'type', format_type(s.seqtypid, NULL),
    'start', s.seqstart, 'increment', s.seqincrement, 'min', s.seqmin, 'max', s.seqmax,
    'cache', s.seqcache, 'cycle', s.seqcycle,
    'owned_by', (SELECT jsonb_agg(jsonb_build_array(rn.nspname, r.relname, a.attname)
                                  ORDER BY rn.nspname, r.relname, a.attname)
                 FROM pg_depend d JOIN pg_class r ON r.oid = d.refobjid
                 JOIN pg_namespace rn ON rn.oid = r.relnamespace
                 JOIN pg_attribute a ON a.attrelid = r.oid AND a.attnum = d.refobjsubid
                 WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
                   AND d.refclassid = 'pg_class'::regclass AND d.deptype IN ('a','i')))
  FROM relations c JOIN pg_sequence s ON s.seqrelid = c.oid
  UNION ALL
  SELECT 'functions', jsonb_build_object(
    'schema', n.nspname, 'name', p.proname,
    'arguments', pg_get_function_identity_arguments(p.oid),
    'definition', pg_get_functiondef(p.oid))
  FROM pg_proc p JOIN namespaces n ON n.oid = p.pronamespace WHERE p.prokind <> 'a'
  UNION ALL
  SELECT 'triggers', jsonb_build_object(
    'schema', c.nspname, 'relation', c.relname, 'name', t.tgname,
    'definition', pg_get_triggerdef(t.oid, false), 'enabled', t.tgenabled)
  FROM pg_trigger t JOIN relations c ON c.oid = t.tgrelid WHERE NOT t.tgisinternal
  UNION ALL
  SELECT 'views', jsonb_build_object(
    'schema', c.nspname, 'name', c.relname, 'definition', pg_get_viewdef(c.oid, false))
  FROM relations c WHERE c.relkind IN ('v','m')
  UNION ALL
  SELECT 'types', jsonb_build_object(
    'schema', n.nspname, 'name', t.typname, 'kind', t.typtype,
    'base', CASE WHEN t.typbasetype <> 0 THEN format_type(t.typbasetype, t.typtypmod) END,
    'not_null', t.typnotnull, 'default', t.typdefault,
    'enum_labels', (SELECT jsonb_agg(e.enumlabel ORDER BY e.enumsortorder)
                    FROM pg_enum e WHERE e.enumtypid = t.oid))
  FROM pg_type t JOIN namespaces n ON n.oid = t.typnamespace
  WHERE t.typtype IN ('d','e','r','m')
  UNION ALL
  SELECT 'policies', jsonb_build_object(
    'schema', c.nspname, 'relation', c.relname, 'name', p.polname,
    'command', p.polcmd, 'permissive', p.polpermissive,
    'roles', (SELECT jsonb_agg(CASE WHEN role = 0 THEN 'PUBLIC' ELSE role::regrole::text END ORDER BY role::regrole::text)
              FROM unnest(p.polroles) role),
    'using', pg_get_expr(p.polqual, p.polrelid, false),
    'check', pg_get_expr(p.polwithcheck, p.polrelid, false))
  FROM pg_policy p JOIN relations c ON c.oid = p.polrelid
  UNION ALL
  SELECT 'rules', jsonb_build_object('schema', c.nspname, 'relation', c.relname,
    'name', r.rulename, 'definition', pg_get_ruledef(r.oid, false), 'enabled', r.ev_enabled)
  FROM pg_rewrite r JOIN relations c ON c.oid = r.ev_class WHERE r.rulename <> '_RETURN'
  UNION ALL
  SELECT 'extensions', jsonb_build_object('schema', n.nspname, 'name', e.extname, 'version', e.extversion)
  FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace
  UNION ALL
  SELECT 'unsupported', jsonb_build_object('schema', n.nspname, 'name', p.proname, 'kind', 'aggregate')
  FROM pg_proc p JOIN namespaces n ON n.oid = p.pronamespace WHERE p.prokind = 'a'
  UNION ALL
  SELECT 'unsupported', jsonb_build_object('schema', c.nspname, 'name', c.relname, 'kind', c.relkind)
  FROM relations c WHERE c.relkind IN ('f','c')
  UNION ALL
  SELECT 'unsupported', jsonb_build_object('schema', n.nspname, 'name', t.typname, 'kind', t.typtype)
  FROM pg_type t JOIN namespaces n ON n.oid = t.typnamespace WHERE t.typtype IN ('r','m')
)
SELECT jsonb_object_agg(category, items) FROM (
  SELECT category, jsonb_agg(item ORDER BY item::text) AS items FROM objects GROUP BY category
) grouped;
"""


class ReleaseError(Exception):
    """Only fixed, safe diagnostics may cross the command/API transport boundary."""


def parse_json(raw):
    try:
        return json.loads(raw, object_pairs_hook=parity.unique_object,
                          parse_constant=parity.reject_constant,
                          parse_float=reject_numeric_fraction)
    except (ValueError, RecursionError):
        raise ReleaseError('invalid JSON; correct the configured input or service response') from None


def reject_numeric_fraction(_value):
    # The effective API emits integer CNY denominations and decimal strings for all rates/credits.
    raise ValueError('use integer denominations and decimal strings in the live snapshot contract')


def read_json(path):
    try:
        with Path(path).open('rb') as source:
            raw = source.read(parity.MAX_INPUT_BYTES + 1)
        if len(raw) > parity.MAX_INPUT_BYTES:
            raise ReleaseError('JSON input exceeds the size limit')
        return parse_json(raw)
    except OSError:
        raise ReleaseError('unable to read the configured JSON input') from None


def exact_object(value, keys, required):
    if not isinstance(value, dict) or set(value) - keys or set(required) - set(value):
        raise ReleaseError('input fields do not match the sanitized release contract')


def load_config(path):
    path = Path(path)
    config = read_json(path)
    exact_object(config, {'dev', 'prod'}, {'dev', 'prod'})
    for environment in ('dev', 'prod'):
        value = config[environment]
        exact_object(value, CONFIG_KEYS, {'url', 'app_container', 'db_container', 'admin_key_file'})
        if any(not isinstance(item, str) or not item or item != item.strip()
               for item in value.values()):
            raise ReleaseError('configuration values must be nonempty trimmed strings')
        try:
            url = urlsplit(value['url'])
            valid_url = (url.scheme == 'https' or
                         (url.scheme == 'http' and url.hostname in ('127.0.0.1', 'localhost', '::1')))
            valid_url = (valid_url and url.hostname and not url.username and not url.password
                         and not url.query and not url.fragment and url.path in ('', '/'))
            url.port
        except ValueError:
            valid_url = False
        if not valid_url:
            raise ReleaseError('use an HTTPS origin or loopback HTTP origin without embedded credentials')
        value['url'] = value['url'].rstrip('/')
        for key in ('app_container', 'db_container'):
            if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', value[key]):
                raise ReleaseError('container names must be Docker identifiers, never command text')
        if 'ssh_host' in value and not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.@-]{0,254}', value['ssh_host']):
            raise ReleaseError('SSH host must be a host alias or user@host')
        if 'ssh_key' in value and 'ssh_host' not in value:
            raise ReleaseError('ssh_key requires ssh_host')
        for key in ('admin_key_file', 'ssh_key', 'acceptance_file'):
            if key in value:
                candidate = Path(value[key]).expanduser()
                value[key] = str(candidate if candidate.is_absolute() else path.parent / candidate)
    if config['dev']['url'] == config['prod']['url']:
        raise ReleaseError('dev and prod must use separate service origins')
    return config


def read_admin_key(path):
    descriptor = None
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        info = os.fstat(descriptor)
        if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600 or info.st_uid != os.getuid():
            raise ReleaseError('admin key must be an owned regular file with mode 0600')
        raw = os.read(descriptor, 8193)
        key = raw.decode('ascii').strip()
        if len(raw) > 8192 or not key or any(ord(character) < 33 or ord(character) > 126 for character in key):
            raise ReleaseError('admin key file must contain one bounded ASCII key')
        return key
    except (OSError, UnicodeError):
        raise ReleaseError('unable to read the protected admin key file') from None
    finally:
        if descriptor is not None:
            os.close(descriptor)


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ReleaseError('redirect refused to protect the admin credential')


class Transport:
    def command(self, config, argv):
        if 'ssh_host' in config:
            prefix = ['ssh', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes',
                      '-o', 'ConnectTimeout=10']
            if 'ssh_key' in config:
                prefix.extend(['-i', config['ssh_key']])
            argv = [*prefix, config['ssh_host'], shlex.join(argv)]
        try:
            result = subprocess.run(argv, capture_output=True, timeout=60, check=False)
            if result.returncode or len(result.stdout) > 32 * 1024 * 1024:
                raise ReleaseError('runtime inspection failed; verify container and database access')
            return result.stdout.decode('utf-8')
        except (OSError, subprocess.TimeoutExpired, UnicodeError):
            raise ReleaseError('runtime inspection failed; verify container and database access') from None

    def api(self, config, method, route, body=None):
        key = read_admin_key(config['admin_key_file'])
        encoded = None if body is None else json.dumps(body).encode()
        request = Request(config['url'] + route, data=encoded, method=method,
                          headers={'x-api-key': key, 'Content-Type': 'application/json'})
        try:
            with build_opener(NoRedirect()).open(request, timeout=30) as response:
                raw = response.read(parity.MAX_INPUT_BYTES + 1)
            if len(raw) > parity.MAX_INPUT_BYTES:
                raise ReleaseError('admin response exceeds the size limit')
            payload = parse_json(raw)
            if not isinstance(payload, dict) or type(payload.get('code')) is not int or payload['code'] != 0:
                raise ReleaseError('admin operation failed; inspect restricted server diagnostics separately')
            if 'data' not in payload:
                raise ReleaseError('admin operation returned no data')
            return payload['data']
        except ReleaseError:
            raise
        except Exception:
            # HTTP exception text and bodies may contain response contents or request URLs.
            raise ReleaseError('admin transport failed; verify service access and authorization') from None


def postgres(config, transport, command, sql=None):
    if command == 'psql':
        script = 'exec psql --no-password -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"'
        args = ['sh', '-c', script, 'commerce-parity', sql]
    else:
        script = 'exec pg_dump --no-password --schema-only --no-owner --no-privileges -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
        args = ['sh', '-c', script]
    return transport.command(config, ['docker', 'exec', config['db_container'], *args])


SQL_LITERAL = r"'(?:[^']|'')*'"
VARCHAR_LITERAL = SQL_LITERAL + r'::character varying'
TEXT_VARCHAR_LITERAL = r'\(' + VARCHAR_LITERAL + r'\)::text'
CHECK_ARRAY_CASTS = (
    re.compile(r'\(ARRAY\[(?P<items>' + VARCHAR_LITERAL + r'(?:, ' + VARCHAR_LITERAL + r')*)\]\)::text\[\]'),
    re.compile(r'ARRAY\[(?P<items>' + TEXT_VARCHAR_LITERAL + r'(?:, ' + TEXT_VARCHAR_LITERAL + r')*)\]'),
)


def normalize_check_definition(definition):
    """Unify only PostgreSQL's two observed literal-varchar-array text coercions."""
    if not isinstance(definition, str) or not definition.startswith('CHECK '):
        return definition
    for pattern in CHECK_ARRAY_CASTS:
        literal_ranges = [(match.start(), match.end()) for match in re.finditer(SQL_LITERAL, definition)]
        def replace(match):
            if any(start <= match.start() < end for start, end in literal_ranges):
                return match.group()
            values = re.findall(SQL_LITERAL, match.group('items'))
            return 'ARRAY[' + ', '.join(value + '::text' for value in values) + ']'
        definition = pattern.sub(replace, definition)
    return definition


SCHEMA_CATEGORIES = ('relations', 'columns', 'constraints', 'indexes', 'sequences',
                     'functions', 'triggers', 'views', 'types', 'policies', 'rules', 'extensions')


def logical_schema(catalog):
    exact_object(catalog, set(SCHEMA_CATEGORIES) | {'unsupported'}, {'relations', 'columns'})
    if catalog.get('unsupported') or not catalog['relations'] or not catalog['columns']:
        raise ReleaseError('catalog contains unsupported objects or lacks required relations and columns')
    normalized = {}
    for category in SCHEMA_CATEGORIES:
        rows = catalog.get(category, [])
        if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
            raise ReleaseError('schema catalog contains invalid object records')
        entries = []
        for row in rows:
            item = dict(row)
            if category == 'constraints' and item.get('type') == 'c':
                item['definition'] = normalize_check_definition(item.get('definition'))
            entries.append(item)
        normalized[category] = sorted(entries, key=canonical_bytes)
    return normalized


def migration_map(records, label):
    if not isinstance(records, list) or not records:
        raise ReleaseError(f'{label}: provide a nonempty migration manifest')
    result = {}
    for record in records:
        exact_object(record, {'version', 'checksum'}, {'version', 'checksum'})
        version, checksum = record['version'], record['checksum']
        if (not isinstance(version, str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]*\.sql', version)
                or not isinstance(checksum, str) or not re.fullmatch(r'[0-9a-f]{64}', checksum)):
            raise ReleaseError(f'{label}: require exact SQL filenames and SHA-256 checksums')
        if version in result:
            raise ReleaseError(f'{label}: duplicate migration filename')
        result[version] = checksum
    return result


def validate_migration_history(applied, current, environment):
    required = migration_map(current, 'current_migrations')
    history = migration_map(applied, 'applied_migrations')
    authority = read_json(RETIRED_MIGRATIONS_FILE)
    exact_object(authority, {'schema_version', 'checksum_algorithm', 'retired_migrations'},
                 {'schema_version', 'checksum_algorithm', 'retired_migrations'})
    if type(authority['schema_version']) is not int or authority['schema_version'] != 1 or authority['checksum_algorithm'] != 'sha256_of_trimmed_sql':
        raise ReleaseError('retired migration authority has an unsupported schema or checksum algorithm')
    retired = {}
    if not isinstance(authority['retired_migrations'], list):
        raise ReleaseError('retired migration authority must contain explicit records')
    for record in authority['retired_migrations']:
        exact_object(record, {'filename', 'checksum', 'observed_in'}, {'filename', 'checksum', 'observed_in'})
        entry = migration_map([{'version': record['filename'], 'checksum': record['checksum']}], 'retired_migrations')
        scopes = record['observed_in']
        if (not isinstance(scopes, list) or not scopes or any(value not in ('dev', 'prod') for value in scopes)
                or len(scopes) != len(set(scopes)) or record['filename'] in retired):
            raise ReleaseError('retired migration authority contains duplicate or invalid environment scopes')
        retired[record['filename']] = (entry[record['filename']], scopes)
    if set(required) & set(retired):
        raise ReleaseError('current and retired migration authorities overlap')
    if any(history.get(version) != checksum for version, checksum in required.items()):
        raise ReleaseError('required migration is missing or mutated; align the database with the current release')
    extra = []
    for version in sorted(set(history) - set(required)):
        if version not in retired or retired[version][0] != history[version] or environment not in retired[version][1]:
            raise ReleaseError('unapproved retired migration history; review the exact filename and checksum')
        extra.append({'version': version, 'checksum': history[version]})
    return ([{'version': version, 'checksum': required[version]} for version in sorted(required)], extra)


def canonical_bytes(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False) + '\n').encode()


def binding_sha256(snapshot):
    bound = {key: value for key, value in snapshot.items()
             if key not in ('acceptance', 'purchase_enabled', 'restock_enabled')}
    bound['products'] = sorted(bound.get('products', []), key=lambda value: value['mapping_key'])
    bound['migrations'] = sorted(bound.get('migrations', []), key=lambda value: str(value['version']))
    return hashlib.sha256(canonical_bytes(bound)).hexdigest()


def acceptance_from_file(path, snapshot):
    evidence = read_json(path)
    exact_object(evidence, {'environment', 'binding_sha256', 'checks'},
                 {'environment', 'binding_sha256', 'checks'})
    if evidence['environment'] != snapshot['environment'] or evidence['binding_sha256'] != binding_sha256(snapshot):
        raise ReleaseError('acceptance binding is stale; repeat checks against the collected image and configuration')
    exact_object(evidence['checks'], set(parity.ACCEPTANCE_FIELDS), set(parity.ACCEPTANCE_FIELDS))
    flags = {}
    for field in parity.ACCEPTANCE_FIELDS:
        check = evidence['checks'][field]
        exact_object(check, {'passed', 'evidence_file', 'evidence_sha256'},
                     {'passed', 'evidence_file', 'evidence_sha256'})
        if type(check['passed']) is not bool or not isinstance(check['evidence_file'], str):
            raise ReleaseError('acceptance checks require a boolean and an evidence file')
        reference = Path(check['evidence_file'])
        if reference.is_absolute() or '..' in reference.parts or not reference.parts:
            raise ReleaseError('acceptance evidence must use a relative path within its record directory')
        try:
            resolved = (Path(path).parent / reference).resolve()
            resolved.relative_to(Path(path).parent.resolve())
            digest = hashlib.sha256()
            with resolved.open('rb') as source:
                for chunk in iter(lambda: source.read(65536), b''):
                    digest.update(chunk)
        except (OSError, ValueError):
            raise ReleaseError('unable to verify the referenced acceptance evidence file') from None
        if check['evidence_sha256'] != digest.hexdigest():
            raise ReleaseError('acceptance evidence hash mismatch; preserve and review the actual record')
        flags[field] = check['passed']
    return flags


def read_business(config, transport):
    business = transport.api(config, 'GET', PARITY_ROUTE)
    exact_object(business, API_KEYS, API_KEYS)
    return business


def collect(config, environment, transport, audit=None):
    image_id = transport.command(config, ['docker', 'inspect', '--type', 'container', '--format',
                                         '{{.Image}}', config['app_container']]).strip()
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', image_id):
        raise ReleaseError('running container does not expose a valid immutable image ID')
    labels = parse_json(transport.command(config, ['docker', 'image', 'inspect', '--format',
                                                  '{{json .Config.Labels}}', image_id]))
    if not isinstance(labels, dict):
        raise ReleaseError('image OCI provenance labels are missing')
    version = labels.get('org.opencontainers.image.version')
    revision = labels.get('org.opencontainers.image.revision')
    if not isinstance(version, str) or not version.strip() or not isinstance(revision, str) or not re.fullmatch(r'[0-9a-f]{40,64}', revision):
        raise ReleaseError('image must retain a version and full source commit in OCI labels')
    migrations = parse_json(postgres(config, transport, 'psql', MIGRATIONS_SQL))
    raw_identity = postgres(config, transport, 'psql', IDENTITY_SQL).strip()
    identity = parse_json(raw_identity)
    if (not isinstance(identity, list) or len(identity) != 2 or
            any(not isinstance(value, str) or not value for value in identity)):
        raise ReleaseError('database identity inspection is incomplete')
    schema = logical_schema(parse_json(postgres(config, transport, 'psql', SCHEMA_SQL)))
    database_identity = hashlib.sha256(raw_identity.encode()).hexdigest()
    business = read_business(config, transport)
    if business['database_identity'] != database_identity:
        raise ReleaseError('admin API and inspected database identities differ; correct the environment origin or containers')
    current_migrations, retired = validate_migration_history(migrations, business.pop('current_migrations'), environment)
    if audit is not None:
        audit['retired_migrations'] = retired
        audit['schema_objects'] = {category: len(rows) for category, rows in schema.items()}
        audit['retired_authority_sha256'] = hashlib.sha256(canonical_bytes(read_json(RETIRED_MIGRATIONS_FILE))).hexdigest()
    snapshot = dict(environment=environment, version=version, source_commit=revision,
                    image_id=image_id, migrations=current_migrations,
                    schema_hash=hashlib.sha256(canonical_bytes(schema)).hexdigest(),
                    **business)
    validator = parity.Validator()
    validator.snapshot(snapshot, environment, 'artifact')
    if validator.blockers:
        raise ReleaseError('collected snapshot violates the sanitized artifact contract; inspect configuration')
    if 'acceptance_file' in config:
        snapshot['acceptance'] = acceptance_from_file(config['acceptance_file'], snapshot)
    return snapshot


def same_business(left, right):
    ignored = {'purchase_enabled', 'restock_enabled'}
    return (migration_map(left['current_migrations'], 'current_migrations') ==
            migration_map(right['migrations'], 'current_migrations') and
            {key: left[key] for key in BUSINESS_KEYS - ignored} == {
        key: right[key] for key in BUSINESS_KEYS - ignored}
            )


def promote(config, snapshots, transport):
    changed = []
    try:
        for environment in ('dev', 'prod'):
            current = read_business(config[environment], transport)
            if not same_business(current, snapshots[environment]):
                raise ReleaseError('business configuration changed after parity; collect and validate again')
            original = snapshots[environment]['restock_enabled']
            if current['restock_enabled'] != original:
                raise ReleaseError('worker state changed after parity; collect and validate again')
            if not original:
                # Record before transmission: a lost response can still represent a committed write.
                changed.append((environment, 'restock_enabled', original))
                transport.api(config[environment], 'POST', WORKER_ROUTE, {'enabled': True})
            current = read_business(config[environment], transport)
            if current['restock_enabled'] is not True or not same_business(current, snapshots[environment]):
                raise ReleaseError('worker enablement readback failed or business configuration changed')
        current = read_business(config['prod'], transport)
        original = snapshots['prod']['purchase_enabled']
        if current['purchase_enabled'] != original:
            raise ReleaseError('purchase state changed after parity; collect and validate again')
        if not original:
            changed.append(('prod', 'purchase_enabled', original))
            transport.api(config['prod'], 'PUT', SETTINGS_ROUTE, {'purchase_subscription_enabled': True})
        for environment in ('dev', 'prod'):
            current = read_business(config[environment], transport)
            if (current['restock_enabled'] is not True or
                    (environment == 'prod' and current['purchase_enabled'] is not True) or
                    not same_business(current, snapshots[environment])):
                raise ReleaseError('final enablement readback failed; restoring attempted flags')
        return {'enabled': True, 'rollback': 'not_needed'}
    except Exception:
        rollback_failed = False
        for environment, field, original in reversed(changed):
            try:
                route = SETTINGS_ROUTE if field == 'purchase_enabled' else WORKER_ROUTE
                body = {'purchase_subscription_enabled': original} if field == 'purchase_enabled' else {'enabled': original}
                transport.api(config[environment], 'PUT' if field == 'purchase_enabled' else 'POST', route, body)
                if read_business(config[environment], transport)[field] is not original:
                    rollback_failed = True
            except Exception:
                rollback_failed = True
        raise ReleaseError('enablement failed; rollback incomplete, manual recovery required' if rollback_failed
                           else 'enablement failed; attempted flags restored and verified') from None


def execute(config, stage, enable_sales, transport):
    if enable_sales and stage != 'sales':
        raise ReleaseError('--enable-sales requires --stage sales')
    audit = {environment: {} for environment in ('dev', 'prod')}
    snapshots = {environment: collect(config[environment], environment, transport, audit[environment])
                 for environment in ('dev', 'prod')}
    hashes = {environment: hashlib.sha256(canonical_bytes(value)).hexdigest()
              for environment, value in snapshots.items()}
    result = parity.check_snapshots(snapshots['dev'], snapshots['prod'], stage, hashes)
    result['acceptance_binding_sha256'] = {environment: binding_sha256(value)
                                          for environment, value in snapshots.items()}
    result['snapshots'] = snapshots
    result['collection_audit'] = audit
    result['enablement'] = {'requested': enable_sales, 'enabled': False}
    if enable_sales:
        for environment, snapshot in snapshots.items():
            if not any(product['enabled'] is True and product['cny_amount'] == 5 and
                       parity.Decimal(product['usd_credit']) == parity.Decimal('5')
                       for product in snapshot['products']):
                result['passed'] = False
                result['blockers'].append(f'{environment}: enable the CNY 5 / USD 5 product and repeat acceptance before opening sales')
    if enable_sales and result['passed']:
        try:
            result['enablement'].update(promote(config, snapshots, transport))
        except ReleaseError as error:
            result['passed'] = False
            result['blockers'].append(str(error))
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True, type=Path)
    parser.add_argument('--stage', choices=('artifact', 'sales'), default='artifact')
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--enable-sales', action='store_true')
    args = parser.parse_args(argv)
    try:
        result = execute(load_config(args.config), args.stage, args.enable_sales, Transport())
    except ReleaseError as error:
        result = {'passed': False, 'stage': args.stage, 'blockers': [str(error)],
                  'enablement': {'requested': args.enable_sales, 'enabled': False}}
    try:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        descriptor = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
        with os.fdopen(descriptor, 'w') as output:
            os.fchmod(output.fileno(), 0o600)
            json.dump(result, output, indent=2, allow_nan=False)
            output.write('\n')
    except OSError:
        print('unable to write protected release result', file=sys.stderr)
        return 2
    print('Commerce release workflow check passed; human acceptance remains separate.' if result['passed']
          else 'Commerce release workflow blocked; inspect the protected result file.', file=sys.stderr)
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
