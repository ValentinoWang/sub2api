import { ALARM_MINUTES, REASONS, RestockError, bindingID, initialState, publicState, runCycle, safeReason, validateSite } from './core.js';
import { merchantOperation } from './merchant-adapter.js';
import { createBackend, validateDeviceKey } from './backend.js';

const ALARM = 'ldxp-stock-check';
let active = false;
const stopRequests = new Set();
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
    if (stopRequests.size) {
      const { states = {} } = await chrome.storage.local.get('states');
      for (const id of stopRequests) {
        if (states[id]) Object.assign(states[id], { enabled: false, checked: false, reason: states[id].batch?.status === 'uncertain' ? 'uncertain' : 'manual' });
      }
      await chrome.storage.local.set({ states });
      stopRequests.clear();
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
  const { bindings = {}, states = {} } = await chrome.storage.local.get(['bindings', 'states']);
  return { bindings: Object.entries(bindings).map(([id, binding]) => publicState(binding, states[id] || initialState())), busy: active };
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
  await runCycle({ binding, state, save, merchant: adapter, backend: createBackend(binding), mode, cancelled: () => stopRequests.has(id) });
  if (stopRequests.has(id)) await save({ ...state, enabled: false, checked: false, reason: state.batch?.status === 'uncertain' ? 'uncertain' : 'manual' });
  stopRequests.delete(id);
}
function trustedSender(sender) {
  if (sender.id !== chrome.runtime.id) return false;
  if (sender.origin && sender.origin !== `chrome-extension://${chrome.runtime.id}`) return false;
  if (sender.tab?.url && sender.tab.url !== sender.url) return false;
  return ['popup.html', 'options.html'].some(page => sender.url === chrome.runtime.getURL(page));
}
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!trustedSender(sender)) return false;
  (async () => {
    if (message?.type === 'snapshot') return snapshot();
    if (message?.type === 'pause' && active) {
      stopRequests.add(message.id);
      return snapshot();
    }
    return exclusive(async () => {
      const { bindings = {}, states = {} } = await chrome.storage.local.get(['bindings', 'states']);
      if (message?.type === 'inspect') {
        const value = message.binding;
        if (!value) throw new RestockError('binding_mismatch');
        validateSite(value.site);
        validateDeviceKey(value.device_key);
        if (!(await chrome.permissions.contains({ origins: [permissionOrigin(value.site)] }))) throw new RestockError('permission');
        const config = await createBackend(value).config();
        return { device_id: String(config.device.id), products: config.products.filter(product => product.enabled).map(product => ({ goods_id: product.goods_id, title: product.title || `${product.cny_amount}元额度` })) };
      } else if (message?.type === 'bind') {
        const value = message.binding;
        if (!value) throw new RestockError('binding_mismatch');
        validateDeviceKey(value.device_key);
        validateSite(value.site);
        if (!(await chrome.permissions.contains({ origins: [permissionOrigin(value.site)] }))) throw new RestockError('permission');
        const binding = {
          site: value.site, config_id: 'ldxp', device_id: value.device_id, goods_id: Number(value.goods_id),
          device_key: value.device_key, name: String(value.name || '').slice(0, 80),
        };
        const id = bindingID(binding);
        // Verify server-owned scope before persisting any new credentials.
        await createBackend(binding).verifyBinding(binding);
        if (states[id]?.batch) throw new RestockError('uncertain');
        bindings[id] = binding;
        states[id] = { ...initialState(), reason: 'manual' };
        await chrome.storage.local.set({ bindings, states });
      } else if (message?.type === 'remove') {
        if (!bindings[message.id] || states[message.id]?.enabled || states[message.id]?.batch) throw new RestockError('uncertain');
        delete bindings[message.id];
        delete states[message.id];
        await chrome.storage.local.set({ bindings, states });
      } else if (message?.type === 'pause') {
        if (!bindings[message.id]) throw new RestockError('binding_mismatch');
        states[message.id] = { ...(states[message.id] || initialState()), enabled: false, checked: false, reason: 'manual' };
        await chrome.storage.local.set({ states });
      } else if (message?.type === 'check' || message?.type === 'start') {
        if (message.type === 'start' && !states[message.id]?.checked) throw new RestockError('interrupted');
        await cycle(message.id, message.type);
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
