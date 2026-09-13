import { SITE_CONFIG } from './config.js';

export const SITES = Object.freeze(SITE_CONFIG.map(config => config.site));
export const ALARM_MINUTES = 1;
export const MAX_SILENCE_MS = 180_000;
export const REASONS = Object.freeze({
  manual: '已手动暂停', setup: '请先绑定补货设备', login_required: '小铺登录已失效；重新登录后点击「保存并自动补货」',
  no_tab: '未找到小铺商户标签页；打开并登录后点击「保存并自动补货」',
  offline: '浏览器离线；联网后点击「保存并自动补货」', sleep: '浏览器休眠或中断；恢复后点击「保存并自动补货」重新核验',
  interrupted: '上次任务中断；正在保留原批次，需先检查库存', uncertain: '上传结果尚未核实；仅核验原批次，不会自动重传',
  inventory_invalid: '库存信息不完整或分页变化，无法逐码核验', binding_mismatch: '站点、配置、设备或商品绑定不匹配',
  adapter_unverified: '小铺网页接口尚未验证，已停止补货', merchant_error: '小铺请求失败；检查登录后点击「保存并自动补货」重新核验',
  backend_error: '补货服务请求失败；恢复后点击「保存并自动补货」重新核验', protocol_error: '服务响应不符合补货合同',
  permission: '当前绑定站点尚未授权访问', busy: '已有任务执行中',
  target_invalid: '每种额度的库存目标必须是 1–999 之间的整数',
  upgrade_required: '该站点后端还不支持在插件中设置库存，请更新后端后再开始',
  site_disabled: '该站点已关闭自动补货，请在对应站点的补货管理中开启',
  device_unauthorized: '该站点的设备凭据已失效，请在绑定设置中重新填写',
  credential_required: '该站点还没有保存设备凭据，请先填写一次',
  product_conflict: '该小铺商品已绑定其他站点或设备，请为两个站点使用独立商品',
});
export class RestockError extends Error {
  constructor(code) { super(code); this.code = code; }
}
export function fail(code) { throw new RestockError(code); }
export function safeReason(error) { return Object.hasOwn(REASONS, error?.code) ? error.code : 'backend_error'; }
export function validateSite(value) {
  if (!SITES.includes(value)) fail('binding_mismatch');
  return value;
}
export function bindingID(binding) {
  validateSite(binding.site);
  for (const key of ['config_id', 'device_id']) {
    if (typeof binding[key] !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(binding[key])) fail('binding_mismatch');
  }
  if (!Number.isSafeInteger(binding.goods_id) || binding.goods_id < 1) fail('binding_mismatch');
  return `${binding.site}|${binding.config_id}|${binding.device_id}|${binding.goods_id}`;
}
export function initialState() {
  return { enabled: false, reason: 'setup', phase: 'idle', lastWake: 0, lastSuccess: 0, stock: null, batch: null, checked: false };
}
export function publicState(binding, state) {
  return {
    id: bindingID(binding), site: binding.site, config_id: binding.config_id, device_id: binding.device_id,
    goods_id: binding.goods_id, name: binding.name, target_stock: state.target_stock ?? binding.target_stock ?? null, enabled: state.enabled, reason: state.reason,
    reason_text: REASONS[state.reason] || '', phase: state.phase, stock: state.stock,
    lastSuccess: state.lastSuccess, checked: state.checked, pending: Boolean(state.batch),
  };
}
export function validateHashes(hashes) {
  if (!Array.isArray(hashes) || hashes.length > 100_000 || hashes.some(h => typeof h !== 'string' || !/^[a-f0-9]{64}$/.test(h)) || new Set(hashes).size !== hashes.length) fail('inventory_invalid');
  return hashes;
}
export function verifyInventory(report) {
  if (!report || report.complete !== true || !Number.isSafeInteger(report.total) || report.total < 0) fail('inventory_invalid');
  validateHashes(report.hashes);
  if (report.total !== report.hashes.length) fail('inventory_invalid');
  return report;
}
export function validateBatch(batch, binding) {
  if (!batch || typeof batch.batch_id !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(batch.batch_id)) fail('protocol_error');
  if (batch.config_id !== binding.config_id || batch.device_id !== binding.device_id || batch.goods_id !== binding.goods_id) fail('binding_mismatch');
  if (!['claimed', 'uncertain', 'verified'].includes(batch.status)) fail('protocol_error');
  validateHashes(batch.code_hashes);
  if (!Array.isArray(batch.codes) || batch.codes.length < 1 || batch.codes.length > 500 || batch.codes.length !== batch.code_hashes.length || new Set(batch.codes).size !== batch.codes.length) fail('protocol_error');
  if (batch.codes.some(code => typeof code !== 'string' || !code || code.length > 4096 || code.trim() !== code || /[\r\n]/.test(code))) fail('protocol_error');
  return batch;
}
export async function sha256(value) {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
}
export async function checkBatchHashes(batch) {
  const actual = await Promise.all(batch.codes.map(sha256));
  if (actual.some((hash, index) => hash !== batch.code_hashes[index])) fail('protocol_error');
}

