import { DEFAULT_STOCK_TARGET, PROJECT_DIRECTORY, SITE_CONFIG } from './config.js';

const form = document.querySelector('#form');
const feedback = document.querySelector('#feedback');
const submit = form.querySelector('button[type="submit"]');
const inspectAll = document.querySelector('#inspect-all');
const stockTarget = document.querySelector('#stock-target');
const pauseAll = document.querySelector('#pause-all');
stockTarget.value = DEFAULT_STOCK_TARGET;
let loadedTarget = false;
const sites = SITE_CONFIG.map(site => ({ ...site }));
document.querySelector('#project-directory').textContent = PROJECT_DIRECTORY;
let snapshot = { bindings: [], sites: [], busy: false };
let busy = false;
function element(tag, text, className) {
  const node = document.createElement(tag);
  if (text) node.textContent = text;
  if (className) node.className = className;
  return node;
}
async function request(message) {
  const response = await chrome.runtime.sendMessage(message);
  if (!response?.ok) throw new Error(response?.message || '操作失败');
  return response.data;
}
function selected(site) {
  return [...site.products.querySelectorAll('input:checked:not([data-conflict="true"])')].map(input => Number(input.value));
}
function updateControls() {
  const amounts = sites.map(site => selected(site).length);
  const count = amounts.reduce((sum, value) => sum + value, 0);
  const siteCount = amounts.filter(Boolean).length;
  document.querySelector('#selection-summary').textContent = count ? `已选 ${siteCount} 个站点 · ${count} 个额度` : '尚未选择额度';
  const locked = busy || snapshot.busy;
  for (const site of sites) {
    site.key.disabled = locked;
    site.inspect.disabled = locked;
    site.all.disabled = locked || !site.inspected;
    const choices = [...site.products.querySelectorAll('input:not([data-conflict="true"])')];
    site.all.textContent = choices.length && choices.every(input => input.checked) ? '取消全选' : '全选可用额度';
    for (const input of site.products.querySelectorAll('input')) input.disabled = locked || input.dataset.conflict === 'true';
  }
  inspectAll.disabled = locked;
  stockTarget.disabled = locked;
  submit.disabled = locked || !count || !stockTarget.checkValidity();
  submit.textContent = busy ? '正在核对并启动…' : '保存并自动补货';
  pauseAll.disabled = !snapshot.bindings.length && !locked;
  for (const button of document.querySelectorAll('#bindings button')) button.disabled = locked || button.dataset.locked === 'true';
}
function renderProducts(site) {
  site.products.replaceChildren();
  site.all.hidden = !site.inspected?.products.length;
  if (!site.inspected) {
    site.products.append(element('p', '读取后默认全选可用额度，可取消不需要的额度。', 'product-empty'));
    return;
  }
  if (!site.inspected.products.length) {
    site.products.append(element('p', '该设备没有已启用商品。请在对应站点的补货管理中检查商品与设备授权。', 'product-empty'));
    return;
  }
  for (const product of site.inspected.products) {
    const existing = snapshot.bindings.find(item => item.goods_id === product.goods_id);
    const conflict = existing && (existing.site !== site.site || existing.device_id !== site.inspected.device_id);
    const label = element('label', '', 'product-choice');
    const input = document.createElement('input');
    input.type = 'checkbox'; input.value = product.goods_id;
    input.name = `${site.id}-goods`; input.checked = !conflict;
    input.dataset.conflict = String(Boolean(conflict)); input.disabled = Boolean(conflict);
    const text = element('span');
    text.append(element('strong', product.title));
    text.append(element('small', `商品 ${product.goods_id}${existing ? conflict ? ' · 已绑定其他站点或设备' : ' · 已绑定' : ''}`));
    label.append(input, text);
    if (conflict) label.classList.add('conflict');
    input.addEventListener('change', updateControls);
    site.products.append(label);
  }
}
function buildSite(site) {
  const card = element('section', '', 'site-card'); card.dataset.site = site.site;
  const heading = element('div', '', 'title');
  const title = element('h2', site.label); title.id = `${site.id}-title`; card.setAttribute('aria-labelledby', title.id);
  heading.append(title, element('span', site.kind, `badge ${site.id}`));
  const address = element('p', site.site, 'site-address');
  const adminLink = element('a', '打开此站点的补货管理', 'admin-link');
  adminLink.href = site.adminURL; adminLink.target = '_blank'; adminLink.rel = 'noopener noreferrer';
  const credentialHelp = element('p', '在补货管理中创建 Chrome 设备，选择「全部已启用商品」，将生成的补货设备凭据粘贴到下方。每个站点只需填写一次。', 'help');
  site.credentialStatus = element('p', '', 'credential-status');
  const label = element('label', '补货设备凭据');
  site.key = document.createElement('input'); site.key.type = 'password'; site.key.name = `${site.id}-device-key`;
  site.key.autocomplete = 'new-password'; site.key.spellcheck = false;
  label.append(site.key);
  site.key.addEventListener('input', () => {
    site.inspected = null;
    site.status.textContent = '凭据已更改，请重新读取商品。';
    renderProducts(site); updateControls();
  });
  site.inspect = element('button', '读取商品', 'secondary'); site.inspect.type = 'button';
  site.inspect.addEventListener('click', () => inspectSites([site]));
  site.status = element('p', '', 'site-status'); site.status.setAttribute('role', 'status'); site.status.setAttribute('aria-live', 'polite');
  const toolbar = element('div', '', 'product-toolbar');
  site.all = element('button', '全选可用额度', 'text-button'); site.all.type = 'button'; site.all.hidden = true;
  site.all.addEventListener('click', () => {
    const inputs = [...site.products.querySelectorAll('input:not(:disabled)')];
    const check = !inputs.every(input => input.checked);
    for (const input of inputs) input.checked = check;
    updateControls();
  });
  toolbar.append(element('h3', '选择额度'), site.all);
  site.products = element('div', '', 'product-list');
  card.append(heading, address, adminLink, credentialHelp, site.credentialStatus, label, site.inspect, site.status, toolbar, site.products);
  document.querySelector('#sites').append(card);
  renderProducts(site);
}
function renderSnapshot(data) {
  snapshot = data;
  if (!loadedTarget) { stockTarget.value = data.target_stock ?? DEFAULT_STOCK_TARGET; loadedTarget = true; }
  for (const site of sites) {
    const saved = data.sites?.find(item => item.site === site.site)?.has_credential;
    site.credentialStatus.textContent = saved ? '凭据已保存 · 可直接读取商品' : '尚未保存凭据 · 首次填写一次即可';
    site.key.placeholder = saved ? '留空使用该站点已保存的凭据' : '粘贴此站点的补货设备凭据';
  }
  const list = document.querySelector('#bindings'); list.replaceChildren();
  document.querySelector('#binding-count').textContent = `${data.bindings.length} 个额度`;
  for (const site of sites) {
    const group = element('section', '', 'bound-group');
    const items = data.bindings.filter(item => item.site === site.site);
    const title = element('div', '', 'title'); title.append(element('h3', site.label), element('span', `${items.length} 个额度`, 'badge'));
    group.append(title);
    if (!items.length) group.append(element('p', '尚未绑定额度', 'help'));
    for (const item of items) {
      const card = element('article', '', 'binding-row');
      const info = element('div');
      info.append(element('h4', item.name), element('p', `商品 ${item.goods_id}`, 'help'));
      info.append(element('p', `${item.enabled ? item.stock !== null && item.target_stock && item.stock < item.target_stock ? '正在补足库存' : '自动维持中' : item.reason_text} · 库存 ${item.stock ?? '未核验'} / ${item.target_stock ?? '未设置'}`, 'help'));
      if (item.pending) info.append(element('p', '有待核实批次，请保留此绑定。', 'reason'));
      const remove = element('button', '移除', 'secondary'); remove.type = 'button';
      remove.dataset.locked = String(Boolean(item.enabled || item.pending));
      remove.setAttribute('aria-label', `移除${site.label}${item.name}绑定`);
      remove.addEventListener('click', async () => {
        busy = true; updateControls();
        try {
          renderSnapshot(await request({ type: 'remove', id: item.id }));
          for (const site of sites) renderProducts(site);
        } catch (error) { feedback.textContent = error.message; }
        finally { busy = false; updateControls(); }
      });
      card.append(info, remove); group.append(card);
    }
    list.append(group);
  }
  updateControls();
}
async function inspectSites(targets) {
  if (busy || snapshot.busy) return;
  // Request host permissions directly from the click, before asynchronous work.
  const origins = targets.map(site => site.site === 'http://127.0.0.1:8080' ? 'http://127.0.0.1/*' : `${site.site}/*`);
  const permission = chrome.permissions.request({ origins });
  const values = targets.map(site => ({ site: site.site, device_key: site.key.value.trim() }));
  busy = true; updateControls(); feedback.textContent = '';
  try {
    if (!(await permission)) throw new Error('需要允许访问所选站点，才能读取商品。');
    for (const [index, site] of targets.entries()) {
      site.status.textContent = '正在读取…';
      site.inspected = null;
      try {
        site.inspected = await request({ type: 'inspect', binding: values[index] });
        site.status.textContent = `已读取 ${site.inspected.products.length} 个额度，默认全选可绑定额度。`;
      } catch (error) { site.status.textContent = error.message; }
      renderProducts(site);
    }
  } catch (error) { feedback.textContent = error.message; }
  finally { busy = false; updateControls(); }
}
for (const site of sites) buildSite(site);
inspectAll.addEventListener('click', () => inspectSites(sites));
stockTarget.addEventListener('input', updateControls);
pauseAll.addEventListener('click', async () => {
  pauseAll.disabled = true;
  try { renderSnapshot(await request({ type: 'pause-all' })); feedback.textContent = '已请求暂停全部商品；进行中的上传会保留批次并核对结果。'; }
  catch (error) { feedback.textContent = error.message; }
  finally { updateControls(); }
});
form.addEventListener('submit', async event => {
  event.preventDefault();
  if (busy || snapshot.busy) return;
  const selections = sites.filter(site => site.inspected && selected(site).length).map(site => ({
    site: site.site, device_key: site.key.value.trim(), device_id: site.inspected.device_id, goods_ids: selected(site),
  }));
  if (!selections.length || !stockTarget.checkValidity()) return;
  busy = true; updateControls(); feedback.textContent = '';
  try {
    const data = await request({ type: 'maintain', sites: selections, target_stock: Number(stockTarget.value) });
    for (const site of sites) if (selections.some(selection => selection.site === site.site)) site.key.value = '';
    renderSnapshot(data);
    for (const site of sites) renderProducts(site);
    const { requested, running, issues } = data.maintenance;
    feedback.textContent = `${requested} 个商品已配对，${running} 个已开启自动补货，目标每种额度 ${stockTarget.value} 张。${running < requested ? '其余商品请查看下方暂停原因。' : ''}${issues.length ? ' ' + issues.map(issue => `${sites.find(site => site.site === issue.site)?.label}：${issue.message}`).join('；') : ''}`;
  } catch (error) { feedback.textContent = `操作未完成：${error.message}。请查看下方已保存状态和暂停原因。`; }
  finally { busy = false; updateControls(); }
});
chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== 'local' || !changes.uiRevision || busy) return;
  request({ type: 'snapshot' }).then(renderSnapshot).catch(error => { feedback.textContent = error.message; });
});
try { renderSnapshot(await request({ type: 'snapshot' })); }
catch (error) { feedback.textContent = error.message; snapshot.busy = true; updateControls(); }
