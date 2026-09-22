#!/usr/bin/env python3
"""Maintain local test-product stock using the existing Sub2API device/batch API."""
import argparse
from contextlib import contextmanager
from decimal import Decimal, InvalidOperation
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import signal
import subprocess
import sys
import threading
import time
from urllib.error import HTTPError

import ldxp_http_probe as probe

PRIVATE = Path.home() / '.local/share/sub2api/ldxp-http-restock'
CONFIG = Path(__file__).with_name('ldxp-restock.local.json')
RECHECKABLE_ERRORS = frozenset({'network_error', 'non_json', 'upstream_unavailable'})
RECOVERABLE_PAUSES = RECHECKABLE_ERRORS | {'uncertain', 'recovery_wait'}
MESSAGES = {
    'binding_changed': '商品、站点或设备绑定发生变化，请重新核对配置。',
    'inventory_mismatch': '小铺库存与本站可兑换额度不一致，补货已暂停。',
    'uncertain': '原批次上传结果未核实，已暂停；只核对，不自动重传。',
    'backend_error': '本站补货接口请求失败，已停止本轮。',
    'backend_auth': '本站设备授权失效，请重新配置设备。',
    'busy': '已有补货进程运行。',
    'manual': '已暂停，运行 resume 可重新核对并恢复。',
    'setup_required': '请先运行 setup，自动绑定本地测试商品。',
    'disabled': '本站已关闭补货。',
    'invalid_config': '配置无效，仅支持本地站点和 1–999 张目标库存。',
    'state_invalid': '补货状态文件无效，已停止操作。',
    'recovery_wait': '原批次正在等待下一次核对或重试，不会生成替代兑换码。',
    'recovery_exhausted': '原批次重试次数已用完，请查看故障记录。',
    'dedup_unverified': '商户跨未售、已售库存去重能力尚未验证，禁止重传。',
    'backend_upgrade_required': '本站后端尚不支持已售卡密交付证明，请更新本地后端。',
}
MANUAL_VERIFICATION_PAUSES = RECHECKABLE_ERRORS | {'verification_required', 'login_required', 'login_rejected'}


class RestockError(Exception):
    def __init__(self, kind):
        self.kind = kind
        super().__init__(MESSAGES[kind])


def require(value, kind='binding_changed'):
    if not value:
        raise RestockError(kind)


def load_config(path):
    try:
        config = json.loads(Path(path).read_text())
        require(config['site'] == probe.LOCAL_SITE, 'invalid_config')
        for key, low, high in [('target_stock', 1, 999), ('batch_size', 1, 20), ('interval_seconds', 30, 3600)]:
            require(type(config[key]) is int and low <= config[key] <= high, 'invalid_config')
        ids = config['goods_ids']
        require(ids == 'all' or (isinstance(ids, list) and 0 < len(ids) <= 30 and all(type(i) is int and i > 0 for i in ids) and len(set(ids)) == len(ids)), 'invalid_config')
        recovery = config.get('recovery', {})
        require(isinstance(recovery, dict) and type(recovery.get('enabled', False)) is bool, 'invalid_config')
        if recovery.get('enabled'):
            require(type(recovery.get('max_attempts')) is int and 1 <= recovery['max_attempts'] <= 5, 'invalid_config')
            require(type(recovery.get('retry_seconds')) is int and 30 <= recovery['retry_seconds'] <= 900, 'invalid_config')
        return config
    except (OSError, ValueError, KeyError, TypeError) as exc:
        raise RestockError('invalid_config') from exc


class Backend:
    def __init__(self, key, *, admin=False):
        require(bool(re.fullmatch(r'admin-[a-f0-9]{64}' if admin else r'ldxpd_[a-f0-9]{64}', key)), 'backend_auth')
        self.key, self.admin = key, admin
        self.http = probe.opener()

    def call(self, route, body=None, method=None):
        # A fixed loopback origin prevents forwarding a device or admin secret elsewhere.
        prefix = '/api/v1/admin/tools/ldxp/browser' if self.admin else '/api/v1/ldxp/device'
        allowed = {'/status', '/config', '/devices'} if self.admin else {'/config', '/stock-target', '/inventory', '/heartbeat', '/runtime', '/recheck/claim', '/resume', '/claim'}
        require(route in allowed or (not self.admin and (
            re.fullmatch(r'/batches/[A-Za-z0-9_-]{1,128}/(start|result)', route) or
            re.fullmatch(r'/recheck/[a-f0-9]{32}/result', route))), 'backend_error')
        headers = {'Content-Type': 'application/json', 'Accept': 'application/json'}
        headers['x-api-key' if self.admin else 'Authorization'] = self.key if self.admin else 'Bearer ' + self.key
        request = probe.Request(probe.LOCAL_SITE + prefix + route, headers=headers, data=None if body is None else json.dumps(body).encode(), method=method or ('GET' if body is None else 'POST'))
        try:
            with self.http.open(request, timeout=20) as response:
                raw = response.read(2_000_001)
            require(len(raw) <= 2_000_000, 'backend_error')
            result = json.loads(raw)
            require(result['code'] == 0 and isinstance(result['data'], dict), 'backend_error')
            return result['data']
        except HTTPError as exc:
            raise RestockError('backend_auth' if exc.code in (401, 403) else 'backend_error') from exc
        except (OSError, ValueError, KeyError, TypeError, probe.ProbeError) as exc:
            raise RestockError('backend_error') from exc


