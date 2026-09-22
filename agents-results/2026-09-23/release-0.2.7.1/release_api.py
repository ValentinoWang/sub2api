"""Explicit release API actions with sanitized receipts; no automatic sales enablement."""
import argparse
import json
import os
from pathlib import Path
import sys
import time
from urllib.request import Request, build_opener

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/quality'))
from commerce_release import NoRedirect, read_admin_key
from release_common import candidate, receipt, require, sha

ORIGINS = {'dev': 'http://127.0.0.1:8080', 'prod': 'https://ai.rest2build.lol'}


def api(environment, key_path, route, method='GET', body=None):
    headers = {'x-api-key': read_admin_key(key_path)}
    if body is not None:
        headers['Content-Type'] = 'application/json'
    request = Request(ORIGINS[environment] + route,
                      data=None if body is None else json.dumps(body).encode(),
                      headers=headers, method=method)
    with build_opener(NoRedirect()).open(request, timeout=60) as response:
        result = json.load(response)
    require(result.get('code') == 0, 'API did not report success')
    return result['data']


def execute(args, call):
    release = candidate(args.candidate)
    result = {'candidate_sha256': sha(args.candidate), 'environment': args.environment,
              'created_at': time.time()}
    route = '/api/v1/admin/announcements'
    if args.action == 'maintenance':
        current = call(route, 'POST', {
            'title': '系统维护通知', 'content': '正在升级服务，期间可能短暂中断。遇到请求失败，请稍后重试。维护结束后将发布恢复通知。',
            'status': 'active', 'notify_mode': 'popup', 'targeting': {}})
        require(type(current.get('id')) is int and current['id'] > 0, 'Missing announcement ID')
        current = call(route + '/' + str(current['id']))
        require(current['status'] == 'active' and current['notify_mode'] == 'popup' and
                current.get('targeting') in ({}, None), 'Maintenance readback mismatch')
        result.update(status='MAINTENANCE_ACTIVE', maintenance_id=current['id'])
    elif args.action in ('recovery', 'delayed'):
        require(args.maintenance is not None, 'Maintenance receipt required')
        # A delayed maintenance may legitimately last longer than 30 minutes.
        notice = json.loads(args.maintenance.read_text())
        require(notice['candidate_sha256'] == sha(args.candidate) and
                notice['environment'] == args.environment and notice['status'] == 'MAINTENANCE_ACTIVE',
                'Maintenance receipt mismatch')
        maintenance_id = notice['maintenance_id']
        require(type(maintenance_id) is int and maintenance_id > 0, 'Invalid announcement ID')
        current = call(route + '/' + str(maintenance_id))
        require(current['title'] == '系统维护通知' and current['status'] == 'active',
                'Maintenance no longer active')
        if args.action == 'delayed':
            call(route + '/' + str(maintenance_id), 'PUT', {
                'content': '维护时间延长，服务仍在核验中。请暂缓充值和兑换，恢复后另行通知。',
                'status': 'active'})
            updated = call(route + '/' + str(maintenance_id))
            require(updated['status'] == 'active' and '维护时间延长' in updated['content'],
                    'Delayed maintenance readback failed')
            result.update(status='MAINTENANCE_DELAYED', maintenance_id=maintenance_id)
        else:
            require(args.acceptance is not None, 'Fresh functional acceptance required')
            accepted = receipt(args.acceptance, args.candidate, args.environment, 'FUNCTIONAL_ACCEPTANCE')
            require(accepted.get('health_passed') is True and accepted.get('functional_checks_passed') is True,
                    'Health and functional checks must pass before recovery')
            created = call(route, 'POST', {
                'title': '服务已恢复', 'content': '服务维护已完成，功能核验通过，可以继续使用。',
                'status': 'active', 'notify_mode': 'popup', 'targeting': {}})
            require(type(created.get('id')) is int and created['id'] > 0, 'Missing recovery ID')
            recovery = call(route + '/' + str(created['id']))
            require(recovery['status'] == 'active' and recovery['notify_mode'] == 'popup',
                    'Recovery readback failed')
            # The maintenance stays visible until recovery publication is read back.
            call(route + '/' + str(maintenance_id), 'PUT', {'status': 'archived'})
            require(call(route + '/' + str(maintenance_id))['status'] == 'archived',
                    'Maintenance archive readback failed')
            result.update(status='RECOVERED', maintenance_id=maintenance_id, recovery_id=recovery['id'])
    else:
        if args.action == 'repair':
            call('/api/v1/admin/tools/ldxp/installation', 'POST', {'repair': True})
        installation = call('/api/v1/admin/tools/ldxp/installation')
        require(installation['ready'] and installation['version'] == release['version'],
                'Toolkit candidate mismatch')
        result.update(status='TOOLKIT_VERIFIED', version=installation['version'], toolkit_sha256=installation['sha256'])
        if args.action == 'verify':
            browser = call('/api/v1/admin/tools/ldxp/browser/status')
            restock = call('/api/v1/admin/tools/ldxp/status')
            require(browser['enabled'] is False and restock['enabled'] is False,
                    'Initial release requires both restock workers disabled')
            settings = call('/api/v1/admin/settings')
            result.update(status='CANDIDATE_API_VERIFIED', browser_enabled=False, restock_enabled=False,
                          product_count=len(browser['products']), purchase_enabled=settings['purchase_subscription_enabled'])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('maintenance', 'delayed', 'recovery', 'repair', 'verify'))
    parser.add_argument('--environment', required=True, choices=('dev', 'prod'))
    parser.add_argument('--candidate', required=True, type=Path)
    parser.add_argument('--admin-key-file', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--maintenance', type=Path)
    parser.add_argument('--acceptance', type=Path)
    args = parser.parse_args()
    os.umask(0o077)
    # Reserve evidence before writes, preventing a successful action with an unusable output target.
    with args.output.open('x') as output:
        result = execute(args, lambda route, method='GET', body=None:
                         api(args.environment, args.admin_key_file, route, method, body))
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Never expose credentials, raw responses or HTTP diagnostics.
        print(json.dumps({'status': 'FAILED', 'error_type': type(error).__name__,
                          'ambiguous_write_requires_readback_before_retry': True}), file=sys.stderr)
        sys.exit(1)
