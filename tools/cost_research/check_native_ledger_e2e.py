#!/usr/bin/env python3
"""Real loopback HTTP and restart acceptance for the native Go ledger (Linux file adapter)."""
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.error
import urllib.request

from run_native_workbench import build


def run(root: Path) -> dict:
    results = []
    with tempfile.TemporaryDirectory(prefix='native-ledger-e2e-') as folder:
        home = Path(folder)
        binary = build(root, home / 'build')
        ledger_dir = home / 'private'
        ledger_dir.mkdir(mode=0o700)
        ledger_path = ledger_dir / 'ledger.json'
        process = None

        def start():
            nonlocal process
            process = subprocess.Popen([str(binary), '--ledger', str(ledger_path)], stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, text=True)
            lines = [process.stdout.readline().strip() for _ in range(3)]
            if not lines[0].startswith('URL=') or not lines[1].startswith('TOKEN='):
                raise AssertionError('Native acceptance process did not start')
            return lines[0][4:], lines[1][6:]

        def stop():
            nonlocal process
            if process is not None:
                process.terminate()
                process.communicate(timeout=10)
                if process.returncode != 0:
                    raise AssertionError('Native server did not stop cleanly')
                process = None

        url, token = start()

        def request(path, command=None, key=None, auth=True, origin=None):
            headers = {'Content-Type': 'application/json'}
            if auth:
                headers['Authorization'] = 'Bearer ' + token
            if key:
                headers['Idempotency-Key'] = key
            if origin:
                headers['Origin'] = origin
            req = urllib.request.Request(url + '/api/v1/admin/cost-center/ledger/' + path,
                                         data=None if command is None else json.dumps(command).encode(), headers=headers)
            try:
                with urllib.request.urlopen(req, timeout=10) as res:
                    return res.status, json.loads(res.read())
            except urllib.error.HTTPError as exc:
                raw = exc.read()
                try:
                    data = json.loads(raw)
                except ValueError:
                    data = {'message': 'non-JSON rejection'}
                return exc.code, data

        def check(name, condition):
            if not condition:
                raise AssertionError(name)
            results.append({'name': name, 'status': 'PASS'})

        try:
            check('unauthenticated_denied', request('health', auth=False)[0] == 401)
            check('origin_rebinding_denied', request('health', origin='https://example.invalid')[0] == 403)
            purchases = []
            for tier, amount, quantity in [('plus', '20', '80'), ('pro5x', '100', '400'), ('pro20x', '200', '1600')]:
                purchase = {'kind': 'purchase', 'purchase': {
                    'reference': 'synthetic-' + tier, 'supplier': 'synthetic-supplier', 'asset': 'synthetic-' + tier,
                    'tier': tier, 'kind': 'subscription', 'currency': 'USD', 'amount': amount,
                    'paid_at': '2026-08-01T00:00:00Z', 'service_start': '2026-08-01T00:00:00Z',
                    'service_end': '2026-09-01T00:00:00Z', 'evidence': 'synthetic', 'evidence_ref': 'e2e-only'}}
                status, result = request('commands', purchase, 'purchase-' + tier)
                check('create_' + tier, status == 201)
                purchases.append(purchase)
                status, _ = request('commands', {'kind': 'delivery', 'delivery': {
                    'reference': 'delivery-' + tier, 'asset': 'synthetic-' + tier, 'tier': tier,
                    'unit': 'qualified-task-v1', 'model': 'synthetic-model', 'quantity': quantity,
                    'period_start': '2026-08-01T00:00:00Z', 'period_end': '2026-09-01T00:00:00Z',
                    'evidence': 'synthetic'}}, 'delivery-' + tier)
                check('delivery_' + tier, status == 201)
            shared = json.loads(json.dumps(purchases[0]))
            shared['purchase'].update(reference='shared', asset='host', tier='unallocated', kind='server', amount='60')
            status, data = request('commands', shared, 'shared')
            check('shared_purchase', status == 201)
            shared_event = data['data']['event']
            allocation = {'kind': 'allocate', 'allocation': {'purchase_id': shared_event['id'], 'parts': [
                {'tier': 'plus', 'weight': '0.1'}, {'tier': 'pro5x', 'weight': '0.3'}, {'tier': 'pro20x', 'weight': '0.6'}]}}
            status, data = request('commands', allocation, 'allocation')
            check('allocation', status == 201)
            allocation_event = data['data']['event']
            with ThreadPoolExecutor(max_workers=8) as executor:
                statuses = list(executor.map(lambda _: request('commands', purchases[0], 'purchase-plus')[0], range(32)))
            check('32_concurrent_idempotent_replays', statuses == [200] * 32 and request('health')[1]['data']['event_count'] == 8)
            conflict = json.loads(json.dumps(purchases[0])); conflict['purchase']['amount'] = '21'
            check('changed_payload_same_key_conflict', request('commands', conflict, 'purchase-plus')[0] == 409)
            forged = dict(purchases[0], actor_id=999)
            check('client_actor_not_accepted', request('commands', forged, 'actor-forgery')[0] == 400)
            status, page = request('events?limit=3')
            check('real_cursor_pagination', status == 200 and len(page['data']['items']) == 3 and page['data']['next_after'] == 3)
            scope = 'summary?start=2026-08-01T00:00:00Z&end=2026-09-01T00:00:00Z&currency=USD&model=synthetic-model&unit=qualified-task-v1'
            status, report = request(scope)
            check('native_three_tier_allocation_report', status == 200 and [x['recognized_expense'] for x in report['data']['rows']] == ['26.000000','118.000000','236.000000','0.000000'])
            check('native_recorded_scope_unit_costs', [x['recorded_scope_unit_cost'] for x in report['data']['rows'][:3]] == ['0.325000','0.295000','0.147500'])
            stop(); url, token = start()
            check('process_restart_persists_events', request('health')[1]['data']['event_count'] == 8)
            check('process_restart_persists_totals', request(scope)[1]['data']['rows'] == report['data']['rows'])
            def void(event, key):
                return request('commands', {'kind': 'void', 'void': {'target_id': event['id'], 'expected_hash': event['request_hash'], 'reason': 'synthetic booking correction, not a refund'}}, key)
            check('dependent_source_void_rejected', void(shared_event, 'source-void')[0] == 422)
            check('allocation_void_appended', void(allocation_event, 'allocation-void')[0] == 201)
            check('source_void_appended', void(shared_event, 'source-void')[0] == 201)
            check('current_report_after_correction', [x['recognized_expense'] for x in request(scope)[1]['data']['rows']] == ['20.000000','100.000000','200.000000','0.000000'])
            cutoff = urllib.parse.quote(allocation_event['recorded_at'], safe='')
            check('historical_knowledge_cutoff_replay', request(scope + '&as_of=' + cutoff)[1]['data']['rows'] == report['data']['rows'])
            check('original_records_retained', request('health')[1]['data']['event_count'] == 10)
            return {'status': 'PASS', 'implementation': 'real Go process + HTTP + Linux file store',
                    'checks': results, 'synthetic_report': report['data'],
                    'not_covered': ['PostgreSQL execution', 'Vue compilation/render', 'whole Sub2API application build', 'production data'],
                    'credentials_saved': False}
        finally:
            stop()


if __name__ == '__main__':
    root = Path(__file__).resolve().parents[2]
    print(json.dumps(run(root), ensure_ascii=False, indent=2))
