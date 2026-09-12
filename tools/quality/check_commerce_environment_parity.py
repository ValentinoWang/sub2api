#!/usr/bin/env python3
"""Compare sanitized commerce snapshots; a pass is not production acceptance."""

import argparse
from decimal import Decimal, InvalidOperation
import hashlib
import json
from pathlib import Path
import sys


MAX_BLOCKERS = 64
MAX_INPUT_BYTES = 2 * 1024 * 1024
IDENTITY_FIELDS = ('version', 'source_commit', 'image_id', 'schema_hash')
CONFIG_FIELDS = ('merchant_configured', 'code_secret_configured', 'purchase_url_configured', 'purchase_enabled',
                 'restock_enabled', 'reconciliation_required')
ACCEPTANCE_FIELDS = ('upload', 'delivery', 'redeem', 'duplicate_redeem', 'refill',
                     'duplicate_refill', 'inventory_reconciled')
RULE_FIELDS = ('credit_currency', 'goods_currency', 'goods_to_credit_ratio',
               'native_balance_multiplier', 'refund_policy', 'restock_interval_seconds')
PRODUCT_FIELDS = ('mapping_key', 'version', 'goods_id', 'cny_amount', 'usd_credit',
                  'grant_type', 'target_stock', 'threshold', 'restock_count', 'enabled')
SNAPSHOT_FIELDS = (*IDENTITY_FIELDS, *CONFIG_FIELDS, 'environment', 'database_identity',
                   'code_secret_digest', 'business_rules', 'migrations', 'products', 'acceptance')


