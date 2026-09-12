#!/usr/bin/env python3
"""Exercise promotion blockers using isolated sanitized snapshot fixtures."""

import copy
from decimal import Decimal
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / 'check_commerce_environment_parity.py'
SPEC = importlib.util.spec_from_file_location('commerce_parity', SCRIPT)
PARITY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PARITY)


def snapshot(environment):
    return {
        'environment': environment,
        'version': '0.2.4.9',
        'source_commit': 'a' * 40,
        'image_id': 'sha256:' + 'b' * 64,
        'schema_hash': 'c' * 64,
        'migrations': [{'version': '001', 'checksum': 'd' * 64},
                       {'version': '002', 'checksum': 'e' * 64}],
        'database_identity': 'fixture-db-' + environment,
        'code_secret_digest': ('f' if environment == 'dev' else '0') * 64,
        'business_rules': {
            'credit_currency': 'USD', 'goods_currency': 'CNY', 'goods_to_credit_ratio': '1',
            'native_balance_multiplier': '1', 'refund_policy': 'unused_only',
            'restock_interval_seconds': 300,
        },
        'products': [
            {'mapping_key': 'balance-five', 'version': 1,
             'goods_id': 101 if environment == 'dev' else 201,
             'cny_amount': 5, 'usd_credit': '5', 'grant_type': 'balance',
             'target_stock': 5, 'threshold': 2, 'restock_count': 3, 'enabled': True},
            {'mapping_key': 'balance-ten', 'version': 1,
             'goods_id': 102 if environment == 'dev' else 202,
             'cny_amount': 10, 'usd_credit': '10', 'grant_type': 'balance',
             'target_stock': 5, 'threshold': 2, 'restock_count': 3, 'enabled': True},
        ],
        'merchant_configured': True, 'code_secret_configured': True, 'purchase_url_configured': True,
        'purchase_enabled': False, 'restock_enabled': False, 'reconciliation_required': False,
        'acceptance': dict.fromkeys(PARITY.ACCEPTANCE_FIELDS, True),
    }