// The runner writes progress before every side effect. On restart only reconciliation
// is allowed for a batch whose upload may have started.
export async function runCycle({ binding, state, save, merchant, backend, now = Date.now, mode = 'automatic', cancelled = () => false }) {
  bindingID(binding);
  if (mode === 'automatic' && !state.enabled) return state;
  const persist = async patch => { Object.assign(state, patch); await save(state); };
  const pause = async reason => persist({ enabled: false, checked: false, reason, phase: 'idle' });
  if (mode === 'automatic' && state.lastWake && now() - state.lastWake > MAX_SILENCE_MS) {
    await pause('sleep');
    return state;
  }
  const mayUpload = mode === 'automatic' || mode === 'start';
  try {
    await persist({ phase: 'checking', checked: false, lastWake: now() });
    const inventory = verifyInventory(await merchant.inventory(binding.goods_id));
    await persist({ stock: inventory.total });
    const reconciliation = await backend.reconcile(binding, inventory, state.batch?.batch_id);
    if (Number.isSafeInteger(reconciliation.target_stock) && reconciliation.target_stock > 0) await persist({ target_stock: reconciliation.target_stock });
    if (reconciliation.pending_batch) {
      const pending = validateBatch(reconciliation.pending_batch, binding);
      await checkBatchHashes(pending);
      if (state.batch && state.batch.batch_id !== pending.batch_id) fail('binding_mismatch');
      await persist({ batch: { batch_id: pending.batch_id, status: pending.status, code_hashes: pending.code_hashes } });
    } else if (state.batch && reconciliation.batch_resolved === true) {
      await persist({ batch: null });
    }
    if (reconciliation.blocked || (state.batch && ['uploading', 'uncertain'].includes(state.batch.status))) {
      await pause('uncertain');
      return state;
    }
    if (!mayUpload) {
      await persist({ phase: 'idle', checked: true, lastSuccess: now(), reason: state.enabled ? '' : 'manual' });
      return state;
    }
    if (cancelled()) { await pause('manual'); return state; }
    if (mode === 'start') await backend.resume?.();
    await persist({ enabled: true, reason: '', phase: 'claiming' });
    const batch = await backend.claim(binding, inventory);
    if (!batch) {
      await persist({ phase: 'idle', checked: true, lastSuccess: now() });
      return state;
    }
    validateBatch(batch, binding);
    await checkBatchHashes(batch);
    if (state.batch && state.batch.batch_id !== batch.batch_id) fail('binding_mismatch');
    await persist({ batch: { batch_id: batch.batch_id, status: batch.status, code_hashes: batch.code_hashes }, phase: 'claimed' });
    if (batch.status !== 'claimed') {
      await pause('uncertain');
      return state;
    }
    if (cancelled()) { await pause('manual'); return state; }
    // Even a lost start response forbids re-upload: the server may have latched it.
    await persist({ batch: { batch_id: batch.batch_id, status: 'uncertain', code_hashes: batch.code_hashes }, phase: 'uploading' });
    await backend.start(binding, batch.batch_id);
    if (cancelled()) fail('uncertain');
    await merchant.upload(binding.goods_id, batch.codes);
    await backend.report?.(batch.batch_id, 'imported');
    const after = verifyInventory(await merchant.inventory(binding.goods_id));
    const confirmed = await backend.reconcile(binding, after, batch.batch_id);
    if (confirmed.blocked || confirmed.batch_resolved !== true) fail('uncertain');
    await persist({ batch: null, stock: after.total, checked: true, lastSuccess: now(), phase: 'idle', ...(cancelled() ? { enabled: false, reason: 'manual' } : {}) });
  } catch (error) {
    // Reporting failure cannot clear the locally persisted upload uncertainty.
    try {
      if (error?.code === 'login_required') await backend.authorizationFailed?.();
      if (state.batch?.status === 'uncertain') await backend.report?.(state.batch.batch_id, error?.code === 'login_required' ? 'authorization_failed' : 'uncertain');
    } catch { /* A later inventory response must confirm the same batch is resolved. */ }
    await pause(state.batch?.status === 'uncertain' ? 'uncertain' : safeReason(error));
  }
  return state;
}