class Validator:
    def __init__(self):
        self.blockers = []

    def error(self, path, instruction):
        # Paths contain only fixed schema names and numeric indexes, never input values.
        if len(self.blockers) < MAX_BLOCKERS:
            self.blockers.append(f'{path}: {instruction}')

    def object(self, value, path, fields):
        if not isinstance(value, dict):
            self.error(path, 'provide a JSON object')
            return {}
        if set(value) - set(fields):
            self.error(path, 'remove fields outside the sanitized snapshot contract')
        return value

    def string(self, value, path, allow_empty=False):
        if not isinstance(value, str) or len(value) > 1024 or (
                not allow_empty and not value.strip()) or value != value.strip():
            self.error(path, 'provide a nonempty trimmed string' if not allow_empty
                       else 'provide a trimmed string')
            return None
        return value

    def boolean(self, value, path):
        if type(value) is not bool:
            self.error(path, 'provide a JSON boolean')
            return None
        return value

    def integer(self, value, path, minimum=1):
        if type(value) is not int or value < minimum:
            self.error(path, f'provide an integer of at least {minimum}')
            return None
        return value

    def decimal(self, value, path, string=False):
        valid_type = isinstance(value, str) if string else type(value) in (int, float, Decimal)
        if not valid_type or len(str(value)) > 256:
            self.error(path, 'provide a finite positive decimal string' if string
                       else 'provide a finite positive JSON number')
            return None
        try:
            number = Decimal(str(value))
        except InvalidOperation:
            self.error(path, 'provide a finite positive decimal')
            return None
        if not number.is_finite() or number <= 0:
            self.error(path, 'provide a finite positive decimal')
            return None
        return number

    def array(self, value, path):
        if not isinstance(value, list) or len(value) > 4096:
            self.error(path, 'provide an array with at most 4096 entries')
            return []
        return value

    def snapshot(self, value, environment, stage):
        data = self.object(value, environment, SNAPSHOT_FIELDS)
        normalized = {}
        if data.get('environment') != environment:
            self.error(f'{environment}.environment', 'use the environment label matching the CLI input')
        for key in (*IDENTITY_FIELDS, 'database_identity'):
            normalized[key] = self.string(data.get(key), f'{environment}.{key}')

        migrations = {}
        migration_entries = self.array(data.get('migrations'), f'{environment}.migrations')
        if not migration_entries:
            self.error(f'{environment}.migrations', 'provide the nonempty applied migration history')
        for i, raw in enumerate(migration_entries):
            path = f'{environment}.migrations[{i}]'
            migration = self.object(raw, path, ('version', 'checksum'))
            version = migration.get('version')
            if type(version) is int:
                version = self.integer(version, f'{path}.version', minimum=0)
                version = str(version) if version is not None else None
            else:
                version = self.string(version, f'{path}.version')
            checksum = self.string(migration.get('checksum'), f'{path}.checksum')
            if version is not None:
                if version in migrations:
                    self.error(path, 'remove the duplicate migration version')
                migrations[version] = checksum
        normalized['migrations'] = migrations

        rules = self.object(data.get('business_rules'), f'{environment}.business_rules', RULE_FIELDS)
        normalized_rules = {}
        for key, expected in (('credit_currency', 'USD'), ('goods_currency', 'CNY'),
                              ('refund_policy', 'unused_only')):
            if rules.get(key) != expected:
                self.error(f'{environment}.business_rules.{key}', f'set the required value {expected}')
            normalized_rules[key] = rules.get(key)
        for key in ('goods_to_credit_ratio', 'native_balance_multiplier'):
            normalized_rules[key] = self.decimal(rules.get(key), f'{environment}.business_rules.{key}',
                                                 string=True)
        if normalized_rules['goods_to_credit_ratio'] != Decimal('1'):
            self.error(f'{environment}.business_rules.goods_to_credit_ratio', 'set the ratio to 1')
        interval = self.integer(rules.get('restock_interval_seconds'),
                                f'{environment}.business_rules.restock_interval_seconds')
        if interval != 300:
            self.error(f'{environment}.business_rules.restock_interval_seconds', 'set the interval to 300 seconds')
        normalized_rules['restock_interval_seconds'] = interval
        normalized['business_rules'] = normalized_rules

        for key in CONFIG_FIELDS:
            if stage == 'sales' or key in data:
                flag = self.boolean(data.get(key), f'{environment}.{key}')
                if stage == 'sales' and key in ('merchant_configured', 'code_secret_configured', 'purchase_url_configured') and flag is not True:
                    self.error(f'{environment}.{key}', 'complete the required configuration before opening sales')
                if stage == 'sales' and key == 'reconciliation_required' and flag is not False:
                    self.error(f'{environment}.{key}', 'complete reconciliation before opening sales')
        if stage == 'sales' or 'code_secret_digest' in data:
            normalized['code_secret_digest'] = self.string(data.get('code_secret_digest'),
                                                           f'{environment}.code_secret_digest',
                                                           allow_empty=stage == 'artifact')

        products, goods_ids = {}, set()
        has_five = False
        if stage == 'sales' or 'products' in data:
            entries = self.array(data.get('products'), f'{environment}.products')
            if stage == 'sales' and not entries:
                self.error(f'{environment}.products', 'configure at least one balance product')
            for i, raw in enumerate(entries):
                path = f'{environment}.products[{i}]'
                product = self.object(raw, path, PRODUCT_FIELDS)
                key = self.string(product.get('mapping_key'), f'{path}.mapping_key')
                version = self.integer(product.get('version'), f'{path}.version')
                goods_id = self.integer(product.get('goods_id'), f'{path}.goods_id')
                amount = self.decimal(product.get('cny_amount'), f'{path}.cny_amount')
                credit = self.decimal(product.get('usd_credit'), f'{path}.usd_credit', string=True)
                stock = self.integer(product.get('target_stock'), f'{path}.target_stock')
                threshold = self.integer(product.get('threshold'), f'{path}.threshold', minimum=0)
                restock_count = self.integer(product.get('restock_count'), f'{path}.restock_count')
                enabled = self.boolean(product.get('enabled'), f'{path}.enabled')
                if product.get('grant_type') != 'balance':
                    self.error(f'{path}.grant_type', 'use balance for the recharge mapping')
                if stage == 'sales':
                    if amount is not None and credit is not None and amount != credit:
                        self.error(path, 'align the CNY goods amount with the USD credit amount at 1:1')
                    if stock is not None and stock > 10:
                        self.error(f'{path}.target_stock', 'use at most 10 units for initial sales acceptance')
                    if restock_count is not None and restock_count > 10:
                        self.error(f'{path}.restock_count', 'use at most 10 units for initial replenishment')
                if threshold is not None and stock is not None and threshold > stock:
                    self.error(f'{path}.threshold', 'set the restock threshold at or below target stock')
                has_five |= amount == Decimal('5') and credit == Decimal('5')
                if goods_id is not None:
                    if goods_id in goods_ids:
                        self.error(path, 'assign a unique goods ID within this environment')
                    goods_ids.add(goods_id)
                if key is not None:
                    if key in products:
                        self.error(path, 'assign a unique logical mapping key')
                    products[key] = (version, amount, credit, product.get('grant_type'), stock,
                                     threshold, restock_count, enabled)
            if stage == 'sales' and not has_five:
                self.error(f'{environment}.products', 'include a CNY 5 product granting USD 5')
        normalized['products'] = products
        normalized['goods_ids'] = goods_ids

        if stage == 'sales' or 'acceptance' in data:
            acceptance = self.object(data.get('acceptance'), f'{environment}.acceptance', ACCEPTANCE_FIELDS)
            for key in ACCEPTANCE_FIELDS:
                flag = self.boolean(acceptance.get(key), f'{environment}.acceptance.{key}')
                if stage == 'sales' and flag is not True:
                    self.error(f'{environment}.acceptance.{key}', 'record a successful check before opening sales')
        return normalized


