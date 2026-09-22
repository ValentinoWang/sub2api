import { chromium, expect } from '/Users/vsiyo/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/test.mjs';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import assert from 'node:assert/strict';
const source = resolve('tools/ldxp-browser-extension');
const output = resolve('agents-results/2026-09-13/ldxp-maintain-stock/acceptance/browser');
const profile = await mkdtemp(join(tmpdir(), 'ldxp-stock-fixture-'));
let context;
try {
  context = await chromium.launchPersistentContext(profile, {
    executablePath: '/Users/vsiyo/Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
    headless: true, args: [`--disable-extensions-except=${source}`, `--load-extension=${source}`],
    viewport: { width: 1280, height: 1000 },
  });
  const worker = context.serviceWorkers()[0] || await context.waitForEvent('serviceworker');
  const extensionID = new URL(worker.url()).host;
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto(`chrome-extension://${extensionID}/options.html`);
  await expect(page.getByRole('heading', { name: '自动维持库存', exact: true })).toBeVisible();
  await worker.evaluate(async () => {
    const local = 'http://127.0.0.1:8080';
    const production = 'https://ai.rest2build.lol';
    const inventories = new Map();
    const batches = new Map();
    const uploads = [];
    const calls = [];
    let sequence = 0;
    const products = site => [5,10,20,50,100].map((amount, i) => ({ goods_id: (site === local ? 100 : 200) + i, cny_amount: amount, usd_credit: amount, title: `${amount}元额度`, enabled: true, target_stock: 999, batch_size: 20 }));
    for (const site of [local, production]) for (const p of products(site)) inventories.set(p.goods_id, Array.from({ length: 998 }, (_,i) => (p.goods_id * 10000 + i).toString(16).padStart(64,'0')));
    const hash = async code => [...new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(code)))].map(x => x.toString(16).padStart(2, '0')).join('');
    globalThis.fixture = { inventories, batches, uploads, calls };
    // This fresh profile uses fictional inventory and in-process API fixtures only.
    globalThis.fetch = async (url, options) => {
      const site = new URL(url).origin;
      if (![local,production].includes(site)) throw new Error('Unexpected fixture origin');
      if (options.headers.Authorization !== `Bearer ldxpd_${(site === local ? 'a' : 'b').repeat(64)}`) throw new Error('Wrong fixture credential');
      const path = new URL(url).pathname.replace('/api/v1/ldxp/device','');
      const body = options.body ? JSON.parse(options.body) : {};
      calls.push({ site, path });
      let data;
      if (path === '/config') data = { enabled: true, device: { id: site === local ? 'local-fixture' : 'prod-fixture' }, products: products(site) };
      else if (path === '/stock-target') {
        if (body.target_stock !== 999 || body.goods_ids.some(id => !products(site).some(p => p.goods_id === id))) throw new Error('Wrong target/scope');
        data = { products: body.goods_ids.map(goods_id => ({ goods_id, target_stock: 999 })) };
      } else if (path === '/heartbeat' || path === '/resume') data = { enabled: true, paused_reason: '' };
      else if (path === '/inventory') {
        const pending = batches.get(body.batch_id);
        const resolved = !!pending && pending.code_hashes.every(h => body.hashes.includes(h));
        if (resolved) pending.status = 'verified';
        data = { blocked: false, identity_verified: true, batch_resolved: resolved, pending_batch: null, target_stock: 999 };
      } else if (path === '/claim') {
        const count = Math.min(20, 999 - inventories.get(body.goods_id).length);
        let batch = null;
        if (count > 0) {
          const batch_id = `fixture_${++sequence}`;
          const codes = Array.from({length:count}, (_,i) => `FICTIONAL_${batch_id}_${i}`);
          batch = { batch_id, goods_id: body.goods_id, status: 'claimed', codes, code_hashes: await Promise.all(codes.map(hash)) };
          batches.set(batch_id, batch);
        }
        data = { batch };
      } else if (/^\/batches\/fixture_\d+\/start$/.test(path)) {
        const batch = batches.get(path.split('/')[2]); batch.status = 'uncertain'; data = { batch };
      } else if (/^\/batches\/fixture_\d+\/result$/.test(path)) data = { received: true };
      else throw new Error(`Unmocked path ${path}`);
      return { ok: true, json: async () => ({ code: 0, data: structuredClone(data) }) };
    };
    chrome.permissions.contains = async () => true;
    chrome.tabs.query = async () => [{id:99,url:'https://www.ldxp.cn/merchant/goods/list',status:'complete'}];
    chrome.scripting.executeScript = async request => {
      const [operation, goods, codes] = request.args;
      if (!inventories.has(goods)) throw new Error('Wrong fixture product');
      if (operation === 'upload') {
        const hashes = await Promise.all(codes.map(hash));
        if (hashes.some(h => inventories.get(goods).includes(h))) throw new Error('Duplicate upload');
        inventories.get(goods).push(...hashes);
        uploads.push({ goods, count: hashes.length });
        return [{frameId:0,result:{ok:true}}];
      }
      if (operation !== 'inventory') throw new Error('Unexpected merchant operation');
      const hashes = [...inventories.get(goods)];
      return [{frameId:0,result:{ok:true,inventory:{complete:true,total:hashes.length,hashes}}}];
    };
  });
  await page.evaluate(() => { chrome.permissions.request = async () => true; });
  await expect(page.locator('#stock-target')).toHaveValue('999');
  await page.getByRole('region', { name:'本地站',exact:true }).getByLabel('补货设备凭据').fill(`ldxpd_${'a'.repeat(64)}`);
  await page.getByRole('region', { name:'生产站',exact:true }).getByLabel('补货设备凭据').fill(`ldxpd_${'b'.repeat(64)}`);
  await page.getByRole('button', { name:'读取两个站点的商品' }).click();
  await expect(page.getByRole('checkbox')).toHaveCount(10);
  for (const box of await page.getByRole('checkbox').all()) await expect(box).toBeChecked();
  const maintain = page.getByRole('button',{name:'保存并自动补货',exact:true});
  await maintain.click();
  await expect(page.locator('#feedback')).toContainText('10 个已开启自动补货');
  await expect(page.locator('#bindings').getByText('自动维持中 · 库存 999 / 999', {exact:true})).toHaveCount(10);
  assert.deepEqual(await worker.evaluate(() => fixture.uploads.map(x=>x.count)), Array(10).fill(1));
  await maintain.click();
  await expect(page.locator('#feedback')).toContainText('10 个已开启自动补货');
  assert.equal(await worker.evaluate(() => fixture.uploads.length), 10);
  // Simulate sales, then let a real extension alarm drive the automatic cycle.
  await worker.evaluate(async () => {
    for (const hashes of fixture.inventories.values()) hashes.splice(0,2);
    await chrome.alarms.create('ldxp-stock-check', {when:Date.now()+500});
  });
  await expect.poll(() => worker.evaluate(() => fixture.uploads.length), {timeout:15000}).toBe(20);
  await expect.poll(() => worker.evaluate(async () => Object.values((await chrome.storage.local.get('states')).states).every(s=>s.stock===999&&s.phase==='idle'&&!s.batch)), {timeout:15000}).toBe(true);
  assert.deepEqual(await worker.evaluate(() => fixture.uploads.slice(10).map(x=>x.count)), Array(10).fill(2));
  const layout = [];
  for (const viewport of [{width:1280,height:1000},{width:390,height:844}]) {
    await page.setViewportSize(viewport);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    assert.equal(overflow,false); layout.push({...viewport,overflow});
    await page.screenshot({path:join(output,`fixture-${viewport.width}.png`),fullPage:true});
  }
  await page.getByRole('button',{name:'暂停全部',exact:true}).click();
  await expect(page.locator('#feedback')).toContainText('已请求暂停全部商品');
  assert.equal(await worker.evaluate(async () => Object.values((await chrome.storage.local.get('states')).states).some(s=>s.enabled)),false);
  await page.reload();
  await expect(page.locator('#stock-target')).toHaveValue('999');
  await expect(page.locator('.credential-status').filter({hasText:'凭据已保存'})).toHaveCount(2);
  const popup = await context.newPage();
  popup.on('pageerror', error => errors.push(error.message));
  await popup.goto(`chrome-extension://${extensionID}/popup.html`);
  await expect(popup.getByRole('button',{name:'设置库存与自动补货',exact:true})).toBeVisible();
  await popup.screenshot({path:join(output,'fixture-popup.png'),fullPage:true});
  assert.deepEqual(errors,[]);
  const report = { result:'PASS', boundary:'isolated Chromium; real extension options/worker/alarm flow with fictional merchant and backend fixtures; no user profile, real codes, real inventory, or deployment', checks:['default999 and all ten products selected','one action starts ten products across two fictional sites','998 to 999 uploads one per product','repeat save at999 uploads none','real alarm refills two simulated sales per product','pause all','saved target and credentials survive reload','popup entry','no page errors or horizontal overflow'], uploads:await worker.evaluate(()=>fixture.uploads), layout, errors };
  await writeFile(join(output,'result.json'),JSON.stringify(report,null,2)+'\n');
  console.log(JSON.stringify(report));
} finally { await context?.close(); await rm(profile,{recursive:true,force:true}); }
