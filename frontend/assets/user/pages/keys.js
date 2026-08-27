// API Keys 页
import { whenReady, MODELS } from '../state.js';
import { api, esc } from '../api.js';
import { $, safe, injectIcons } from '../ui.js';

const MD = window.MD;
let keysCache = [];
let modelsCache = [];

function renderModelChips(list) {
  if (!list || !list.length) return '<span class="tag tag--gray">全部模型</span>';
  const shown = list.slice(0, 3).map((m) => '<span class="tag-inline">' + esc(m) + '</span>').join('');
  return list.length > 3 ? shown + '<span class="tag-inline">+' + (list.length - 3) + '</span>' : shown;
}

function loadKeys() {
  const tbody = $('keys-tbody');
  tbody.innerHTML = MD.loadingRow(7);
  safe(api.get('/user/api-keys')).then((res) => {
    if (!res) { tbody.innerHTML = ''; return; }
    const items = res.data || [];
    keysCache = items;
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="padding:0;border:0;">' +
        MD.emptyState('key', '暂无 API Key', '点击右上角「新建 Key」创建第一个密钥') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map((k) => {
      const statusTag = k.status === 'active'
        ? '<span class="tag tag--success">' + MD.icon('check', 12) + '已启用</span>'
        : '<span class="tag tag--gray">已禁用</span>';
      return '<tr>' +
        '<td style="color:var(--text-1);font-weight:500;">' + esc(k.name) + '</td>' +
        '<td class="mono">' + esc(k.key_prefix) + '…</td>' +
        '<td>' + renderModelChips(k.model_access) + '</td>' +
        '<td>' + statusTag + '</td>' +
        '<td>' + (k.last_used_at ? MD.fmtDate(k.last_used_at) : '从未使用') + '</td>' +
        '<td>' + MD.fmtDate(k.created_at) + '</td>' +
        '<td><div class="t-actions">' +
        '<button class="btn-ghost btn-sm" onclick="openViewKey(' + k.id + ')">' + MD.icon('eye', 13) + '查看</button>' +
        '<button class="btn-ghost btn-sm" onclick="copyKeyFull(' + k.id + ')">' + MD.icon('copy', 13) + '复制</button>' +
        '<button class="btn-danger btn-sm" onclick="delKey(' + k.id + ')">' + MD.icon('trash', 13) + '删除</button>' +
        '</div></td></tr>';
    }).join('');
  });
}

export function openCreateKey() {
  if (modelsCache.length) { renderCreateKeyModal(); return; }
  api.get('/models').then((res) => {
    const list = (res && res.data) || [];
    modelsCache = list.map((m) => m.id);
    if (MODELS.length === 0) list.forEach((m) => MODELS.push(m.id));
    renderCreateKeyModal();
  }).catch(() => { renderCreateKeyModal(); });
}

function renderCreateKeyModal() {
  const checks = modelsCache.map((m) =>
    '<label class="model-check">' +
    '<input type="checkbox" value="' + esc(m) + '" onclick="this.parentElement.classList.toggle(\'is-on\',this.checked)">' + esc(m) + '</label>'
  ).join('');
  MD.openModal({
    title: '新建 API Key',
    body:
      '<div class="field"><label>密钥名称<span class="req">*</span></label>' +
      '<input class="input" id="mk-name" placeholder="例如：生产环境" maxlength="32"></div>' +
      '<div class="field"><label>模型权限</label>' +
      '<div class="model-checks">' + checks + '</div>' +
      '<div class="hint">不勾选任何模型时，该密钥默认可访问全部模型</div></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '创建密钥', cls: 'btn-primary', onClick: submitCreateKey }
    ]
  });
  setTimeout(() => { const n = $('mk-name'); if (n) n.focus(); }, 60);
}

function submitCreateKey() {
  const name = ($('mk-name').value || '').trim();
  if (!name) { MD.toast('请输入密钥名称', 'warn'); return; }
  const models = [];
  document.querySelectorAll('.model-check input:checked').forEach((c) => models.push(c.value));
  api.post('/user/api-keys', { name: name, model_access: models })
    .then((res) => { showCreatedKey(res.data); loadKeys(); })
    .catch((e) => MD.toast(e.message, 'error'));
}

