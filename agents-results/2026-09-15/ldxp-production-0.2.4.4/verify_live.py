"""Read live deployment APIs and run one minimal real model request; retain no credentials."""
import argparse
import json
import os
from pathlib import Path
import time
from urllib.request import Request, build_opener
from urllib.error import HTTPError
from release_api import api, ORIGINS, NoRedirect
from release_common import candidate, require, sha

p = argparse.ArgumentParser()
p.add_argument('--environment', choices=('dev', 'prod'), required=True)
p.add_argument('--candidate', type=Path, required=True)
p.add_argument('--admin-key-file', type=Path, required=True)
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
os.umask(0o077)
r = {'candidate_sha256': sha(a.candidate), 'environment': a.environment,
     'created_at': time.time(), 'status': 'FAILED', 'health_passed': False,
     'functional_checks_passed': False}
with a.output.open('x') as output:
    try:
        release = candidate(a.candidate)
        origin = ORIGINS[a.environment]
        opener = build_opener(NoRedirect())
        with opener.open(origin + '/health', timeout=30) as response:
            require(response.status == 200, 'Health failed')
        r['health_passed'] = True
        call = lambda route: api(a.environment, a.admin_key_file, route)
        version = call('/api/v1/admin/system/version')['version']
        require(version == release['version'], 'Version mismatch')
        r['version'] = version
        settings = call('/api/v1/admin/settings')
        require(settings['purchase_subscription_enabled'] is True, 'Purchase disabled')
        expected_shop = {'dev': 'https://wzyp.cn/item/e8yrh4', 'prod': 'https://wzyp.cn/shop/MGDY0ZE4'}[a.environment]
        require(settings['purchase_subscription_url'] == expected_shop, 'Shop URL mismatch')
        r['shop_url'] = settings['purchase_subscription_url']
        browser = call('/api/v1/admin/tools/ldxp/browser/status')
        require(browser['enabled'] is (a.environment == 'dev'), 'Replenishment configuration drift')
        require(len(browser['products']) == (5 if a.environment == 'dev' else 0), 'Product configuration drift')
        r['browser_enabled'] = browser['enabled']
        r['configured_product_count'] = len(browser['products'])
        try:
            with opener.open(origin + '/api/v1/ldxp/products', timeout=30) as response:
                products = json.load(response)
                require(products.get('code') == 0, 'Catalog failed')
                r['catalog_http_status'] = response.status
        except HTTPError as error:
            require(error.code == 404 and not browser['products'], 'Unexpected catalog failure')
            r['catalog_http_status'] = 404
        for route in ('/purchase', '/redeem'):
            with opener.open(origin + route, timeout=30) as response:
                page = response.read().decode()
                require(response.status == 200 and '<div id="app"' in page, 'Page shell failed')
        r['page_shells_passed'] = True
        keys = call('/api/v1/admin/users/1/api-keys?page_size=100')['items']
        key = next(x for x in keys if x['id'] == 3)
        require(key['status'] == 'active' and key['group_id'] == 3, 'Baseline key changed')
        request = Request(origin + '/v1/responses', method='POST',
            headers={'Authorization': 'Bearer ' + key['key'], 'Content-Type': 'application/json'},
            data=json.dumps({'model': 'gpt-5.6-luna', 'input': 'Reply with OK.',
                             'stream': True, 'max_output_tokens': 32}).encode())
        completed = False
        with opener.open(request, timeout=120) as response:
            require(response.status == 200, 'Model HTTP failure')
            for line in response:
                if line.startswith(b'data: '):
                    data = line[6:].strip()
                    if data == b'[DONE]':
                        break
                    event = json.loads(data)
                    require(event.get('type') not in ('error', 'response.failed'), 'Model stream failure')
                    if event.get('type') == 'response.completed':
                        require(event.get('response', {}).get('status') == 'completed', 'Model incomplete')
                        completed = True
                        break
        require(completed, 'Missing completed event')
        r.update(model='gpt-5.6-luna', model_stream_completed=True,
                 functional_checks_passed=True, status='FUNCTIONAL_ACCEPTANCE')
    except Exception as error:
        r['error_type'] = type(error).__name__
    r['finished_at'] = time.time()
    json.dump(r, output, indent=2)
    output.write('\n')
print(json.dumps(r))
raise SystemExit(0 if r['functional_checks_passed'] else 1)
