import { SITES, ALARM_MINUTES, REASONS, RestockError, bindingID, initialState, publicState, runCycle, safeReason, validateSite } from './core.js';
import { merchantOperation } from './merchant-adapter.js';
import { createBackend, validateDeviceKey } from './backend.js';

const ALARM = 'ldxp-stock-check';
let active = false;
let stopAllRequested = false;
const ready = (async () => {
  await chrome.storage.local.setAccessLevel({ accessLevel: 'TRUSTED_CONTEXTS' });
  const { bindings = {}, states = {} } = await chrome.storage.local.get(['bindings', 'states']);
  for (const id of Object.keys(bindings)) {
    const state = states[id] || initialState();
    if (state.phase !== 'idle') {
      state.enabled = false;
      state.checked = false;
      state.reason = state.batch ? 'uncertain' : 'interrupted';
      state.phase = 'idle';
      states[id] = state;
    }
  }
  await chrome.storage.local.set({ states });
  await chrome.alarms.create(ALARM, { periodInMinutes: ALARM_MINUTES });
})();

async function exclusive(work) {
  await ready;
  if (active) throw new RestockError('busy');
  active = true;
  try { return await work(); } finally {
    if (stopAllRequested) {
      const { states = {} } = await chrome.storage.local.get('states');
      for (const id of Object.keys(states)) {
        if (states[id]) Object.assign(states[id], { enabled: false, checked: false, reason: states[id].batch?.status === 'uncertain' ? 'uncertain' : 'manual' });
      }
      await chrome.storage.local.set({ states });
      stopAllRequested = false;
    }
    active = false;
    await chrome.storage.local.set({ uiRevision: Date.now() });
  }
}
async function merchant() {
  if (!navigator.onLine) throw new RestockError('offline');
  const tabs = await chrome.tabs.query({ url: 'https://www.ldxp.cn/*' });
  const tab = tabs.find(item => item.id && item.url && new URL(item.url).pathname.startsWith('/merchant') && item.status === 'complete');
  if (!tab) throw new RestockError('no_tab');
  const call = async (operation, goods, codes) => {
    let results;
    try {
      results = await chrome.scripting.executeScript({
        target: { tabId: tab.id, frameIds: [0] }, world: 'MAIN',
        func: merchantOperation, args: [operation, goods, codes || []],
      });
    } catch { throw new RestockError('no_tab'); }
    if (results.length !== 1 || results[0].frameId !== 0 || !results[0].result?.ok) throw new RestockError(results[0]?.result?.error || 'merchant_error');
    return results[0].result;
  };
  return {
    inventory: async goods => (await call('inventory', goods)).inventory,
    upload: async (goods, codes) => call('upload', goods, codes),
  };
}
async function snapshot() {
  await ready;
  const { bindings = {}, states = {}, siteCredentials = {}, targetStock = null } = await chrome.storage.local.get(['bindings', 'states', 'siteCredentials', 'targetStock']);
  return {
    target_stock: targetStock,
    bindings: Object.entries(bindings).map(([id, binding]) => publicState(binding, states[id] || initialState())),
    sites: SITES.map(site => {
      const saved = savedCredential(site, bindings, siteCredentials);
      return { site, has_credential: Boolean(saved), device_id: saved?.device_id || '' };
    }),
    busy: active,
  };
}
async function cycle(id, mode) {
  const { bindings = {}, states = {} } = await chrome.storage.local.get(['bindings', 'states']);
  const binding = bindings[id];
  if (!binding || bindingID(binding) !== id) throw new RestockError('binding_mismatch');
  const state = states[id] || initialState();
  const save = async value => {
    states[id] = value;
    await chrome.storage.local.set({ states });
  };
  let adapter;
  try { adapter = await merchant(); }
  catch (error) {
    await save({ ...state, enabled: false, checked: false, phase: 'idle', reason: safeReason(error) });
    return;
  }
  await runCycle({ binding, state, save, merchant: adapter, backend: createBackend(binding), mode, cancelled: () => stopAllRequested });

}
function savedCredential(site, bindings, siteCredentials) {
  if (siteCredentials[site]) return siteCredentials[site];
  // Older installs store the credential on each product binding. Reuse it only
  // when there is one unambiguous device for this site; never try another site.
  const candidates = Object.values(bindings).filter(binding => binding.site === site);
  const unique = new Map(candidates.map(binding => [`${binding.device_id}|${binding.device_key}`, binding]));
  return unique.size === 1 ? [...unique.values()][0] : null;
}
function resolveCredential(value, bindings, siteCredentials) {
  if (!value) throw new RestockError('binding_mismatch');
  validateSite(value.site);
  const saved = savedCredential(value.site, bindings, siteCredentials);
  const key = value.device_key || saved?.device_key;
  if (!key) throw new RestockError('credential_required');
  validateDeviceKey(key);
  return { site: value.site, device_key: key };
}
async function readConfig(value) {
  if (!(await chrome.permissions.contains({ origins: [permissionOrigin(value.site)] }))) throw new RestockError('permission');
  return createBackend(value).config();
}
function trustedSender(sender) {
  if (sender.id !== chrome.runtime.id) return false;
  if (sender.origin && sender.origin !== `chrome-extension://${chrome.runtime.id}`) return false;
  if (sender.tab?.url && sender.tab.url !== sender.url) return false;
  return sender.url === chrome.runtime.getURL('options.html');
}
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!trustedSender(sender)) return false;
  (async () => {
    if (message?.type === 'snapshot') return snapshot();
    if (message?.type === 'pause-all' && active) {
      stopAllRequested = true;
      return snapshot();
    }
    return exclusive(async () => {
      const { bindings = {}, states = {}, siteCredentials = {} } = await chrome.storage.local.get(['bindings', 'states', 'siteCredentials']);
      if (message?.type === 'inspect') {
        const value = resolveCredential(message.binding, bindings, siteCredentials);
        const config = await readConfig(value);
        return { device_id: String(config.device.id), products: config.products.filter(product => product.enabled).map(product => ({ goods_id: product.goods_id, title: product.title || `${product.cny_amount}元额度`, cny_amount: product.cny_amount, usd_credit: product.usd_credit, target_stock: product.target_stock, batch_size: product.batch_size })) };
      } else if (message?.type === 'maintain') {
        const selections = message.sites;
        if (!Array.isArray(selections) || selections.length < 1 || selections.length > SITES.length || new Set(selections.map(item => item?.site)).size !== selections.length) throw new RestockError('binding_mismatch');
        if (!Number.isSafeInteger(message.target_stock) || message.target_stock < 1 || message.target_stock > 999) throw new RestockError('target_invalid');
        const siteConfigs = new Map();
        const selectedIDs = [];
        // Stage the entire selection before one storage write. A failure at either
        // site must not leave a partly saved setup or discard an active batch.
        for (const selection of selections) {
          const value = resolveCredential(selection, bindings, siteCredentials);
          const goods = selection.goods_ids;
          if (!Array.isArray(goods) || goods.length < 1 || goods.length > 30 || new Set(goods).size !== goods.length || goods.some(id => !Number.isSafeInteger(id) || id < 1)) throw new RestockError('binding_mismatch');
          const config = await readConfig(value);
          siteConfigs.set(value.site, config);
          if (String(config.device.id) !== selection.device_id) throw new RestockError('binding_mismatch');
          for (const goods_id of goods) {
            const product = config.products.find(product => product.goods_id === goods_id && product.enabled);
            if (!product) throw new RestockError('binding_mismatch');
            const binding = { site: value.site, config_id: 'ldxp', device_id: selection.device_id, goods_id, target_stock: product.target_stock, device_key: value.device_key, name: String(product.title || `${product.cny_amount}元额度`).slice(0, 80) };
            const id = bindingID(binding);
            selectedIDs.push(id);
            if (Object.entries(bindings).some(([otherID, other]) => otherID !== id && other.goods_id === goods_id)) throw new RestockError('product_conflict');
            if (bindings[id] && bindings[id].device_key !== binding.device_key && (states[id]?.batch || states[id]?.enabled)) throw new RestockError('uncertain');
            bindings[id] = binding;
            states[id] ||= { ...initialState(), reason: 'manual' };
          }
          siteCredentials[value.site] = { device_id: selection.device_id, device_key: value.device_key };
        }
        for (const id of selectedIDs) states[id] = { ...states[id], enabled: false, checked: false, reason: 'manual' };
        await chrome.storage.local.set({ bindings, states, siteCredentials, targetStock: message.target_stock });
        const issues = [];
        for (const selection of selections) {
          const ids = selectedIDs.filter(id => bindings[id].site === selection.site);
          try {
            if (siteConfigs.get(selection.site).enabled === false) throw new RestockError('site_disabled');
            if (stopAllRequested) continue;
            await createBackend(bindings[ids[0]]).setStockTarget(selection.goods_ids, message.target_stock);
            // Only display a target once its site has confirmed persistence.
            const current = await chrome.storage.local.get(['bindings', 'states']);
            for (const id of ids) {
              current.bindings[id].target_stock = message.target_stock;
              current.states[id].target_stock = message.target_stock;
            }
            await chrome.storage.local.set(current);
            for (const id of ids) {
              if (!stopAllRequested) await cycle(id, 'start');
            }
          } catch (error) {
            const reason = safeReason(error);
            issues.push({ site: selection.site, reason, message: REASONS[reason] });
            const { states: currentStates = {} } = await chrome.storage.local.get('states');
            for (const id of ids) Object.assign(currentStates[id], { enabled: false, checked: false, reason });
            await chrome.storage.local.set({ states: currentStates });
          }
        }
        const result = await snapshot();
        const selected = result.bindings.filter(binding => selectedIDs.includes(binding.id));
        return { ...result, maintenance: { requested: selected.length, running: selected.filter(binding => binding.enabled).length, issues } };
      } else if (message?.type === 'remove') {
        if (!bindings[message.id] || states[message.id]?.enabled || states[message.id]?.batch) throw new RestockError('uncertain');
        delete bindings[message.id];
        delete states[message.id];
        await chrome.storage.local.set({ bindings, states });
      } else if (message?.type === 'pause-all') {
        for (const id of Object.keys(bindings)) states[id] = { ...(states[id] || initialState()), enabled: false, checked: false, reason: states[id]?.batch ? 'uncertain' : 'manual' };
        await chrome.storage.local.set({ states });
      } else throw new RestockError('protocol_error');
      return snapshot();
    }).then(data => data && Array.isArray(data.bindings) ? { ...data, busy: false } : data);
  })().then(data => sendResponse({ ok: true, data }), error => sendResponse({ ok: false, error: safeReason(error), message: REASONS[safeReason(error)] }));
  return true;
});
export function permissionOrigin(site) {
  return validateSite(site) === 'http://127.0.0.1:8080' ? 'http://127.0.0.1/*' : `${site}/*`;
}
chrome.alarms.onAlarm.addListener(alarm => {
  if (alarm.name !== ALARM) return;
  exclusive(async () => {
    const { bindings = {}, states = {} } = await chrome.storage.local.get(['bindings', 'states']);
    for (const id of Object.keys(bindings)) {
      if (states[id]?.enabled) await cycle(id, 'automatic');
    }
  }).catch(() => { /* The running task owns the durable pause/error status. */ });
});
chrome.runtime.onStartup.addListener(() => {
  exclusive(async () => {
    const { states = {} } = await chrome.storage.local.get('states');
    for (const state of Object.values(states)) Object.assign(state, { enabled: false, checked: false, reason: 'sleep', phase: 'idle' });
    await chrome.storage.local.set({ states });
  }).catch(() => {});
});

chrome.action.onClicked.addListener(() => chrome.runtime.openOptionsPage());
