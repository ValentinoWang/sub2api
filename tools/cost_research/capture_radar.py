#!/usr/bin/env python3
"""Optional anonymous browser capture of rendered public Radar; no registration or POST."""
import argparse
import asyncio
import hashlib
import json
from pathlib import Path
from urllib.parse import urlsplit
from costlab import RADAR, UA, check_robots, now, radar_parse, save


async def capture(folder: Path):
    from playwright.async_api import async_playwright
    check_robots(RADAR)
    folder.mkdir(parents=True, exist_ok=True)
    folder.chmod(0o700)
    responses = []
    tasks = []
    hosts = {'codexradar.com', 'deng.codexradar.com', 'api.codexradar.com'}
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        context = await browser.new_context(user_agent=UA, service_workers='block')
        async def route_handler(route):
            req = route.request
            url = urlsplit(req.url)
            if req.method != 'GET' or url.scheme != 'https' or url.hostname not in hosts:
                await route.abort()
            else:
                await route.continue_()
        await context.route('**/*', route_handler)
        page = await context.new_page()
        async def capture_response(res):
            if res.request.method != 'GET' or res.status != 200:
                return
            if 'application/json' not in res.headers.get('content-type', ''):
                return
            try:
                raw = await res.body()
                if len(raw) > 2_000_000:
                    return
                # An empty anonymous context is used; no authorization/cookie/HAR is persisted.
                url = urlsplit(res.url)
                responses.append({'source_path': url.scheme + '://' + url.netloc + url.path,
                                  'sha256': hashlib.sha256(raw).hexdigest(),
                                  'data': json.loads(raw)})
            except Exception:
                return
        page.on('response', lambda res: tasks.append(asyncio.create_task(capture_response(res))))
        await page.goto(RADAR, wait_until='domcontentloaded', timeout=45000)
        await page.locator('body').wait_for()
        try:
            await page.wait_for_function("/GPT-6|额度雷达|Quota Radar/.test(document.body.innerText)", timeout=12000)
        except Exception:
            pass
        await page.wait_for_timeout(3000)  # Bounded grace period for ordinary page GETs, no probing.
        html = await page.content()
        text = await page.locator('body').inner_text()
        await page.screenshot(path=str(folder / 'radar.png'), full_page=True)
        await asyncio.gather(*tasks, return_exceptions=True)
        result = radar_parse(html)
        result.update(transport='anonymous_browser', collected_at=now(),
                      page_sha256=hashlib.sha256(html.encode()).hexdigest())
        save(str(folder / 'parsed.json'), result)
        save(str(folder / 'public-json-candidates.json'), responses)
        for filename, body in [('radar.html', html), ('radar.txt', text)]:
            path = folder / filename
            path.write_text(body, encoding='utf-8')
            path.chmod(0o600)
        await browser.close()
    # JSON candidates are evidence only. No generic number is silently promoted to weekly quota/history.


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-dir', required=True)
    args = parser.parse_args()
    asyncio.run(capture(Path(args.output_dir)))
