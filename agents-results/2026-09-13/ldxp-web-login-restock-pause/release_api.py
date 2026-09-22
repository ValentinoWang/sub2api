"""Bounded release operations. Emit only selected non-secret readback fields."""
import json
from pathlib import Path
import sys
from urllib.request import Request, build_opener

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/quality'))
from commerce_release import NoRedirect, read_admin_key

PRIVATE = Path('/Users/vsiyo/.local/share/sub2api/commerce-release')


def api(environment, route, method='GET', body=None):
    base = {'dev': 'http://127.0.0.1:8080', 'prod': 'https://ai.rest2build.lol'}[environment]
    headers = {'x-api-key': read_admin_key(PRIVATE / (environment + '-admin.key'))}
    if body is not None:
        headers['Content-Type'] = 'application/json'
    request = Request(base + route, data=None if body is None else json.dumps(body).encode(),
                      headers=headers, method=method)
    with build_opener(NoRedirect()).open(request, timeout=60) as response:
        result = json.load(response)
    if result.get('code') != 0:
        raise RuntimeError('API did not report success')
    return result['data']


def main():
    action, environment = sys.argv[1:3]
    if action == 'verify':
        installation = api(environment, '/api/v1/admin/tools/ldxp/installation')
        state = api(environment, '/api/v1/admin/tools/ldxp/status')
        settings = api(environment, '/api/v1/admin/settings')
        result = {'environment': environment,
                  'installation': {k: installation[k] for k in ('ready', 'version', 'sha256')},
                  'restock': {k: state.get(k) for k in ('enabled', 'merchant_token_configured',
                      'code_secret_configured', 'session_verification_required')},
                  'purchase_enabled': settings['purchase_subscription_enabled'],
                  'purchase_url': settings['purchase_subscription_url']}
        if environment == 'prod':
            records = [api(environment, '/api/v1/admin/redeem-codes/' + str(i)) for i in (4, 5, 6)]
            result['redeem_records'] = [{k: r.get(k) for k in ('id', 'status', 'type', 'value', 'used_by', 'used_at')} for r in records]
            assert [r['status'] for r in records] == ['used', 'unused', 'unused']
            assert all(r['type'] == 'balance' and r['value'] == 5 for r in records)
            assert result['purchase_enabled'] and result['purchase_url'] == 'https://wzyp.cn/item/x6uqgh'
        assert installation['ready'] and installation['version'] == '0.2.4.2'
        assert not state['enabled'] and state['session_verification_required'] is False
    elif action == 'repair':
        api(environment, '/api/v1/admin/tools/ldxp/installation', 'POST', {'repair': True})
        result = api(environment, '/api/v1/admin/tools/ldxp/installation')
        result = {k: result[k] for k in ('ready', 'version', 'sha256')}
        assert result['ready'] and result['version'] == '0.2.4.2'
    elif action == 'maintenance':
        created = api(environment, '/api/v1/admin/announcements', 'POST', {
            'title': '系统维护通知',
            'content': '正在升级服务，期间可能短暂中断。遇到请求失败，请稍后重试。维护结束后将发布恢复通知。',
            'status': 'active', 'notify_mode': 'popup', 'targeting': {}})
        current = api(environment, '/api/v1/admin/announcements/' + str(created['id']))
        result = {k: current[k] for k in ('id', 'title', 'status', 'notify_mode')}
        assert result['status'] == 'active' and result['notify_mode'] == 'popup'
    elif action == 'recovery':
        maintenance_id = int(sys.argv[3])
        current = api(environment, '/api/v1/admin/announcements/' + str(maintenance_id))
        assert current['title'] == '系统维护通知' and current['status'] == 'active'
        created = api(environment, '/api/v1/admin/announcements', 'POST', {
            'title': '服务已恢复', 'content': '服务升级已完成，充值入口与兑换功能已检查，可以继续使用。',
            'status': 'active', 'notify_mode': 'popup', 'targeting': {}})
        current = api(environment, '/api/v1/admin/announcements/' + str(created['id']))
        assert current['status'] == 'active'
        # Keep the maintenance visible if recovery publication or readback fails.
        # On an ambiguous network result, inspect announcements before retrying.
        api(environment, '/api/v1/admin/announcements/' + str(maintenance_id), 'PUT', {'status': 'archived'})
        archived = api(environment, '/api/v1/admin/announcements/' + str(maintenance_id))
        assert archived['status'] == 'archived' and current['status'] == 'active'
        result = {'maintenance_id': maintenance_id, 'maintenance_status': archived['status'],
                  'recovery_id': current['id'], 'recovery_status': current['status']}
    else:
        raise RuntimeError('Unsupported release operation')
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'status': 'FAILED', 'error_type': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