class CommerceParityTest(unittest.TestCase):
    def setUp(self):
        self.dev = snapshot('dev')
        self.prod = snapshot('prod')

    def check(self, stage='sales'):
        return PARITY.check_snapshots(self.dev, self.prod, stage)

    def assertBlocked(self, field, stage='sales'):
        result = self.check(stage)
        self.assertFalse(result['passed'])
        self.assertTrue(any(field in item for item in result['blockers']), result)
        return result

    def test_sales_passes_with_lifecycle_and_isolation_differences(self):
        self.prod['purchase_enabled'] = True
        self.prod['restock_enabled'] = True
        self.assertTrue(self.check()['passed'])

    def test_artifact_does_not_require_sales_fields(self):
        for data in (self.dev, self.prod):
            for field in (*PARITY.CONFIG_FIELDS, 'products', 'code_secret_digest', 'acceptance'):
                del data[field]
        self.assertTrue(self.check('artifact')['passed'])
        self.assertBlocked('merchant_configured')

    def test_artifact_permits_unconfigured_sales_and_false_acceptance(self):
        for data in (self.dev, self.prod):
            data.update(products=[], merchant_configured=False, code_secret_configured=False,
                        code_secret_digest='', reconciliation_required=True)
            data['acceptance'] = dict.fromkeys(PARITY.ACCEPTANCE_FIELDS, False)
        self.assertTrue(self.check('artifact')['passed'])
        self.assertBlocked('acceptance')

    def test_reordered_arrays_and_equivalent_decimal_strings_pass(self):
        self.prod['migrations'].reverse()
        self.prod['products'].reverse()
        self.prod['business_rules']['native_balance_multiplier'] = '1.00'
        self.prod['business_rules']['goods_to_credit_ratio'] = '1e0'
        self.prod['products'][0]['usd_credit'] = '10.00'
        self.prod['products'][1]['cny_amount'] = Decimal('5.00')
        self.assertTrue(self.check()['passed'])

    def test_artifact_identity_drift_blocks(self):
        for field in PARITY.IDENTITY_FIELDS:
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod[field] = 'different'
                self.assertBlocked(field, 'artifact')

    def test_migration_checksum_or_missing_migration_blocks(self):
        self.prod['migrations'][0]['checksum'] = 'different'
        self.assertBlocked('migrations', 'artifact')
        self.prod = snapshot('prod')
        self.prod['migrations'].pop()
        self.assertBlocked('migrations', 'artifact')

    def test_duplicate_migrations_block_even_if_both_snapshots_agree(self):
        for data in (self.dev, self.prod):
            data['migrations'].append(copy.deepcopy(data['migrations'][0]))
        self.assertBlocked('duplicate migration', 'artifact')

    def test_empty_migration_histories_block(self):
        for data in (self.dev, self.prod):
            data['migrations'] = []
        self.assertBlocked('migrations', 'artifact')

    def test_business_rule_drift_blocks(self):
        for field, value in (('credit_currency', 'CNY'), ('goods_currency', 'USD'),
                             ('refund_policy', 'any'), ('goods_to_credit_ratio', '2'),
                             ('native_balance_multiplier', '2'), ('restock_interval_seconds', 60)):
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod['business_rules'][field] = value
                self.assertBlocked('business_rules', 'artifact')

    def test_same_wrong_business_rules_still_block(self):
        for data in (self.dev, self.prod):
            data['business_rules']['goods_to_credit_ratio'] = '2'
        self.assertBlocked('goods_to_credit_ratio', 'artifact')

    def test_reused_database_blocks_both_stages(self):
        self.prod['database_identity'] = self.dev['database_identity']
        for stage in ('artifact', 'sales'):
            self.assertBlocked('database_identity', stage)

    def test_reused_code_secret_blocks_sales(self):
        self.prod['code_secret_digest'] = self.dev['code_secret_digest']
        self.assertBlocked('code_secret_digest')

    def test_reused_goods_id_across_different_mapping_keys_blocks(self):
        self.prod['products'][1]['goods_id'] = self.dev['products'][0]['goods_id']
        self.assertBlocked('products.goods_id')

    def test_duplicate_product_keys_and_ids_block(self):
        for field in ('mapping_key', 'goods_id'):
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod['products'][1][field] = self.prod['products'][0][field]
                self.assertBlocked('products')

    def test_all_logical_mapping_drift_blocks(self):
        for field, value in (('mapping_key', 'another'), ('version', 2), ('cny_amount', 6),
                             ('usd_credit', '6'), ('grant_type', 'subscription'),
                             ('target_stock', 6), ('threshold', 1), ('restock_count', 2),
                             ('enabled', False)):
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod['products'][0][field] = value
                self.assertBlocked('products')

    def test_equal_but_incorrect_credit_amounts_block(self):
        for data in (self.dev, self.prod):
            data['products'][0]['usd_credit'] = '50'
        self.assertBlocked('1:1')

    def test_decimal_precision_drift_is_not_rounded_away(self):
        self.prod['products'][0]['usd_credit'] = '5.0000000000000000000000000001'
        self.assertBlocked('1:1')

    def test_five_yuan_product_is_required(self):
        for data in (self.dev, self.prod):
            data['products'].pop(0)
        self.assertBlocked('CNY 5')

    def test_small_initial_stock_is_required_even_when_equal(self):
        for data in (self.dev, self.prod):
            data['products'][0]['target_stock'] = 11
        self.assertBlocked('target_stock')

    def test_small_initial_restock_count_is_required(self):
        for data in (self.dev, self.prod):
            data['products'][0]['restock_count'] = 11
        self.assertBlocked('restock_count')

    def test_threshold_must_not_exceed_stock(self):
        for data in (self.dev, self.prod):
            data['products'][0]['threshold'] = 6
        self.assertBlocked('threshold')

    def test_disabled_products_can_pass_before_enablement(self):
        for data in (self.dev, self.prod):
            for product in data['products']:
                product['enabled'] = False
        self.assertTrue(self.check()['passed'])

    def test_unconfigured_credentials_or_reconciliation_blocks(self):
        for field, value in (('merchant_configured', False), ('code_secret_configured', False),
                             ('purchase_url_configured', False),
                             ('code_secret_digest', ''), ('reconciliation_required', True)):
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod[field] = value
                self.assertBlocked(field)

    def test_every_missing_or_false_acceptance_blocks(self):
        for field in PARITY.ACCEPTANCE_FIELDS:
            for missing in (False, True):
                with self.subTest(field=field, missing=missing):
                    self.prod = snapshot('prod')
                    if missing:
                        del self.prod['acceptance'][field]
                    else:
                        self.prod['acceptance'][field] = False
                    self.assertBlocked('acceptance.' + field)

    def test_missing_required_fields_block(self):
        for field in PARITY.SNAPSHOT_FIELDS:
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                del self.prod[field]
                self.assertBlocked(field)

    def test_nonfinite_nonpositive_and_wrong_type_amounts_block(self):
        for field, values in (
            ('cny_amount', [float('nan'), float('inf'), -5, 0, True, '5', None, {}, []]),
            ('usd_credit', ['NaN', 'sNaN', 'Infinity', '-Infinity', '-5', '0', 'bad', 5, True, None]),
        ):
            for value in values:
                with self.subTest(field=field, value=value):
                    self.prod = snapshot('prod')
                    self.prod['products'][0][field] = value
                    self.assertBlocked(field)

    def test_ill_typed_identity_and_flags_block(self):
        for field, value in (('version', None), ('source_commit', ''), ('image_id', []),
                             ('schema_hash', {}), ('database_identity', ' '),
                             ('merchant_configured', 1), ('purchase_enabled', 'false'),
                             ('restock_enabled', 0), ('reconciliation_required', None)):
            with self.subTest(field=field):
                self.prod = snapshot('prod')
                self.prod[field] = value
                self.assertBlocked(field)

    def test_bool_is_not_integer(self):
        for field in ('version', 'goods_id', 'target_stock', 'threshold', 'restock_count'):
            self.prod = snapshot('prod')
            self.prod['products'][0][field] = True
            self.assertBlocked(field)

    def test_unknown_fields_are_rejected_without_echoing_values_or_keys(self):
        marker = 'fixture-secret-do-not-display'
        self.prod[marker] = marker
        self.prod['products'][0]['grant_type'] = marker
        self.prod['image_id'] = marker
        result = self.assertBlocked('contract')
        self.assertNotIn(marker, json.dumps(result))

    def test_error_count_is_bounded(self):
        self.prod['products'] = [{} for _ in range(100)]
        result = self.assertBlocked('products')
        self.assertLessEqual(len(result['blockers']), PARITY.MAX_BLOCKERS)

    def test_invalid_roots_are_blocked(self):
        for value in (None, [], True, 'invalid', 3):
            self.prod = value
            self.assertBlocked('prod')


