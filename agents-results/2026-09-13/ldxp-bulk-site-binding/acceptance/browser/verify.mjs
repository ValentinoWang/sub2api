import { chromium, expect } from '/Users/vsiyo/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/test.mjs';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import assert from 'node:assert/strict';

const source = resolve('tools/ldxp-browser-extension');
const output = resolve('agents-results/2026-09-13/ldxp-bulk-site-binding/acceptance/browser');
const profile = await mkdtemp(join(tmpdir(), 'ldxp-bulk-fixture-'));
let context;
try {
  context = await chromium.launchPersistentContext(profile, {
    executablePath: '/Users/vsiyo/Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
    headless: true,
    args: [`--disable-extensions-except=${source}`, `--load-extension=${source}`],
    viewport: { width: 1280, height: 1000 },
  });
  const worker = context.serviceWorkers()[0] || await context.waitForEvent('serviceworker');
  const extensionID = new URL(worker.url()).host;
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto(`chrome-extension://${extensionID}/options.html`);
  await expect(page.getByRole('heading', { name: '站点与额度绑定', exact: true })).toBeVisible();
  await worker.evaluate(async () => {
    const site = 'http://127.0.0.1:8080';
    const binding = { site, config_id: 'ldxp', device_id: 'local-fixture', goods_id: 100, name: '5元额度', device_key: `ldxpd_${'a'.repeat(64)}` };
    const id = `${site}|ldxp|local-fixture|100`;
    await chrome.storage.local.set({ bindings: { [id]: binding }, states: { [id]: { enabled: false, reason: 'manual', phase: 'idle', stock: 2, checked: false, lastWake: 0, lastSuccess: 0, batch: null } } });
    // This fresh profile uses fixture configs only. No requests reach either site.
    globalThis.fixtureCalls = [];
    globalThis.fetch = async (url, options) => {
      globalThis.fixtureCalls.push({ url, method: options.method });
      if (!url.endsWith('/api/v1/ldxp/device/config')) throw new Error('Only config reads are allowed in this fixture');
      const isLocal = new URL(url).origin === site;
      const char = isLocal ? 'a' : 'b';
      if (options.headers.Authorization !== `Bearer ldxpd_${char.repeat(64)}`) return { ok: false };
      return { ok: true, json: async () => ({ code: 0, data: {
        device: { id: isLocal ? 'local-fixture' : 'production-fixture' },
        products: [5, 10, 20, 50, 100].map((amount, index) => ({ goods_id: (isLocal ? 100 : 200) + index, cny_amount: amount, usd_credit: amount, title: `${amount}元额度`, enabled: true })),
      } }) };
    };
    chrome.permissions.contains = async () => true;
  });
  await page.reload();
  await page.evaluate(() => { chrome.permissions.request = async () => true; });
  const local = page.getByRole('region', { name: '本地站', exact: true });
  const production = page.getByRole('region', { name: '生产站', exact: true });
  await expect(local.getByText('凭据已保存 · 可直接读取商品')).toBeVisible();
  await production.getByLabel('补货设备凭据').fill(`ldxpd_${'b'.repeat(64)}`);
  await page.getByRole('button', { name: '读取两个站点的商品' }).click();
  await expect(page.getByText('已选 2 个站点 · 10 个额度', { exact: true })).toBeVisible();
  await expect(page.getByRole('checkbox')).toHaveCount(10);
  for (const checkbox of await page.getByRole('checkbox').all()) await expect(checkbox).toBeChecked();
  await page.getByRole('button', { name: '保存全部绑定' }).click();
  await expect(page.locator('#feedback')).toContainText('已保存 2 个站点、10 个额度');
  const first = await worker.evaluate(async () => {
    const { bindings, states } = await chrome.storage.local.get(['bindings', 'states']);
    return { count: Object.keys(bindings).length, states, sites: [...new Set(Object.values(bindings).map(item => item.site))] };
  });
  assert.equal(first.count, 10);
  assert.equal(first.sites.length, 2);
  assert.equal(Object.values(first.states).filter(state => state.stock === 2).length, 1);
  assert.equal(Object.values(first.states).filter(state => state.enabled).length, 0);
  for (const field of await page.getByLabel('补货设备凭据').all()) await expect(field).toHaveValue('');
  await page.getByRole('button', { name: '保存全部绑定' }).click();
  await expect(page.locator('#feedback')).toContainText('已保存 2 个站点、10 个额度');
  assert.deepEqual(await worker.evaluate(async () => (await chrome.storage.local.get('states')).states), first.states);
  const sizes = [{ width: 1280, height: 1000 }, { width: 390, height: 844 }];
  const layout = [];
  for (const viewport of sizes) {
    await page.setViewportSize(viewport);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    assert.equal(overflow, false);
    for (const region of [local, production]) await expect(region).toBeVisible();
    layout.push({ ...viewport, overflow });
    await page.screenshot({ path: join(output, `fixture-${viewport.width}.png`), fullPage: true });
  }
  await page.reload();
  await page.evaluate(() => { chrome.permissions.request = async () => true; });
  await expect(page.locator('.credential-status').filter({ hasText: '凭据已保存' })).toHaveCount(2);
  await page.getByRole('button', { name: '读取两个站点的商品' }).click();
  await expect(page.getByText('已选 2 个站点 · 10 个额度', { exact: true })).toBeVisible();
  await expect(page.locator('#binding-count')).toHaveText('10 个额度');
  await page.getByRole('button', { name: '保存全部绑定' }).click();
  await expect(page.locator('#feedback')).toContainText('已保存 2 个站点、10 个额度');
  assert.deepEqual(await worker.evaluate(async () => (await chrome.storage.local.get('states')).states), first.states);
  assert.deepEqual(errors, []);
  const calls = await worker.evaluate(() => globalThis.fixtureCalls);
  assert.ok(calls.every(call => call.method === 'GET' && call.url.endsWith('/config')));
  const report = { result: 'PASS', boundary: 'isolated Chromium extension with fictional products and mocked config reads; no real credentials or inventory operations', checks: ['both sites and ten amounts selected by default', 'single bulk save', 'local credential reuse', 'reopen reuses both credentials', 'duplicate save preserves inventory and states', 'new bindings remain paused', 'only config GET calls', 'no console errors'], layout, errors };
  await writeFile(join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report));
} finally {
  await context?.close();
  await rm(profile, { recursive: true, force: true });
}