class Merchant(probe.Merchant):
    def rows(self, route, filters):
        # One page covers the configured 999-card target, keeping verification fresh
        # across all products without weakening the complete double scan.
        page_size = 1000 if route == '/merchantApi/goodsCardStorage/list' else 100
        return super().rows(route, filters, page_size=page_size)

    def upload(self, goods_id, codes):
        require(type(goods_id) is int and goods_id > 0)
        require(isinstance(codes, list) and 0 < len(codes) <= 20 and len(set(codes)) == len(codes))
        require(all(isinstance(code, str) and re.fullmatch(r'[a-f0-9]{32}', code) for code in codes))
        return self._request('/merchantApi/GoodsCardStorage/add', {'goods_id': goods_id, 'content': '\n'.join(codes), 'first': 0, 'remove_repeat': 1})

    def delivery_snapshot(self, goods_id):
        def scan():
            groups, card_ids = [], set()
            for status in ('0', '1'):
                records = {}
                for row in self.rows('/merchantApi/goodsCardStorage/list', {'goods_id': goods_id, 'status': status, 'first': '', 'keywords': ''}):
                    code, card_id = row.get('secret'), row.get('id')
                    require(isinstance(code, str) and code and code.strip() == code and '\n' not in code and '\r' not in code, 'inventory_mismatch')
                    require(type(card_id) is int and card_id > 0 and card_id not in card_ids, 'inventory_mismatch')
                    require(str(row.get('status')) == status, 'inventory_mismatch')
                    require('goods_id' not in row or str(row['goods_id']) == str(goods_id), 'inventory_mismatch')
                    digest = hashlib.sha256(code.encode()).hexdigest()
                    require(digest not in records, 'inventory_mismatch')
                    records[digest] = card_id
                    card_ids.add(card_id)
                groups.append(records)
            require(not set(groups[0]) & set(groups[1]), 'inventory_mismatch')
            return {'unsold': groups[0], 'sold': groups[1]}
        first, second = scan(), scan()
        require(first == second, 'recovery_wait')
        return second


def verify_dedup_capability(directory, merchant, goods_ids):
    path = Path(directory) / 'dedup-capability.json'
    require(path.exists(), 'dedup_unverified')
    capability = json.loads(probe.private_read(path))
    require(capability.get('schema') == 1 and capability.get('origin') == probe.ORIGIN, 'dedup_unverified')
    require(capability.get('scope') == 'goods_unsold_and_sold' and
            set(goods_ids).issubset(set(capability.get('goods_ids', []))), 'dedup_unverified')
    require(0 <= time.time() - capability.get('verified_at', 0) <= 30 * 86400, 'dedup_unverified')
    require(capability.get('cases') == {'existing_unsold': True, 'existing_sold': True, 'concurrent_new': True}, 'dedup_unverified')
    account = merchant.post('/merchantApi/user/userinfo', {})
    subject = hashlib.sha256(str(account.get('id')).encode()).hexdigest()
    require(account.get('id') is not None and subject == capability.get('merchant_subject_sha256'), 'dedup_unverified')


def report_delivery(backend, goods_id, batch, snapshot):
    unsold = sorted(snapshot['unsold'])
    proofs = [{'code_hash': h, 'card_id': snapshot['sold'][h]} for h in batch['code_hashes'] if h in snapshot['sold']]
    heartbeat = backend.call('/heartbeat', {'authorization': 'verified'})
    outcome = backend.call('/inventory', {'goods_id': goods_id, 'batch_id': batch['batch_id'],
        'complete': True, 'total': len(unsold), 'hashes': unsold, 'sold_complete': True, 'sold_proofs': proofs})
    if proofs:
        require(outcome.get('delivery_proof_version') == 1, 'backend_upgrade_required')
    require(outcome.get('identity_verified') is True and outcome.get('matched_stock') == len(unsold), 'inventory_mismatch')
    outcome['_device_paused_reason'] = heartbeat.get('paused_reason', '')
    return outcome