def check_snapshots(dev, prod, stage='artifact', snapshot_sha256=None):
    validator = Validator()
    if stage not in ('artifact', 'sales'):
        validator.error('stage', 'select artifact or sales')
    else:
        left = validator.snapshot(dev, 'dev', stage)
        right = validator.snapshot(prod, 'prod', stage)
        for key in (*IDENTITY_FIELDS, 'migrations', 'business_rules'):
            if left[key] != right[key]:
                validator.error(key, 'align dev and prod before promotion')
        if left['database_identity'] == right['database_identity']:
            validator.error('database_identity', 'use separate databases for balances, orders and codes')
        if stage == 'sales':
            if left.get('code_secret_digest') == right.get('code_secret_digest'):
                validator.error('code_secret_digest', 'configure a different code secret in each environment')
            if left['products'] != right['products']:
                validator.error('products', 'align logical mapping keys, versions, amounts, grant types, stock, restock policies and enabled status')
            if left['goods_ids'] & right['goods_ids']:
                validator.error('products.goods_id', 'use disjoint merchant goods IDs in dev and prod')
    return {'passed': not validator.blockers, 'stage': stage, 'blockers': validator.blockers,
            'snapshot_sha256': snapshot_sha256 or {'dev': None, 'prod': None}}


def reject_constant(_value):
    raise ValueError('non-finite JSON number')


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate JSON field')
        result[key] = value
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--dev', required=True, type=Path)
    parser.add_argument('--prod', required=True, type=Path)
    parser.add_argument('--stage', choices=('artifact', 'sales'), default='artifact')
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args(argv)
    snapshots, digests, blockers = {}, {}, []
    for environment in ('dev', 'prod'):
        try:
            with getattr(args, environment).open('rb') as source:
                raw = source.read(MAX_INPUT_BYTES + 1)
            if len(raw) > MAX_INPUT_BYTES:
                raise ValueError('snapshot size limit')
            digests[environment] = hashlib.sha256(raw).hexdigest()
            snapshots[environment] = json.loads(raw, parse_float=Decimal,
                                                 parse_constant=reject_constant,
                                                 object_pairs_hook=unique_object)
        except (OSError, ValueError, InvalidOperation, RecursionError):
            blockers.append(f'{environment}: supply a readable valid sanitized JSON snapshot under 2 MiB')
            digests.setdefault(environment, None)
    if blockers:
        result = {'passed': False, 'stage': args.stage, 'blockers': blockers, 'snapshot_sha256': digests}
    else:
        result = check_snapshots(snapshots['dev'], snapshots['prod'], args.stage, digests)
    try:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
    except OSError:
        print('output: unable to write the parity result file', file=sys.stderr)
        return 2
    print('Commerce parity passed; this is not production acceptance.' if result['passed']
          else 'Commerce parity blocked; inspect the result file.', file=sys.stderr)
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    sys.exit(main())