class CommerceParityCLITest(unittest.TestCase):
    def run_cli(self, dev_raw=None, prod_raw=None):
        with tempfile.TemporaryDirectory(prefix='commerce-parity-test-') as directory:
            root = Path(directory)
            dev_raw = dev_raw if dev_raw is not None else json.dumps(snapshot('dev')).encode()
            prod_raw = prod_raw if prod_raw is not None else json.dumps(snapshot('prod')).encode()
            (root / 'dev.json').write_bytes(dev_raw)
            (root / 'prod.json').write_bytes(prod_raw)
            output = root / 'result.json'
            process = subprocess.run([sys.executable, str(SCRIPT), '--dev', str(root / 'dev.json'),
                                      '--prod', str(root / 'prod.json'), '--stage', 'sales',
                                      '--output', str(output)], capture_output=True, text=True)
            return process, json.loads(output.read_text()), dev_raw, prod_raw

    def test_success_binds_exact_snapshot_bytes(self):
        process, result, dev_raw, prod_raw = self.run_cli()
        self.assertEqual(process.returncode, 0, process.stderr)
        self.assertTrue(result['passed'])
        self.assertEqual(result['stage'], 'sales')
        self.assertEqual(result['snapshot_sha256'],
                         {'dev': hashlib.sha256(dev_raw).hexdigest(),
                          'prod': hashlib.sha256(prod_raw).hexdigest()})
        self.assertIn('not production acceptance', process.stderr)

    def test_drift_exits_nonzero(self):
        prod = snapshot('prod')
        prod['image_id'] = 'different'
        process, result, _, _ = self.run_cli(prod_raw=json.dumps(prod).encode())
        self.assertNotEqual(process.returncode, 0)
        self.assertFalse(result['passed'])

    def test_malformed_nonfinite_duplicate_and_oversized_json_fail_closed(self):
        for raw in (b'{broken fixture-secret', b'{"x":NaN}', b'{"x":Infinity}',
                    b'{"x":1,"x":2}', b'{"x":1e999999999999999999999999}',
                    b'\xff', b'[' * 2000,
                    b' ' * (PARITY.MAX_INPUT_BYTES + 1)):
            with self.subTest(length=len(raw)):
                process, result, _, _ = self.run_cli(prod_raw=raw)
                self.assertNotEqual(process.returncode, 0)
                self.assertFalse(result['passed'])
                self.assertNotIn('fixture-secret', json.dumps(result) + process.stderr)

    def test_json_decimal_precision_is_preserved(self):
        prod_raw = json.dumps(snapshot('prod')).replace('"cny_amount": 5,',
                    '"cny_amount": 5.0000000000000000000000000001,').encode()
        process, result, _, _ = self.run_cli(prod_raw=prod_raw)
        self.assertNotEqual(process.returncode, 0)
        self.assertTrue(any('1:1' in item for item in result['blockers']))


if __name__ == '__main__':
    unittest.main()