function showCreatedKey(key) {
  try { sessionStorage.setItem('mass_api_key_' + key.id, key.full_key); } catch (e) {}
  MD.openModal({
    title: '密钥创建成功',
    body:
      '<div class="key-tip">' + MD.icon('alert', 16) +
      '<span>完整密钥仅本次会话内可再次查看与复制，关闭弹窗后请妥善保存；会话结束后无法找回，只能删除重建。</span></div>' +
      '<div class="key-box"><code>' + esc(key.full_key) + '</code>' +
      '<button class="btn-ghost btn-sm" onclick="MD.copyText(\'' + esc(key.full_key) + '\')">' + MD.icon('copy', 13) + '复制</button></div>' +
      '<div class="field" style="margin:14px 0 0;"><label>密钥名称</label>' +
      '<div style="font-size:13px;color:var(--text-2);">' + esc(key.name) + ' · 前缀 <span class="mono" style="font-family:var(--font-num);">' + esc(key.key_prefix) + '</span></div></div>',
    buttons: [{ label: '一键复制完整密钥', cls: 'btn-primary', onClick: () => { MD.copyText(key.full_key); } }]
  });
}

function getKeyFull(id) {
  try { return sessionStorage.getItem('mass_api_key_' + id) || ''; } catch (e) { return ''; }
}

export function copyKeyFull(id) {
  const full = getKeyFull(id);
  if (!full) { MD.toast('完整密钥仅创建时的会话内可复制，请删除后重建', 'warn'); return; }
  MD.copyText(full);
}

export function openViewKey(id) {
  const k = keysCache.filter((x) => x.id === id)[0] || {};
  if (!k.id) return;
  const full = getKeyFull(id);
  const fullHtml = full
    ? '<div class="key-box"><code>' + esc(full) + '</code>' +
      '<button class="btn-ghost btn-sm" onclick="MD.copyText(\'' + esc(full) + '\')">' + MD.icon('copy', 13) + '复制</button></div>'
    : '<div class="key-tip" style="color:var(--text-3);">' + MD.icon('alert', 16) +
      '<span>完整密钥仅创建时的会话内可查看。如需使用，请删除该密钥后重新创建，新密钥创建后请立即保存。</span></div>';
  MD.openModal({
    title: '密钥详情',
    body:
      '<div class="field"><label>密钥名称</label><div style="font-size:13px;color:var(--text-1);font-weight:500;">' + esc(k.name) + '</div></div>' +
      '<div class="field"><label>密钥前缀</label><div class="key-box" style="max-width:220px;"><code>' + esc(k.key_prefix) + '…</code>' +
      '<button class="btn-ghost btn-sm" onclick="MD.copyText(\'' + esc(k.key_prefix) + '\')">' + MD.icon('copy', 13) + '复制</button></div></div>' +
      '<div class="field"><label>完整密钥</label>' + fullHtml + '</div>' +
      '<div class="field"><label>模型权限</label>' +
      '<div style="font-size:13px;color:var(--text-2);">' + renderModelChips(k.model_access) + '</div></div>' +
      '<div class="field" style="margin-bottom:0;"><label>使用情况</label>' +
      '<div class="hint" style="margin:0;">创建于 ' + MD.fmtDate(k.created_at) + ' · ' + (k.last_used_at ? '最近使用 ' + MD.fmtDate(k.last_used_at) : '从未使用') + '</div></div>',
    buttons: [{ label: '完成', cls: 'btn-primary', onClick: MD.closeModal }]
  });
}

export function delKey(id) {
  const k = keysCache.filter((x) => x.id === id)[0] || {};
  MD.confirm('确定删除密钥「' + esc(k.name || '') + '」吗？<br>删除后使用该密钥的调用将立即失效，且不可恢复。', () => {
    api.del('/user/api-keys/' + id)
      .then(() => { MD.toast('密钥已删除', 'success'); loadKeys(); })
      .catch((e) => MD.toast(e.message, 'error'));
  }, { title: '删除 API Key', okLabel: '删除', danger: true, html: true });
}

whenReady().then(() => {
  loadKeys();
  injectIcons();
  const after = new URLSearchParams(location.search).get('after');
  if (after === 'new') openCreateKey();
});

window.openCreateKey = openCreateKey;
window.openViewKey = openViewKey;
window.copyKeyFull = copyKeyFull;
window.delKey = delKey;