def recover_batches(state, directory, merchant, backend, persist, policy, *, write):
    # Verify every product before a recovery upload, including products without an open batch.
    pending = []
    products = []
    for key, local in state['products'].items():
        require(not write or not (Path(directory) / 'pause').exists(), 'manual')
        outcome, stock = reconcile(backend, merchant, int(key), local.get('batch_id'))
        require(not write or outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
        require(outcome['identity_verified'] and outcome['matched_stock'] == stock, 'inventory_mismatch')
        local.update(stock=stock, matched_stock=stock, target_stock=outcome['target_stock'])
        batch = outcome.get('pending_batch')
        if local.get('batch_id') and outcome['batch_resolved']:
            local.pop('batch_id', None)
            local['phase'] = 'idle'
        if batch:
            verify_batch(batch, int(key))
            require(not local.get('batch_id') or local['batch_id'] == batch['batch_id'], 'uncertain')
            if not local.get('batch_id') and local.get('recovery_attempt', {}).get('batch_id') != batch['batch_id']:
                local.pop('recovery_attempt', None)
            local.update(batch_id=batch['batch_id'], phase='uncertain' if batch['status'] == 'uncertain' else 'claimed')
            pending.append((key, batch))
        else:
            require(not outcome['blocked'] and not local.get('batch_id'), 'uncertain')
        products.append({'goods_id': int(key), 'stock': stock, 'matched_stock': stock, 'uploaded': 0})
        persist()

    # A later product's sold-proof error must be found before repairing an earlier product.
    retry_goods = []
    for key, batch in pending:
        snapshot = merchant.delivery_snapshot(int(key))
        outcome = report_delivery(backend, int(key), batch, snapshot)
        require(not write or outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
        if batch['status'] == 'uncertain' and not outcome.get('batch_resolved'):
            if write:
                require(outcome.get('delivery_proof_version') == 1, 'backend_upgrade_required')
                require(outcome.get('retry_eligible') is True, 'uncertain')
                require(outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
                attempt = state['products'][key].get('recovery_attempt', {})
                require(not attempt or attempt.get('batch_id') == batch['batch_id'], 'state_invalid')
                count = attempt.get('count', 0)
                require(type(count) is int and count < policy['max_attempts'], 'recovery_exhausted')
                require(time.time() >= attempt.get('next_at', 0), 'recovery_wait')
            retry_goods.append(int(key))
    if write and retry_goods:
        verify_dedup_capability(directory, merchant, retry_goods)

    uploaded = 0
    for key, batch in sorted(pending, key=lambda item: item[1]['status'] != 'uncertain'):
        local, goods_id = state['products'][key], int(key)
        if batch['status'] == 'claimed':
            require(write, 'recovery_wait')
            require(not (Path(directory) / 'pause').exists(), 'manual')
            backend.call('/resume', {})
            local['phase'] = 'uncertain'
            persist()
            started = backend.call('/batches/' + batch['batch_id'] + '/start', {})['batch']
            verify_batch(started, goods_id)
            require(started['batch_id'] == batch['batch_id'] and started['status'] == 'uncertain' and started['code_hashes'] == batch['code_hashes'], 'uncertain')
            require(not (Path(directory) / 'pause').exists(), 'manual')
            try:
                merchant.upload(goods_id, batch['codes'])
            except probe.ProbeError as exc:
                if exc.kind not in ('network_error', 'upstream_rejected', 'non_json'):
                    raise
            # A first upload that remains unconfirmed waits for another cycle; never send twice here.
            first_snapshot = merchant.delivery_snapshot(goods_id)
            first_outcome = report_delivery(backend, goods_id, batch, first_snapshot)
            require(first_outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
            require(first_outcome.get('batch_resolved') is True, 'recovery_wait')
            uploaded += len(batch['codes'])
        snapshot = merchant.delivery_snapshot(goods_id)
        outcome = report_delivery(backend, goods_id, batch, snapshot)
        require(not write or outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
        missing = [c for c, h in zip(batch['codes'], batch['code_hashes']) if h not in snapshot['unsold'] and h not in snapshot['sold']]
        if not outcome.get('batch_resolved'):
            require(bool(missing), 'uncertain')
            require(write, 'recovery_wait')
            require(outcome.get('delivery_proof_version') == 1, 'backend_upgrade_required')
            require(outcome.get('retry_eligible') is True, 'uncertain')
            require(outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
            verify_dedup_capability(directory, merchant, [goods_id])
            attempt = local.get('recovery_attempt', {})
            require(not attempt or attempt.get('batch_id') == batch['batch_id'], 'state_invalid')
            count = attempt.get('count', 0)
            require(type(count) is int and count < policy['max_attempts'], 'recovery_exhausted')
            require(time.time() >= attempt.get('next_at', 0), 'recovery_wait')
            require(not (Path(directory) / 'pause').exists(), 'manual')
            # Keep the original codes and batch. The upstream dedup capability is required,
            # because an earlier timed-out request may still finish after this read.
            local['recovery_attempt'] = {'batch_id': batch['batch_id'], 'count': count + 1,
                'payload_sha256': hashlib.sha256('\n'.join(missing).encode()).hexdigest(),
                'missing_count': len(missing), 'started_at': time.time(),
                'next_at': time.time() + min(900, policy['retry_seconds'] * 2 ** count)}
            persist()
            try:
                merchant.upload(goods_id, missing)
                local['recovery_attempt']['response'] = 'received'
            except probe.ProbeError as exc:
                local['recovery_attempt']['response'] = exc.kind
                persist()
                if exc.kind not in ('network_error', 'upstream_rejected', 'non_json'):
                    raise
            persist()
            snapshot = merchant.delivery_snapshot(goods_id)
            outcome = report_delivery(backend, goods_id, batch, snapshot)
            require(outcome.get('_device_paused_reason', '') in ('', 'disconnected'), 'backend_auth')
            require(outcome.get('batch_resolved') is True and not outcome.get('blocked'), 'recovery_wait')
            uploaded += len(missing)
        require(not outcome.get('blocked'), 'uncertain')
        local.pop('batch_id', None)
        local.update(phase='idle', stock=len(snapshot['unsold']), matched_stock=len(snapshot['unsold']))
        local['recovery_resolved_at'] = time.time()
        persist()
        for row in products:
            if row['goods_id'] == goods_id:
                row.update(stock=local['stock'], matched_stock=local['stock'], batch_verified=True,
                    recovered_batch_id=batch['batch_id'])
                if local.get('recovery_attempt'):
                    row['recovery_attempt'] = dict(local['recovery_attempt'])
    if write:
        require(not (Path(directory) / 'pause').exists(), 'manual')
        heartbeat = backend.call('/heartbeat', {'authorization': 'verified'})
        require(heartbeat.get('paused_reason', '') in ('', 'disconnected'), 'backend_auth')
        backend.call('/resume', {})
        state.update(paused=False, reason='')
    return {'status': 'RUNNING' if write else 'VERIFIED', 'site': probe.LOCAL_SITE,
        'products': products, 'uploaded': uploaded, 'recovery_only': True,
        'at_target': all(v.get('stock', -1) >= v.get('target_stock', 999999) for v in state['products'].values())}


def catalog(merchant):
    merchant.post('/merchantApi/user/userinfo', {})
    return {row['id']: row for row in merchant.rows('/merchantApi/Goods/list', {'goods_type': 'card', 'is_proxy': 0, 'status': 999})}


def identity(product, row):
    try:
        require(type(product['goods_id']) is int and product['goods_id'] > 0)
        require(type(product['cny_amount']) is int and product['cny_amount'] > 0)
        require(product['enabled'] is True and product['usd_credit'] == product['cny_amount'])
        require(row['id'] == product['goods_id'] and row['goods_type'] == 'card')
        require(Decimal(str(row['price'])) == Decimal(product['cny_amount']))
        require(isinstance(row['name'], str) and bool(row['name']))
        return {k: product[k] for k in ('goods_id', 'cny_amount', 'usd_credit', 'external_url')} | {'merchant_name': row['name']}
    except (KeyError, TypeError, InvalidOperation) as exc:
        raise RestockError('binding_changed') from exc


@contextmanager
def exclusive(directory, lock_name='worker.lock'):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    info = directory.lstat()
    require(not directory.is_symlink() and info.st_uid == os.getuid() and not info.st_mode & 0o077, 'state_invalid')
    require(lock_name in ('worker.lock', 'scheduler.lock'), 'state_invalid')
    fd = os.open(directory / lock_name, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise RestockError('busy') from exc
        yield
    finally:
        os.close(fd)


def read_state(directory):
    path = Path(directory) / 'state.json'
    if not path.exists():
        raise RestockError('setup_required')
    try:
        state = json.loads(probe.private_read(path))
        require(state['schema'] == 1 and state['site'] == probe.LOCAL_SITE and isinstance(state['products'], dict), 'state_invalid')
        return state
    except (ValueError, TypeError, KeyError) as exc:
        raise RestockError('state_invalid') from exc


def save_state(directory, state):
    state['updated_at'] = time.time()
    probe.write_private(Path(directory) / 'state.json', state)


def device_backend(directory):
    value = json.loads(probe.private_read(Path(directory) / 'device.json'))
    require(value['site'] == probe.LOCAL_SITE)
    return Backend(value['key']), value['id']


def setup(config, directory, session_path, admin_path, target=None):
    target = config['target_stock'] if target is None else target
    require(type(target) is int and 1 <= target <= 999, 'invalid_config')
    merchant = Merchant(session_path).load()
    remote = catalog(merchant)
    admin = Backend(probe.private_read(admin_path).decode().strip(), admin=True)
    status = admin.call('/status')
    require(status['enabled'] is True, 'disabled')
    require(all(b['status'] == 'verified' for b in status['batches']), 'uncertain')
    products = [p for p in status['products'] if p['enabled'] and (config['goods_ids'] == 'all' or p['goods_id'] in config['goods_ids'])]
    require(bool(products) and (config['goods_ids'] == 'all' or {p['goods_id'] for p in products} == set(config['goods_ids'])))
    pins = {str(p['goods_id']): identity(p, remote.get(p['goods_id'], {})) for p in products}
    device_path = Path(directory) / 'device.json'
    if device_path.exists():
        backend, device_id = device_backend(directory)
        old = read_state(directory)
        require(old['pins'] == pins)
        require(not any(p.get('batch_id') for p in old['products'].values()), 'uncertain')
        require(set(backend.call('/config')['device']['goods_ids']) == {p['goods_id'] for p in products})
    else:
        created = admin.call('/devices', {'name': '本机 HTTP 补货脚本', 'goods_ids': [p['goods_id'] for p in products]})
        device_id = created['device']['id']
        probe.write_private(device_path, {'site': probe.LOCAL_SITE, 'id': device_id, 'key': created['device_key']})
    # Preserve all other products and settings; only selected stock policy changes.
    for product in status['products']:
        if str(product['goods_id']) in pins:
            product.update(target_stock=target, batch_size=config['batch_size'])
    admin.call('/config', {'enabled': status['enabled'], 'products': status['products']}, method='PUT')
    state = {'schema': 1, 'site': probe.LOCAL_SITE, 'device_id': device_id, 'pins': pins, 'paused': False,
             'reason': '', 'target_stock': target, 'products': {key: {'phase': 'idle'} for key in pins}}
    save_state(directory, state)
    return {'status': 'CONFIGURED', 'products': len(pins), 'target_stock': target}


def verify_batch(batch, goods_id):
    require(isinstance(batch, dict) and batch.get('goods_id') == goods_id)
    require(isinstance(batch.get('batch_id'), str) and bool(re.fullmatch(r'[A-Za-z0-9_-]{1,128}', batch['batch_id'])))
    require(batch.get('status') in ('claimed', 'uncertain', 'verified'))
    codes, hashes = batch.get('codes'), batch.get('code_hashes')
    require(isinstance(codes, list) and 0 < len(codes) <= 20 and batch.get('code_count') == len(codes))
    require(all(isinstance(code, str) and re.fullmatch(r'[a-f0-9]{32}', code) for code in codes))
    require(len(set(codes)) == len(codes) and [hashlib.sha256(c.encode()).hexdigest() for c in codes] == hashes)


def reconcile(backend, merchant, goods_id, batch_id=None):
    hashes = merchant.inventory_hashes(goods_id)
    heartbeat = backend.call('/heartbeat', {'authorization': 'verified'})
    body = {'goods_id': goods_id, 'complete': True, 'total': len(hashes), 'hashes': hashes}
    if batch_id:
        body['batch_id'] = batch_id
    result = backend.call('/inventory', body)
    result['_device_paused_reason'] = heartbeat.get('paused_reason', '')
    require(all(type(result.get(k)) is bool for k in ('blocked', 'identity_verified', 'batch_resolved')), 'backend_error')
    require(type(result.get('matched_stock')) is int and 1 <= result.get('target_stock', 0) <= 999, 'backend_error')
    return result, len(hashes)


def run_product(state, key, merchant, backend, persist, *, write, resume):
    goods_id, local = int(key), state['products'][key]
    outcome, stock = reconcile(backend, merchant, goods_id, local.get('batch_id'))
    local.update(stock=stock, matched_stock=outcome['matched_stock'], target_stock=outcome['target_stock'])
    persist()
    require(outcome['identity_verified'] and outcome['matched_stock'] == stock, 'inventory_mismatch')
    if local.get('batch_id') and outcome['batch_resolved']:
        local.pop('batch_id', None)
        local['phase'] = 'idle'
        persist()
    pending = outcome.get('pending_batch')
    if pending:
        verify_batch(pending, goods_id)
        require(not local.get('batch_id') or local['batch_id'] == pending['batch_id'], 'uncertain')
        local['batch_id'] = pending['batch_id']
        if pending['status'] == 'uncertain':
            local['phase'] = 'uncertain'
        persist()
    require(not outcome['blocked'] and local.get('phase') != 'uncertain', 'uncertain')
    if not write:
        return {'goods_id': goods_id, 'stock': stock, 'matched_stock': outcome['matched_stock'], 'uploaded': 0}
    if resume or (not state['paused'] and outcome.get('_device_paused_reason') == 'disconnected'):
        backend.call('/resume', {})
    batch = backend.call('/claim', {'goods_id': goods_id}).get('batch')
    if batch is None:
        require(not local.get('batch_id'), 'uncertain')
        require(stock >= outcome['target_stock'], 'backend_error')
        return {'goods_id': goods_id, 'stock': stock, 'matched_stock': stock, 'uploaded': 0}
    verify_batch(batch, goods_id)
    require(batch['status'] == 'claimed' and (not local.get('batch_id') or local['batch_id'] == batch['batch_id']), 'uncertain')
    require(len(batch['codes']) <= max(0, outcome['target_stock'] - stock), 'uncertain')
    # The local latch must survive even a lost response to the server-side latch.
    if local.get('recovery_attempt', {}).get('batch_id') != batch['batch_id']:
        local.pop('recovery_attempt', None)
    local.update(batch_id=batch['batch_id'], phase='uncertain')
    persist()
    started = backend.call('/batches/' + batch['batch_id'] + '/start', {})['batch']
    require(started['batch_id'] == batch['batch_id'] and started['goods_id'] == goods_id and started['status'] == 'uncertain', 'uncertain')
    try:
        merchant.upload(goods_id, batch['codes'])
    except probe.ProbeError as exc:
        if exc.kind not in ('network_error', 'upstream_rejected', 'non_json'):
            raise
        # A lost or malformed acknowledgement is resolved by reading, never retrying the write.
    confirmed, after = reconcile(backend, merchant, goods_id, batch['batch_id'])
    require(confirmed['identity_verified'] and confirmed['matched_stock'] == after and not confirmed['blocked'] and confirmed['batch_resolved'], 'uncertain')
    local.pop('batch_id')
    local.update(phase='idle', stock=after, matched_stock=confirmed['matched_stock'])
    persist()
    return {'goods_id': goods_id, 'stock_before': stock, 'stock': after, 'matched_stock': confirmed['matched_stock'],
            'uploaded': len(batch['codes']), 'batch_id': batch['batch_id'], 'batch_verified': True}


def cycle(directory, merchant, backend, device_id, *, write=True, resume=False, recovery=None):
    state = read_state(directory)
    state['last_started_at'] = time.time()
    report = {'status': 'PAUSED', 'site': probe.LOCAL_SITE, 'products': [], 'uploaded': 0}
    persist = lambda: save_state(directory, state)
    try:
        require(state['device_id'] == device_id)
        require(not write or not (Path(directory) / 'pause').exists(), 'manual')
        recovering = write and state['paused'] and not resume and state.get('reason') in RECHECKABLE_ERRORS
        repair = bool(recovery and recovery.get('enabled') and
                      (state.get('reason') in RECOVERABLE_PAUSES or
                       any(p.get('batch_id') for p in state['products'].values())))
        protected = state['paused'] and state.get('reason') not in RECOVERABLE_PAUSES
        repair = repair and (not protected or resume)
        if write and state['paused'] and not resume and not recovering and not repair:
            reason = state.get('reason')
            require(reason in MESSAGES or reason in probe.ERRORS, 'state_invalid')
            report.update(error=reason, message=MESSAGES.get(reason, probe.ERRORS.get(reason)))
            return report
        recheck = state.get('response_recheck', {})
        if write and not resume and state['paused'] and state.get('reason') in RECHECKABLE_ERRORS and time.time() < recheck.get('next_at', 0):
            reason = state['reason']
            report.update(error=reason, message=probe.ERRORS[reason], next_recheck_at=recheck['next_at'])
            if recheck.get('diagnostics'):
                report['diagnostics'] = recheck['diagnostics']
            return report
        remote = catalog(merchant)
        config = backend.call('/config')
        require(config['device']['id'] == device_id and not config['device']['revoked'])
        products = {str(p['goods_id']): p for p in config['products'] if p['enabled']}
        require(set(state['pins']) == {str(i) for i in config['device']['goods_ids']})
        for key, pin in state['pins'].items():
            require(identity(products.get(key, {}), remote.get(int(key), {})) == pin)
        require(not write or config['enabled'], 'disabled')
        if repair:
            require(not write or config.get('paused_reason', '') in ('', 'disconnected'), 'backend_auth')
            report = recover_batches(state, directory, merchant, backend, persist, recovery, write=write)
            if write:
                state.pop('response_recheck', None)
            state['last_success_at'] = time.time()
            merchant.save()
            return report
        if recovering:
            # Every product and original batch must be verified before any write can resume.
            for key in state['pins']:
                require(not (Path(directory) / 'pause').exists(), 'manual')
                run_product(state, key, merchant, backend, persist, write=False, resume=False)
                require(not state['products'][key].get('batch_id'), 'uncertain')
            state.update(paused=False, reason='')
        reconnect = not state['paused'] and config.get('paused_reason') == 'disconnected'
        for key in state['pins']:
            if write:
                require(not (Path(directory) / 'pause').exists(), 'manual')
            result = run_product(state, key, merchant, backend, persist, write=write, resume=resume or reconnect)
            report['products'].append(result)
            report['uploaded'] += result['uploaded']
        if write:
            state.update(paused=False, reason='')
            state.pop('response_recheck', None)
        state['last_success_at'] = time.time()
        report['status'] = 'VERIFIED' if not write else 'RUNNING'
        report['at_target'] = all(p.get('stock', -1) >= p.get('target_stock', 999999) for p in state['products'].values())
        merchant.save()
    except (probe.ProbeError, RestockError) as exc:
        reason = exc.kind
        if reason in RECHECKABLE_ERRORS and state['paused'] and state.get('reason') not in RECHECKABLE_ERRORS:
            # A failed verification cannot downgrade a protected pause into an automatic retry.
            report['verification_error'] = reason
            previous = state.get('reason')
            reason = previous if isinstance(previous, str) and previous in (MESSAGES | probe.ERRORS) else 'state_invalid'
        state.update(paused=True, reason=reason)
        report.update(error=reason, message=MESSAGES.get(reason, probe.ERRORS.get(reason)))
        diagnostic = getattr(exc, 'diagnostics', None)
        if diagnostic:
            report['diagnostics'] = diagnostic
        if reason in RECHECKABLE_ERRORS and (reason in ('non_json', 'upstream_unavailable') or state.get('response_recheck')):
            attempts = state.get('response_recheck', {}).get('attempts', 0) + 1
            next_at = time.time() + min(900, 60 * 2 ** min(attempts - 1, 4))
            state['response_recheck'] = {'attempts': attempts, 'next_at': next_at, 'diagnostics': diagnostic or {}}
            report['next_recheck_at'] = next_at
        if exc.kind == 'login_required':
            try:
                backend.call('/heartbeat', {'authorization': 'failed'})
            except RestockError:
                report['pause_reported_to_backend'] = False
            else:
                report['pause_reported_to_backend'] = True
    except Exception:
        state.update(paused=True, reason='state_invalid')
        report.update(error='state_invalid', message=MESSAGES['state_invalid'])
    finally:
        state['last_report'] = report
        persist()
    return report


def pause_failure(directory, error, backend):
    state = read_state(directory)
    state.update(paused=True, reason=error.kind)
    report = {'status': 'PAUSED', 'error': error.kind, 'message': str(error), 'uploaded': 0}
    if error.kind in ('login_required', 'unsafe_session'):
        try:
            backend.call('/heartbeat', {'authorization': 'failed'})
        except RestockError:
            report['pause_reported_to_backend'] = False
        else:
            report['pause_reported_to_backend'] = True
    state['last_report'] = report
    save_state(directory, state)
    return report


def publish_runtime(directory, backend, report, *, execution_mode='http'):
    """Report safe operator state without refreshing merchant authorization."""
    state = read_state(directory)
    paused = state['paused'] or (Path(directory) / 'pause').exists()
    reason = 'manual' if (Path(directory) / 'pause').exists() else state.get('reason', '')
    if paused and reason not in (MESSAGES | probe.ERRORS):
        reason = 'state_invalid'
    diagnostic = report.get('diagnostics', {})
    body = {'state': 'paused' if paused else 'running', 'reason': reason if paused else '',
            'browser_verification_required': paused and
                (reason == 'verification_required' or
                 (reason == 'non_json' and diagnostic.get('response_kind') == 'browser_verification'))}
    if execution_mode == 'browser':
        body['execution_mode'] = execution_mode
    timestamps = {'checked_at': diagnostic.get('checked_at'),
                  'next_check_at': report.get('next_recheck_at') if paused else None}
    if not paused:
        timestamps['checked_at'] = state.get('last_success_at')
    for key, stamp in timestamps.items():
        if type(stamp) in (int, float) and 0 < stamp < 253402300799:
            body[key] = datetime.fromtimestamp(stamp, timezone.utc).isoformat()
    try:
        backend.call('/runtime', body)
    except RestockError:
        report['runtime_reported'] = False
    else:
        report['runtime_reported'] = True


def check_requested_verification(directory, merchant, backend, device_id, recovery):
    original = read_state(directory)
    if (Path(directory) / 'pause').exists():
        report = {'status': 'PAUSED', 'error': 'manual', 'message': MESSAGES['manual'], 'uploaded': 0}
        result = {'state': 'failed', 'reason': 'manual', 'resumed': False}
    else:
        # write=False bypasses the response backoff while retaining all identity/inventory checks.
        report = cycle(directory, merchant, backend, device_id, write=False, recovery=recovery)
        result = {'state': 'failed', 'reason': report.get('error', 'state_invalid'), 'resumed': False}
        if report['status'] == 'VERIFIED':
            state = read_state(directory)
            config = backend.call('/config')
            require(config['device']['id'] == device_id and not config['device']['revoked'], 'backend_auth')
            can_resume = (not (Path(directory) / 'pause').exists() and config['enabled']
                          and (not original['paused'] or original.get('reason') in MANUAL_VERIFICATION_PAUSES))
            if can_resume:
                backend.call('/resume', {})
                state.update(paused=False, reason='')
                state.pop('response_recheck', None)
                report.update(status='RUNNING', manual_recheck=True)
            elif not config['enabled']:
                state.update(paused=True, reason='disabled')
            elif (Path(directory) / 'pause').exists():
                state.update(paused=True, reason='manual')
            state['last_report'] = report
            save_state(directory, state)
            result = {'state': 'passed', 'reason': '', 'resumed': can_resume}
    return report, result


def process_recheck(directory, merchant, backend, device_id, request, recovery, *, session_path=None,
                    merchant_factory=None, execution_mode='http'):
    """An admin click requests a real read-only check, never a successful verdict."""
    require(isinstance(request, dict) and isinstance(request.get('id'), str)
            and re.fullmatch(r'[a-f0-9]{32}', request['id'])
            and request.get('device_id') == device_id and request.get('state') == 'checking', 'backend_error')
    state = read_state(directory)
    cached = state.get('manual_recheck_result', {})
    if cached.get('id') == request['id']:
        result = cached['result']
        report = dict(state.get('last_report', {'status': 'PAUSED', 'uploaded': 0}))
    else:
        try:
            if merchant is None:
                merchant = merchant_factory() if merchant_factory else Merchant(session_path).load()
            report, result = check_requested_verification(directory, merchant, backend, device_id, recovery)
        except (probe.ProbeError, RestockError) as exc:
            report = pause_failure(directory, exc, backend)
            result = {'state': 'failed', 'reason': exc.kind, 'resumed': False}
        state = read_state(directory)
        state['manual_recheck_result'] = {'id': request['id'], 'result': result, 'completed_at': time.time()}
        save_state(directory, state)
    publish_runtime(directory, backend, report, execution_mode=execution_mode)
    try:
        backend.call('/recheck/' + request['id'] + '/result', result)
    except RestockError:
        # Replay only the saved result after a lost acknowledgement, not another merchant scan.
        report['recheck_result_reported'] = False
    else:
        report['recheck_result_reported'] = True
    report['recheck_id'] = request['id']
    return report


def run_worker(args, config, stopped, *, merchant_factory=None, on_tick=None, execution_mode='http'):
    next_cycle = 0.0
    with exclusive(args.private_dir, 'scheduler.lock'):
        while not stopped.is_set():
            try:
                if on_tick:
                    on_tick()
                with exclusive(args.private_dir):
                    backend, device_id = device_backend(args.private_dir)
                    claimed = backend.call('/recheck/claim', {}).get('recheck')
                    due = time.monotonic() >= next_cycle
                    if claimed or due:
                        if claimed:
                            report = process_recheck(args.private_dir, None, backend, device_id, claimed,
                                                     config.get('recovery'), session_path=args.session,
                                                     merchant_factory=merchant_factory, execution_mode=execution_mode)
                        else:
                            try:
                                merchant = merchant_factory() if merchant_factory else Merchant(args.session).load()
                            except probe.ProbeError as exc:
                                report = pause_failure(args.private_dir, exc, backend)
                            else:
                                report = cycle(args.private_dir, merchant, backend, device_id, recovery=config.get('recovery'))
                            publish_runtime(args.private_dir, backend, report, execution_mode=execution_mode)
                        receipt(report, args.evidence)
                        attention(report, args.private_dir, args.notify)
                        next_cycle = time.monotonic() + config['interval_seconds']
            except (probe.ProbeError, RestockError) as exc:
                if exc.kind != 'busy' and time.monotonic() >= next_cycle:
                    receipt({'status': 'PAUSED', 'error': exc.kind, 'message': str(exc)}, args.evidence)
                    next_cycle = time.monotonic() + config['interval_seconds']
            if stopped.wait(5):
                break
    return 0


def receipt(report, evidence):
    evidence = Path(evidence)
    evidence.mkdir(parents=True, exist_ok=True)
    name = 'restock-' + time.strftime('%Y%m%dT%H%M%S') + '-' + os.urandom(4).hex() + '.json'
    with (evidence / name).open('x') as stream:
        source = {Path(p).name: hashlib.sha256(Path(p).read_bytes()).hexdigest() for p in (__file__, probe.__file__)}
        json.dump(report | {'finished_at': time.time(), 'source_sha256': source}, stream, ensure_ascii=False, indent=2)
        stream.write('\n')
    print(json.dumps(report, ensure_ascii=False), flush=True)


def attention(report, directory, notify=False):
    path = Path(directory) / 'attention.json'
    if report['status'] != 'PAUSED' or report.get('error') == 'busy':
        if report['status'] == 'RUNNING':
            path.unlink(missing_ok=True)
        return
    previous = json.loads(probe.private_read(path)) if path.exists() else {}
    alert = {k: report[k] for k in ('status', 'error', 'message') if k in report}
    if previous == alert:
        return
    probe.write_private(path, alert)
    if notify and sys.platform == 'darwin':
        # Use fixed text so neither upstream responses nor credentials reach the notification process.
        message = '小铺登录已失效，请打开本机登录页重新授权。' if report.get('error') == 'login_required' else '补货已暂停，请查看本机补货状态。'
        try:
            result = subprocess.run(['/usr/bin/osascript', '-e', 'display notification ' + json.dumps(message, ensure_ascii=False) + ' with title "小铺自动补货"'], capture_output=True, timeout=10, check=False)
            return result.returncode == 0
        except (OSError, subprocess.TimeoutExpired):
            # The durable attention file remains available if the desktop notification service is absent.
            return False


def install_service(args, config):
    require(sys.platform == 'darwin', 'invalid_config')
    state = read_state(args.private_dir)
    label = 'lol.rest2build.ldxp-http-local'
    target = f'gui/{os.getuid()}/{label}'
    destination = Path.home() / 'Library/LaunchAgents' / (label + '.plist')
    if destination.exists():
        previous = plistlib.loads(destination.read_bytes())
        require(str(Path(__file__).resolve()) in previous.get('ProgramArguments', []), 'binding_changed')
    running = subprocess.run(['/bin/launchctl', 'print', target], capture_output=True)
    if running.returncode == 0:
        require(destination.exists() and str(Path(__file__).resolve()).encode() in running.stdout, 'binding_changed')
        require(subprocess.run(['/bin/launchctl', 'bootout', target], capture_output=True).returncode == 0, 'backend_error')
    directory = args.private_dir.resolve()
    spec = {
        'Label': label,
        'ProgramArguments': [sys.executable, '-B', str(Path(__file__).resolve()), 'run', '--notify',
                             '--config', str(args.config.resolve()), '--private-dir', str(directory),
                             '--session', str(args.session.resolve()), '--evidence', str(directory / 'runs')],
        'WorkingDirectory': str(Path(__file__).resolve().parents[2]),
        'RunAtLoad': True, 'StartInterval': config['interval_seconds'], 'ProcessType': 'Background',
        'Umask': 0o077, 'StandardOutPath': str(directory / 'service.log'),
        'StandardErrorPath': str(directory / 'service-error.log'),
    }
    destination.parent.mkdir(parents=True, exist_ok=True)
    with destination.open('wb') as stream:
        plistlib.dump(spec, stream)
    destination.chmod(0o600)
    require(subprocess.run(['/bin/launchctl', 'bootstrap', f'gui/{os.getuid()}', str(destination)], capture_output=True).returncode == 0, 'backend_error')
    require(subprocess.run(['/bin/launchctl', 'print', target], capture_output=True).returncode == 0, 'backend_error')
    return {'status': 'SCHEDULED', 'interval_seconds': config['interval_seconds'], 'target_stock': state['target_stock'], 'products': len(state['pins'])}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['setup', 'check', 'once', 'run', 'resume', 'status', 'pause', 'install-service'])
    parser.add_argument('--config', type=Path, default=CONFIG)
    parser.add_argument('--private-dir', type=Path, default=PRIVATE)
    parser.add_argument('--session', type=Path, default=probe.DEFAULT_PRIVATE / 'session.json')
    parser.add_argument('--admin-key', type=Path, default=probe.DEFAULT_ADMIN)
    parser.add_argument('--target', type=int, help='Override stock target for setup only')
    parser.add_argument('--notify', action='store_true', help='Show a macOS notification when replenishment pauses')
    parser.add_argument('--evidence', type=Path, default=Path('agents-results') / time.strftime('%Y-%m-%d') / 'ldxp-http-restock-local/acceptance')
    args = parser.parse_args()
    os.umask(0o077)
    try:
        config = load_config(args.config)
        require(args.target is None or args.action == 'setup', 'invalid_config')
        if args.action == 'install-service':
            receipt(install_service(args, config), args.evidence)
            return 0
        if args.action == 'pause':
            probe.write_private(args.private_dir / 'pause', {'requested_at': time.time()})
            receipt({'status': 'PAUSE_REQUESTED', 'message': '当前上传核对完成后暂停。'}, args.evidence)
            return 0
        if args.action == 'status':
            state = read_state(args.private_dir)
            receipt({'status': 'PAUSED' if state['paused'] or (args.private_dir / 'pause').exists() else 'READY',
                     'target_stock': state['target_stock'], 'products': state['products'], 'reason': state['reason'],
                     'last_success_at': state.get('last_success_at')}, args.evidence)
            return 0
        if args.action == 'run':
            stopped = threading.Event()
            signal.signal(signal.SIGTERM, lambda *_: stopped.set())
            signal.signal(signal.SIGINT, lambda *_: stopped.set())
            return run_worker(args, config, stopped)
        with exclusive(args.private_dir):
            if args.action == 'setup':
                receipt(setup(config, args.private_dir, args.session, args.admin_key, args.target), args.evidence)
                return 0
            backend, device_id = device_backend(args.private_dir)
            if args.action == 'resume':
                (args.private_dir / 'pause').unlink(missing_ok=True)
            stopped = threading.Event()
            signal.signal(signal.SIGTERM, lambda *_: stopped.set())
            signal.signal(signal.SIGINT, lambda *_: stopped.set())
            while True:
                try:
                    merchant = Merchant(args.session).load()
                except probe.ProbeError as exc:
                    report = pause_failure(args.private_dir, exc, backend)
                    publish_runtime(args.private_dir, backend, report)
                    receipt(report, args.evidence)
                    attention(report, args.private_dir, args.notify)
                    return 1
                report = cycle(args.private_dir, merchant, backend, device_id, write=args.action != 'check', resume=args.action == 'resume', recovery=config.get('recovery'))
                publish_runtime(args.private_dir, backend, report)
                receipt(report, args.evidence)
                attention(report, args.private_dir, args.notify)
                if args.action != 'run' or report['status'] == 'PAUSED' or stopped.wait(config['interval_seconds']):
                    return 1 if report['status'] == 'PAUSED' else 0
    except (probe.ProbeError, RestockError) as exc:
        receipt({'status': 'PAUSED', 'error': exc.kind, 'message': str(exc)}, args.evidence)
        return 1
    except Exception:
        receipt({'status': 'PAUSED', 'error': 'state_invalid', 'message': MESSAGES['state_invalid']}, args.evidence)
        return 1


if __name__ == '__main__':
    sys.exit(main())
